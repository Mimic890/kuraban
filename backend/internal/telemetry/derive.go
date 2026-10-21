package telemetry

import (
	"strings"
)

// apiFamilies maps Garage metric prefixes to our series namespace.
var apiFamilies = []struct{ prefix, name string }{
	{"api_s3_", "s3"},
	{"api_k2v_", "k2v"},
	{"api_admin_", "admin"},
	{"web_", "web"},
}

// histState remembers cumulative histograms between scrapes so percentiles
// reflect only the interval since the previous scrape.
type histState map[string]histogram

// DeriveSamples converts a Prometheus scrape into the curated series set.
// prev holds cumulative histograms from the previous scrape and is updated.
func DeriveSamples(prom []PromSample, prev histState) []Sample {
	var out []Sample
	sums := map[string]float64{}
	sumKinds := map[string]Kind{}
	addSum := func(name string, kind Kind, v float64) {
		sums[name] += v
		sumKinds[name] = kind
	}
	hists := map[string]histogram{}
	addHist := func(name, le string, v float64) {
		h := hists[name]
		if h == nil {
			h = histogram{}
			hists[name] = h
		}
		h.add(le, v)
	}
	emit := func(name string, kind Kind, v float64, labels map[string]string) {
		out = append(out, Sample{Name: name, Kind: kind, Value: v, Labels: labels})
	}

	for _, p := range prom {
		n := p.Name
		matchedAPI := false
		for _, fam := range apiFamilies {
			if !strings.HasPrefix(n, fam.prefix) {
				continue
			}
			rest := strings.TrimPrefix(n, fam.prefix)
			endpoint := p.Labels["api_endpoint"]
			if endpoint == "" {
				endpoint = p.Labels["method"]
			}
			switch rest {
			case "request_counter":
				emit(fam.name+".requests", KindCounter, p.Value, map[string]string{"endpoint": endpoint})
				matchedAPI = true
			case "error_counter":
				status := p.Labels["status_code"]
				if i := strings.IndexByte(status, ' '); i > 0 {
					status = status[:i]
				}
				emit(fam.name+".errors", KindCounter, p.Value, map[string]string{"endpoint": endpoint, "status": status, "class": statusClass(status)})
				matchedAPI = true
			case "request_duration_sum":
				emit(fam.name+".duration_sum", KindCounter, p.Value, map[string]string{"endpoint": endpoint})
				matchedAPI = true
			case "request_duration_bucket":
				if fam.name == "s3" || fam.name == "web" {
					addHist(fam.name, p.Labels["le"], p.Value)
				}
				matchedAPI = true
			case "request_duration_count":
				matchedAPI = true
			}
		}
		if matchedAPI {
			continue
		}
		switch {
		case n == "block_bytes_read":
			emit("block.bytes_read", KindCounter, p.Value, nil)
		case n == "block_bytes_written":
			emit("block.bytes_written", KindCounter, p.Value, nil)
		case n == "block_read_duration_sum":
			emit("block.read_seconds", KindCounter, p.Value, nil)
		case n == "block_read_duration_count":
			emit("block.reads", KindCounter, p.Value, nil)
		case n == "block_write_duration_sum":
			emit("block.write_seconds", KindCounter, p.Value, nil)
		case n == "block_write_duration_count":
			emit("block.writes", KindCounter, p.Value, nil)
		case n == "block_delete_counter":
			emit("block.deletes", KindCounter, p.Value, nil)
		case n == "block_resync_counter":
			emit("block.resyncs", KindCounter, p.Value, nil)
		case n == "block_resync_error_counter":
			emit("block.resync_errors_total", KindCounter, p.Value, nil)
		case n == "block_resync_queue_length":
			emit("block.resync_queue", KindGauge, p.Value, nil)
		case n == "block_resync_errored_blocks":
			emit("block.resync_errored", KindGauge, p.Value, nil)
		case n == "block_rc_size":
			emit("block.rc_size", KindGauge, p.Value, nil)
		case n == "block_ram_buffer_free_kb":
			emit("block.ram_buffer_free", KindGauge, p.Value*1024, nil)
		case n == "garage_local_disk_avail":
			emit("disk.avail", KindGauge, p.Value, map[string]string{"volume": p.Labels["volume"]})
		case n == "garage_local_disk_total":
			emit("disk.total", KindGauge, p.Value, map[string]string{"volume": p.Labels["volume"]})
		case n == "cluster_layout_node_connected":
			emit("node.connected", KindGauge, p.Value, map[string]string{"node": p.Labels["id"], "zone": p.Labels["role_zone"]})
		case n == "cluster_layout_node_disconnected_time":
			emit("node.disconnected_seconds", KindGauge, p.Value, map[string]string{"node": p.Labels["id"], "zone": p.Labels["role_zone"]})
		case n == "rpc_request_counter":
			addSum("rpc.requests", KindCounter, p.Value)
		case n == "rpc_duration_sum":
			addSum("rpc.seconds", KindCounter, p.Value)
		case n == "rpc_duration_bucket":
			addHist("rpc", p.Labels["le"], p.Value)
		case n == "rpc_netapp_error_counter" || n == "rpc_timeout_counter" || n == "rpc_error_counter":
			addSum("rpc.errors", KindCounter, p.Value)
		case n == "table_get_request_counter":
			addSum("table.gets", KindCounter, p.Value)
		case n == "table_put_request_counter":
			addSum("table.puts", KindCounter, p.Value)
		case n == "table_internal_update_counter":
			addSum("table.updates", KindCounter, p.Value)
		case n == "table_gc_todo_queue_length":
			addSum("table.gc_queue", KindGauge, p.Value)
		case n == "table_merkle_updater_todo_queue_length":
			addSum("table.merkle_queue", KindGauge, p.Value)
		case n == "table_insert_queue_length":
			addSum("table.insert_queue", KindGauge, p.Value)
		case n == "table_size":
			emit("table.items", KindGauge, p.Value, map[string]string{"table": p.Labels["table_name"]})
		case n == "table_merkle_tree_size":
			addSum("table.merkle_size", KindGauge, p.Value)
		}
	}
	for name, v := range sums {
		emit(name, sumKinds[name], v, nil)
	}
	for fam, h := range hists {
		before, had := prev[fam]
		prev[fam] = h
		if !had {
			continue
		}
		d := h.delta(before)
		for _, q := range []struct {
			q    float64
			name string
		}{{0.5, "p50"}, {0.95, "p95"}, {0.99, "p99"}} {
			if v, ok := d.quantile(q.q); ok {
				emit(fam+".latency", KindGauge, v, map[string]string{"quantile": q.name})
			}
		}
	}
	return out
}

func statusClass(status string) string {
	if status == "" {
		return "other"
	}
	switch status[0] {
	case '4':
		return "4xx"
	case '5':
		return "5xx"
	default:
		return "other"
	}
}
