// Package dashboard turns recorded runs into the trend dashboard: a
// self-contained HTML page (JS + CSS + data inlined) that `cubit serve` hosts
// locally and CI uploads as an artifact.
//
// The React/Vite bundle is built from ../../web and embedded here as a single
// index.html; Build assembles the per-benchmark series from the cubit-state
// branch and Render injects it as a window global the page reads on load.
package dashboard

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"

	"wardnet/cubit/internal/gitstore"
)

//go:embed assets/index.html
var indexHTML []byte

type point struct {
	Commit      string  `json:"commit"`
	ShortCommit string  `json:"shortCommit"`
	Timestamp   string  `json:"timestamp"`
	Value       float64 `json:"value"`
	Lower       float64 `json:"lower"`
	Upper       float64 `json:"upper"`
}

type benchSeries struct {
	ID     string  `json:"id"`
	Unit   string  `json:"unit"`
	Points []point `json:"points"`
}

// Data is the payload the page renders.
type Data struct {
	GeneratedAt string        `json:"generatedAt"`
	Branch      string        `json:"branch"`
	Benchmarks  []benchSeries `json:"benchmarks"`
}

// Build assembles the dashboard payload from the store's recorded runs,
// optionally filtered to one branch, with each benchmark's points ordered
// oldest-first (the manifest order).
func Build(store *gitstore.Store, branch string, now time.Time) (Data, error) {
	runs, err := store.History(branch)
	if err != nil {
		return Data{}, err
	}
	byID := map[string]*benchSeries{}
	var ids []string
	for _, run := range runs {
		for _, ms := range run.Measurements {
			s, exists := byID[ms.ID]
			if !exists {
				s = &benchSeries{ID: ms.ID, Unit: ms.Unit}
				byID[ms.ID] = s
				ids = append(ids, ms.ID)
			}
			s.Points = append(s.Points, point{
				Commit:      run.Commit,
				ShortCommit: short(run.Commit),
				Timestamp:   run.Timestamp,
				Value:       ms.Value,
				Lower:       ms.Lower,
				Upper:       ms.Upper,
			})
		}
	}
	sort.Strings(ids)
	d := Data{GeneratedAt: now.UTC().Format(time.RFC3339), Branch: branch}
	for _, id := range ids {
		d.Benchmarks = append(d.Benchmarks, *byID[id])
	}
	return d, nil
}

// Render returns a standalone HTML page with data injected as a window global
// just before </head>, so it is defined before the page's deferred module
// script runs.
func Render(d Data) ([]byte, error) {
	raw, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	// Guard against a "</script>" sequence inside the JSON prematurely closing
	// the injected tag.
	safe := strings.ReplaceAll(string(raw), "</", `<\/`)
	inject := []byte("<script>window.__CUBIT_DATA__=" + safe + ";</script></head>")
	return bytes.Replace(indexHTML, []byte("</head>"), inject, 1), nil
}

// Serve builds the dashboard on every request (so it reflects the current
// branch state) and serves it at addr until the process is stopped.
func Serve(store *gitstore.Store, branch, addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		d, err := Build(store, branch, time.Now())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		html, err := Render(d)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(html)
	})

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	fmt.Printf("cubit dashboard: http://%s  (ctrl-c to stop)\n", ln.Addr())
	return http.Serve(ln, mux) // #nosec G114 -- local dev dashboard, no untrusted network exposure intended
}

func short(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}
