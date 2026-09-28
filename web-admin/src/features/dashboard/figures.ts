/**
 * The decisions of the leadership overview, as pure functions: which figure shows what, when a
 * comparison is said, where a figure leads, and how the three "Cần xử lý ngay" queues merge.
 *
 * NO BUSINESS ARITHMETIC OF ITS OWN. Every count comes from the owning service. The one ratio here
 * (`on_time / on_time_sample`) is the division the server documents for its own two fields
 * (`service-petitions/internal/http/summary.go` `taskSummaryOut`), and it has no value when the
 * sample is 0 — never 0%.
 *
 * NOTHING HERE COMPUTES HOW LATE ANYTHING IS. Spec §5 draws `quá hạn {N} ngày {M} giờ`; that is
 * wall-clock arithmetic on a deadline, which rule 10 forbids outside identity (working hours, ADR
 * 0007). A row says the deadline it missed and whether the server judged it critical.
 */

import { LINH_VUC_PHAN_ANH } from "@/features/phan-anh/nhan-phieu";
import type { KetQua } from "@/lib/api/goi";
import type { documents_overdueQueueOut, petitions_overdueQueueOut } from "@/lib/api/schema.gen";
import { isPeriodMetric, parseDrillDown } from "@/lib/drill-down";
import type { CitizenReportMetric, IncomingDocumentMetric, TaskMetric } from "@/lib/drill-down";
import { coQuyen, REPORT_READ_PERMISSION } from "@/lib/quyen";

import { formatDateTime } from "./period";

/** A figure that has no value — failed call, empty sample. NEVER `0`. */
export const NO_VALUE = "—";

/** One muted line for a block or cell with no source data in wave 1. Never 0, never invented. */
export const NO_SOURCE_DATA = "Chưa có dữ liệu nguồn";

/** Spec §8, verbatim. */
export const NOTHING_URGENT = "Không có việc nào cần xử lý ngay.";

const COUNT_FORMAT = new Intl.NumberFormat("vi-VN", { maximumFractionDigits: 0 });
const ONE_DECIMAL = new Intl.NumberFormat("vi-VN", {
  minimumFractionDigits: 1,
  maximumFractionDigits: 1,
});

/** `1.234` — vi-VN grouping. */
export function formatCount(n: number): string {
  return COUNT_FORMAT.format(n);
}

/**
 * `on_time / sample` as a percentage, or `null` when the sample is 0.
 *
 * `null` IS NOT 0%: a period in which no task with a deadline was completed has no on-time rate.
 * Printing 0% there reports the commune as the worst in the province for doing nothing wrong.
 */
export function ratioPercent(numerator: number, sample: number): number | null {
  if (!(sample > 0)) return null;
  return (numerator / sample) * 100;
}

