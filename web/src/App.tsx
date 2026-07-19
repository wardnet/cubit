import { useEffect, useState } from "react";
import type { Dashboard } from "./types";
import { loadData } from "./data";
import { LineChart } from "./LineChart";

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

  return (
    <main className="wrap">
      <header>
        <h1>cubit — performance</h1>
        <p className="muted">
          {data.benchmarks.length} benchmarks · branch <code>{data.branch}</code> ·
          generated {new Date(data.generatedAt).toISOString().replace("T", " ").slice(0, 16)}
        </p>
      </header>
      <div className="grid">
        {data.benchmarks.map((b) => (
          <LineChart key={b.id} series={b} />
        ))}
      </div>
    </main>
  );
}
