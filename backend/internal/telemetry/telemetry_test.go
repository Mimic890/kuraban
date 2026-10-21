package telemetry

import (
	"context"
	"math"
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "t.db")
	db, err := OpenDB(path)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	s, err := NewStore(db, path)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return s
}

const sampleScrape = `# HELP api_s3_request_counter Number of API calls
# TYPE api_s3_request_counter counter
api_s3_request_counter{api_endpoint="GetObject"} 10
api_s3_request_counter{api_endpoint="PutObject"} 4
api_s3_error_counter{api_endpoint="HeadObject",status_code="404"} 3
api_s3_request_duration_sum{api_endpoint="GetObject"} 0.5
api_s3_request_duration_bucket{api_endpoint="GetObject",le="0.1"} 5
api_s3_request_duration_bucket{api_endpoint="GetObject",le="1"} 10
api_s3_request_duration_bucket{api_endpoint="GetObject",le="+Inf"} 10
web_error_counter{method="GET",status_code="404 Not Found"} 1
block_bytes_read 2048
block_ram_buffer_free_kb 2
garage_local_disk_avail{volume="data"} 100
rpc_request_counter{from="a",rpc_endpoint="x",to="b"} 3
rpc_request_counter{from="a",rpc_endpoint="y",to="b"} 4
table_size{table_name="object"} 7
cluster_layout_node_connected{id="abc",role_capacity="1",role_gateway="0",role_zone="dc1"} 1
weird_line_without_value
bad{unterminated="x 1
nan_metric NaN
`

func TestParsePrometheus(t *testing.T) {
	got := ParsePrometheus(sampleScrape)
	if len(got) != 15 {
		t.Fatalf("parsed %d samples, want 15: %+v", len(got), got)
	}
	if got[0].Name != "api_s3_request_counter" || got[0].Labels["api_endpoint"] != "GetObject" || got[0].Value != 10 {
		t.Errorf("first sample = %+v", got[0])
	}
	esc := ParsePrometheus(`m{a="x\"y\\z",b="n\nl"} 2`)
	if len(esc) != 1 || esc[0].Labels["a"] != `x"y\z` || esc[0].Labels["b"] != "n\nl" {
		t.Errorf("escapes: %+v", esc)
	}
}

func TestDeriveSamples(t *testing.T) {
	prev := histState{}
	out := DeriveSamples(ParsePrometheus(sampleScrape), prev)
	find := func(name string, labels map[string]string) (float64, bool) {
		for _, s := range out {
			if s.Name == name && matches(s.Labels, labels) {
				return s.Value, true
			}
		}
		return 0, false
	}
	checks := []struct {
		name   string
		labels map[string]string
		want   float64
	}{
		{"s3.requests", map[string]string{"endpoint": "GetObject"}, 10},
		{"s3.errors", map[string]string{"status": "404", "class": "4xx"}, 3},
		{"web.errors", map[string]string{"status": "404", "endpoint": "GET"}, 1},
		{"block.bytes_read", nil, 2048},
		{"block.ram_buffer_free", nil, 2048},
		{"disk.avail", map[string]string{"volume": "data"}, 100},
		{"rpc.requests", nil, 7},
		{"table.items", map[string]string{"table": "object"}, 7},
		{"node.connected", map[string]string{"node": "abc", "zone": "dc1"}, 1},
	}
	for _, c := range checks {
		if v, ok := find(c.name, c.labels); !ok || v != c.want {
			t.Errorf("%s%v = %v (found %v), want %v", c.name, c.labels, v, ok, c.want)
		}
	}
	if _, ok := find("s3.latency", nil); ok {
		t.Error("first scrape must not emit percentiles")
	}
	// Second scrape: 10 more requests, all in the first bucket.
	second := ParsePrometheus(`api_s3_request_duration_bucket{api_endpoint="GetObject",le="0.1"} 15
api_s3_request_duration_bucket{api_endpoint="GetObject",le="1"} 20
api_s3_request_duration_bucket{api_endpoint="GetObject",le="+Inf"} 20`)
	out = DeriveSamples(second, prev)
	p50, ok := find("s3.latency", map[string]string{"quantile": "p50"})
	if !ok || math.Abs(p50-0.05) > 1e-9 {
		t.Errorf("p50 = %v (%v), want 0.05", p50, ok)
	}
}

