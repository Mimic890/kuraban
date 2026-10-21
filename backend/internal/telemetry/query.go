package telemetry

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"
)

// Query functions.
const (
	FnAvg      = "avg"      // gauge: mean per step
	FnMax      = "max"      // gauge: max per step
	FnMin      = "min"      // gauge: min per step
	FnLast     = "last"     // gauge: last value per step
	FnRate     = "rate"     // counter: per-second rate
	FnIncrease = "increase" // counter: increase per step
	FnRatio    = "ratio"    // counter/counter: increase(metric) / increase(denominator)
)

// Group aggregations across series.
const (
	AggSum = "sum"
	AggAvg = "avg"
	AggMax = "max"
	AggMin = "min"
)

// Query selects and transforms one metric.
type Query struct {
	ID          string            `json:"id"`
	Metric      string            `json:"metric"`
	Fn          string            `json:"fn"`
	Denominator string            `json:"denominator,omitempty"`
	Match       map[string]string `json:"match,omitempty"`
	GroupBy     string            `json:"group_by,omitempty"`
	Agg         string            `json:"agg,omitempty"`
	Scale       float64           `json:"scale,omitempty"`
	TopK        int               `json:"top_k,omitempty"`
}

// RangeRequest asks for several queries over one time window.
type RangeRequest struct {
	From    int64   `json:"from"`
	To      int64   `json:"to"`
	Step    int64   `json:"step,omitempty"`
	Queries []Query `json:"queries"`
}

// SeriesOut is one resulting line.
type SeriesOut struct {
	Query  string     `json:"query"`
	Name   string     `json:"name"`
	Values []*float64 `json:"values"`
}

// RangeResponse carries aligned timestamps shared by every series.
type RangeResponse struct {
	From       int64       `json:"from"`
	To         int64       `json:"to"`
	Step       int64       `json:"step"`
	Tier       string      `json:"tier"`
	Timestamps []int64     `json:"timestamps"`
	Series     []SeriesOut `json:"series"`
}

const maxPoints = 480

// maxQueries bounds one request; the dashboard sends every visible panel's
// queries together (about 60 today).
const maxQueries = 256

// staleness is how long a gauge value stays valid without a new sample, like
// Prometheus' lookback delta. It bridges series scraped less often than the
// query step (bucket statistics) without hiding real outages.
const staleness = 300

// forwardFill copies each value into following empty steps, at most limit.
func forwardFill(values []*float64, limit int) {
	var last *float64
	gap := 0
	for i, v := range values {
		if v != nil {
			last, gap = v, 0
			continue
		}
		gap++
		if last != nil && gap <= limit {
			c := *last
			values[i] = &c
		}
	}
}

var niceSteps = []int64{5, 10, 15, 30, 60, 120, 300, 600, 900, 1800, 3600, 7200, 10800, 21600, 43200, 86400}

// PickStep returns a step that keeps at most maxPoints points per series and
// is never finer than the source resolution.
func PickStep(from, to, minStep int64) int64 {
	span := to - from
	want := int64(math.Ceil(float64(span) / maxPoints))
	if want < minStep {
		want = minStep
	}
	for _, s := range niceSteps {
		if s >= want {
			return s
		}
	}
	return (want/86400 + 1) * 86400
}

// PickTier chooses the cheapest tier that still has data for the window.
func PickTier(from int64, step int64, now time.Time) Tier {
	age := now.Sub(time.Unix(from, 0))
	switch {
	case step < Tier5m.Resolution && age <= TierRaw.MaxKeep:
		return TierRaw
	case step < Tier1h.Resolution && age <= Tier5m.MaxKeep:
		return Tier5m
	default:
		return Tier1h
	}
}

