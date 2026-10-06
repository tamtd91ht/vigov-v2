/**
 * What the Tổng quan export files contain — pure: no library, no DOM, no clock of its own.
 *
 * THE FILES ARE BUILT FROM THE FIGURES ALREADY ON SCREEN (user decision 06/10/2026), never from a
 * second computation. `view.tsx` `exportBlocks` turns the very `Figure` objects the tiles draw into
 * `ExportFigure`s, so the PDF, the workbook, the slides and the screen cannot show four numbers for one
 * period. Nothing here counts, divides or rounds a business figure.
 *
 * AGGREGATES ONLY (rule 3). The one block of the page that lists RECORDS — "Cần xử lý ngay" — goes into
 * the files as a count and a sentence, never as rows: a file sent upward leaves the system, and its
 * rows (record codes, deadlines) are not aggregates. The screen keeps the list.
 *
 * THE COMMUNE NAME COMES FROM THE RUNTIME PER-COMMUNE CONFIGURATION the page already holds
 * (`useCauHinhXa`, resolved server-side from `Host`) — never a constant, never the bundle (rule 1,
 * invariant 10). No name → no export: `overview.tsx` passes no source, and the buttons stay disabled.
 */

import { MODULE_LABEL, NOTHING_URGENT, URGENT_ROW_CAP } from "./figures";
import type { MergedQueue } from "./figures";
import { formatDateTime, periodMetaLabel, toRfc3339 } from "./period";
import type { PeriodKind, PeriodWindows } from "./period";

/** The three file kinds of spec §2's `[PDF][XLSX][PPTX]`. */
export type ExportFormat = "pdf" | "xlsx" | "pptx";

export const EXPORT_FORMATS: readonly ExportFormat[] = ["pdf", "xlsx", "pptx"];

export const EXPORT_MIME: Readonly<Record<ExportFormat, string>> = {
  pdf: "application/pdf",
  xlsx: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
  pptx: "application/vnd.openxmlformats-officedocument.presentationml.presentation",
};

/** Written in place of a figure the page does not have yet ("?" on screen). NEVER a number. */
export const EXPORT_NO_FIGURE = "Chưa có số liệu";

/** The sentence that replaces the "Cần xử lý ngay" rows in every file (rule 3 — see the header). */
export const URGENT_ROWS_OMITTED =
  "Danh sách từng việc (mã, hạn đã lỡ) không đưa vào tệp xuất; tệp chỉ mang số liệu tổng hợp. " +
  "Xem danh sách trên màn Tổng quan điều hành.";

/** One figure as the tile shows it. */
export type ExportFigure = {
  readonly label: string;
  /** The tile's text, verbatim — `—` for a failed call, a server sentence, `Chưa có số liệu`. */
  readonly value: string;
  /** The full amount when the tile shortened it (the tile's hover text). */
  readonly exact?: string;
  /**
   * The server's own number behind `value`, for the spreadsheet to sort and total — a count, or an
   * amount in đồng. Absent for ratios (the screen's text is the figure) and for anything not a number.
   */
  readonly number?: number;
  /** what `number` counts — a count of records, or đồng */
  readonly unit?: "count" | "dong";
  readonly note?: string;
  /** The tile's delta line, verbatim (`+12,5% so với kỳ trước`, `Kỳ trước: 0`). */
  readonly comparison?: string;
  /** red on screen (overdue / late, non-zero) */
  readonly alert?: boolean;
  /** an unbuilt or no-source figure: the value is a sentence, not a measurement */
  readonly noFigure?: boolean;
};

export type ExportBlock = {
  readonly title: string;
  readonly figures: readonly ExportFigure[];
  /** Scope sentences drawn under the block on screen ("Luỹ kế năm…", how the balance is computed). */
  readonly notes: readonly string[];
  /** The block's load errors, with the server's sentence — a failed call must not read as calm. */
  readonly errors: readonly string[];
  /** The block is an unbuilt part (ADR 0068 §14): every figure is `EXPORT_NO_FIGURE`; this says why. */
  readonly pending?: string;
};

export type ExportUrgent = {
  readonly title: string;
  readonly lines: readonly string[];
};