func TestHistogramQuantileAndReset(t *testing.T) {
	h := histogram{0.1: 2, 1: 4, math.Inf(1): 4}
	if v, ok := h.quantile(1); !ok || v != 1 {
		t.Errorf("q1 = %v %v", v, ok)
	}
	if _, ok := (histogram{math.Inf(1): 0}).quantile(0.5); ok {
		t.Error("empty histogram must report !ok")
	}
	d := histogram{0.1: 1, math.Inf(1): 1}.delta(histogram{0.1: 5, math.Inf(1): 5})
	if d[0.1] != 1 {
		t.Errorf("reset delta = %v", d)
	}
	inf := histogram{0.1: 0, math.Inf(1): 3}
	if v, _ := inf.quantile(0.5); v != 0.1 {
		t.Errorf("overflow quantile = %v", v)
	}
}

func TestLabelsRoundTrip(t *testing.T) {
	in := map[string]string{"b": "x,y=z", "a": `back\slash`}
	enc := EncodeLabels(in)
	if enc != `a=back\\slash,b=x\,y\=z` {
		t.Errorf("encoded = %q", enc)
	}
	dec := DecodeLabels(enc)
	if dec["a"] != in["a"] || dec["b"] != in["b"] {
		t.Errorf("decoded = %v", dec)
	}
	if len(DecodeLabels("")) != 0 || EncodeLabels(nil) != "" {
		t.Error("empty labels")
	}
}

