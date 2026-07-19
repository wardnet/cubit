package main

import "github.com/spf13/cobra"

func newRecordCmd() *cobra.Command {
	var (
		criterionDir string
		commit       string
		branch       string
		out          string
	)
	cmd := &cobra.Command{
		Use:   "record",
		Short: "Ingest a benchmark run and write it as a baseline record",
		Long: "Ingests a criterion output directory into a normalized run record, " +
			"keyed by commit SHA. In CI this runs on the default branch to update " +
			"the baseline the next PR compares against.",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			run, err := buildRun(criterionDir, commit, branch, "")
			if err != nil {
				return err
			}
			return writeJSON(out, run)
		},
	}
	f := cmd.Flags()
	f.StringVar(&criterionDir, "criterion-dir", "target/criterion", "criterion output directory to ingest")
	f.StringVar(&commit, "commit", "", "commit SHA this run was measured at")
	f.StringVar(&branch, "branch", "", "branch this run was measured on")
	f.StringVar(&out, "out", "-", "write the run record JSON here (\"-\" for stdout)")
	return cmd
}
