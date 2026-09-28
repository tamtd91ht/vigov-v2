/**
 * The decision half of "⬆ Nhập từ Excel" on the org chart (`docs/ui-ux/14-cau-hinh.md §1`). Pure, so
 * the rows staff read when a file is refused — "dòng 4, cột Bộ phận cha: …" — have a test.
 *
 * THE FLOW IS TWO REQUESTS ON PURPOSE: preview (writes nothing) then import (all or nothing). The
 * import is only offered once a preview of THE SAME FILE came back `valid`, and its
 * `Idempotency-Key` is minted at that moment — one key per attempt at importing one previewed file,
 * kept across retries of that attempt, dropped when a new file is chosen or the attempt succeeds.
 */

import type { identity_orgUnitImportErrorOut, identity_orgUnitImportUnitOut } from "@/lib/api/schema.gen";

export type ErrorRow = { readonly row: string; readonly column: string; readonly message: string };

/**
 * Server errors → table rows. `row: 0` is an error of the whole file and `column: ""` one of the
 * whole row (`org_unit_import.go`, `orgUnitImportErrorOut`): said in words, never shown as "0".
 */
export function errorRows(errors: readonly identity_orgUnitImportErrorOut[]): readonly ErrorRow[] {
  return errors.map((e) => ({
    row: e.row === 0 ? "Cả tệp" : String(e.row),
    column: e.column === "" ? "—" : e.column,
    message: e.message,
  }));
}

/**
 * Where a planned unit hangs: an existing unit (by code), a row of the same file, or the top level.
 * A preview row whose parent is in the same file has no id yet — `parent_row` names it.
 */
export function parentText(u: identity_orgUnitImportUnitOut): string {
  if (u.parent_code !== "") {
    return u.parent_row !== undefined && u.parent_row > 0
      ? `${u.parent_code} (dòng ${u.parent_row} của tệp)`
      : u.parent_code;
  }
  if (u.parent_row !== undefined && u.parent_row > 0) return `Dòng ${u.parent_row} của tệp`;
  return "Cấp cao nhất";
}

/** Whether the import button may be offered: a valid preview with something to create. */
export function canImport(preview: { valid: boolean; units: readonly unknown[] } | null): boolean {
  return preview !== null && preview.valid && preview.units.length > 0;
}

/**
 * The key for the import attempt. Kept if one exists (a retry of the SAME attempt), minted
 * otherwise. `mint` is a parameter so the test counts calls; the panel passes `crypto.randomUUID`.
 */
export function keyForAttempt(current: string | null, mint: () => string): string {
  return current ?? mint();
}

export const IMPORT_BUTTON = "⬆ Nhập từ Excel";
export const TEMPLATE_BUTTON = "Tải tệp mẫu";
export const FILE_LABEL = "Chọn tệp Excel đã điền (.xlsx)";
export const PREVIEW_BUTTON = "Kiểm tra tệp";
export const PREVIEW_SENDING = "Đang kiểm tra tệp…";
export const CONFIRM_IMPORT_BUTTON = "Nhập các bộ phận này";
export const IMPORT_SENDING = "Đang nhập…";
export const CLOSE_BUTTON = "Đóng";

export const IMPORT_EXPLANATION =
  "Tải tệp mẫu, điền mỗi bộ phận một dòng rồi chọn tệp để kiểm tra. Hệ thống kiểm tra toàn bộ tệp " +
  "trước, chưa ghi gì; chỉ khi tệp không có lỗi mới nhập được, và nhập thì nhập cả tệp hoặc không " +
  "nhập gì. Bộ phận đã có trên sơ đồ không bị sửa.";

export const ERRORS_HEADING = "Tệp có lỗi — chưa bộ phận nào được tạo. Hãy sửa các dòng dưới đây rồi kiểm tra lại:";
export const NOTHING_TO_CREATE = "Tệp không có dòng nào để nhập.";

export function previewLead(count: number): string {
  return `Tệp hợp lệ. Sẽ tạo ${count} bộ phận:`;
}

export function importedSentence(count: number | null): string {
  return count === null
    ? "Tệp đã được nhập ở lần gửi trước. Sơ đồ tổ chức đã được tải lại."
    : `Đã nhập ${count} bộ phận. Sơ đồ tổ chức đã được tải lại.`;
}

export const TEMPLATE_FILE_NAME = "mau-nhap-so-do-to-chuc.xlsx";