// Range evaluates req for cluster. scrapeInterval is the raw resolution.
func (s *Store) Range(ctx context.Context, cluster string, req RangeRequest, scrapeInterval int64) (*RangeResponse, error) {
	now := time.Now()
	if req.To <= 0 {
		req.To = now.Unix()
	}
	if req.From <= 0 || req.From >= req.To {
		return nil, errors.New("from must be positive and before to")
	}
	if len(req.Queries) == 0 || len(req.Queries) > maxQueries {
		return nil, errors.New(fmt.Sprintf("between 1 and %d queries are required", maxQueries))
	}
	if scrapeInterval <= 0 {
		scrapeInterval = 15
	}
	step := req.Step
	if step <= 0 {
		step = PickStep(req.From, req.To, scrapeInterval)
	} else if (req.To-req.From)/step > maxPoints*4 {
		return nil, fmt.Errorf("step too small: at most %d points", maxPoints*4)
	}
	tier := PickTier(req.From, step, now)
	if tier.Resolution > step {
		step = PickStep(req.From, req.To, tier.Resolution)
	}
	start := req.From - req.From%step
	n := int((req.To-start)/step) + 1
	resp := &RangeResponse{From: start, To: req.To, Step: step, Tier: tier.Name, Timestamps: make([]int64, n), Series: []SeriesOut{}}
	for i := range resp.Timestamps {
		resp.Timestamps[i] = start + int64(i)*step
	}
	for _, q := range req.Queries {
		out, err := s.evalQuery(ctx, cluster, q, tier, start, req.To, step, n)
		if err != nil {
			return nil, fmt.Errorf("query %q: %w", q.ID, err)
		}
		resp.Series = append(resp.Series, out...)
	}
	return resp, nil
}

// cell accumulates one step bucket of one group.
type cell struct {
	num, den, dt float64
	n            int
	min, max     float64
	last         float64
	lastTs       int64
}

func (c *cell) addGauge(p point) {
	if c.n == 0 {
		c.min, c.max = p.min, p.max
	} else {
		c.min = math.Min(c.min, p.min)
		c.max = math.Max(c.max, p.max)
	}
	c.num += p.avg
	c.n++
	if p.ts >= c.lastTs {
		c.last, c.lastTs = p.last, p.ts
	}
}

func (s *Store) evalQuery(ctx context.Context, cluster string, q Query, tier Tier, start, to, step int64, n int) ([]SeriesOut, error) {
	if q.Fn == "" {
		q.Fn = FnAvg
	}
	if q.Agg == "" {
		q.Agg = AggSum
	}
	if q.Scale == 0 {
		q.Scale = 1
	}
	switch q.Fn {
	case FnAvg, FnMax, FnMin, FnLast, FnRate, FnIncrease:
	case FnRatio:
		if q.Denominator == "" {
			return nil, errors.New("ratio requires a denominator")
		}
	default:
		return nil, fmt.Errorf("unknown fn %q", q.Fn)
	}
	counter := q.Fn == FnRate || q.Fn == FnIncrease || q.Fn == FnRatio
	// Counters need the point before the window to compute the first delta.
	lookback := int64(0)
	if counter {
		lookback = step + maxInt64(tier.Resolution, 60)*2
	}

	groups := map[string][]cell{}
	load := func(metric string, isDen bool) error {
		list, err := s.ListSeries(ctx, cluster, metric)
		if err != nil {
			return err
		}
		for _, si := range list {
			if !matches(si.Labels, q.Match) {
				continue
			}
			key := ""
			if q.GroupBy != "" {
				key = si.Labels[q.GroupBy]
			}
			pts, err := s.points(ctx, tier, si.ID, start-lookback, to)
			if err != nil {
				return err
			}
			cells := make([]cell, n)
			if counter {
				// A rollup bucket also knows how far the counter moved inside
				// it (last - min). Use that for the first bucket, which has no
				// predecessor; otherwise a window holding a single bucket
				// (e.g. 90 days of 1h data collected for less than an hour)
				// would show no rate at all.
				if tier.Resolution > 0 && len(pts) > 0 {
					if f := pts[0]; f.ts > start && f.last > f.min {
						spreadDelta(cells, start, step, maxInt64(f.start, start), f.ts, f.last-f.min, isDen)
					}
				}
				for i := 1; i < len(pts); i++ {
					prev, cur := pts[i-1], pts[i]
					if cur.ts <= start || cur.ts <= prev.ts {
						continue
					}
					d := cur.last - prev.last
					if d < 0 {
						d = cur.last
					}
					spreadDelta(cells, start, step, prev.ts, cur.ts, d, isDen)
				}
			} else {
				for _, p := range pts {
					idx := int((p.ts - start) / step)
					if idx < 0 || idx >= n {
						continue
					}
					cells[idx].addGauge(p)
				}
			}
			groups[key] = mergeCells(groups[key], cells, q, counter)
		}
		return nil
	}
	if err := load(q.Metric, false); err != nil {
		return nil, err
	}
	if q.Fn == FnRatio {
		if err := load(q.Denominator, true); err != nil {
			return nil, err
		}
	}

	type ranked struct {
		out  SeriesOut
		mean float64
	}
	var results []ranked
	for key, cells := range groups {
		values := make([]*float64, n)
		var sum float64
		var cnt int
		for i, c := range cells {
			v, ok := finalize(c, q, step)
			if !ok {
				continue
			}
			v *= q.Scale
			values[i] = &v
			sum += v
			cnt++
		}
		if cnt == 0 {
			continue
		}
		if !counter {
			forwardFill(values, int(staleness/step))
		}
		name := key
		if name == "" {
			name = q.ID
		}
		results = append(results, ranked{out: SeriesOut{Query: q.ID, Name: name, Values: values}, mean: sum / float64(cnt)})
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].mean != results[j].mean {
			return results[i].mean > results[j].mean
		}
		return results[i].out.Name < results[j].out.Name
	})
	if q.TopK > 0 && len(results) > q.TopK {
		results = results[:q.TopK]
	}
	out := make([]SeriesOut, len(results))
	for i, r := range results {
		out[i] = r.out
	}
	return out, nil
}

