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

// formatValue renders a nanosecond metric with an adaptive unit, matching the
// PR-comment formatter so the dashboard and comment read the same.
export function formatValue(ns: number): string {
  if (ns >= 1e9) return `${(ns / 1e9).toFixed(2)} s`;
  if (ns >= 1e6) return `${(ns / 1e6).toFixed(2)} ms`;
  if (ns >= 1e3) return `${(ns / 1e3).toFixed(2)} µs`;
  return `${ns.toFixed(1)} ns`;
}
