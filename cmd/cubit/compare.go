package main

import (
	"github.com/spf13/cobra"

	"wardnet/cubit/internal/model"
	"wardnet/cubit/internal/report"
)

func newCompareCmd() *cobra.Command {
	var (
		criterionDir string
		baselinePath string
		commit       string
		branch       string
		threshold    float64
		dashboardURL string
		out          string
	)
	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Compare a benchmark run against a baseline and render a report",
		Long: "Ingests a criterion output directory, diffs it against a baseline " +
			"recorded by `cubit record`, and writes a Markdown report suitable for " +
			"a PR comment. Advisory only — it never fails the build.",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cur, err := buildRun(criterionDir, commit, branch, "")
			if err != nil {
				return err
			}
			var baseline model.Run
			if baselinePath != "" {
				baseline, err = loadRun(baselinePath)
				if err != nil {
					return err
				}
			}
			deltas := model.Compare(cur, baseline, threshold)
			md := report.Markdown(deltas, cur, threshold, dashboardURL)
			return writeOut(out, []byte(md))
		},
	}
	f := cmd.Flags()
	f.StringVar(&criterionDir, "criterion-dir", "target/criterion", "criterion output directory to ingest")
	f.StringVar(&baselinePath, "baseline", "", "baseline run JSON (from `cubit record`); omit to report the current run with no comparison")
	f.StringVar(&commit, "commit", "", "commit SHA to label the current run with")
	f.StringVar(&branch, "branch", "", "branch to label the current run with")
	f.Float64Var(&threshold, "threshold", 15, "flag benchmarks slower than baseline by more than this percent")
	f.StringVar(&dashboardURL, "dashboard-url", "", "trend-dashboard URL to link from the report")
	f.StringVar(&out, "out", "-", "write the Markdown report here (\"-\" for stdout)")
	return cmd
}
