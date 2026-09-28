/**
 * `⬆ Nhập từ Excel` (§8) — pure half: the words, the file check before sending, the key rule, and
 * the report lines. The calls are `lib/api/task-import.ts`.
 */

import type { petitions_taskImportErrorOut } from "@/lib/api/schema.gen";

export const IMPORT_OPEN_BUTTON = "⬆ Nhập từ Excel";
export const IMPORT_TITLE = "Nhập nhiệm vụ từ Excel";
export const IMPORT_TEMPLATE_BUTTON = "⬇ Tải mẫu nhiệm vụ";
export const IMPORT_FILE_LABEL = "Chọn tệp .xlsx";
export const IMPORT_DROP_HINT = "hoặc kéo thả tệp vào đây";
export const IMPORT_CHECK_BUTTON = "Kiểm tra trước";
export const IMPORT_SUBMIT_BUTTON = "Nhập";
export const IMPORT_CLOSE_BUTTON = "Đóng";

/**
 * The description §8 draws first. Every fact is the server's (`task_import.go`, `domain/task_import.go`):
 * 2 MB, 500 rows, all-or-nothing, auto-issued codes in the NV01… series (§8 line 278).
 */
export const IMPORT_DESCRIPTION =
  "Tải mẫu, điền mỗi dòng một nhiệm vụ rồi chọn tệp để nhập (tối đa 500 dòng, 2 MB). Máy chủ kiểm " +
  "mọi dòng trước: chỉ cần một dòng sai là chưa nhiệm vụ nào được nhập. Mã nhiệm vụ được cấp tự " +
  "động theo dãy NV01, NV02…";

/**
 * Said in the dialog because it is the refusal a clerk meets first: a date alone is refused — the
 * software never picks the hour a commitment falls due (`domain/task_import.go:10-11`, rule 10).
 */
export const IMPORT_DEADLINE_NOTE =
  "Ô “Hạn hoàn thành (ngày giờ)” phải ghi cả ngày và giờ, ví dụ 30/09/2026 17:00. Chỉ ghi ngày thì " +
  "dòng ấy bị từ chối; để trống nếu nhiệm vụ không có hạn.";

export const IMPORT_CHECK_NOTE =
  "“Kiểm tra trước” kiểm cả tệp như khi nhập thật nhưng không nhập gì.";

export const IMPORT_CHECKING = "Đang kiểm tra tệp…";
export const IMPORT_SENDING = "Đang nhập… Mỗi dòng được kiểm trước; chưa có gì được ghi cho tới khi máy chủ trả lời.";

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

export function checkedOkText(totalRows: number): string {
  return `Tệp hợp lệ: ${totalRows} dòng sẽ được nhập. Chưa nhập gì — bấm “${IMPORT_SUBMIT_BUTTON}” để nhập thật.`;
}

/** A real send the server answered 200 without refusing a row — nothing was written. */
export const IMPORT_NOT_COMMITTED = "Máy chủ chưa nhập tệp này và không nêu dòng nào sai. Hãy kiểm tra trước rồi thử lại.";

export function refusedHeading(errorCount: number, dryRun: boolean): string {
  return dryRun
    ? `Tệp có ${errorCount} lỗi. Sửa các dòng dưới đây rồi kiểm tra lại:`
    : `Tệp có ${errorCount} lỗi — chưa nhiệm vụ nào được nhập. Sửa các dòng dưới đây rồi thử lại:`;
}

/** One refusal: spreadsheet row (1 = heading row), column heading, the server's sentence. */
export function importErrorLine(e: petitions_taskImportErrorOut): string {
  return e.column === "" ? `Dòng ${e.row}: ${e.message}` : `Dòng ${e.row} · cột “${e.column}”: ${e.message}`;
}

/** Rows in spreadsheet order, then column — the order a clerk walks the sheet. */
export function sortedImportErrors(
  errors: readonly petitions_taskImportErrorOut[],
): petitions_taskImportErrorOut[] {
  return [...errors].sort((a, b) => a.row - b.row || a.column.localeCompare(b.column, "vi"));
}

export function importedText(codes: readonly string[]): string {
  return codes.length === 0
    ? "Đã nhập tệp."
    : `Đã nhập ${codes.length} nhiệm vụ: ${codes.join(", ")}.`;
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
