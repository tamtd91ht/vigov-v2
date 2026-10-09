/**
 * What `/bao-cao`'s PDF / XLSX / PPTX contain — Tổng quan's export shape (`features/dashboard/
 * export-model.ts`) filled with THIS page's figures, owner decision 09/10/2026 (ADR 0053 §Sửa đổi
 * 09/10/2026 lần 2, D3). Pure: no library, no DOM, no clock of its own.
 *
 * FROM THE FIGURES ALREADY ON SCREEN, nothing recomputed: the blocks go through `exportBlocks(…,
 * "report")` — the very `Figure` objects the tiles draw — the unit table is the rows the table draws,
 * and the comparison is the chart's own rows written as a table (a file carries numbers, not bars).
 *
 * AGGREGATES ONLY (rule 3): counts per block, per organisational unit, per figure. No record, no
 * person, no code. The unit names are the commune's org-unit catalogue.
 *
 * THE COMMUNE NAME IS THE RUNTIME CONFIGURATION resolved from `Host` (rule 1 inv. 10) — no name, no
 * file: `buildReportExport` throws and the click says "Không xuất được báo cáo."
 *
 * NOT AUDITED — the debt Tổng quan's export already carries (`export-actions.tsx`): no server route
 * records a staff "export" event, so rule 3 invariant 4 ("a full export is audited") cannot be met
 * from the browser. These files are aggregate-only, which keeps them out of that invariant's reach,
 * but "who exported which report when" is still unanswerable until such a route exists.
 */

import { formatCount, NO_VALUE } from "@/features/dashboard/figures";
import { EXPORT_SOURCE_NOTE } from "@/features/dashboard/export-model";
import type { DashboardExport, ExportCommune, ExportTable } from "@/features/dashboard/export-model";
import { formatDateTime, toRfc3339 } from "@/features/dashboard/period";
import type { PeriodKind } from "@/features/dashboard/period";
import { comparedFigures, exportBlocks } from "@/features/dashboard/view";
import type { BlocksData, Loaded } from "@/features/dashboard/view";

import { COMPARISON_EMPTY, COMPARISON_TITLE, barTone, signedPercent } from "./comparison-chart";
import type { ReportAccess } from "./report-access";
import { CUSTOM_PERIOD, reportComparisonNote, reportPeriodLabel } from "./report-period";
import type { ReportWindows } from "./report-period";
import { onTimeCell, UNIT_COLUMNS, UNIT_TABLE_EMPTY, UNIT_TABLE_NOTE, UNIT_TABLE_TITLE } from "./unit-table";
import type { UnitRow } from "./unit-table";

export const REPORT_TITLE = "Báo cáo điều hành";

/** Where the file came from — Tổng quan's sentence, naming this page. */
export const REPORT_EXPORT_SOURCE_NOTE = EXPORT_SOURCE_NOTE.replace("Tổng quan điều hành", REPORT_TITLE);

const PERIOD_SLUG: Readonly<Record<PeriodKind, string>> = {
  week: "tuan",
  month: "thang",
  quarter: "quy",
  year: "nam",
};

const day = (instant: number) => toRfc3339(instant).slice(0, 10).replaceAll("-", "");

/**
 * `bao-cao-thang-20261001` · `bao-cao-tuy-chon-20260901-20260917` — kind + the period's days in Viet
 * Nam. ASCII only; no commune name and no personal data.
 */
export function reportFileStem(w: ReportWindows): string {
  if (w.kind === CUSTOM_PERIOD) return `bao-cao-tuy-chon-${day(w.period.start)}-${day(w.period.end - 1)}`;
  return `bao-cao-${PERIOD_SLUG[w.kind]}-${day(w.period.start)}`;
}

/** The unit table as the screen draws it — or its error sentence, or nothing while it loads. */
function unitTable(units: Loaded<UnitRow[]>): ExportTable {
  const columns = [UNIT_COLUMNS.unit, UNIT_COLUMNS.total, UNIT_COLUMNS.overdue, UNIT_COLUMNS.completed, UNIT_COLUMNS.onTime];
  if (units === null || !units.ok) {
    return {
      title: UNIT_TABLE_TITLE,
      columns,
      rows: [],
      empty: UNIT_TABLE_EMPTY,
      notes: [UNIT_TABLE_NOTE],
      errors: units === null ? [] : [`Chưa tải được bảng theo bộ phận: ${units.thongBao}`],
    };
  }
  return {
    title: UNIT_TABLE_TITLE,
    columns,
    rows: units.duLieu.map((r) => [
      { text: r.name },
      { text: formatCount(r.total), number: r.total },
      { text: formatCount(r.overdue), number: r.overdue },
      { text: formatCount(r.completed), number: r.completed },
      { text: onTimeCell(r)?.text ?? NO_VALUE },
    ]),
    empty: UNIT_TABLE_EMPTY,
    notes: [UNIT_TABLE_NOTE],
    errors: [],
  };
}

const TONE_WORD = { good: "Tốt lên", bad: "Xấu đi", neutral: "—" } as const;

function comparisonTable(data: BlocksData, access: ReportAccess): ExportTable {
  const rows = comparedFigures(data, access.blocks) ?? [];
  return {
    title: COMPARISON_TITLE,
    columns: ["Chỉ số", "Kỳ này", "Kỳ trước", "Thay đổi", "Đánh giá"],
    rows: rows.map((r) => [
      { text: r.label },
      { text: r.currentText },
      { text: r.previousText },
      { text: signedPercent(r.delta) },
      { text: TONE_WORD[barTone(r)] },
    ]),
    empty: COMPARISON_EMPTY,
    notes: [],
    errors: [],
  };
}

/** Does the page show a compared block — i.e. does the comparison card exist at all? */
export function hasComparedBlock(access: ReportAccess): boolean {
  return access.blocks.tasks || access.blocks.incomingDocuments || access.blocks.citizenReports;
}

export function buildReportExport(input: {
  commune: ExportCommune;
  windows: ReportWindows;
  fetchedAt: number;
  generatedAt: number;
  figures: BlocksData;
  access: ReportAccess;
  units: Loaded<UnitRow[]>;
}): DashboardExport {
  // Fail closed: a file printed under no commune name is a file nobody can attribute.
  if (input.commune.displayName.trim() === "") {
    throw new Error("buildReportExport: no commune name in the runtime configuration");
  }
  const compared = hasComparedBlock(input.access);
  return {
    communeName: input.commune.displayName,
    parentAuthority: input.commune.parentAuthority,
    title: REPORT_TITLE,
    periodLabel: reportPeriodLabel(input.windows),
    asOfLine: `Số liệu tính đến ${formatDateTime(input.fetchedAt)}`,
    generatedLine: `Xuất lúc ${formatDateTime(input.generatedAt)}`,
    blocks: exportBlocks(input.figures, input.access.blocks, "report"),
    tables: [
      // A table the account may not see on screen is not in the file either.
      ...(input.access.unitTable ? [unitTable(input.units)] : []),
      ...(compared ? [comparisonTable(input.figures, input.access)] : []),
    ],
    urgent: null,
    footnotes: [...(compared ? [reportComparisonNote(input.windows)] : []), REPORT_EXPORT_SOURCE_NOTE],
    fileStem: reportFileStem(input.windows),
  };
}
