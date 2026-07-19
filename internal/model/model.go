// Package model defines Cubit's normalized, tool-agnostic benchmark schema.
//
// Every benchmark runner (criterion today, others later) is ingested into a
// [Run]: a set of [Measurement]s captured at one commit. Runs are what get
// persisted to the cubit-state branch and what the comparison + report layers
// operate on, so nothing downstream ever needs to know which runner produced
// the numbers.
package model

import "sort"

// SchemaVersion is bumped whenever the on-disk [Run] shape changes
// incompatibly, so the ingest/read layers can refuse or migrate old records.
const SchemaVersion = 1

// Measurement is a single benchmark's summary for one run. Value/Lower/Upper
// are expressed in Unit (nanoseconds for criterion latency benches); Lower and
// Upper are the confidence-interval bounds the runner reported.
type Measurement struct {
	ID         string  `json:"id"`
	Unit       string  `json:"unit"`
	Value      float64 `json:"value"`
	Lower      float64 `json:"lower"`
	Upper      float64 `json:"upper"`
	Throughput float64 `json:"throughput,omitempty"` // elements/sec, when the bench declares it
}

// Run is a full benchmark run captured at one commit. Timestamp is set by the
// caller (RFC3339) rather than at ingest, so replays and back-fills stay
// deterministic.
type Run struct {
	Schema       int           `json:"schema"`
	Tool         string        `json:"tool"`
	Commit       string        `json:"commit"`
	Branch       string        `json:"branch"`
	Timestamp    string        `json:"timestamp"`
	Measurements []Measurement `json:"measurements"`
}

// Index returns the run's measurements keyed by benchmark ID.
func (r Run) Index() map[string]Measurement {
	m := make(map[string]Measurement, len(r.Measurements))
	for _, x := range r.Measurements {
		m[x.ID] = x
	}
	return m
}

// Delta is a per-benchmark comparison of a current run against a baseline.
// PctChange is signed: positive means the current run is slower (a regression
// for latency units).
type Delta struct {
	ID        string
	Unit      string
	Baseline  float64
	Current   float64
	PctChange float64
	HasBase   bool // false when the baseline run had no matching benchmark (new bench)
	Regressed bool // PctChange exceeded the report threshold
}

// Compare diffs current against baseline, flagging any benchmark whose slowdown
// exceeds thresholdPct (a positive percentage). A benchmark absent from the
// baseline is reported with HasBase=false and never counts as a regression —
// it is new, not slower. The gate is advisory: Regressed is a hint for the
// human reviewer, not a build failure.
func Compare(current, baseline Run, thresholdPct float64) []Delta {
	base := baseline.Index()
	out := make([]Delta, 0, len(current.Measurements))
	for _, c := range current.Measurements {
		d := Delta{ID: c.ID, Unit: c.Unit, Current: c.Value}
		if b, ok := base[c.ID]; ok {
			d.HasBase = true
			d.Baseline = b.Value
			if b.Value > 0 {
				d.PctChange = (c.Value - b.Value) / b.Value * 100
			}
			d.Regressed = d.PctChange > thresholdPct
		}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