export type DashboardExport = {
  readonly communeName: string;
  /** "" = none declared; print nothing (`TenantConfig.parentAuthority`). */
  readonly parentAuthority: string;
  readonly title: string;
  /** `Kỳ tháng này: 1/10/2026 – 31/10/2026`, as the header line. */
  readonly periodLabel: string;
  /** `Số liệu tính đến 16:43 06/10/2026` — the page's own "tính đến". */
  readonly asOfLine: string;
  /** `Xuất lúc 16:45 06/10/2026` — the click, not the figures. */
  readonly generatedLine: string;
  readonly blocks: readonly ExportBlock[];
  readonly urgent: ExportUrgent | null;
  /** Page-wide notes: what "kỳ trước" means, where the file came from. */
  readonly footnotes: readonly string[];
  /** `tong-quan-thang-20261001` — no personal data, no commune name (task rule). */
  readonly fileStem: string;
};

/** The commune half of the header — the display part of the runtime configuration. */
export type ExportCommune = {
  readonly displayName: string;
  readonly parentAuthority: string;
};

const PERIOD_SLUG: Readonly<Record<PeriodKind, string>> = {
  week: "tuan",
  month: "thang",
  quarter: "quy",
  year: "nam",
};

/** `tong-quan-thang-20261001` — kind + the period's first day in Viet Nam. ASCII only. */
export function exportFileStem(windows: Pick<PeriodWindows, "kind" | "period">): string {
  const day = toRfc3339(windows.period.start).slice(0, 10).replaceAll("-", "");
  return `tong-quan-${PERIOD_SLUG[windows.kind]}-${day}`;
}

/**
 * "Cần xử lý ngay" as an aggregate: how many overdue items the merged list holds, and which queues
 * failed. The merged list is CAPPED at `URGENT_ROW_CAP`, so a full list says "từ 10 trở lên" — a
 * capped count written as an exact one would understate the backlog.
 */
export function urgentSummary(queue: MergedQueue): ExportUrgent {
  const lines: string[] = [];
  const n = queue.rows.length;
  if (n === 0 && queue.failures.length === 0) {
    lines.push(NOTHING_URGENT);
  } else if (n > 0) {
    const critical = queue.rows.filter((r) => r.critical).length;
    lines.push(
      n < URGENT_ROW_CAP
        ? `${n} việc quá hạn cần xử lý ngay${critical > 0 ? `, trong đó ${critical} việc nghiêm trọng` : ""}.`
        : `Từ ${URGENT_ROW_CAP} việc quá hạn trở lên cần xử lý ngay (màn hình liệt kê ${URGENT_ROW_CAP} việc quá hạn lâu nhất).`,
    );
    lines.push(URGENT_ROWS_OMITTED);
  }
  for (const f of queue.failures) {
    lines.push(`Chưa tải được danh sách ${MODULE_LABEL[f.module].toLowerCase()}: ${f.message}`);
  }
  return { title: "Cần xử lý ngay", lines };
}

/** The sentence every file ends with: where the figures came from. */
export const EXPORT_SOURCE_NOTE =
  "Tệp được tạo trên trình duyệt từ đúng số liệu đang hiện trên màn Tổng quan điều hành; " +
  "số liệu do các phân hệ sở hữu tính, tệp không tính lại.";

export function buildDashboardExport(input: {
  commune: ExportCommune;
  title: string;
  windows: PeriodWindows;
  fetchedAt: number;
  generatedAt: number;
  blocks: readonly ExportBlock[];
  urgent: ExportUrgent | null;
  comparisonNote: string | null;
}): DashboardExport {
  // Fail closed: a file printed under no commune name is a file nobody can attribute.
  if (input.commune.displayName.trim() === "") {
    throw new Error("buildDashboardExport: no commune name in the runtime configuration");
  }
  return {
    communeName: input.commune.displayName,
    parentAuthority: input.commune.parentAuthority,
    title: input.title,
    periodLabel: periodMetaLabel(input.windows),
    asOfLine: `Số liệu tính đến ${formatDateTime(input.fetchedAt)}`,
    generatedLine: `Xuất lúc ${formatDateTime(input.generatedAt)}`,
    blocks: input.blocks,
    urgent: input.urgent,
    footnotes: [...(input.comparisonNote === null ? [] : [input.comparisonNote]), EXPORT_SOURCE_NOTE],
    fileStem: exportFileStem(input.windows),
  };
}
