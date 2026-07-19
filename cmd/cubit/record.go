package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"wardnet/cubit/internal/gitstore"
)

func newRecordCmd() *cobra.Command {
	var (
		criterionDir string
		commit       string
		branch       string
		out          string
		persist      bool
		repo         string
		stateBranch  string
	)
	cmd := &cobra.Command{
		Use:   "record",
		Short: "Ingest a benchmark run and write it as a baseline record",
		Long: "Ingests a criterion output directory into a normalized run record, " +
			"keyed by commit SHA. With --persist it commits the record to the " +
			"cubit-state branch (via git plumbing, without a checkout) — the CI " +
			"default-branch step that updates the baseline the next PR compares against.",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			run, err := buildRun(criterionDir, commit, branch, "")
			if err != nil {
				return err
			}
			if persist {
				if err := gitstore.New(repo, stateBranch).Append(run); err != nil {
					return err
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "recorded %d benchmarks for %s on %s\n",
					len(run.Measurements), run.Commit, stateBranch)
			}
			// Emit JSON to an explicit --out, or to stdout only when we are not
			// purely persisting (so `record --persist` stays quiet on stdout).
			if out != "" {
				return writeJSON(out, run)
			}
			if !persist {
				return writeJSON("-", run)
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&criterionDir, "criterion-dir", "target/criterion", "criterion output directory to ingest")
	f.StringVar(&commit, "commit", "", "commit SHA this run was measured at")
	f.StringVar(&branch, "branch", "", "branch this run was measured on")
	f.StringVar(&out, "out", "", "also write the run record JSON here (\"-\" for stdout)")
	f.BoolVar(&persist, "persist", false, "commit the record to the cubit-state branch")
	f.StringVar(&repo, "repo", ".", "git repository holding the cubit-state branch")
	f.StringVar(&stateBranch, "state-branch", gitstore.DefaultBranch, "branch to store the record on")
	return cmd
}
