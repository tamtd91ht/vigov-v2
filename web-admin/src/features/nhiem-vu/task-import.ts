/**
 * `Nhập từ Excel` of the Nhiệm vụ screen — pure half: the words (spec 09, `ExcelImportDialog` of the
 * Giải ngân spec 05), the file check before sending, the key rule, and the report. The calls are
 * `lib/api/task-import.ts`.
 */

import type { petitions_taskImportErrorOut, petitions_taskImportResultOut } from "@/lib/api/schema.gen";

/** The header button (spec 02 §1); the spec's `⬆` is a lucide `Upload` (ADR 0068). */
export const IMPORT_OPEN_BUTTON = "Nhập từ Excel";
/** Spec 09, verbatim. */
export const IMPORT_TITLE = "Giao việc hàng loạt từ Excel";
export const IMPORT_TEMPLATE_BUTTON = "Tải mẫu giao việc";
export const IMPORT_FILE_LABEL = "Chọn tệp .xlsx";
export const IMPORT_SUBMIT_BUTTON = "Nhập";
export const IMPORT_CLOSE_BUTTON = "Đóng";
export const IMPORT_DESCRIPTION =
  "Tệp được kiểm trước và chưa ghi gì. Còn một dòng sai thì không dòng nào được nhận — sửa tệp rồi nhập lại.";
export const IMPORT_CHECKING = "Đang kiểm tệp…";
/** Spec 09: the template is saved under this name. */
export const IMPORT_TEMPLATE_SAVE_NAME = "mau-giao-viec.xlsx";
/** Spec 09 (Giải ngân 05) toast when the real import came back with row errors. */
export const IMPORT_ROWS_STILL_WRONG = "Tệp còn dòng sai, chưa ghi dòng nào.";

/** Server bound (`taskImportMaxBytes`). Checked here only to spare an upload; the server decides. */
export const IMPORT_MAX_BYTES = 2 << 20;

export const IMPORT_NOT_XLSX = "Chỉ nhận tệp Excel .xlsx. Hãy tải mẫu, điền vào đó rồi lưu dưới dạng .xlsx.";
export const IMPORT_FILE_TOO_LARGE =
  "Tệp lớn hơn 2 MB nên chưa gửi. Hãy chia thành nhiều tệp nhỏ hơn rồi nhập lần lượt.";

/** A sentence stopping the send before any byte leaves, or `null`. */
export function importFileProblem(file: { readonly name: string; readonly size: number }): string | null {
  if (!file.name.toLowerCase().endsWith(".xlsx")) return IMPORT_NOT_XLSX;
  if (file.size > IMPORT_MAX_BYTES) return IMPORT_FILE_TOO_LARGE;
  return null;
}

/**
 * The key of a REAL import: kept for the same file until it succeeds (a retry after a lost answer
 * replays, never books twice), minted when there is none. A CHECK never uses it — see
 * `submitTaskImport`: it takes a fresh key each time.
 */
export function importKeyFor(current: string | null, mint: () => string): string {
  return current ?? mint();
}

/* ── the report ────────────────────────────────────────────────────────────────────────────── */

/** Distinct rows named by the errors: one row with three wrong cells is one wrong row. */
export function wrongRowCount(errors: readonly petitions_taskImportErrorOut[]): number {
  return new Set(errors.map((e) => e.row)).size;
}

/** Spec 09 result heading: `{n} dòng hợp lệ, sẵn sàng nhập` / `{e} dòng sai trên tổng số {n} dòng`. */
export function previewHeading(report: Pick<petitions_taskImportResultOut, "total_rows" | "errors">): string {
  return report.errors.length === 0
    ? `${report.total_rows} dòng hợp lệ, sẵn sàng nhập`
    : `${wrongRowCount(report.errors)} dòng sai trên tổng số ${report.total_rows} dòng`;
}

/** Spec 09: the confirm button — `Nhập {n} nhiệm vụ` once a clean check is read; `Nhập` before. */
export function importButtonLabel(report: Pick<petitions_taskImportResultOut, "total_rows" | "errors"> | null): string {
  return report !== null && report.errors.length === 0 ? `${IMPORT_SUBMIT_BUTTON} ${report.total_rows} nhiệm vụ` : IMPORT_SUBMIT_BUTTON;
}

/** Whether the real import may be sent: a clean check of THIS file with at least one row. */
export function canCommitImport(report: Pick<petitions_taskImportResultOut, "total_rows" | "errors"> | null): boolean {
  return report !== null && report.errors.length === 0 && report.total_rows > 0;
}

/** Spec 09 success toast: `Đã nhập {n} nhiệm vụ.` — the codes the server issued, else its count. */
export function importedToast(report: Pick<petitions_taskImportResultOut, "codes" | "created">): string {
  return `Đã nhập ${report.codes.length > 0 ? report.codes.length : report.created} nhiệm vụ.`;
}

/** A real send the server answered 200 without refusing a row — nothing was written. */
export const IMPORT_NOT_COMMITTED = "Máy chủ chưa nhập tệp này và không nêu dòng nào sai. Hãy chọn lại tệp rồi thử lại.";

/** Rows in spreadsheet order, then column — the order a clerk walks the sheet. */
export function sortedImportErrors(
  errors: readonly petitions_taskImportErrorOut[],
): petitions_taskImportErrorOut[] {
  return [...errors].sort((a, b) => a.row - b.row || a.column.localeCompare(b.column, "vi"));
}

/**
 * A replayed answer: the file was ALREADY imported by an earlier send of this same attempt (its answer
 * was lost). Said plainly, so nobody imports it again to "make sure".
 */
export function replayedText(firstCode: string): string {
  return firstCode === ""
    ? "Tệp này đã được nhập ở lần gửi trước. Không nhập lại lần nữa."
    : `Tệp này đã được nhập ở lần gửi trước (mã đầu tiên: ${firstCode}). Không nhập lại lần nữa.`;
}
