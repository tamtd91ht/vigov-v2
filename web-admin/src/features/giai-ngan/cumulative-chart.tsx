"use client";

import { type KeyboardEvent, useId, useState } from "react";

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
 * for a screen reader.
 *
 * READING ONE MONTH (prototype `CumulativeChart.tsx:71-86`): an HTML overlay above the SVG holds one
 * hover column per month and is itself ONE tab stop — arrows / Home / End move between months, Escape
 * hides. One stop, not twelve: a keyboard user crossing the page should not tab through every month to
 * reach the table under it. The tooltip is React text; its figures are the same full-đồng strings as
 * the hidden table, never the axis's rounded `8,5 tỷ`.
 *
 * `Hoàn thành dự kiến` (prototype `:88-100`): a dashed vertical line at the month the SERVER names.
 * A month not among the drawn points draws nothing — a line pinned to the frame's edge would claim a
 * month the chart does not show. At 320px the chart scrolls sideways inside its card (`min-w`), like every table
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

/**
 * Keeps the tooltip inside the frame: anchored on its left edge near January, its right edge near
 * December, centred between — a centred box at the first month would hang off the card.
 */
function tooltipPosition(share: number): { left?: string; right?: string; transform?: string } {
  if (share < 0.25) return { left: `${share * 100}%` };
  if (share > 0.75) return { right: `${(1 - share) * 100}%` };
  return { left: `${share * 100}%`, transform: "translateX(-50%)" };
}

export function CumulativeChart({
  points,
  caption,
  height = 240,
  emptyText,
  expectedEndMonth,
}: {
  points: readonly finance_curvePointOut[];
  /** Names the chart for the hidden table, e.g. "Luỹ kế giải ngân năm 2026 so với kế hoạch". */
  caption: string;
  height?: number;
  /** Said instead of a chart when there is nothing to scale (no plan, nothing paid). */
  emptyText: string;
  /** The server's `expected_end_month` (one project's curve only); absent → no reference line. */
  expectedEndMonth?: number | null;
}) {
  // `cursor` survives Escape so the next arrow moves on from where the reader was, not from January.
  const [cursor, setCursor] = useState(0);
  const [open, setOpen] = useState(false);
  const tooltipId = useId();
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
  const last = points.length - 1;
  const endIndex =
    expectedEndMonth === undefined || expectedEndMonth === null
      ? -1
      : points.findIndex((p) => p.month === expectedEndMonth);
  // Share of the SVG's width, so the HTML overlay lines up with the scaled SVG at any width.
  const pct = (px: number) => `${(px / WIDTH) * 100}%`;
  const shown = open ? points[Math.min(cursor, last)] : undefined;

  function show(i: number): void {
    setCursor(Math.max(0, Math.min(i, last)));
    setOpen(true);
  }

  function onKeyDown(e: KeyboardEvent<HTMLDivElement>): void {
    const from = Math.min(cursor, last);
    const step: Record<string, number> = { ArrowRight: from + 1, ArrowLeft: from - 1, Home: 0, End: last };
    if (e.key === "Escape") {
      setOpen(false);
      return;
    }
    const to = step[e.key];
    if (to === undefined) return;
    // Arrows would otherwise scroll the page (or the chart's own sideways scroll at 320px).
    e.preventDefault();
    show(to);
  }

  return (
    <figure className="m-0 min-w-0" data-cumulative-chart="">
      <div className="overflow-x-auto">
        <div className="relative min-w-[480px]">
          <svg viewBox={`0 0 ${WIDTH} ${height}`} className="block h-auto w-full" aria-hidden="true" focusable="false">
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
            <polyline points={plan} fill="none" className="stroke-ink-400" strokeWidth={2} strokeDasharray="6 5" data-line="plan" />
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
            {endIndex >= 0 && (
              <g>
                <line
                  x1={x(endIndex)}
                  x2={x(endIndex)}
                  y1={PAD.top}
                  y2={PAD.top + plotH}
                  className="stroke-warning-600"
                  strokeWidth={1.5}
                  strokeDasharray="4 4"
                  data-expected-end=""
                />
                {/* Left of the line as the prototype's `insideTopRight`, unless that would leave the frame. */}
                <text
                  x={endIndex === 0 ? x(endIndex) + 4 : x(endIndex) - 4}
                  y={PAD.top + 10}
                  textAnchor={endIndex === 0 ? "start" : "end"}
                  fontSize={10.5}
                  className="fill-warning-600"
                  data-expected-end-label=""
                >
                  Hoàn thành dự kiến
                </text>
              </g>
            )}
            {shown !== undefined && (
              <line
                x1={x(Math.min(cursor, last))}
                x2={x(Math.min(cursor, last))}
                y1={PAD.top}
                y2={PAD.top + plotH}
                className="stroke-line-strong"
                strokeWidth={1}
                data-active-month={shown.month}
              />
            )}
            {segments.flat().map((r) => (
              <circle key={`dot-${r.index}`} cx={x(r.index)} cy={y(r.value)} r={3.5} className="fill-brand-500" data-dot="" />
            ))}
          </svg>
          <div
            role="group"
            tabIndex={0}
            aria-label={`${caption}. Dùng phím mũi tên trái, phải để xem số liệu từng tháng.`}
            aria-describedby={shown !== undefined ? tooltipId : undefined}
            className="absolute inset-0 rounded-md focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-500"
            onFocus={() => show(cursor)}
            onBlur={() => setOpen(false)}
            onKeyDown={onKeyDown}
            onMouseLeave={() => setOpen(false)}
            data-chart-focus=""
          >
            {points.map((p, i) => {
              // Each month owns the band halfway to its neighbours, so any x over the plot reads a month.
              const from = i === 0 ? PAD.left : (x(i - 1) + x(i)) / 2;
              const to = i === last ? WIDTH - PAD.right : (x(i) + x(i + 1)) / 2;
              return (
                <div
                  key={p.month}
                  className="absolute inset-y-0"
                  style={{ left: pct(from), width: pct(to - from) }}
                  onMouseEnter={() => show(i)}
                  data-chart-hit={p.month}
                />
              );
            })}
          </div>
          {shown !== undefined && (
            <div
              id={tooltipId}
              role="tooltip"
              className="pointer-events-none absolute top-1 z-10 rounded-md border border-line bg-surface px-2.5 py-1.5 text-xs text-ink-700 shadow-sm"
              style={tooltipPosition(x(Math.min(cursor, last)) / WIDTH)}
              data-chart-tooltip=""
            >
              <p className="m-0 font-semibold text-ink-900">Tháng {shown.month}</p>
              <p className="m-0">Kế hoạch luỹ kế: {nhanTien(shown.planned_cumulative)}</p>
              <p className="m-0">
                Thực hiện luỹ kế:{" "}
                {shown.disbursed_cumulative === null ? "Chưa đến tháng này" : nhanTien(shown.disbursed_cumulative)}
              </p>
            </div>
          )}
        </div>
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
