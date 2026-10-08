/**
 * "Nhập từ Excel" of the citizen-letter register — the pure half: the prototype's words
 * (`DocumentWorkspace.tsx:402-409` → `ExcelImportDialog` with `title`, `templateLabel`, `unit="đơn thư"`)
 * and the mapping of the server's preview onto the shared view (`TaskImportView`). The calls are
 * `lib/api/excel-import.ts` on `LETTER_IMPORT_ROUTES`.
 *
 * ALL OR NOTHING IS THE SERVER'S RULE (one transaction, numbers issued only on success). The dialog
 * offers "Nhập" only after a clean check of THE SAME file, and keeps one Idempotency-Key per attempt.
 */

import type { ImportPreview } from "@/lib/api/excel-import";
import type { LetterImportRow } from "@/lib/api/citizen-letter-import";
import type { petitions_taskImportResultOut } from "@/lib/api/schema.gen";

import { replayedText } from "@/features/nhiem-vu/task-import";

/** Prototype `DocumentWorkspace.tsx:406-408`, verbatim. */
export const LETTER_IMPORT_TITLE = "Nhập sổ đơn thư từ Excel";
export const LETTER_IMPORT_TEMPLATE_BUTTON = "Tải mẫu sổ đơn thư";
export const LETTER_IMPORT_UNIT = "đơn thư";
export const LETTER_IMPORT_FILE_INPUT_ID = "tep-nhap-don-thu";

/** A preview that is neither valid nor names a row: nothing to fix, nothing to import. Said, not guessed. */
export const LETTER_PREVIEW_UNEXPLAINED =
  "Máy chủ chưa nhận tệp này và không nêu dòng nào sai. Hãy chọn lại tệp rồi thử lại.";
/** A clean check with no letter in it. */
export const LETTER_IMPORT_EMPTY = "Tệp không có dòng nào để nhập.";

/** The shared view's report shape, from a preview. `total_rows` is known only for a clean preview. */
export type PreviewReport = petitions_taskImportResultOut;

/**
 * A preview → the report the view draws, or a sentence when there is nothing to draw. A REFUSED preview
 * carries no row count (the server stops at the errors), so its report has `total_rows: 0` and the
 * heading never claims a total.
 */
export function previewReport(preview: ImportPreview<LetterImportRow>): { report: PreviewReport } | { problem: string } {
  if (!preview.valid) {
    if (preview.errors.length === 0) return { problem: LETTER_PREVIEW_UNEXPLAINED };
    return { report: { total_rows: 0, created: 0, committed: false, errors: [...preview.errors], codes: [] } };
  }
  if (preview.rows.length === 0) return { problem: LETTER_IMPORT_EMPTY };
  return { report: { total_rows: preview.rows.length, created: 0, committed: false, errors: [], codes: [] } };
}

/** Rows named by the errors — row 0 (the whole file) is not a row. */
function wrongRows(errors: PreviewReport["errors"]): number {
  return new Set(errors.filter((e) => e.row > 0).map((e) => e.row)).size;
}

/**
 * The result heading: the prototype's `{n} dòng hợp lệ, sẵn sàng nhập` for a clean check; for a refused
 * one its `{e} dòng sai` — without "trên tổng số …", a total this server does not count on a refused
 * file. Only file-level errors: one sentence, no figure.
 */
export function letterPreviewHeading(report: PreviewReport): string {
  if (report.errors.length === 0) return `${report.total_rows} dòng hợp lệ, sẵn sàng nhập`;
  const rows = wrongRows(report.errors);
  return rows === 0 ? "Tệp chưa nhập được" : `${rows} dòng sai`;
}

/** Prototype: `Nhập {n} đơn thư` once a clean check is read; `Nhập` before. */
export function letterImportButtonLabel(report: PreviewReport | null): string {
  return report !== null && report.errors.length === 0 && report.total_rows > 0
    ? `Nhập ${report.total_rows} ${LETTER_IMPORT_UNIT}`
    : "Nhập";
}

/** Prototype success toast `Đã nhập {n} đơn thư.`; a replay (`null`) says the file is already in. */
export function letterImportedToast(created: readonly LetterImportRow[] | null): string {
  return created === null ? replayedText("") : `Đã nhập ${created.length} ${LETTER_IMPORT_UNIT}.`;
}
