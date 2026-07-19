# cubit

Continuous benchmarking and performance-regression tracking.

`cubit` runs your benchmarks, records baselines in a git branch, and reports how
a change moves performance — locally for a developer, or as a **non-blocking PR
comment** in CI. It is a *light gate*: the numbers and the trend dashboard inform
a human go/no-go, nothing hard-fails.

Sibling to [bulwark](https://github.com/wardnet/bulwark) (code-quality scanning),
built for the [wardnet](https://github.com/wardnet) daemon's benchmarks.

## How it works

1. Your CI runs the benchmarks (today: Rust [criterion](https://github.com/bheisler/criterion.rs)).
2. `cubit compare` ingests the output, diffs it against the baseline recorded on
   the `cubit-state` branch, and posts a Markdown table (baseline vs current, Δ%)
   plus a link to a trend dashboard.
3. On the default branch, `cubit record` updates the baseline — keyed by commit
   SHA — for the next PR to compare against.

Baselines and history live in an orphan git branch (no database, no hosted
service). The dashboard renders from that branch as a CI artifact and via a local
`cubit serve`, so it works on private repos and repos whose GitHub Pages slot is
already taken.

## Commands

| Command | Purpose |
|---|---|
| `cubit compare` | Ingest a run, diff against a baseline, render the report |
| `cubit record`  | Ingest a run and write it as a baseline record (SHA-keyed) |
| `cubit update`  | Self-update to the latest release |
| `cubit version` | Print the version |

## Install

```sh
curl -fsSL https://github.com/wardnet/cubit/releases/latest/download/install.sh | sh
```

`cubit update` self-updates the CLI in place.

## Status

Early. The ingest → compare → report spine works; baseline persistence
(`cubit-state` branch), the release/action plumbing, and the React trend
dashboard are in progress.

## License

MIT — see [LICENSE](LICENSE).
