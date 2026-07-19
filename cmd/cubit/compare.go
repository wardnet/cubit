package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"wardnet/cubit/internal/gitstore"
	"wardnet/cubit/internal/model"
	"wardnet/cubit/internal/report"
)

func newCompareCmd() *cobra.Command {
	var (
		criterionDir    string
		baselinePath    string
		baselineCritDir string
		baselineRef     string
		repo            string
		stateBranch     string
		commit          string
		branch          string
		threshold       float64
		dashboardURL    string
		out             string
	)
	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Compare a benchmark run against a baseline and render a report",
		Long: "Ingests a criterion output directory, diffs it against a baseline, and " +
			"writes a Markdown report suitable for a PR comment. The baseline comes " +
			"from --baseline (a file), --baseline-criterion-dir (a second criterion " +
			"output, e.g. a paired same-runner run of the base commit), or " +
			"--baseline-ref (a record on the cubit-state branch). Advisory only — it " +
			"never fails the build.",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cur, err := buildRun(criterionDir, commit, branch, "")
			if err != nil {
				return err
			}
			baseline, err := resolveBaseline(baselinePath, baselineCritDir, baselineRef, repo, stateBranch)
			if err != nil {
				return err
			}
			deltas := model.Compare(cur, baseline, threshold)
			md := report.Markdown(deltas, cur, threshold, dashboardURL)
			return writeOut(out, []byte(md))
		},
	}
	f := cmd.Flags()
	f.StringVar(&criterionDir, "criterion-dir", "target/criterion", "criterion output directory to ingest")
	f.StringVar(&baselinePath, "baseline", "", "baseline run JSON file (highest precedence)")
	f.StringVar(&baselineCritDir, "baseline-criterion-dir", "", "criterion output dir to use as the baseline (a paired same-runner run of the base commit); takes precedence over --baseline-ref")
	f.StringVar(&baselineRef, "baseline-ref", "", "baseline from the cubit-state branch: a commit SHA, \"latest\", or \"latest:<branch>\"")
	f.StringVar(&repo, "repo", ".", "git repository holding the cubit-state branch")
	f.StringVar(&stateBranch, "state-branch", gitstore.DefaultBranch, "branch storing recorded runs")
	f.StringVar(&commit, "commit", "", "commit SHA to label the current run with")
	f.StringVar(&branch, "branch", "", "branch to label the current run with")
	f.Float64Var(&threshold, "threshold", 15, "flag benchmarks slower than baseline by more than this percent")
	f.StringVar(&dashboardURL, "dashboard-url", "", "trend-dashboard URL to link from the report")
	f.StringVar(&out, "out", "-", "write the Markdown report here (\"-\" for stdout)")
	return cmd
}

// resolveBaseline loads the baseline run from (in precedence order) an explicit
// file, a second criterion output directory (the paired same-runner base run),
// or a reference on the cubit-state branch. It returns an empty run (everything
// reported as new) when none is given, or when the paired baseline dir produced
// no benchmarks — a base bench that failed to run must degrade to "all new"
// rather than erroring, so the comment still posts.
func resolveBaseline(path, critDir, ref, repo, stateBranch string) (model.Run, error) {
	if path != "" {
		return loadRun(path)
	}
	if critDir != "" {
		run, err := buildRun(critDir, "", "", "")
		if err != nil {
			// No benchmarks in the base run (e.g. the base bench errored) — treat
			// as no baseline rather than failing the comparison.
			return model.Run{}, nil
		}
		return run, nil
	}
	if ref == "" {
		return model.Run{}, nil
	}
	st := gitstore.New(repo, stateBranch)
	switch {
	case ref == "latest":
		run, _, err := st.Latest("")
		return run, err
	case strings.HasPrefix(ref, "latest:"):
		run, _, err := st.Latest(strings.TrimPrefix(ref, "latest:"))
		return run, err
	default:
		run, found, err := st.Read(ref)
		if err != nil {
			return model.Run{}, err
		}
		if !found {
			return model.Run{}, fmt.Errorf("no recorded baseline for %q on %q", ref, stateBranch)
		}
		return run, nil
	}
}
