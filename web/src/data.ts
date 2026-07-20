import type { Dashboard } from "./types";

declare global {
  interface Window {
    __CUBIT_DATA__?: Dashboard | null;
  }
}

// loadData reads the dashboard payload. The Go side injects it as a global for
// the standalone/served page; a dev server (`yarn dev`) falls back to fetching
// ./data.json.
export async function loadData(): Promise<Dashboard | null> {
  if (window.__CUBIT_DATA__) {
    return window.__CUBIT_DATA__;
  }
  try {
    const res = await fetch("./data.json");
    if (res.ok) {
      return (await res.json()) as Dashboard;
    }
  } catch {
    /* no dev data available */
  }
  return null;
}

// groupKey returns a benchmark's group: the leading id segment before the first
// "/". criterion nests ids as "group/function/value", so this is the
// benchmark_group it belongs to — the axis the dashboard folds a large suite
// along. A top-level bench with no "/" has no group and yields "" (an
// "ungrouped" section). Mirrors model.GroupKey on the Go side.
export function groupKey(id: string): string {
  const i = id.indexOf("/");
  return i >= 0 ? id.slice(0, i) : "";
}

// formatValue renders a nanosecond metric with an adaptive unit, matching the
// PR-comment formatter so the dashboard and comment read the same.
export function formatValue(ns: number): string {
  if (ns >= 1e9) return `${(ns / 1e9).toFixed(2)} s`;
  if (ns >= 1e6) return `${(ns / 1e6).toFixed(2)} ms`;
  if (ns >= 1e3) return `${(ns / 1e3).toFixed(2)} µs`;
  return `${ns.toFixed(1)} ns`;
}
