// The dashboard data shape, produced by the Go side (internal/dashboard) from
// the runs recorded on the cubit-state branch. One BenchSeries per benchmark
// id; points are ordered oldest-first along the main branch.
export interface Point {
  commit: string;
  shortCommit: string;
  timestamp: string;
  value: number;
  lower: number;
  upper: number;
}

export interface BenchSeries {
  id: string;
  unit: string;
  points: Point[];
}

export interface Dashboard {
  generatedAt: string;
  branch: string;
  benchmarks: BenchSeries[];
}
