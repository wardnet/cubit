import type { BenchSeries } from "./types";
import { formatValue } from "./data";

const W = 720;
const H = 240;
const PAD = { top: 16, right: 16, bottom: 28, left: 64 };

// LineChart draws one benchmark's history: a median line with the
// confidence-interval band shaded behind it, so a reviewer can see whether a
// new point sits inside the run-to-run noise or steps out of it. label overrides
// the visible caption (the grouped view passes the id with its group prefix
// stripped); the full id still drives the accessible label.
export function LineChart({ series, label }: { series: BenchSeries; label?: string }) {
  const pts = series.points;
  const plotW = W - PAD.left - PAD.right;
  const plotH = H - PAD.top - PAD.bottom;

  if (pts.length === 0) {
    return <div className="chart-empty">no data</div>;
  }

  const lows = pts.map((p) => p.lower);
  const highs = pts.map((p) => p.upper);
  let yMin = Math.min(...lows);
  let yMax = Math.max(...highs);
  if (yMin === yMax) {
    yMin *= 0.95;
    yMax *= 1.05;
  }
  const span = yMax - yMin || 1;

  const x = (i: number) =>
    PAD.left + (pts.length === 1 ? plotW / 2 : (i / (pts.length - 1)) * plotW);
  const y = (v: number) => PAD.top + plotH - ((v - yMin) / span) * plotH;

  const line = pts.map((p, i) => `${x(i)},${y(p.value)}`).join(" ");
  const band =
    pts.map((p, i) => `${x(i)},${y(p.upper)}`).join(" ") +
    " " +
    pts
      .map((p, i) => `${x(pts.length - 1 - i)},${y(pts[pts.length - 1 - i].lower)}`)
      .join(" ");

  const last = pts[pts.length - 1];

  return (
    <figure className="chart">
      <figcaption>
        <span className="chart-id">{label ?? series.id}</span>
        <span className="chart-last">{formatValue(last.value)}</span>
      </figcaption>
      <svg viewBox={`0 0 ${W} ${H}`} role="img" aria-label={`${series.id} over time`}>
        {[0, 0.25, 0.5, 0.75, 1].map((f) => {
          const gy = PAD.top + plotH * f;
          const val = yMax - span * f;
          return (
            <g key={f}>
              <line className="grid" x1={PAD.left} y1={gy} x2={W - PAD.right} y2={gy} />
              <text className="ytick" x={PAD.left - 8} y={gy + 4} textAnchor="end">
                {formatValue(val)}
              </text>
            </g>
          );
        })}
        <polygon className="band" points={band} />
        <polyline className="line" points={line} fill="none" />
        {pts.map((p, i) => (
          <circle key={p.commit + i} className="dot" cx={x(i)} cy={y(p.value)} r={3}>
            <title>
              {p.shortCommit} · {formatValue(p.value)} ({new Date(p.timestamp).toISOString().slice(0, 10)})
            </title>
          </circle>
        ))}
        {pts.map((p, i) =>
          i === 0 || i === pts.length - 1 || pts.length <= 6 ? (
            <text key={"x" + i} className="xtick" x={x(i)} y={H - 8} textAnchor="middle">
              {p.shortCommit}
            </text>
          ) : null,
        )}
      </svg>
    </figure>
  );
}
