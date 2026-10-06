import type { finance_curvePointOut } from "@/lib/api/schema.gen";

import { nhanTien } from "./nhan-du-an";

/**
 * The cumulative chart of spec §4 (year, list screen) and §8.3 (one project, `Biểu đồ` tab): plan
 * against actual, month by month.
 *
 * INLINE SVG, NO CHART LIBRARY: web-admin ships none (`package.json`), and two polylines with dots do
 * not justify a dependency — rule 13 #6 makes every dependency a vulnerability surface to watch.
 *
 * DATAVIZ CONVENTION (spec §4): plan dashed grey, actual solid blue with a dot per month, legend
 * `— — Kế hoạch  —●— Thực hiện` under the chart. EVERY POINT IS THE SERVER'S — this file scales and
 * draws, it never adds money up.
 *
 * THE ACTUAL LINE STOPS AT A `null` MONTH: the server sends `null` for a month that has not begun, and
 * drawing it flat to December would read as a forecast nobody made (`disbursement_summary.go`). A gap
 * breaks the line rather than bridging it.
 *
 * The SVG is `aria-hidden`; a visually hidden table under it carries the same figures, in full đồng,
 * for a screen reader. At 320px the chart scrolls sideways inside its card (`min-w`), like every table
 * of this app: shrinking it would shrink its 11px labels to unreadable.
 */

const WIDTH = 640;
const PAD = { left: 64, right: 16, top: 12, bottom: 28 } as const;

/** vi-VN pinned, ≤ 2 decimals: `8,5 tỷ`. Axis labels only — every figure elsewhere is in full đồng. */
const AXIS_NUMBER = new Intl.NumberFormat("vi-VN", { maximumFractionDigits: 2 });

/**
 * Five Y ticks, 0 → top, "tự chia theo kế hoạch năm" (§4): the largest value rounded UP to two
 * significant digits, divided in four. §4's own example: 33,23 tỷ → top 34 tỷ → 0 · 8,5 · 17 · 25,5 · 34.
 *
 * The largest value includes the ACTUAL line: an over-disbursed year would otherwise draw above the
 * frame. Nothing to scale (all zero) → `top` 0, and the caller says so in words instead of a chart.
 */
export function chartScale(points: readonly finance_curvePointOut[]): { top: number; ticks: number[] } {
  let max = 0;
  for (const p of points) {
    if (Number.isFinite(p.planned_cumulative)) max = Math.max(max, p.planned_cumulative);
    if (p.disbursed_cumulative !== null && Number.isFinite(p.disbursed_cumulative)) {
      max = Math.max(max, p.disbursed_cumulative);
    }
  }
  if (max <= 0) return { top: 0, ticks: [0] };
  const unit = 10 ** (Math.floor(Math.log10(max)) - 1);
  const top = Math.ceil(max / unit) * unit;
  return { top, ticks: [0, 1, 2, 3, 4].map((i) => (top * i) / 4) };
}

/** `0 đ` · `500 triệu` · `8,5 tỷ` — the axis's short form. */
export function axisMoneyLabel(dong: number): string {
  const abs = Math.abs(dong);
  if (abs === 0) return "0 đ";
  if (abs >= 1e9) return `${AXIS_NUMBER.format(dong / 1e9)} tỷ`;
  if (abs >= 1e6) return `${AXIS_NUMBER.format(dong / 1e6)} triệu`;
  if (abs >= 1e3) return `${AXIS_NUMBER.format(dong / 1e3)} nghìn`;
  return `${AXIS_NUMBER.format(dong)} đ`;
}

/**
 * Runs of consecutive months that HAVE an actual figure — one polyline each. Exported for the test
 * that pins "the line stops at a null month".
 */
export function actualSegments(
  points: readonly finance_curvePointOut[],
): { index: number; value: number }[][] {
  const runs: { index: number; value: number }[][] = [];
  let run: { index: number; value: number }[] = [];
  points.forEach((p, index) => {
    const v = p.disbursed_cumulative;
    if (v === null || !Number.isFinite(v)) {
      if (run.length > 0) runs.push(run);
      run = [];
      return;
    }
    run.push({ index, value: v });
  });
  if (run.length > 0) runs.push(run);
  return runs;
}

