package report

import (
	"strings"
	"testing"

	"wardnet/cubit/internal/model"
)

func delta(id string, base, cur float64, threshold float64) model.Delta {
	d := model.Delta{ID: id, Unit: "ns", Current: cur}
	if base > 0 {
		d.HasBase = true
		d.Baseline = base
		d.PctChange = (cur - base) / base * 100
		d.Regressed = d.PctChange > threshold
	}
	return d
}

func TestMarkdownGroupsIntoSections(t *testing.T) {
	deltas := []model.Delta{
		delta("dns/parse/a", 100, 102, 15),
		delta("dns/parse/aaaa", 100, 130, 15), // regression
		delta("cache/get/hit", 40, 41, 15),
		delta("cache/put/large", 800, 808, 15),
	}
	md := Markdown(deltas, model.Run{Commit: "abc"}, 15, "")

	if strings.Count(md, "<details") != 2 {
		t.Fatalf("expected 2 group sections, got %d\n%s", strings.Count(md, "<details"), md)
	}
	// The group holding the regression is pre-expanded; the clean one stays folded.
	if !strings.Contains(md, "<details open><summary><strong>dns</strong>") {
		t.Errorf("dns group with a regression should be open:\n%s", md)
	}
	if !strings.Contains(md, "<details><summary><strong>cache</strong>") {
		t.Errorf("clean cache group should be collapsed:\n%s", md)
	}
	// Rows inside a section drop the redundant group prefix.
	if !strings.Contains(md, "| `parse/a` |") {
		t.Errorf("row id should be stripped of its group prefix:\n%s", md)
	}
	if strings.Contains(md, "| `dns/parse/a` |") {
		t.Errorf("row id should not repeat the group prefix:\n%s", md)
	}
}

func TestMarkdownSingleGroupStaysFlat(t *testing.T) {
	deltas := []model.Delta{
		delta("dns/parse/a", 100, 101, 15),
		delta("dns/parse/aaaa", 100, 102, 15),
	}
	md := Markdown(deltas, model.Run{}, 15, "")
	if strings.Contains(md, "<details") {
		t.Errorf("a single group should render a flat table, not sections:\n%s", md)
	}
	// A flat table keeps the full id, since there is no section header for context.
	if !strings.Contains(md, "| `dns/parse/a` |") {
		t.Errorf("flat table should keep the full id:\n%s", md)
	}
}

func TestGroupStatus(t *testing.T) {
	regressed := []model.Delta{delta("g/x", 100, 130, 15)}
	if got := groupStatus(regressed); !strings.HasPrefix(got, "⚠️") {
		t.Errorf("regressed group status = %q, want a ⚠️ slower verdict", got)
	}
	faster := []model.Delta{delta("g/x", 100, 90, 15)}
	if got := groupStatus(faster); got != "🚀 faster" {
		t.Errorf("faster group status = %q, want 🚀 faster", got)
	}
	flat := []model.Delta{delta("g/x", 100, 101, 15)}
	if got := groupStatus(flat); got != "✅" {
		t.Errorf("unchanged group status = %q, want ✅", got)
	}
}

func TestGroupDeltasOrdersUngroupedLast(t *testing.T) {
	deltas := []model.Delta{
		delta("startup", 100, 101, 15), // ungrouped
		delta("router/x", 100, 101, 15),
		delta("cache/y", 100, 101, 15),
	}
	groups := groupDeltas(deltas)
	got := make([]string, len(groups))
	for i, g := range groups {
		got[i] = g.name
	}
	want := []string{"cache", "router", ""}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("group order = %v, want %v (ungrouped last)", got, want)
	}
}
