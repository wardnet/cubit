package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"wardnet/cubit/internal/criterion"
	"wardnet/cubit/internal/model"
)

// buildRun ingests a criterion output directory into a normalized [model.Run]
// stamped with the given identity. An empty timestamp is filled with now (UTC,
// RFC3339) so records sort chronologically.
func buildRun(criterionDir, commit, branch, timestamp string) (model.Run, error) {
	ms, err := criterion.Ingest(criterionDir)
	if err != nil {
		return model.Run{}, err
	}
	if timestamp == "" {
		timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	return model.Run{
		Schema:       model.SchemaVersion,
		Tool:         "criterion",
		Commit:       commit,
		Branch:       branch,
		Timestamp:    timestamp,
		Measurements: ms,
	}, nil
}

// loadRun reads a [model.Run] previously written by `cubit record`.
func loadRun(path string) (model.Run, error) {
	var r model.Run
	data, err := os.ReadFile(path) // #nosec G304 -- path is an explicit user-provided baseline file
	if err != nil {
		return r, fmt.Errorf("read baseline: %w", err)
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return r, fmt.Errorf("parse baseline %s: %w", path, err)
	}
	return r, nil
}

// writeJSON marshals v as indented JSON to path, or to stdout when path is
// empty or "-".
func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writeOut(path, data)
}

// writeOut writes data to path, or to stdout when path is empty or "-".
func writeOut(path string, data []byte) error {
	if path == "" || path == "-" {
		_, err := os.Stdout.Write(data)
		return err
	}
	return os.WriteFile(path, data, 0o644) // #nosec G306 -- benchmark records/reports are not secrets
}