export function CumulativeChart({
  points,
  caption,
  height = 240,
  emptyText,
}: {
  points: readonly finance_curvePointOut[];
  /** Names the chart for the hidden table, e.g. "Luỹ kế giải ngân năm 2026 so với kế hoạch". */
  caption: string;
  height?: number;
  /** Said instead of a chart when there is nothing to scale (no plan, nothing paid). */
  emptyText: string;
}) {
  const { top, ticks } = chartScale(points);
  if (points.length === 0 || top === 0) {
    return <p className="m-0 text-sm text-ink-500">{emptyText}</p>;
  }

  const plotW = WIDTH - PAD.left - PAD.right;
  const plotH = height - PAD.top - PAD.bottom;
  const x = (i: number) => (points.length === 1 ? PAD.left + plotW / 2 : PAD.left + (i * plotW) / (points.length - 1));
  const y = (v: number) => PAD.top + plotH - (v / top) * plotH;
  const plan = points.map((p, i) => `${x(i)},${y(p.planned_cumulative)}`).join(" ");
  const segments = actualSegments(points);

  return (
    <figure className="m-0 min-w-0" data-cumulative-chart="">
      <div className="overflow-x-auto">
        <svg
          viewBox={`0 0 ${WIDTH} ${height}`}
          className="block h-auto w-full min-w-[480px]"
          aria-hidden="true"
          focusable="false"
        >
          {ticks.map((t) => (
            <g key={t}>
              <line x1={PAD.left} x2={WIDTH - PAD.right} y1={y(t)} y2={y(t)} className="stroke-line" strokeWidth={1} />
              <text x={PAD.left - 8} y={y(t)} textAnchor="end" dominantBaseline="middle" fontSize={11} className="fill-ink-500">
                {axisMoneyLabel(t)}
              </text>
            </g>
          ))}
          {points.map((p, i) => (
            <text key={p.month} x={x(i)} y={height - 8} textAnchor="middle" fontSize={11} className="fill-ink-500">
              T{p.month}
            </text>
          ))}
          <polyline
            points={plan}
            fill="none"
            className="stroke-ink-400"
            strokeWidth={2}
            strokeDasharray="6 5"
            data-line="plan"
          />
          {segments.map((run) => (
            <polyline
              key={`line-${run[0]!.index}`}
              points={run.map((r) => `${x(r.index)},${y(r.value)}`).join(" ")}
              fill="none"
              className="stroke-brand-500"
              strokeWidth={2.5}
              data-line="actual"
            />
          ))}
          {segments.flat().map((r) => (
            <circle key={`dot-${r.index}`} cx={x(r.index)} cy={y(r.value)} r={3.5} className="fill-brand-500" data-dot="" />
          ))}
        </svg>
      </div>
      <figcaption className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-ink-500">
        <span className="inline-flex items-center gap-1.5">
          <svg width="24" height="8" aria-hidden="true" focusable="false">
            <line x1="0" x2="24" y1="4" y2="4" className="stroke-ink-400" strokeWidth={2} strokeDasharray="5 3" />
          </svg>
          Kế hoạch
        </span>
        <span className="inline-flex items-center gap-1.5">
          <svg width="24" height="8" aria-hidden="true" focusable="false">
            <line x1="0" x2="24" y1="4" y2="4" className="stroke-brand-500" strokeWidth={2.5} />
            <circle cx="12" cy="4" r="3" className="fill-brand-500" />
          </svg>
          Thực hiện
        </span>
      </figcaption>
      <table className="an-thi-giac">
        <caption>{caption}</caption>
        <thead>
          <tr>
            <th scope="col">Tháng</th>
            <th scope="col">Kế hoạch luỹ kế</th>
            <th scope="col">Thực hiện luỹ kế</th>
          </tr>
        </thead>
        <tbody>
          {points.map((p) => (
            <tr key={p.month}>
              <th scope="row">Tháng {p.month}</th>
              <td>{nhanTien(p.planned_cumulative)}</td>
              <td>{p.disbursed_cumulative === null ? "Chưa đến tháng này" : nhanTien(p.disbursed_cumulative)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </figure>
  );
}
