// Package criterion ingests a criterion-rs output tree into Cubit's normalized
// [model.Measurement]s.
//
// criterion writes one directory per benchmark under target/criterion, each
// containing a new/estimates.json (the timing statistics of the most recent
// run) and a benchmark.json (the benchmark's identity + declared throughput).
// We read estimates.json for the numbers and benchmark.json for a stable ID,
// falling back to the directory path when the metadata file is absent. We take
// the *median* rather than the mean: it is the statistic least perturbed by the
// occasional slow sample on a noisy CI runner.
package criterion

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"wardnet/cubit/internal/model"
)

// estimates mirrors the subset of criterion's new/estimates.json we consume.
// All values are in nanoseconds.
type estimates struct {
	Median struct {
		PointEstimate      float64 `json:"point_estimate"`
		ConfidenceInterval struct {
			LowerBound float64 `json:"lower_bound"`
			UpperBound float64 `json:"upper_bound"`
		} `json:"confidence_interval"`
	} `json:"median"`
}

// benchmarkMeta mirrors the subset of criterion's benchmark.json we consume.
type benchmarkMeta struct {
	GroupID    string `json:"group_id"`
	FunctionID string `json:"function_id"`
	ValueStr   string `json:"value_str"`
	Throughput []struct {
		PerIteration uint64 `json:"per_iteration"`
	} `json:"throughput"`
}

// id builds a stable, human-readable benchmark id from the criterion metadata:
// "group/function/value", skipping the segments criterion left empty.
func (m benchmarkMeta) id() string {
	parts := make([]string, 0, 3)
	for _, p := range []string{m.GroupID, m.FunctionID, m.ValueStr} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, "/")
}

func (m benchmarkMeta) throughput() float64 {
	if len(m.Throughput) == 0 || m.Throughput[0].PerIteration == 0 {
		return 0
	}
	return float64(m.Throughput[0].PerIteration)
}

// Ingest walks criterionDir (typically <target>/criterion), returning one
// [model.Measurement] per benchmark, sorted by ID. It considers only the
// freshly-measured run — the .../<id>/new/estimates.json files — ignoring
// criterion's retained "base" snapshots.
func Ingest(criterionDir string) ([]model.Measurement, error) {
	info, err := os.Stat(criterionDir)
	if err != nil {
		return nil, fmt.Errorf("criterion directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("criterion directory %q is not a directory", criterionDir)
	}

	var out []model.Measurement
	walkErr := filepath.WalkDir(criterionDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() != "estimates.json" {
			return nil
		}
		// Only the current run: .../<id>/new/estimates.json.
		if filepath.Base(filepath.Dir(path)) != "new" {
			return nil
		}
		benchDir := filepath.Dir(filepath.Dir(path))

		est, err := readEstimates(path)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}

		id := deriveID(criterionDir, benchDir)
		var thr float64
		if meta, err := readMeta(filepath.Join(benchDir, "benchmark.json")); err == nil {
			if mid := meta.id(); mid != "" {
				id = mid
			}
			thr = meta.throughput()
		}

		out = append(out, model.Measurement{
			ID:         id,
			Unit:       "ns",
			Value:      est.Median.PointEstimate,
			Lower:      est.Median.ConfidenceInterval.LowerBound,
			Upper:      est.Median.ConfidenceInterval.UpperBound,
			Throughput: thr,
		})
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no benchmarks found under %q (expected */new/estimates.json)", criterionDir)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func readEstimates(path string) (estimates, error) {
	var e estimates
	data, err := os.ReadFile(path) // #nosec G304 -- path comes from our own WalkDir over criterionDir
	if err != nil {
		return e, err
	}
	if err := json.Unmarshal(data, &e); err != nil {
		return e, err
	}
	return e, nil
}

func readMeta(path string) (benchmarkMeta, error) {
	var m benchmarkMeta
	data, err := os.ReadFile(path) // #nosec G304 -- path is benchmark.json beside an ingested estimates.json
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return m, err
	}
	return m, nil
}

// deriveID falls back to the benchmark's path relative to the criterion root
// when benchmark.json is missing, so ingestion still yields a usable ID.
func deriveID(root, benchDir string) string {
	rel, err := filepath.Rel(root, benchDir)
	if err != nil {
		return filepath.Base(benchDir)
	}
	return filepath.ToSlash(rel)
}
