package telemetry

import (
	"bufio"
	"math"
	"sort"
	"strconv"
	"strings"
)

// PromSample is one line of the Prometheus text exposition format.
type PromSample struct {
	Name   string
	Labels map[string]string
	Value  float64
}

// ParsePrometheus parses the text exposition format. Comments, malformed
// lines and non-finite values are skipped rather than failing the scrape.
func ParsePrometheus(text string) []PromSample {
	var out []PromSample
	sc := bufio.NewScanner(strings.NewReader(text))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || line[0] == '#' {
			continue
		}
		if s, ok := parsePromLine(line); ok {
			out = append(out, s)
		}
	}
	return out
}

func parsePromLine(line string) (PromSample, bool) {
	var s PromSample
	rest := line
	if i := strings.IndexAny(line, "{ \t"); i < 0 {
		return s, false
	} else {
		s.Name = line[:i]
		rest = line[i:]
	}
	if strings.HasPrefix(rest, "{") {
		labels, after, ok := parseLabels(rest[1:])
		if !ok {
			return s, false
		}
		s.Labels = labels
		rest = after
	}
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return s, false
	}
	v, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		switch fields[0] {
		case "+Inf":
			v = math.Inf(1)
		case "-Inf":
			v = math.Inf(-1)
		default:
			return s, false
		}
	}
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return s, false
	}
	s.Value = v
	return s, s.Name != ""
}

// parseLabels reads `k="v",k2="v2"}` and returns what follows the brace.
func parseLabels(in string) (map[string]string, string, bool) {
	labels := map[string]string{}
	i := 0
	for {
		for i < len(in) && (in[i] == ' ' || in[i] == ',') {
			i++
		}
		if i >= len(in) {
			return nil, "", false
		}
		if in[i] == '}' {
			return labels, in[i+1:], true
		}
		eq := strings.IndexByte(in[i:], '=')
		if eq < 0 {
			return nil, "", false
		}
		key := strings.TrimSpace(in[i : i+eq])
		i += eq + 1
		if i >= len(in) || in[i] != '"' {
			return nil, "", false
		}
		i++
		var b strings.Builder
		for {
			if i >= len(in) {
				return nil, "", false
			}
			c := in[i]
			if c == '\\' && i+1 < len(in) {
				switch in[i+1] {
				case 'n':
					b.WriteByte('\n')
				default:
					b.WriteByte(in[i+1])
				}
				i += 2
				continue
			}
			if c == '"' {
				i++
				break
			}
			b.WriteByte(c)
			i++
		}
		labels[key] = b.String()
	}
}

// histogram is cumulative bucket counts keyed by upper bound.
type histogram map[float64]float64

func (h histogram) add(le string, v float64) {
	var bound float64
	if le == "+Inf" {
		bound = math.Inf(1)
	} else {
		b, err := strconv.ParseFloat(le, 64)
		if err != nil {
			return
		}
		bound = b
	}
	h[bound] += v
}

// delta returns h - prev per bound; ok is false when a counter went
// backwards (process restart), in which case h itself is the delta.
func (h histogram) delta(prev histogram) histogram {
	out := histogram{}
	reset := false
	for b, v := range h {
		if prev[b] > v {
			reset = true
			break
		}
	}
	for b, v := range h {
		if reset || prev == nil {
			out[b] = v
		} else {
			out[b] = v - prev[b]
		}
	}
	return out
}

// quantile estimates q (0..1) with linear interpolation inside the bucket,
// like PromQL's histogram_quantile. ok is false when there were no events.
func (h histogram) quantile(q float64) (float64, bool) {
	bounds := make([]float64, 0, len(h))
	for b := range h {
		bounds = append(bounds, b)
	}
	sort.Float64s(bounds)
	if len(bounds) == 0 {
		return 0, false
	}
	total := h[bounds[len(bounds)-1]]
	if total <= 0 {
		return 0, false
	}
	rank := q * total
	prevBound, prevCount := 0.0, 0.0
	for _, b := range bounds {
		c := h[b]
		if c >= rank {
			if math.IsInf(b, 1) {
				return prevBound, true
			}
			if c == prevCount {
				return b, true
			}
			return prevBound + (b-prevBound)*(rank-prevCount)/(c-prevCount), true
		}
		prevBound, prevCount = b, c
	}
	return prevBound, true
}
