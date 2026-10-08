"use client";

import { type KeyboardEvent, type RefObject, useEffect, useId, useRef, useState } from "react";

import type { finance_curvePointOut } from "@/lib/api/schema.gen";

import { monotoneXPath } from "./monotone-path";
import { nhanTien } from "./nhan-du-an";

/**
 * The cumulative chart of spec §4 (year, list screen) and §8.3 (one project, `Biểu đồ` tab): plan
 * against actual, month by month.
 *
 * INLINE SVG, NO CHART LIBRARY (ADR 0068 lần 6 #11: the owner kept the hand-drawn SVG, restyled; no
 * recharts — rule 13 #6 makes every dependency a vulnerability surface, and shadcn's `chart` stays
 * banned by §4). The look is spec 02 §4's recharts settings, reproduced: dashed `3 3` grid in the
 * line colour, 11px ink-muted ticks, plan dashed `5 4` ink-muted 2px, actual brand 2.5px with r=2.5
 * dots, legend 11.5px, the expected-end line tangerine dashed `4 4` with its 10.5px label.
 *
 * DRAWN IN PIXELS, NOT SCALED: the SVG's coordinate width is the box's MEASURED width (a
 * `ResizeObserver`), so a tick written at 11px stays 11px at any card width. A fixed `viewBox` with
 * `w-full` scaled every label up with the card — 11px became ~16px on a wide screen (render r1).
 * Before the first measure (server render, jsdom) it draws at `DEFAULT_WIDTH`.
 *
 * EVERY POINT IS THE SERVER'S — this file scales and draws, it never adds money up.
 *
 * THE ACTUAL LINE STOPS AT A `null` MONTH: the server sends `null` for a month that has not begun, and
 * drawing it flat to December would read as a forecast nobody made (`disbursement_summary.go`). A gap
 * breaks the line rather than bridging it.
 *
 * BOTH LINES ARE MONOTONE CUBICS (ADR 0081 #4, prototype `CumulativeChart.tsx:101-117` `type="monotone"`):
 * smooth like the prototype, yet never dipping below a figure already reached (`monotone-path.ts`).
 * Each actual run is curved on its own, so a gap still breaks the line and it still stops at the
 * current month — the curve smooths what was paid, it does not extend it.
 *
 * The SVG is `aria-hidden`; a visually hidden table under it carries the same figures, in full đồng,
 * for a screen reader.
 *
 * READING ONE MONTH (prototype `CumulativeChart.tsx:71-86`): an HTML overlay above the SVG holds one
 * hover column per month and is itself ONE tab stop — arrows / Home / End move between months, Escape
 * hides. The tooltip is React text in the prototype's shape (`T3`, `Kế hoạch: …`, `Thực hiện: …`), its
 * figures the full-đồng strings of the hidden table, never the axis's rounded `8,5 tỷ`.
 *
 * `Hoàn thành dự kiến` (prototype `:88-100`): a dashed vertical line at the month the SERVER names.
 * A month not among the drawn points draws nothing. At 320px the chart scrolls sideways inside its card
 * (`min-w`), like every table of this app.
 */

const DEFAULT_WIDTH = 640;
/** Narrowest drawing width; below it the frame scrolls sideways instead of crushing the months. */
const MIN_WIDTH = 480;
/** Spec 02 §4: `margin {top:8,right:12,bottom:0,left:4}`, Y axis `width={64}`, X ticks under the plot. */
const PAD = { left: 68, right: 12, top: 8, bottom: 26 } as const;

/** vi-VN pinned, ≤ 1 decimal, as the spec's `formatDongShort` prints the axis: `8,5 tỷ`. */
const AXIS_NUMBER = new Intl.NumberFormat("vi-VN", { maximumFractionDigits: 1 });

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

/** `0 đ` · `500 triệu` · `8,5 tỷ` — the axis's short form (spec `formatDongShort`). */
export function axisMoneyLabel(dong: number): string {
  const abs = Math.abs(dong);
  if (abs === 0) return "0 đ";
  if (abs >= 1e9) return `${AXIS_NUMBER.format(dong / 1e9)} tỷ`;
  if (abs >= 1e6) return `${AXIS_NUMBER.format(dong / 1e6)} triệu`;
  return `${AXIS_NUMBER.format(dong)} đ`;
}

