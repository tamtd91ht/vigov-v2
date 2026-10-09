/**
 * The org chart's words for "⬆ Nhập từ Excel" (`docs/ui-ux/14-cau-hinh.md §1`) and its one
 * org-specific rule, `parentText`. The flow itself — preview then import, one key per attempt — is
 * shared by every import and lives in `excel-import-flow.ts`; the target that binds these words to
 * it is `ORG_UNIT_IMPORT_TARGET` (`excel-import-targets.tsx`).
 */

import type { identity_orgUnitImportUnitOut } from "@/lib/api/schema.gen";

// The steps every import shares moved to `excel-import-flow.ts`; re-exported so this file's callers
// and tests keep their imports.
export {
  CLOSE_BUTTON,
  FILE_LABEL,
  IMPORT_BUTTON,
  IMPORT_SENDING,
  NOTHING_TO_CREATE,
  PREVIEW_BUTTON,
  PREVIEW_SENDING,
  TEMPLATE_BUTTON,
  canImport,
  errorRows,
  keyForAttempt,
} from "./excel-import-flow";
export type { ErrorRow } from "./excel-import-flow";

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

export const CONFIRM_IMPORT_BUTTON = "Nhập các bộ phận này";

export const IMPORT_EXPLANATION =
  "Tải tệp mẫu, điền mỗi bộ phận một dòng rồi chọn tệp để kiểm tra. Hệ thống kiểm tra toàn bộ tệp " +
  "trước, chưa ghi gì; chỉ khi tệp không có lỗi mới nhập được, và nhập thì nhập cả tệp hoặc không " +
  "nhập gì. Bộ phận đã có trên sơ đồ không bị sửa.";

// "nhập lại", not "kiểm tra lại": the shared dialog has no separate check button (spec §1, 09/10/2026).
export const ERRORS_HEADING = "Tệp có lỗi — chưa bộ phận nào được tạo. Sửa các dòng dưới đây rồi nhập lại.";

export function previewLead(count: number): string {
  return `Tệp hợp lệ. Sẽ tạo ${count} bộ phận:`;
}

export function importedSentence(count: number | null): string {
  return count === null
    ? "Tệp đã được nhập ở lần gửi trước. Sơ đồ tổ chức đã được tải lại."
    : `Đã nhập ${count} bộ phận. Sơ đồ tổ chức đã được tải lại.`;
}

export const TEMPLATE_FILE_NAME = "mau-nhap-so-do-to-chuc.xlsx";
