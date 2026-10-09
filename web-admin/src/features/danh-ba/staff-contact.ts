/**
 * The Danh bạ tab's add / edit dialog — wording and decisions (prototype `StaffContactForm.tsx`,
 * owner decision 09/10/2026: "Thêm cán bộ" and editing open a dialog in the tab).
 *
 * PURE MODULE, NO JSX: every decision is a value comparison, tested without rendering.
 *
 * WHAT THE DIALOG DOES NOT DO ITSELF: publish. Its "Hiện trên danh bạ Mini App" box only decides
 * whether, AFTER the profile is saved, the screen opens the per-person consent box (`HopCongKhai`,
 * #12, Decree 13) — publishing is its own route, its own key (`content.update`) and its own
 * confirmation. A tick in a profile form that published by itself would publish a personal mobile
 * number without the consent question.
 */

import type { BanNhapCanBo } from "@/components/danh-ba/nhan-ghi-danh-ba";
import type { identity_canBoTomTat, identity_suaCanBoVao } from "@/lib/api/schema.gen";

import type { DangMoCongKhai } from "./hop-cong-khai";

/* ---- wording (prototype, verbatim) ---------------------------------------------------------- */

export const CONTACT_ADD_TITLE = "Thêm cán bộ vào danh bạ";
export const CONTACT_EDIT_TITLE = "Sửa thông tin cán bộ";
export const CONTACT_DESCRIPTION = "Số điện thoại chỉ hiện cho bà con khi bật “Hiện trên danh bạ Mini App”.";

export const FIELD_FULL_NAME = "Họ và tên";
export const FIELD_UNIT = "Bộ phận";
export const UNIT_PLACEHOLDER = "— Chọn bộ phận —";
export const FIELD_POSITION = "Chức vụ";
export const FIELD_EMAIL = "Email công vụ";
export const CHECK_ZALO = "Gọi được qua Zalo";
export const CHECK_MINI_APP = "Hiện trên danh bạ Mini App";
export const EMAIL_NOTE = "Email công vụ không bao giờ hiện cho bà con.";
export const SAVE_BUTTON = "Lưu";

export const FULL_NAME_REQUIRED = "Vui lòng nhập họ và tên";
export const UNIT_REQUIRED = "Vui lòng chọn bộ phận";
/** The prototype's toast when an invalid form is submitted and no field says why. */
export const INVALID_FALLBACK = "Còn ô chưa điền đúng, xem lại giúp anh/chị.";

export const ADDED_TOAST = "Đã thêm vào danh bạ.";
export const SAVED_TOAST = "Đã lưu thay đổi.";

/** The stored unit is no longer in the catalogue: kept selected, said in words (same as `OChon`). */
export const UNIT_NOT_IN_CATALOGUE = "Giá trị đang lưu — không còn trong danh mục";

/* ---- validation ----------------------------------------------------------------------------- */

/** One message per field, under that field. Absent = the field is fine. */
export type ContactErrors = { readonly fullName?: string; readonly unit?: string };

/**
 * The two REQUIRED fields of the prototype: a name, and a unit (the server accepts a row with no
 * unit; the prototype does not, and the owner follows the prototype).
 *
 * NO LENGTH CHECK HERE: the prototype's 255 is looser than the server's own bound (150 runes,
 * `domain.ChuanHoaHoTen`), and a second bound that disagrees with the server's is a rule nobody can
 * explain. Over the bound, the server's sentence is shown verbatim.
 */
export function validateContact(ban: BanNhapCanBo): ContactErrors {
  return {
    ...(ban.hoTen.trim() === "" ? { fullName: FULL_NAME_REQUIRED } : {}),
    ...(ban.boPhanID === "" ? { unit: UNIT_REQUIRED } : {}),
  };
}

/** The first message, for the toast the prototype shows on an invalid submit; `null` = valid. */
export function firstContactError(e: ContactErrors): string | null {
  return e.fullName ?? e.unit ?? null;
}

/* ---- after a save --------------------------------------------------------------------------- */

/**
 * `POST /api/v1/staff` has NO `has_zalo` (`identity_themCanBoVao`): a "Gọi được qua Zalo" ticked while
 * ADDING is written by a second call, the PATCH that owns the field, with every other field `null`
 * ("unchanged"). `null` = nothing to send. Without this the tick would be dropped silently.
 */
export function zaloAfterCreate(ban: BanNhapCanBo): identity_suaCanBoVao | null {
  if (!ban.coZalo) return null;
  return {
    full_name: null,
    position: null,
    email: null,
    org_unit_id: null,
    office_phone: null,
    mobile: null,
    has_zalo: true,
  };
}

/** The person was added but the Zalo flag was refused: say both halves, the server's words last. */
export function zaloNotSavedText(serverMessage: string): string {
  return `Đã thêm vào danh bạ, nhưng chưa lưu được “${CHECK_ZALO}”. ${serverMessage}`;
}

/**
 * What follows a successful save, from the "Hiện trên danh bạ Mini App" box and the row AS THE SERVER
 * RETURNED IT: open the consent box to publish, the confirmation to withdraw, or nothing.
 *
 * `row.published`, NOT the state before the edit: the server itself withdraws a person whose mobile
 * number changed (decision 28/09/2026), so a box left ticked then means "ask again", and does.
 *
 * `canPublish` false (no `content.update`, or session unread) → nothing, fail closed. The box is not
 * even drawn then; this is the second guard, and the server checks the key on the request anyway.
 */
export function publicationStep(
  wanted: boolean,
  row: identity_canBoTomTat,
  canPublish: boolean,
): DangMoCongKhai | null {
  if (!canPublish || wanted === row.published) return null;
  return wanted ? { kieu: "congKhai", canBo: row } : { kieu: "rut", canBo: row };
}
