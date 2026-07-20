// Package report renders a comparison between a current run and its baseline
// into the Markdown that Cubit posts as a sticky PR comment.
//
// The comment is deliberately advisory: it summarizes what moved and flags
// slowdowns past the threshold, but it never asserts pass/fail. The human
// reviewer, with the trend dashboard for context, makes the go/no-go call.
package report

import (
	"fmt"
	"sort"
	"strings"

	"wardnet/cubit/internal/model"
)

// Markdown renders the PR comment body for the given deltas. thresholdPct is
// echoed into the header so a reader knows what "⚠️" means for this run.
func Markdown(deltas []model.Delta, cur model.Run, thresholdPct float64, dashboardURL string) string {
	var b strings.Builder

	// The logo is an absolute raw URL, not a repo-relative path: this comment is
	// posted into the *consuming* repo's PR, where "assets/..." would resolve
	// against that repo and 404. Pinned to cubit's default branch (not a release
	// tag) so the image keeps resolving for consumers on an older cubit version.
	fmt.Fprintf(&b, "## <img src=\"https://raw.githubusercontent.com/wardnet/cubit/main/assets/cubit-logo.png\" alt=\"\" width=\"16\" align=\"top\"> cubit — performance\n\n")
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

	// A flat table is fine for a handful of benches, but a suite of dozens spread
	// across several criterion groups swamps the comment. When there is more than
	// one group, fold each into a collapsible section — clean groups collapsed,
	// groups with a regression expanded — so a reviewer sees what moved without
	// scrolling a wall of rows.
	if groups := groupDeltas(deltas); len(groups) > 1 {
		for _, g := range groups {
			writeGroup(&b, g)
		}
	} else {
		writeTable(&b, deltas, "")
	}

	if newCount > 0 {
		fmt.Fprintf(&b, "_%d benchmark(s) are new (no baseline to compare)._\n\n", newCount)
	}
	if dashboardURL != "" {
		fmt.Fprintf(&b, "📈 [Trend dashboard](%s) — history and variance band.\n", dashboardURL)
	}

	return b.String()
}

// group is a set of deltas sharing a benchmark group (their leading id segment).
type group struct {
	name   string // the shared group; "" for ungrouped, top-level benches
	deltas []model.Delta
}

// groupDeltas buckets deltas by [model.GroupKey], preserving each group's
// internal order (Compare already sorted by id) and returning groups sorted by
// name with the ungrouped bucket last, so the sections read alphabetically and
// the miscellany falls to the bottom.
func groupDeltas(deltas []model.Delta) []group {
	index := map[string]*group{}
	var order []*group
	for _, d := range deltas {
		name := model.GroupKey(d.ID)
		g, ok := index[name]
		if !ok {
			g = &group{name: name}
			index[name] = g
			order = append(order, g)
		}
		g.deltas = append(g.deltas, d)
	}
	sort.Slice(order, func(i, j int) bool {
		a, z := order[i].name, order[j].name
		if (a == "") != (z == "") {
			return a != "" // ungrouped ("") sorts last
		}
		return a < z
	})
	out := make([]group, len(order))
	for i, g := range order {
		out[i] = *g
	}
	return out
}

// writeGroup renders one collapsible section: a summary line carrying the
// group's name, size and status, and — inside — its table. The section is
// pre-expanded when the group holds a regression, so problems are visible
// without a click while healthy groups stay folded.
func writeGroup(b *strings.Builder, g group) {
	name := g.name
	if name == "" {
		name = "ungrouped"
	}
	open := ""
	if countRegressed(g.deltas) > 0 {
		open = " open"
	}
	fmt.Fprintf(b, "<details%s><summary><strong>%s</strong> · %d benchmark%s · %s</summary>\n\n",
		open, name, len(g.deltas), plural(len(g.deltas)), groupStatus(g.deltas))
	writeTable(b, g.deltas, g.name)
	b.WriteString("</details>\n\n")
}

// writeTable renders the benchmark table for deltas. groupName, when non-empty,
// is stripped from each row's id so a "dns" section lists "parse/a" rather than
// repeating "dns/parse/a" on every row.
func writeTable(b *strings.Builder, deltas []model.Delta, groupName string) {
	b.WriteString("| Benchmark | Baseline | Current | Δ | |\n")
	b.WriteString("|---|--:|--:|--:|:-:|\n")
	for _, d := range deltas {
		b.WriteString(row(d, trimGroup(d.ID, groupName)))
	}
	b.WriteString("\n")
}

// groupStatus is the one-glance verdict shown in a group's summary line: a
// regression count wins, otherwise a speedup, otherwise a clean check.
func groupStatus(deltas []model.Delta) string {
	if r := countRegressed(deltas); r > 0 {
		return fmt.Sprintf("⚠️ %d slower", r)
	}
	if countFaster(deltas) > 0 {
		return "🚀 faster"
	}
	return "✅"
}

func trimGroup(id, groupName string) string {
	if groupName == "" {
		return id
	}
	return strings.TrimPrefix(id, groupName+"/")
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func row(d model.Delta, label string) string {
	if !d.HasBase {
		return fmt.Sprintf("| `%s` | — | %s | _new_ | 🆕 |\n", label, formatNs(d.Current))
	}
	return fmt.Sprintf("| `%s` | %s | %s | %s | %s |\n",
		label, formatNs(d.Baseline), formatNs(d.Current), formatPct(d.PctChange), marker(d))
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

// countFaster counts benchmarks that got meaningfully faster — the same
// threshold marker() uses for the 🚀 flag, so the summary and rows agree.
func countFaster(deltas []model.Delta) int {
	n := 0
	for _, d := range deltas {
		if d.HasBase && d.PctChange < -5 {
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
