/**
 * Words and checks of the role-catalogue dialogs on the Phân quyền screen (owner decision 15/10/2026).
 * Pure: no network, no DOM — the dialogs (`role-dialogs.tsx`) draw them, the tests read them.
 *
 * THE CHECKS HERE ARE A CONVENIENCE, NOT THE RULE. An empty or over-long name, an empty reason: the
 * server refuses each with 422 and its own sentence. Checking first only spares the officer a round
 * trip; the server's sentence still wins whenever it answers.
 */

import type { identity_seedRoleTemplatesOut } from "@/lib/api/schema.gen";

/** The contract's cap on a role name (`POST /api/v1/roles`, 422 `invalid_name`). */
export const ROLE_NAME_MAX = 100;

export const ADD_ROLE_BUTTON = "Thêm vai trò";
export const ADD_TEMPLATE_ROLES_BUTTON = "Thêm vai trò mẫu";
export const RENAME_ROLE = "Đổi tên";
export const DELETE_ROLE = "Xoá vai trò";
export const CANCEL = "Huỷ";
export const SAVE = "Lưu";

export const ROLE_NAME_LABEL = "Tên vai trò";
export const ROLE_NAME_PLACEHOLDER = "Ví dụ: Phó Chủ tịch UBND";
export const LEADER_LABEL = "Vai trò lãnh đạo";
export const LEADER_HINT = "Chỉ đổi màn hình mặc định khi đăng nhập, không thêm quyền.";
export const ADD_ROLE_HINT = "Tích quyền cho vai trò ngay trên bảng sau khi thêm — vai trò mới chưa có quyền nào.";
export const RENAME_ROLE_HINT = "Quyền của vai trò giữ nguyên.";
export const RENAME_ROLE_TITLE = "Đổi tên vai trò";

export const DELETE_REASON_LABEL = "Lý do xoá";
export const DELETE_ROLE_EXPLANATION =
  "Vai trò được đánh dấu đã xoá và không gán được cho cán bộ nữa; lịch sử cũ vẫn giữ. Vai trò còn cán bộ giữ thì chưa xoá được.";

export const SEED_TITLE = "Thêm các vai trò mẫu?";
export const SEED_INTRO = "Hệ thống thêm 8 vai trò chuẩn, mỗi vai trò kèm sẵn quyền của mẫu:";
export const SEED_KEEPS =
  "Vai trò đã có trong đơn vị được giữ nguyên, kể cả quyền. Vai trò mẫu đã từng bị xoá không được khôi phục.";
export const BUSY_ADDING = "Đang thêm…";

/** Shown when the account has unsaved ticks: a reload after a role change would drop them. */
export const EDITS_PENDING_HINT = "Lưu hoặc huỷ các ô phân quyền đang sửa trước khi thêm, đổi tên hay xoá vai trò.";

/**
 * The eight template names, in the server's order — A COPY, kept for one reason: the confirm box must
 * say what will be added BEFORE the request, and the contract has no route that lists the templates.
 * The source is `service-identity/internal/domain/role_template.go` (`Order` 1–8); a ninth template
 * there must be added here, or the box under-states what it does. The server's answer, not this list,
 * is what the result toast counts.
 */
export const ROLE_TEMPLATE_NAMES: readonly string[] = [
  "Chủ tịch UBND",
  "Phó Chủ tịch UBND",
  "Chánh Văn phòng",
  "Trưởng bộ phận",
  "Chuyên viên chuyên môn",
  "Kế toán",
  "Cán bộ một cửa",
  "Trưởng thôn, Tổ trưởng dân phố",
];

/** "" when the name can be sent; otherwise the sentence drawn under "Tên vai trò". */
export function roleNameProblem(name: string): string {
  const trimmed = name.trim();
  if (trimmed === "") return "Nhập tên vai trò.";
  // Code points, not UTF-16 units: a Vietnamese letter typed decomposed is still one character to the officer.
  if ([...trimmed].length > ROLE_NAME_MAX) return `Tên vai trò dài tối đa ${ROLE_NAME_MAX} ký tự.`;
  return "";
}

/** "" when the reason can be sent; otherwise the sentence drawn under "Lý do xoá". */
export function deleteReasonProblem(reason: string): string {
  return reason.trim() === "" ? "Nhập lý do xoá vai trò." : "";
}

/**
 * Where a server refusal is drawn: under the field it is about, or at the foot of the form. Decided by
 * the server's `code` (a documented reader of `KetQua.code`, `goi.ts`), never by its wording.
 */
const NAME_CODES: readonly string[] = ["invalid_name", "duplicate_name"];
const REASON_CODES: readonly string[] = ["invalid_reason"];

export function isNameRefusal(code: string | undefined): boolean {
  return code !== undefined && NAME_CODES.includes(code);
}

export function isReasonRefusal(code: string | undefined): boolean {
  return code !== undefined && REASON_CODES.includes(code);
}

export function deleteRoleTitle(name: string): string {
  return `Xoá vai trò ${name}?`;
}

export function renameRoleLabel(name: string): string {
  return `Đổi tên vai trò ${name}`;
}

export function deleteRoleLabel(name: string): string {
  return `Xoá vai trò ${name}`;
}

export function roleCreatedToast(name: string): string {
  return `Đã thêm vai trò ${name}.`;
}

export function roleRenamedToast(name: string): string {
  return `Đã lưu vai trò ${name}.`;
}

export function roleDeletedToast(name: string): string {
  return `Đã xoá vai trò ${name}.`;
}

/** The seed's result, counted from the server's answer — never from `ROLE_TEMPLATE_NAMES`. */
export function seedResultMessage(out: identity_seedRoleTemplatesOut): string {
  const parts = [`Đã thêm ${out.created.length} vai trò mẫu.`];
  if (out.skipped_existing.length > 0) parts.push(`Giữ nguyên ${out.skipped_existing.length} vai trò đã có.`);
  if (out.skipped_deleted.length > 0) parts.push(`Không khôi phục ${out.skipped_deleted.length} vai trò đã xoá.`);
  return parts.join(" ");
}
