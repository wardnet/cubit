// Package report renders a comparison between a current run and its baseline
// into the Markdown that Cubit posts as a sticky PR comment.
//
// The comment is deliberately advisory: it summarizes what moved and flags
// slowdowns past the threshold, but it never asserts pass/fail. The human
// reviewer, with the trend dashboard for context, makes the go/no-go call.
package report

import (
	"fmt"
	"strings"

	"wardnet/cubit/internal/model"
)

// Markdown renders the PR comment body for the given deltas. thresholdPct is
// echoed into the header so a reader knows what "⚠️" means for this run.
func Markdown(deltas []model.Delta, cur model.Run, thresholdPct float64, dashboardURL string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "## 📐 cubit — performance\n\n")
	if cur.Commit != "" {
		fmt.Fprintf(&b, "Commit `%s`", shortSHA(cur.Commit))
		if cur.Branch != "" {
			fmt.Fprintf(&b, " on `%s`", cur.Branch)
		}
		b.WriteString(" · ")
	}
	fmt.Fprintf(&b, "%d benchmarks · flagging Δ > %.0f%% slower\n\n", len(deltas), thresholdPct)

	regressed := countRegressed(deltas)
	newCount := countNew(deltas)
	switch {
	case regressed > 0:
		fmt.Fprintf(&b, "⚠️ **%d benchmark(s) slower than baseline by more than %.0f%%** — review before merge.\n\n", regressed, thresholdPct)
	default:
		b.WriteString("✅ No benchmark exceeded the regression threshold.\n\n")
	}

	b.WriteString("| Benchmark | Baseline | Current | Δ | |\n")
	b.WriteString("|---|--:|--:|--:|:-:|\n")
	for _, d := range deltas {
		b.WriteString(row(d))
	}
	b.WriteString("\n")

	if newCount > 0 {
		fmt.Fprintf(&b, "_%d benchmark(s) are new (no baseline to compare)._\n\n", newCount)
	}
	if dashboardURL != "" {
		fmt.Fprintf(&b, "📈 [Trend dashboard](%s) — history and variance band.\n", dashboardURL)
	}

	return b.String()
}

func row(d model.Delta) string {
	if !d.HasBase {
		return fmt.Sprintf("| `%s` | — | %s | _new_ | 🆕 |\n", d.ID, formatNs(d.Current))
	}
	return fmt.Sprintf("| `%s` | %s | %s | %s | %s |\n",
		d.ID, formatNs(d.Baseline), formatNs(d.Current), formatPct(d.PctChange), marker(d))
}

func marker(d model.Delta) string {
	switch {
	case d.Regressed:
		return "⚠️"
	case d.PctChange < -5:
		return "🚀" // meaningfully faster
	default:
		return ""
	}
}

func countRegressed(deltas []model.Delta) int {
	n := 0
	for _, d := range deltas {
		if d.Regressed {
			n++
		}
	}
	return n
}

func countNew(deltas []model.Delta) int {
	n := 0
	for _, d := range deltas {
		if !d.HasBase {
			n++
		}
	}
	return n
}

// formatNs renders a nanosecond value with an adaptive unit, so a table mixing
// sub-microsecond cache lookups and millisecond pipeline runs stays readable.
func formatNs(ns float64) string {
	switch {
	case ns >= 1e9:
		return fmt.Sprintf("%.2f s", ns/1e9)
	case ns >= 1e6:
		return fmt.Sprintf("%.2f ms", ns/1e6)
	case ns >= 1e3:
		return fmt.Sprintf("%.2f µs", ns/1e3)
	default:
		return fmt.Sprintf("%.1f ns", ns)
	}
}

func formatPct(p float64) string {
	sign := "+"
	if p < 0 {
		sign = ""
	}
	return fmt.Sprintf("%s%.1f%%", sign, p)
}

func shortSHA(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}
