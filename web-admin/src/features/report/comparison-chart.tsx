"use client";

import { useEffect, useId, useRef, useState } from "react";
import type { ReactNode } from "react";

import { formatPercent } from "@/features/dashboard/figures";
import type { ComparedFigure } from "@/features/dashboard/view";

/**
 * "So sánh với kỳ trước" — the prototype's horizontal bar chart (`ReportWorkspace.tsx`
 * `ComparisonChart`), owner decision 09/10/2026 (ADR 0053 §Sửa đổi 09/10/2026 lần 2, D2).
 *
 * HAND-DRAWN SVG, NOT recharts (ADR 0068 §4 / lần 6 #11): no chart library ships in this app, and one
 * chart does not justify one. The geometry follows the prototype's recharts call: one bar per figure,
 * `max(220, 30 · n)` tall, a ~168px label column in 11px ink, a % axis in muted 11px, dashed vertical
 * grid, bars rounded at their outer end, 10% gap above and below each bar.
 *
 * COLOUR BY DIRECTION, NOT BY SIGN — the figure's own `trend` (`view.tsx`, the same one its tile uses):
 *   higher-is-better  Hoàn thành trong kỳ · Đúng hạn trong kỳ (nhiệm vụ) · Đúng hạn trong kỳ (phản ánh)
 *   lower-is-better   Trễ hạn trong kỳ
 *   neutral           Đến trong kỳ (văn bản) · Tiếp nhận trong kỳ (phản ánh)
 * A NEUTRAL bar is grey, not green or red: more letters arriving or more petitions received is neither
 * good nor bad, and the tile right above it already says so in muted text. The prototype coloured those
 * two as "higher is better" — painting a rise in complaints green is the one thing this chart must not
 * say on a public authority's screen.
 *
 * COLOUR IS NEVER ALONE: the sign is in the tooltip (`<title>`), and a visually hidden table under the
 * chart reads each figure, both periods, the change and "tốt lên / xấu đi" in words.
 *
 * WIDTH IS MEASURED (`ResizeObserver`), so text stays 11px at every width instead of scaling with a
 * viewBox; until the first measure (and in a test) it draws at `DEFAULT_WIDTH`, scaled down by CSS.
 */

export const COMPARISON_TITLE = "So sánh với kỳ trước";
export const COMPARISON_EMPTY = "Chưa có kỳ trước để so sánh.";

const DEFAULT_WIDTH = 640;
const MARGIN = { top: 4, right: 24, bottom: 4, left: 8 } as const;
/** Height of the % axis under the plot: tick + 11px label. */
const AXIS_H = 22;
const LABEL_W = 168;
/** Average glyph width of 11px Roboto, to cut a label that would not fit its column. */
const CHAR_W = 5.9;

export type BarTone = "good" | "bad" | "neutral";

/** Which colour a bar takes: its direction against the figure's trend, never its sign alone. */
export function barTone(row: Pick<ComparedFigure, "delta" | "trend">): BarTone {
  if (row.trend === "neutral" || Math.abs(row.delta) < 0.05) return "neutral";
  return (row.delta > 0) === (row.trend === "higher-is-better") ? "good" : "bad";
}

const TONE_FILL: Readonly<Record<BarTone, string>> = {
  good: "fill-leaf",
  bad: "fill-danger",
  neutral: "fill-ink-400",
};

const TONE_WORD: Readonly<Record<BarTone, string>> = {
  good: "tốt lên",
  bad: "xấu đi",
  neutral: "không đánh giá tốt hay xấu",
};

/** `+12,5%` / `-3,2%` / `0,0%` — the change as the tooltip and the hidden table say it. */
export function signedPercent(delta: number): string {
  return `${delta > 0 ? "+" : ""}${formatPercent(delta)}`;
}

/**
 * Round tick values covering `[lo, hi]` (both include 0), about `count` of them — 1, 2, 2,5 or 5 times
 * a power of ten apart, as a chart library would choose.
 */