/**
 * Runs of consecutive months that HAVE an actual figure — one path each. Exported for the test
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

/** The frame's measured width, `DEFAULT_WIDTH` until measured (and wherever `ResizeObserver` is absent). */
function useMeasuredWidth(): [RefObject<HTMLDivElement | null>, number] {
  const ref = useRef<HTMLDivElement>(null);
  const [width, setWidth] = useState(DEFAULT_WIDTH);
  useEffect(() => {
    const el = ref.current;
    if (el === null || typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver((entries) => {
      const w = entries[0]?.contentRect.width ?? 0;
      if (w > 0) setWidth(Math.max(MIN_WIDTH, Math.round(w)));
    });
    observer.observe(el);
    return () => observer.disconnect();
  }, []);
  return [ref, width];
}

export function CumulativeChart({
  points,
  caption,
  height = 260,
  emptyText,
  expectedEndMonth,
}: {
  points: readonly finance_curvePointOut[];
  /** Names the chart for the hidden table, e.g. "Luỹ kế giải ngân năm 2026 so với kế hoạch". */
  caption: string;
  /** Spec 02 §4: 260 on the list; 230 on the project's `Biểu đồ` tab (spec 07). */
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
  const [frame, width] = useMeasuredWidth();
  const { top, ticks } = chartScale(points);
  if (points.length === 0 || top === 0) {
    return <p className="text-ink-muted m-0 text-[12.5px]">{emptyText}</p>;
  }

  const plotW = width - PAD.left - PAD.right;
  const plotH = height - PAD.top - PAD.bottom;
  const x = (i: number) => (points.length === 1 ? PAD.left + plotW / 2 : PAD.left + (i * plotW) / (points.length - 1));
  const y = (v: number) => PAD.top + plotH - (v / top) * plotH;
  const plan = monotoneXPath(points.map((p, i) => ({ x: x(i), y: y(p.planned_cumulative) })));
  const segments = actualSegments(points);
  const last = points.length - 1;
  const endIndex =
    expectedEndMonth === undefined || expectedEndMonth === null
      ? -1
      : points.findIndex((p) => p.month === expectedEndMonth);
  // Share of the drawing's width, so the HTML overlay lines up with the SVG.
  const pct = (px: number) => `${(px / width) * 100}%`;
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
        <div ref={frame} className="relative" style={{ minWidth: MIN_WIDTH }}>
          <svg
            width={width}
            height={height}
            viewBox={`0 0 ${width} ${height}`}
            className="block max-w-none"
            aria-hidden="true"
            focusable="false"
          >
            {ticks.map((t) => (
              <g key={t}>
                <line
                  x1={PAD.left}
                  x2={width - PAD.right}
                  y1={y(t)}
                  y2={y(t)}
                  className="stroke-line"
                  strokeWidth={1}
                  strokeDasharray="3 3"
                  data-grid=""
                />
                <text x={PAD.left - 8} y={y(t)} textAnchor="end" dominantBaseline="middle" fontSize={11} className="fill-ink-muted">
                  {axisMoneyLabel(t)}
                </text>
              </g>
            ))}
            {/* Vertical grid lines too, as recharts' `CartesianGrid` draws them. */}
            {points.map((p, i) => (
              <line
                key={`v-${p.month}`}
                x1={x(i)}
                x2={x(i)}
                y1={PAD.top}
                y2={PAD.top + plotH}
                className="stroke-line"
                strokeWidth={1}
                strokeDasharray="3 3"
                data-grid=""
              />
            ))}
            {/* X axis line, `axisLine={{stroke: line}}`, no tick marks. */}
            <line x1={PAD.left} x2={width - PAD.right} y1={PAD.top + plotH} y2={PAD.top + plotH} className="stroke-line" strokeWidth={1} />
            {points.map((p, i) => (
              <text key={p.month} x={x(i)} y={height - 8} textAnchor="middle" fontSize={11} className="fill-ink-muted">
                T{p.month}
              </text>
            ))}
            <path d={plan} fill="none" className="stroke-ink-muted" strokeWidth={2} strokeDasharray="5 4" data-line="plan" />
            {segments.map((run) => (
              <path
                key={`line-${run[0]!.index}`}
                d={monotoneXPath(run.map((r) => ({ x: x(r.index), y: y(r.value) })))}
                fill="none"
                className="stroke-brand"
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
                  className="stroke-tangerine"
                  strokeWidth={1}
                  strokeDasharray="4 4"
                  data-expected-end=""
                />
                {/* Left of the line as the prototype's `insideTopRight`, unless that would leave the frame. */}
                <text
                  x={endIndex === 0 ? x(endIndex) + 4 : x(endIndex) - 4}
                  y={PAD.top + 10}
                  textAnchor={endIndex === 0 ? "start" : "end"}
                  fontSize={10.5}
                  className="fill-tangerine"
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
                className="stroke-ink-muted"
                strokeWidth={1}
                data-active-month={shown.month}
              />
            )}
            {segments.flat().map((r) => (
              <circle
                key={`dot-${r.index}`}
                cx={x(r.index)}
                cy={y(r.value)}
                r={2.5}
                className="fill-white stroke-brand"
                strokeWidth={2}
                data-dot=""
              />
            ))}
          </svg>
          <div
            role="group"
            tabIndex={0}
            aria-label={`${caption}. Dùng phím mũi tên trái, phải để xem số liệu từng tháng.`}
            aria-describedby={shown !== undefined ? tooltipId : undefined}
            className="absolute inset-0 rounded-md focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand"
            onFocus={() => show(cursor)}
            onBlur={() => setOpen(false)}
            onKeyDown={onKeyDown}
            onMouseLeave={() => setOpen(false)}
            data-chart-focus=""
          >
            {points.map((p, i) => {
              // Each month owns the band halfway to its neighbours, so any x over the plot reads a month.
              const from = i === 0 ? PAD.left : (x(i - 1) + x(i)) / 2;
              const to = i === last ? width - PAD.right : (x(i) + x(i + 1)) / 2;
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
            // Spec 02 §4 tooltip: `borderRadius:10, border: 1px line, fontSize:12`, full đồng.
            <div
              id={tooltipId}
              role="tooltip"
              className="border-line text-ink pointer-events-none absolute top-1 z-10 rounded-[10px] border border-solid bg-white px-2.5 py-1.5 text-[12px]"
              style={tooltipPosition(x(Math.min(cursor, last)) / width)}
              data-chart-tooltip=""
            >
              <p className="text-navy m-0 font-semibold">T{shown.month}</p>
              <p className="text-ink-muted m-0">Kế hoạch: {nhanTien(shown.planned_cumulative)}</p>
              <p className="text-brand m-0">
                Thực hiện:{" "}
                {shown.disbursed_cumulative === null ? "Chưa đến tháng này" : nhanTien(shown.disbursed_cumulative)}
              </p>
            </div>
          )}
        </div>
      </div>
      {/* Spec 02 §4 `Legend wrapperStyle={{fontSize:11.5}}`, centred under the plot as recharts puts it. */}
      <figcaption className="text-ink-muted mt-2 flex flex-wrap items-center justify-center gap-x-4 gap-y-1 text-[11.5px]">
        <span className="inline-flex items-center gap-1.5">
          <svg width="24" height="8" aria-hidden="true" focusable="false">
            <line x1="0" x2="24" y1="4" y2="4" className="stroke-ink-muted" strokeWidth={2} strokeDasharray="5 4" />
          </svg>
          Kế hoạch
        </span>
        <span className="inline-flex items-center gap-1.5">
          <svg width="24" height="8" aria-hidden="true" focusable="false">
            <line x1="0" x2="24" y1="4" y2="4" className="stroke-brand" strokeWidth={2.5} />
            <circle cx="12" cy="4" r="2.5" className="fill-white stroke-brand" strokeWidth={2} />
          </svg>
          Thực hiện
        </span>
      </figcaption>
      {/* The visually-hidden class sits on a WRAPPING DIV, never on the <table>: a table ignores
          `height: 1px` and `overflow: hidden`, so the absolutely positioned table kept its full twelve
          rows (~340px) and pushed the page's scroll height past the body — a blank band under the
          `Biểu đồ` tab, and the sticky sidebar scrolling away once the page outgrew its grid (brief
          §3.2, measured 08/10/2026: scrollHeight 1253 vs body 909). */}
      <div className="an-thi-giac" data-chart-table="">
        <table>
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
      </div>
    </figure>
  );
}