func TestStoreWriteAndRange(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	base := time.Now().Add(-10 * time.Minute).Truncate(time.Minute)
	for i := 0; i < 20; i++ {
		ts := base.Add(time.Duration(i*15) * time.Second)
		counter := float64(i * 30)
		if i >= 15 {
			counter = float64((i - 15) * 30) // restart
		}
		err := s.Write(ctx, "c1", ts, []Sample{
			{Name: "s3.requests", Kind: KindCounter, Value: counter, Labels: map[string]string{"endpoint": "Get"}},
			{Name: "s3.requests", Kind: KindCounter, Value: counter / 2, Labels: map[string]string{"endpoint": "Put"}},
			{Name: "s3.duration_sum", Kind: KindCounter, Value: counter / 10, Labels: map[string]string{"endpoint": "Get"}},
			{Name: "disk", Kind: KindGauge, Value: float64(i), Labels: map[string]string{"node": "n1"}},
			{Name: "disk", Kind: KindGauge, Value: float64(100 + i), Labels: map[string]string{"node": "n2"}},
		})
		if err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	req := RangeRequest{From: base.Unix(), To: base.Add(5 * time.Minute).Unix(), Step: 15, Queries: []Query{
		{ID: "rate", Metric: "s3.requests", Fn: FnRate, GroupBy: "endpoint"},
		{ID: "total", Metric: "s3.requests", Fn: FnRate},
		{ID: "inc", Metric: "s3.requests", Fn: FnIncrease, Match: map[string]string{"endpoint": "Get"}},
		{ID: "lat", Metric: "s3.duration_sum", Fn: FnRatio, Denominator: "s3.requests", Match: map[string]string{"endpoint": "Get"}, Scale: 1000},
		{ID: "disk", Metric: "disk", Fn: FnAvg},
		{ID: "diskmax", Metric: "disk", Fn: FnMax, Agg: AggMax},
		{ID: "diskavg", Metric: "disk", Fn: FnLast, Agg: AggAvg},
		{ID: "top", Metric: "disk", GroupBy: "node", TopK: 1},
	}}
	resp, err := s.Range(ctx, "c1", req, 15)
	if err != nil {
		t.Fatalf("Range: %v", err)
	}
	if resp.Tier != "raw" || resp.Step != 15 {
		t.Errorf("tier/step = %s/%d", resp.Tier, resp.Step)
	}
	byQuery := map[string][]SeriesOut{}
	for _, se := range resp.Series {
		byQuery[se.Query] = append(byQuery[se.Query], se)
	}
	if len(byQuery["rate"]) != 2 || byQuery["rate"][0].Name != "Get" {
		t.Fatalf("rate series = %+v", byQuery["rate"])
	}
	at := func(q string, idx int) float64 {
		v := byQuery[q][0].Values[idx]
		if v == nil {
			t.Fatalf("%s[%d] is null", q, idx)
		}
		return *v
	}
	if got := at("rate", 3); got != 2 {
		t.Errorf("Get rate = %v, want 2", got)
	}
	if got := at("total", 3); got != 3 {
		t.Errorf("total rate = %v, want 3", got)
	}
	if got := at("inc", 3); got != 30 {
		t.Errorf("increase = %v, want 30", got)
	}
	if got := at("lat", 3); math.Abs(got-100) > 1e-6 {
		t.Errorf("ratio = %v, want 100", got)
	}
	if got := at("disk", 3); got != 106 {
		t.Errorf("disk sum = %v, want 106", got)
	}
	if got := at("diskmax", 3); got != 103 {
		t.Errorf("disk max = %v, want 103", got)
	}
	if got := at("diskavg", 3); got != 53 {
		t.Errorf("disk avg = %v, want 53", got)
	}
	if len(byQuery["top"]) != 1 || byQuery["top"][0].Name != "n2" {
		t.Errorf("topk = %+v", byQuery["top"])
	}
	// Reset at i=15 must not produce a negative rate.
	for _, v := range byQuery["rate"][0].Values {
		if v != nil && *v < 0 {
			t.Fatalf("negative rate after reset: %v", *v)
		}
	}

	// Rollup tiers kick in for coarse steps.
	resp, err = s.Range(ctx, "c1", RangeRequest{From: base.Add(-time.Hour).Unix(), To: time.Now().Unix(), Step: 300, Queries: []Query{{ID: "d", Metric: "disk"}}}, 15)
	if err != nil || resp.Tier != "5m" {
		t.Fatalf("5m tier: %v %+v", err, resp)
	}
	resp, err = s.Range(ctx, "c1", RangeRequest{From: base.Add(-48 * time.Hour).Unix(), To: time.Now().Unix(), Step: 3600, Queries: []Query{{ID: "d", Metric: "disk"}}}, 15)
	if err != nil || resp.Tier != "1h" || len(resp.Series) != 1 {
		t.Fatalf("1h tier: %v %+v", err, resp)
	}

	stats, err := s.Stats(ctx, "c1")
	if err != nil || stats.Series != 5 || stats.RawSamples != 100 || stats.OldestTs == nil {
		t.Errorf("stats = %+v, %v", stats, err)
	}
	list, _ := s.ListSeries(ctx, "c1", "disk")
	if len(list) != 2 || list[0].Labels["node"] != "n1" {
		t.Errorf("list = %+v", list)
	}
}

func TestRangeValidation(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().Unix()
	bad := []RangeRequest{
		{From: 0, To: now, Queries: []Query{{ID: "a", Metric: "m"}}},
		{From: now - 60, To: now},
		{From: now - 60, To: now, Queries: []Query{{ID: "a", Metric: "m", Fn: "nope"}}},
		{From: now - 60, To: now, Queries: []Query{{ID: "a", Metric: "m", Fn: FnRatio}}},
		{From: now - 86400, To: now, Step: 1, Queries: []Query{{ID: "a", Metric: "m"}}},
	}
	for i, r := range bad {
		if _, err := s.Range(ctx, "c", r, 15); err == nil {
			t.Errorf("case %d: expected error", i)
		}
	}
	resp, err := s.Range(ctx, "c", RangeRequest{From: now - 60, Queries: []Query{{ID: "a", Metric: "m"}}}, 0)
	if err != nil || len(resp.Series) != 0 {
		t.Errorf("empty query: %v %+v", err, resp)
	}
}

func TestPruneAndPurge(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	old := time.Now().Add(-100 * time.Hour)
	for _, c := range []string{"a", "b"} {
		if err := s.Write(ctx, c, old, []Sample{{Name: "g", Value: 1}}); err != nil {
			t.Fatal(err)
		}
		if err := s.Write(ctx, c, time.Now(), []Sample{{Name: "g", Value: 2}}); err != nil {
			t.Fatal(err)
		}
	}
	n, err := s.Prune(ctx, 24*time.Hour)
	if err != nil || n != 6 {
		t.Errorf("prune removed %d rows (%v), want 6", n, err)
	}
	if err := s.Purge(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	if st, _ := s.Stats(ctx, "a"); st.Series != 0 {
		t.Errorf("cluster a not purged: %+v", st)
	}
	if st, _ := s.Stats(ctx, "b"); st.Series != 1 {
		t.Errorf("cluster b affected: %+v", st)
	}
	if err := s.Purge(ctx, ""); err != nil {
		t.Fatal(err)
	}
	if err := s.Vacuum(ctx); err != nil {
		t.Fatal(err)
	}
	if st, _ := s.Stats(ctx, ""); st.Series != 0 || st.SizeBytes == 0 {
		t.Errorf("after purge all: %+v", st)
	}
	if err := s.Write(ctx, "a", time.Now(), nil); err != nil {
		t.Errorf("empty write: %v", err)
	}
}

func TestPickStep(t *testing.T) {
	if got := PickStep(0, 3600, 15); got != 15 {
		t.Errorf("1h step = %d", got)
	}
	if got := PickStep(0, 7*86400, 15); got != 1800 {
		t.Errorf("7d step = %d", got)
	}
	if got := PickStep(0, 3650*86400, 15); got%86400 != 0 {
		t.Errorf("10y step = %d", got)
	}
}

func TestForwardFill(t *testing.T) {
	one := 1.0
	vals := []*float64{nil, &one, nil, nil, nil}
	forwardFill(vals, 2)
	if vals[0] != nil || vals[2] == nil || *vals[3] != 1 || vals[4] != nil {
		t.Errorf("forwardFill = %v", vals)
	}
}

func TestSpreadDelta(t *testing.T) {
	cells := make([]cell, 4)
	// 30 over 15s starting mid-bucket: 10s step buckets [0,10) [10,20) [20,30).
	spreadDelta(cells, 0, 10, 5, 20, 30, false)
	if cells[0].num != 10 || cells[1].num != 20 || cells[2].n != 0 {
		t.Fatalf("spread = %+v", cells)
	}
	if r := cells[1].num / cells[1].dt; r != 2 {
		t.Errorf("rate = %v", r)
	}
	gap := make([]cell, 100)
	spreadDelta(gap, 0, 10, 0, 900, 900, false)
	if gap[0].n != 0 || gap[89].n != 1 || gap[89].num/gap[89].dt != 1 {
		t.Errorf("gap spread = %+v", gap[89])
	}
	den := make([]cell, 2)
	spreadDelta(den, 0, 10, 0, 10, 4, true)
	if den[0].den != 4 || den[0].num != 0 {
		t.Errorf("den = %+v", den[0])
	}
}

// A long range served from the 1h tier while history is shorter than an hour
// holds a single bucket per series; the rate must still come from inside it.
func TestRangeCounterSingleRollupBucket(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now()
	base := now.Truncate(time.Hour).Add(time.Minute)
	if base.After(now.Add(-6 * time.Minute)) {
		base = base.Add(-time.Hour)
	}
	for i := 0; i < 20; i++ {
		ts := base.Add(time.Duration(i*15) * time.Second)
		if err := s.Write(ctx, "c1", ts, []Sample{{Name: "s3.requests", Kind: KindCounter, Value: float64(i * 30)}}); err != nil {
			t.Fatal(err)
		}
	}
	resp, err := s.Range(ctx, "c1", RangeRequest{From: now.Add(-90 * 24 * time.Hour).Unix(), To: now.Unix(), Queries: []Query{
		{ID: "inc", Metric: "s3.requests", Fn: FnIncrease},
	}}, 15)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Tier != "1h" || len(resp.Series) != 1 {
		t.Fatalf("tier=%s series=%+v", resp.Tier, resp.Series)
	}
	var total float64
	for _, v := range resp.Series[0].Values {
		if v != nil {
			total += *v
		}
	}
	if math.Abs(total-19*30) > 1e-6 {
		t.Errorf("total increase = %v, want %v", total, 19*30)
	}
}