export function niceTicks(lo: number, hi: number, count = 5): number[] {
  let min = Math.min(0, lo);
  let max = Math.max(0, hi);
  if (max - min < 1e-9) max = min + 1;
  const rough = (max - min) / Math.max(1, count - 1);
  const mag = 10 ** Math.floor(Math.log10(rough));
  const norm = rough / mag;
  const step = (norm <= 1 ? 1 : norm <= 2 ? 2 : norm <= 2.5 ? 2.5 : norm <= 5 ? 5 : 10) * mag;
  min = Math.floor(min / step) * step;
  max = Math.ceil(max / step) * step;
  const ticks: number[] = [];
  for (let v = min; v <= max + step / 2; v += step) ticks.push(Math.round(v * 1e6) / 1e6);
  return ticks;
}

const TICK_FORMAT = new Intl.NumberFormat("vi-VN", { maximumFractionDigits: 1 });

export type ChartBar = {
  readonly row: ComparedFigure;
  readonly tone: BarTone;
  /** label as drawn — cut with "…" when longer than its column */
  readonly label: string;
  readonly labelY: number;
  readonly path: string;
};

export type ChartGeometry = {
  readonly width: number;
  readonly height: number;
  readonly plotLeft: number;
  readonly plotRight: number;
  readonly plotTop: number;
  readonly plotBottom: number;
  readonly labelRight: number;
  readonly ticks: readonly { readonly x: number; readonly text: string }[];
  readonly bars: readonly ChartBar[];
};

/** A bar from `x0` to `x1`, rounded `r` at its OUTER end (away from 0), square at the axis. */
function barPath(x0: number, x1: number, y: number, h: number, r: number): string {
  const len = Math.abs(x1 - x0);
  if (len < 0.5) return "";
  const k = Math.min(r, len, h / 2);
  if (x1 >= x0) {
    return `M${x0},${y}H${x1 - k}Q${x1},${y} ${x1},${y + k}V${y + h - k}Q${x1},${y + h} ${x1 - k},${y + h}H${x0}Z`;
  }
  return `M${x0},${y}H${x1 + k}Q${x1},${y} ${x1},${y + k}V${y + h - k}Q${x1},${y + h} ${x1 + k},${y + h}H${x0}Z`;
}

/** Everything drawn, as numbers — pure, so the geometry is tested without a browser. */
export function chartGeometry(rows: readonly ComparedFigure[], width: number): ChartGeometry {
  const n = rows.length;
  const height = Math.max(220, 30 * n);
  // Below ~480px the 168px label column would leave the bars no room: it shrinks to 40%.
  const labelW = width < 480 ? Math.max(96, Math.round(width * 0.4)) : LABEL_W;
  const plotLeft = MARGIN.left + labelW;
  const plotRight = Math.max(plotLeft + 40, width - MARGIN.right);
  const plotTop = MARGIN.top;
  const plotBottom = height - MARGIN.bottom - AXIS_H;
  const values = rows.map((r) => r.delta);
  const tickValues = niceTicks(Math.min(0, ...values), Math.max(0, ...values));
  const lo = tickValues[0] ?? 0;
  const hi = tickValues[tickValues.length - 1] ?? 1;
  const x = (v: number) => plotLeft + ((v - lo) / (hi - lo)) * (plotRight - plotLeft);
  const band = n === 0 ? 0 : (plotBottom - plotTop) / n;
  const maxChars = Math.max(6, Math.floor((labelW - 8) / CHAR_W));
  return {
    width,
    height,
    plotLeft,
    plotRight,
    plotTop,
    plotBottom,
    labelRight: plotLeft - 6,
    ticks: tickValues.map((v) => ({ x: x(v), text: `${TICK_FORMAT.format(v)}%` })),
    bars: rows.map((row, i) => {
      const y = plotTop + i * band + band * 0.1;
      const h = band * 0.8;
      return {
        row,
        tone: barTone(row),
        label: row.label.length > maxChars ? `${row.label.slice(0, maxChars - 1)}…` : row.label,
        labelY: plotTop + i * band + band / 2,
        path: barPath(x(0), x(row.delta), y, h, 4),
      };
    }),
  };
}