// spreadDelta distributes a counter increase over every step bucket the
// interval (t0, t1] overlaps, proportionally to the overlap. Rates then stay
// continuous whatever the relation between scrape interval and step. Gaps
// longer than staleness (collector or Garage down) are attributed to the
// closing bucket only, so outages are not painted over.
func spreadDelta(cells []cell, start, step, t0, t1 int64, d float64, isDen bool) {
	n := int64(len(cells))
	// The increase always spreads at its true average rate; a long gap only
	// limits which buckets receive it.
	span := float64(t1 - t0)
	if t1-t0 > staleness {
		t0 = maxInt64(t0, t1-step)
	}
	for b := (t0 - start) / step; b < n; b++ {
		if b < 0 {
			continue
		}
		bs, be := start+b*step, start+(b+1)*step
		if bs >= t1 {
			break
		}
		lo, hi := maxInt64(bs, t0), minInt64(be, t1)
		if hi <= lo {
			continue
		}
		frac := float64(hi-lo) / span
		c := &cells[b]
		if isDen {
			c.den += d * frac
		} else {
			c.num += d * frac
			c.dt += float64(hi - lo)
		}
		c.n++
	}
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// mergeCells folds one series' cells into its group.
func mergeCells(acc, cells []cell, q Query, counter bool) []cell {
	if acc == nil {
		acc = make([]cell, len(cells))
		for i := range cells {
			if !counter && cells[i].n > 0 {
				acc[i] = collapse(cells[i], q.Fn)
			} else {
				acc[i] = cells[i]
			}
		}
		return acc
	}
	for i, c := range cells {
		if c.n == 0 {
			continue
		}
		a := &acc[i]
		if counter {
			a.num += c.num
			a.den += c.den
			// Rates of parallel series add up; keep the widest covered span.
			a.dt = math.Max(a.dt, c.dt)
			a.n += c.n
			continue
		}
		v := collapse(c, q.Fn).num
		if a.n == 0 {
			*a = cell{num: v, n: 1, min: v, max: v}
			continue
		}
		switch q.Agg {
		case AggMax:
			a.num = math.Max(a.num, v)
		case AggMin:
			a.num = math.Min(a.num, v)
		default:
			a.num += v
		}
		a.min = math.Min(a.min, v)
		a.max = math.Max(a.max, v)
		a.n++
	}
	return acc
}

// collapse reduces a gauge cell to one value stored in num with n=1 so group
// aggregation works on per-series values rather than raw points.
func collapse(c cell, fn string) cell {
	var v float64
	switch fn {
	case FnMax:
		v = c.max
	case FnMin:
		v = c.min
	case FnLast:
		v = c.last
	default:
		v = c.num / float64(c.n)
	}
	return cell{num: v, n: 1, min: v, max: v, last: v, lastTs: c.lastTs, den: 1}
}

func finalize(c cell, q Query, step int64) (float64, bool) {
	if c.n == 0 {
		return 0, false
	}
	switch q.Fn {
	case FnRate:
		if c.dt <= 0 {
			return 0, false
		}
		return c.num / c.dt, true
	case FnIncrease:
		// The observed increase, not rate×step: a step only partly covered
		// by history must not be extrapolated to its full width.
		return c.num, true
	case FnRatio:
		if c.den <= 0 {
			return 0, false
		}
		return c.num / c.den, true
	default:
		if q.Agg == AggAvg && c.n > 0 {
			return c.num / float64(c.n), true
		}
		return c.num, true
	}
}

func matches(labels, match map[string]string) bool {
	for k, v := range match {
		if labels[k] != v {
			return false
		}
	}
	return true
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
