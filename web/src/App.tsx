import { useEffect, useState } from "react";
import type { BenchSeries, Dashboard } from "./types";
import { groupKey, loadData } from "./data";
import { LineChart } from "./LineChart";
import logo from "./assets/cubit-logo.png";

interface Group {
  name: string;
  series: BenchSeries[];
}

// groupSeries buckets benchmarks by their leading id segment, preserving each
// group's incoming order (already id-sorted by the Go side) and returning groups
// sorted by name with the ungrouped bucket last — mirroring the PR comment's
// sectioning so the two views read the same.
function groupSeries(benchmarks: BenchSeries[]): Group[] {
  const byName = new Map<string, BenchSeries[]>();
  for (const b of benchmarks) {
    const key = groupKey(b.id);
    const bucket = byName.get(key);
    if (bucket) bucket.push(b);
    else byName.set(key, [b]);
  }
  return [...byName.entries()]
    .map(([name, series]) => ({ name, series }))
    .sort((a, b) => {
      if ((a.name === "") !== (b.name === "")) return a.name === "" ? 1 : -1;
      return a.name < b.name ? -1 : a.name > b.name ? 1 : 0;
    });
}

// stripGroup drops the redundant "group/" prefix from a benchmark's id for its
// caption inside that group's section (the full id still labels the chart).
function stripGroup(id: string, group: string): string {
  return group && id.startsWith(group + "/") ? id.slice(group.length + 1) : id;
}

export function App() {
  const [data, setData] = useState<Dashboard | null | undefined>(undefined);

  useEffect(() => {
    loadData().then(setData);
  }, []);

  if (data === undefined) {
    return <main className="wrap">Loading…</main>;
  }
  if (data === null || data.benchmarks.length === 0) {
    return (
      <main className="wrap">
        <h1>cubit</h1>
        <p className="muted">
          No benchmark history yet. Record a run on the default branch to start
          the trend.
        </p>
      </main>
    );
  }

  const groups = groupSeries(data.benchmarks);

  return (
    <main className="wrap">
      <header>
        <h1>
          <img className="logo" src={logo} alt="" /> cubit — performance
        </h1>
        <p className="muted">
          {data.benchmarks.length} benchmarks · {groups.length} group
          {groups.length === 1 ? "" : "s"} · branch <code>{data.branch}</code> ·
          generated {new Date(data.generatedAt).toISOString().replace("T", " ").slice(0, 16)}
        </p>
      </header>
      {groups.length <= 1 ? (
        <div className="grid">
          {data.benchmarks.map((b) => (
            <LineChart key={b.id} series={b} />
          ))}
        </div>
      ) : (
        groups.map((g) => (
          <details className="group" key={g.name || "ungrouped"} open>
            <summary>
              <span className="group-name">{g.name || "ungrouped"}</span>
              <span className="group-count">
                {g.series.length} benchmark{g.series.length === 1 ? "" : "s"}
              </span>
            </summary>
            <div className="grid">
              {g.series.map((b) => (
                <LineChart key={b.id} series={b} label={stripGroup(b.id, g.name)} />
              ))}
            </div>
          </details>
        ))
      )}
    </main>
  );
}