/** `33,3%`, or `—` for no value. */
export function formatPercent(p: number | null): string {
  return p === null ? NO_VALUE : `${ONE_DECIMAL.format(p)}%`;
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * COMPARISON WITH THE PREVIOUS PERIOD
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * STOCK figures describe the register NOW (`in_progress`, `overdue`, `suspended`, `open`): the
 * period does not move them, so a "previous period" of a stock figure does not exist and no line is
 * drawn — not even "chưa có kỳ trước". PERIOD figures count what happened inside `[from, to)`.
 */
export type FigureKind = "stock" | "period";

/**
 * Which direction is good. `neutral` for volumes whose rise is neither good nor bad — more citizen
 * reports received, more documents arrived — so they are never painted red or green.
 */
export type Trend = "higher-is-better" | "lower-is-better" | "neutral";

export type Tone = "better" | "worse" | "neutral";

export type ComparisonLine = { readonly text: string; readonly tone: Tone };

/**
 * The comparison line under a figure, or `null` for none.
 *
 * The change is a percentage ONLY when the previous value is neither missing nor 0 (user decision):
 * from 0 to 3 is not "+∞%", and from a failed call it is nothing at all. In both cases the line
 * states the previous value instead, so the reader still has the number.
 */
export function comparisonLine(
  kind: FigureKind,
  current: number | null,
  previous: number | null,
  trend: Trend,
  format: (n: number) => string,
): ComparisonLine | null {
  if (kind === "stock") return null;
  if (current === null) return null;
  if (previous === null) return { text: `Kỳ trước: ${NO_VALUE}`, tone: "neutral" };
  if (previous === 0) return { text: `Kỳ trước: ${format(0)}`, tone: "neutral" };
  if (current === previous) return { text: "Không đổi so với kỳ trước", tone: "neutral" };

  const change = ((current - previous) / previous) * 100;
  const up = change > 0;
  const text = `${up ? "↑ +" : "↓ -"}${ONE_DECIMAL.format(Math.abs(change))}% so với kỳ trước`;
  const tone: Tone =
    trend === "neutral"
      ? "neutral"
      : (trend === "higher-is-better") === up
        ? "better"
        : "worse";
  return { text, tone };
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * DRILL-DOWN — every figure leads to the list behind it (spec §9)
 *
 * THE RECEIVING SIDE OWNS THE CONTRACT: `lib/drill-down.ts` parses `/nhiem-vu?metric=<m>[&from&to]`,
 * `/phan-anh?…` and `/van-ban?…`, holds the metric table (label, stock or period), and is imported
 * here rather than copied — two tables of "which metric takes a period" drift, and the drifted one
 * opens a whole register under a heading that names one figure. It parses strictly: a period on a
 * stock metric, a missing period on a period metric, or an unencoded `+` all read as invalid.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export type DrillTarget =
  | { readonly list: "tasks"; readonly metric: TaskMetric }
  | { readonly list: "citizen-reports"; readonly metric: CitizenReportMetric }
  | { readonly list: "incoming-documents"; readonly metric: IncomingDocumentMetric };

const LIST_PATH: Readonly<Record<DrillTarget["list"], string>> = {
  tasks: "/nhiem-vu",
  "citizen-reports": "/phan-anh",
  "incoming-documents": "/van-ban",
};

/** Is this a period figure — answered by the receiving side's table. */
export function isPeriodTarget(target: DrillTarget): boolean {
  switch (target.list) {
    case "tasks":
      return isPeriodMetric("tasks", target.metric);
    case "citizen-reports":
      return isPeriodMetric("citizen-reports", target.metric);
    case "incoming-documents":
      return isPeriodMetric("incoming-documents", target.metric);
  }
}

/**
 * The list behind a figure, with the same metric and — for a period figure only — the same
 * `[from, to)` the figure counted. `URLSearchParams` encodes the `+` of `+07:00` as `%2B`; left raw
 * it would arrive as a space and the receiver would call the link invalid.
 */
export function drillHref(target: DrillTarget, period: { from: string; to: string }): string {
  const query = new URLSearchParams();
  query.set("metric", target.metric);
  if (isPeriodTarget(target)) {
    query.set("from", period.from);
    query.set("to", period.to);
  }
  return `${LIST_PATH[target.list]}?${query.toString()}`;
}

/**
 * How the receiving list names a metric — read back through `parseDrillDown` on the very link the
 * figure carries, so a count cell's label IS the heading of the list it opens, from one table.
 *
 * `null` means the link would be invalid on arrival — a defect the tests make red, never a state.
 */
export function drillTargetLabel(
  target: DrillTarget,
  period: { from: string; to: string },
): string | null {
  const href = drillHref(target, period);
  const params = Object.fromEntries(new URLSearchParams(href.slice(href.indexOf("?") + 1)));
  const parsed = parseDrillDown(target.list, params);
  return parsed.kind === "active" ? parsed.label : null;
}

/** The fiscal figures lead to the existing Thu - Chi screen; it has no metric filter. */
export const FISCAL_SCREEN_PATH = "/giai-ngan/thu-chi";

/** Spec §4 / §9, verbatim: every figure is a control with this name. */
export function drillLabel(label: string): string {
  return `Xem danh sách đằng sau: ${label}`;
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * ACCESS — a block is shown only when BOTH keys its routes check are held
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * `report.read` AND the module key. Composed here rather than widening `CongQuyen`, whose single-key
 * signature is deliberate (`quyetDinhTheoKhoa`). Convenience only: the server checks both keys.
 */
export function canSeeBlock(permissions: readonly string[], moduleKey: string): boolean {
  return coQuyen(permissions, REPORT_READ_PERMISSION) && coQuyen(permissions, moduleKey);
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * "CẦN XỬ LÝ NGAY" — three queues, two row shapes, one list
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export type QueueModule = "task" | "citizen-report" | "incoming-document";

/**
 * One merged row. EXACTLY what a row may show (user decision; rule 3): no title, no content, no
 * holding unit, no person. The document queue's `id` and `holding_unit` are dropped at the merge —
 * a field that is not carried cannot be rendered by a later edit.
 */
export type UrgentRow = {
  readonly module: QueueModule;
  readonly code: string;
  readonly kind: string;
  /** Task type or petition field code; `""` = unclassified petition; `null` = none (documents). */
  readonly categoryCode: string | null;
  /** RFC 3339 — the stored deadline that passed. */
  readonly missedAt: string;
  readonly critical: boolean;
};

export type QueueSource =
  | { readonly module: "task" | "citizen-report"; readonly result: KetQua<petitions_overdueQueueOut> }
  | { readonly module: "incoming-document"; readonly result: KetQua<documents_overdueQueueOut> };

export type QueueFailure = { readonly module: QueueModule; readonly message: string };

export type MergedQueue = {
  readonly rows: readonly UrgentRow[];
  /** One per failed queue, with the server's sentence. A 503 lands here — never as "nothing". */
  readonly failures: readonly QueueFailure[];
};

/** Rows kept after the merge — spec §5 "tối đa 10 mục". */
export const URGENT_ROW_CAP = 10;

const MODULE_ORDER: Readonly<Record<QueueModule, number>> = {
  task: 0,
  "citizen-report": 1,
  "incoming-document": 2,
};

function instantOf(rfc3339: string): number {
  const t = Date.parse(rfc3339);
  // An unreadable deadline sorts LAST rather than first: it cannot be claimed to be the oldest.
  return Number.isNaN(t) ? Number.POSITIVE_INFINITY : t;
}

/**
 * Merge, sort by the missed deadline OLDEST FIRST (the longest overdue on top), keep 10.
 *
 * Ties break by module then code so the order is stable between two loads of the same data.
 */
export function mergeQueues(sources: readonly QueueSource[]): MergedQueue {
  const rows: UrgentRow[] = [];
  const failures: QueueFailure[] = [];

  for (const s of sources) {
    if (!s.result.ok) {
      failures.push({ module: s.module, message: s.result.thongBao });
      continue;
    }
    if (s.module === "incoming-document") {
      for (const it of s.result.duLieu.items) {
        rows.push({
          module: s.module,
          code: it.code,
          kind: it.kind,
          categoryCode: null,
          missedAt: it.due_at,
          critical: it.critical,
        });
      }
    } else {
      for (const it of s.result.duLieu.items) {
        rows.push({
          module: s.module,
          code: it.code,
          kind: it.kind,
          categoryCode: it.category_code,
          missedAt: it.missed_deadline,
          critical: it.critical,
        });
      }
    }
  }

  rows.sort(
    (a, b) =>
      instantOf(a.missedAt) - instantOf(b.missedAt) ||
      MODULE_ORDER[a.module] - MODULE_ORDER[b.module] ||
      (a.code < b.code ? -1 : a.code > b.code ? 1 : 0),
  );

  return { rows: rows.slice(0, URGENT_ROW_CAP), failures };
}

export const MODULE_LABEL: Readonly<Record<QueueModule, string>> = {
  task: "Nhiệm vụ",
  "citizen-report": "Phản ánh",
  "incoming-document": "Văn bản đến",
};

/**
 * Which deadline was missed. The kind values are ADR 0011 enum values from the servers
 * (`summary.go` `overdueItemOut.Kind`, `incoming_dashboard.go` `overdueQueueKind`).
 */
const KIND_LABEL: Readonly<Record<string, string>> = {
  "han-xu-ly": "Hạn xử lý",
  "han-phan-loai": "Hạn phân loại",
  "han-xu-ly-xong": "Hạn xử lý xong",
  "van-ban-den": "Hạn xử lý",
};

/** An unknown kind shows the raw code and says so, rather than guessing a label. */
export function kindLabel(kind: string): string {
  return KIND_LABEL[kind] ?? `${kind} (loại hạn chưa có nhãn trên màn hình này)`;
}

/**
 * The field / category label of a row, or `null` for none.
 *
 * Tasks: resolved against the commune's task-type catalogue when it loaded; otherwise the code.
 * Petitions: the platform's closed field list (`LINH_VUC_PHAN_ANH`, ADR 0026); `""` is
 * "Chưa phân loại". Documents carry no category.
 */
export function categoryLabel(
  row: UrgentRow,
  taskTypeLabels: ReadonlyMap<string, string> | null,
): string | null {
  if (row.categoryCode === null) return null;
  if (row.module === "citizen-report") {
    if (row.categoryCode === "") return "Chưa phân loại";
    return LINH_VUC_PHAN_ANH.find((l) => l.ma === row.categoryCode)?.nhan ?? row.categoryCode;
  }
  if (row.categoryCode === "") return null;
  return taskTypeLabels?.get(row.categoryCode) ?? row.categoryCode;
}

/** `quá hạn từ 08:00 01/09/2026` — the deadline itself, never a duration since it. */
export function missedSinceText(missedAt: string): string {
  const t = Date.parse(missedAt);
  return Number.isNaN(t) ? `quá hạn từ ${missedAt}` : `quá hạn từ ${formatDateTime(t)}`;
}
