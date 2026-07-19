// Command cubit is a continuous-benchmarking CLI: it ingests a benchmark
// runner's output, compares it against a recorded baseline, and reports the
// delta — locally for a developer, or as a non-blocking PR comment in CI.
package main

import (
	"fmt"
	"os"
)

// version is overridden at release via -ldflags "-X main.version=<tag>".
// A "dev" build (built from source) disables self-update.
var version = "dev"

func main() {
	// A best-effort, once-a-day nudge when a newer release exists. Silent in
	// CI, non-interactive, and from-source builds; never blocks the command.
	maybeNudgeUpdate()

	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
