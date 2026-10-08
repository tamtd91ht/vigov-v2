/**
 * An SVG path `d` through points as a MONOTONE-X cubic curve — the curve the prototype's recharts
 * `<Line type="monotone">` draws (`vigov-require/.../CumulativeChart.tsx:101-117`), which is d3's
 * `curveMonotoneX` (Steffen 1990: Fritsch–Carlson-style monotone Hermite tangents). ADR 0081 #4.
 *
 * WHY MONOTONE AND NOT ANY SMOOTH CURVE: a cumulative amount never goes down. A Catmull-Rom or a
 * natural spline overshoots around a flat run (months with no payment) and draws a dip below a figure
 * already paid — a fall in money disbursed that never happened. Monotone tangents keep each segment
 * between its two end values, so the curve only smooths, it never invents a value.
 *
 * Written here rather than imported (d3-shape) because rule 13 #6 makes every dependency a
 * vulnerability surface, and this is forty lines (ADR 0068 lần 6 #11: no chart library).
 *
 * Points must be in increasing `x` (months are). 0 points → `""`; 1 → a bare move; 2 → a straight
 * segment, as d3 does.
 */
export function monotoneXPath(points: readonly { x: number; y: number }[]): string {
  const n = points.length;
  if (n === 0) return "";
  const p0 = points[0]!;
  if (n === 1) return `M${num(p0.x)},${num(p0.y)}`;
  if (n === 2) {
    const p1 = points[1]!;
    return `M${num(p0.x)},${num(p0.y)}L${num(p1.x)},${num(p1.y)}`;
  }

  const t = monotoneTangents(points);
  let d = `M${num(p0.x)},${num(p0.y)}`;
  for (let i = 0; i < n - 1; i++) {
    const a = points[i]!;
    const b = points[i + 1]!;
    const dx = (b.x - a.x) / 3;
    d +=
      `C${num(a.x + dx)},${num(a.y + dx * t[i]!)}` +
      `,${num(b.x - dx)},${num(b.y - dx * t[i + 1]!)}` +
      `,${num(b.x)},${num(b.y)}`;
  }
  return d;
}

/**
 * The tangent at each point (n ≥ 3), exactly d3's `slope3` inside and `slope2` at both ends.
 * Exported for the test that pins "a flat run stays flat" and "a peak gets a zero tangent".
 */
export function monotoneTangents(points: readonly { x: number; y: number }[]): number[] {
  const n = points.length;
  const t = new Array<number>(n).fill(0);
  for (let i = 1; i < n - 1; i++) {
    const prev = points[i - 1]!;
    const here = points[i]!;
    const next = points[i + 1]!;
    const h0 = here.x - prev.x;
    const h1 = next.x - here.x;
    const s0 = h0 === 0 ? 0 : (here.y - prev.y) / h0;
    const s1 = h1 === 0 ? 0 : (next.y - here.y) / h1;
    const p = (s0 * h1 + s1 * h0) / (h0 + h1);
    // Opposite signs (a local peak or trough) → 0; same sign → the smallest of the three, so the
    // segment's control points cannot pass either neighbour's value.
    t[i] = (sign(s0) + sign(s1)) * Math.min(Math.abs(s0), Math.abs(s1), 0.5 * Math.abs(p)) || 0;
  }
  t[0] = endTangent(points[0]!, points[1]!, t[1]!);
  t[n - 1] = endTangent(points[n - 2]!, points[n - 1]!, t[n - 2]!);
  return t;
}

/** d3 `slope2`: the one-sided three-point estimate at an end of the line. */
function endTangent(a: { x: number; y: number }, b: { x: number; y: number }, neighbour: number): number {
  const h = b.x - a.x;
  return h === 0 ? neighbour : ((3 * (b.y - a.y)) / h - neighbour) / 2;
}

/** d3's sign: zero counts as positive, so a flat secant beside a rising one still yields 0. */
function sign(v: number): number {
  return v < 0 ? -1 : 1;
}

/** At most 2 decimals, no trailing zeros — a sub-pixel is invisible and keeps `d` short. */
function num(v: number): string {
  const r = Math.round(v * 100) / 100;
  return Object.is(r, -0) ? "0" : String(r);
}
