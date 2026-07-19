package main

import (
	"time"

	"github.com/spf13/cobra"

	"wardnet/cubit/internal/dashboard"
	"wardnet/cubit/internal/gitstore"
)

func newServeCmd() *cobra.Command {
	var (
		repo        string
		stateBranch string
		branch      string
		addr        string
		out         string
	)
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve the trend dashboard, or render it to a standalone HTML file",
		Long: "Builds the per-benchmark trend dashboard from the cubit-state branch. " +
			"With --out, writes a standalone HTML file (data inlined) and exits — the " +
			"form CI uploads as an artifact. Otherwise serves it on --addr for local viewing.",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			store := gitstore.New(repo, stateBranch)
			if out != "" {
				d, err := dashboard.Build(store, branch, time.Now())
				if err != nil {
					return err
				}
				html, err := dashboard.Render(d)
				if err != nil {
					return err
				}
				return writeOut(out, html)
			}
			return dashboard.Serve(store, branch, addr)
		},
	}
	f := cmd.Flags()
	f.StringVar(&repo, "repo", ".", "git repository holding the cubit-state branch")
	f.StringVar(&stateBranch, "state-branch", gitstore.DefaultBranch, "branch storing recorded runs")
	f.StringVar(&branch, "branch", "", "only chart runs recorded on this branch (empty = all)")
	f.StringVar(&addr, "addr", "127.0.0.1:7777", "address to serve the dashboard on")
	f.StringVar(&out, "out", "", "render a standalone HTML file here and exit (\"-\" for stdout)")
	return cmd
}