/**
 * The card body: the chart, or the prototype's empty sentence. `rows === null` = the figures are still
 * loading — a quiet line, never the empty sentence (which would claim there is no previous period).
 */
export function ComparisonChart({ rows }: { rows: readonly ComparedFigure[] | null }) {
  const ref = useRef<HTMLDivElement>(null);
  const [width, setWidth] = useState(DEFAULT_WIDTH);
  const titleId = useId();

  useEffect(() => {
    const el = ref.current;
    if (el === null || typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver((entries) => {
      const w = Math.round(entries[0]?.contentRect.width ?? 0);
      if (w > 0) setWidth(w);
    });
    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  let body: ReactNode;
  if (rows === null) {
    body = (
      <p role="status" className="m-0 py-8 text-center text-[13px] text-ink-muted">
        Đang tải…
      </p>
    );
  } else if (rows.length === 0) {
    body = (
      <p role="status" className="m-0 py-8 text-center text-[13px] text-ink-muted">
        {COMPARISON_EMPTY}
      </p>
    );
  } else {
    const g = chartGeometry(rows, width);
    body = (
      <>
        <svg
          role="img"
          aria-labelledby={titleId}
          width={g.width}
          height={g.height}
          viewBox={`0 0 ${g.width} ${g.height}`}
          className="block h-auto max-w-full"
          data-comparison-chart=""
        >
          <title id={titleId}>
            Biểu đồ thay đổi của từng chỉ số so với kỳ trước, tính theo phần trăm. Số liệu chi tiết ở bảng
            ngay sau biểu đồ.
          </title>
          {g.ticks.map((t) => (
            <line
              key={`grid-${t.x}`}
              x1={t.x}
              x2={t.x}
              y1={g.plotTop}
              y2={g.plotBottom}
              className="stroke-line"
              strokeDasharray="3 3"
            />
          ))}
          <line x1={g.plotLeft} x2={g.plotRight} y1={g.plotBottom} y2={g.plotBottom} className="stroke-ink-400" />
          {g.ticks.map((t) => (
            <g key={`tick-${t.x}`}>
              <line x1={t.x} x2={t.x} y1={g.plotBottom} y2={g.plotBottom + 5} className="stroke-ink-400" />
              <text x={t.x} y={g.plotBottom + 17} textAnchor="middle" fontSize={11} className="fill-ink-muted">
                {t.text}
              </text>
            </g>
          ))}
          {g.bars.map((b) => (
            <g key={b.row.id} data-tone={b.tone}>
              <title>{`${b.row.label}: ${signedPercent(b.row.delta)}`}</title>
              <text
                x={g.labelRight}
                y={b.labelY}
                textAnchor="end"
                dominantBaseline="central"
                fontSize={11}
                className="fill-ink"
              >
                {b.label}
              </text>
              {b.path !== "" && <path d={b.path} className={TONE_FILL[b.tone]} />}
            </g>
          ))}
        </svg>
        <table className="an-thi-giac">
          <caption>{COMPARISON_TITLE}</caption>
          <thead>
            <tr>
              <th scope="col">Chỉ số</th>
              <th scope="col">Kỳ này</th>
              <th scope="col">Kỳ trước</th>
              <th scope="col">Thay đổi</th>
              <th scope="col">Đánh giá</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((r) => (
              <tr key={r.id}>
                <th scope="row">{r.label}</th>
                <td>{r.currentText}</td>
                <td>{r.previousText}</td>
                <td>{signedPercent(r.delta)}</td>
                <td>{TONE_WORD[barTone(r)]}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </>
    );
  }

  return (
    <div ref={ref} className="w-full min-w-0">
      {body}
    </div>
  );
}
