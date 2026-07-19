package main

import "github.com/spf13/cobra"

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "cubit",
		Short: "Continuous benchmarking and performance-regression tracking",
		Long: "cubit runs benchmarks, records baselines, and reports how a change " +
			"moves performance — locally for a developer, or as a non-blocking PR " +
			"comment in CI.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(
		newCompareCmd(),
		newRecordCmd(),
		newUpdateCmd(),
		newVersionCmd(),
	)
	return root
}
