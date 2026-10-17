/**
 * The words of the citizen directory (`/nguoi-dan`) — `docs/ui-ux/16-danh-ba-nguoi-dan.md`, verbatim —
 * and the pure decisions behind them. No JSX, no network: every sentence a commune reads on this screen
 * is pinned here by a string test, and the components only put them on the page.
 *
 * Where chapter 16 marks a deliberate deviation from the prototype (`[Lệch prototype — Dn]`), the
 * chapter's wording wins; the reasons are ADR 0097 and are not repeated here.
 */

import { isCitizenContactFilterEmpty, type CitizenContact, type CitizenContactFilter } from "@/lib/api/citizen-contacts";
import type { CitizenGroup } from "@/lib/api/citizen-groups";
import type { ImportError } from "@/lib/api/excel-import";
import type { petitions_taskImportResultOut } from "@/lib/api/schema.gen";

import { replayedText } from "@/features/nhiem-vu/task-import";

/* ---- page ------------------------------------------------------------------------------------ */

export const CITIZEN_DIRECTORY_TITLE = "Danh bạ người dân";
/** Chapter 16 §1 — the chapter's sentence, not the prototype's (D3, D13: no Zalo id, no messaging yet). */
export const CITIZEN_DIRECTORY_SUBTITLE =
  "Số điện thoại để xã liên lạc với bà con, phân theo nhóm và gắn nhãn. Cột Zalo cho biết số đã đăng nhập Mini App của xã.";
export const IMPORT_OPEN_BUTTON = "Nhập từ Excel";
export const ADD_CONTACT_BUTTON = "Thêm người dân";

/** Gate sentence: names WHICH right is missing (`KhungQuyen`'s contract). */
export const NO_READ_PERMISSION =
  "Tài khoản của bạn không có quyền xem danh bạ người dân (citizen.read), nên danh bạ không hiển thị. " +
  "Liên hệ quản trị viên của đơn vị nếu bạn cần quyền này.";

export const TAB_CONTACTS = "Danh sách";
export const TAB_GROUPS = "Nhóm";

/** `Danh sách (n)` / `Nhóm (m)` — the bracket only with a figure the server gave; `null` = no bracket, never "(0)". */
export function tabLabel(base: string, count: number | null): string {
  return count === null ? base : `${base} (${count.toLocaleString("vi-VN")})`;
}

/* ---- filters (§3) ---------------------------------------------------------------------------- */

export const SEARCH_LABEL = "Tìm người dân";
/** ADR 0101 #7: the keyword searches full name, phone and email (on the search engine). */
export const SEARCH_PLACEHOLDER = "Tìm theo tên, số điện thoại, email…";
/** The server's own sentence for a keyword over 200 characters — said before the keyword leaves the browser. */
export const KEYWORD_TOO_LONG = "Từ khoá tìm kiếm quá dài (tối đa 200 ký tự).";
export const GROUP_FILTER_LABEL = "Lọc theo nhóm";
export const ALL_GROUPS = "Tất cả nhóm";
export const ONLY_WITH_ZALO = "Chỉ người có Zalo";
export const TAG_STRIP_LABEL = "Nhãn đang dùng";

/* ---- advanced filters (ADR 0101 #10) ---------------------------------------------------------- */

export const FILTER_TOGGLE = "Bộ lọc";
/** `Bộ lọc (2)` — the number of conditions applied, none = the bare word. */
export function filterToggleLabel(count: number): string {
  return count > 0 ? `${FILTER_TOGGLE} (${count})` : FILTER_TOGGLE;
}
export const FILTER_PANEL_LABEL = "Bộ lọc nâng cao";
export const FILTER_CREATED = "Ngày tạo";
export const FILTER_FROM = "Từ";
export const FILTER_TO = "Đến";
export const FILTER_APPLY = "Áp dụng";
export const FILTER_CLEAR = "Xoá lọc";
export const FILTER_RANGE_HINT = "Chỉ cần điền một đầu.";
export const DROP_ADVANCED_FILTER = "Bỏ bộ lọc nâng cao";

/** `Tên nhóm (số thành viên)` — the prototype's option text (`CitizenWorkspace.tsx:114`). */
export function groupOptionText(g: Pick<CitizenGroup, "name" | "member_count">): string {
  return `${g.name} (${g.member_count})`;
}

/** `hộ kinh doanh · 12` — the prototype's chip text (`CitizenWorkspace.tsx:150`). */
export function tagChipText(tag: string, count: number): string {
  return `${tag} · ${count}`;
}

/* ---- table (§4) ------------------------------------------------------------------------------ */

export const COLUMN_NAME = "Họ tên";
export const COLUMN_PHONE = "Điện thoại";
export const COLUMN_UNIT = "Thôn, tổ";
export const COLUMN_TAGS = "Nhãn";
export const COLUMN_ZALO = "Zalo";
/** Tooltip of the Zalo column header — ADR 0097 D13, verbatim. */
export const ZALO_COLUMN_TOOLTIP = "Đã đăng nhập Mini App của xã bằng số này";
export const ZALO_YES = "Có";
export const EMPTY_CELL = "—";
export const TABLE_LABEL = "Bảng người dân";

/** `Xoá {họ tên}` — the trash icon's accessible name (§4). */
export function deleteContactLabel(fullName: string): string {
  return `Xoá ${fullName}`;
}

/** The two empty sentences of §4: the directory is empty, or the filter matches nobody. */
export const EMPTY_DIRECTORY = "Danh bạ chưa có ai. Bấm “Thêm người dân” để bắt đầu.";
export const EMPTY_FILTERED = "Không có ai khớp bộ lọc.";

/**
 * Which empty sentence: "nobody at all" is a statement about the commune and is said only when nothing
 * narrows the list (`isCitizenContactFilterEmpty`) — with a filter on, an empty page says the filter
 * matched nobody.
 */
export function emptySentence(filter: CitizenContactFilter): string {
  return isCitizenContactFilterEmpty(filter) ? EMPTY_DIRECTORY : EMPTY_FILTERED;
}

export const PAGINATION_LABEL = "Phân trang danh bạ người dân";

/** `Trang k` — the position in the cursor stack, the one figure the client knows (D8: no "/N"). */
export function pageLabel(pageNumber: number): string {
  return `Trang ${pageNumber}`;
}

/** `Nhãn` cell: tags joined by a comma, or the dash (§4). */
export function tagsCellText(tags: readonly string[]): string {
  return tags.length > 0 ? tags.join(", ") : EMPTY_CELL;
}

/** `Thôn, tổ` cell: the unit's name as the server joined it, or the dash. */
export function unitCellText(row: Pick<CitizenContact, "residential_unit_name">): string {
  return row.residential_unit_name === "" ? EMPTY_CELL : row.residential_unit_name;
}

/* ---- delete (§4, D7) ------------------------------------------------------------------------- */

export function deleteContactQuestion(fullName: string): string {
  return `Xoá ${fullName} khỏi danh bạ?`;
}
export const DELETE_CONTACT_EXPLANATION =
  "Người này sẽ không còn trong danh bạ liên lạc của xã. Dòng được giữ lại kèm lý do, không xoá hẳn.";
export const DELETE_REASON_LABEL = "Lý do gỡ";
export const DELETE_REASON_REQUIRED = "Nhập lý do trước khi gỡ.";
export const DELETE_CONFIRM_BUTTON = "Xoá";
export const CANCEL_BUTTON = "Huỷ";

export function removedToast(fullName: string): string {
  return `Đã gỡ ${fullName}.`;
}
export const REMOVE_FAILED_TOAST = "Chưa gỡ được.";

/** A reason is text, not whitespace: the server refuses an empty one (400), so the box refuses it first. */
export function deleteReasonProblem(reason: string): string | null {
  return reason.trim() === "" ? DELETE_REASON_REQUIRED : null;
}

/* ---- add / edit dialog (§5) ------------------------------------------------------------------ */

export const CONTACT_ADD_TITLE = "Thêm người dân";
export const CONTACT_EDIT_TITLE = "Sửa thông tin người dân";
export const CONTACT_DIALOG_DESCRIPTION = "Bắt buộc họ tên và số điện thoại. Những ô còn lại điền được tới đâu hay tới đó.";
export const FIELD_FULL_NAME = "Họ và tên";
export const FIELD_PHONE = "Số điện thoại";
/** ADR 0101 #6 — optional; answered masked like the number. */
export const FIELD_EMAIL = "Email";
export const EMAIL_INVALID = "Email chưa đúng, ví dụ: ten@example.com.";
/** On edit the stored address arrives masked: replacing it is an explicit act, so the mask is never sent back. */
export const EDIT_EMAIL_BUTTON = "Nhập email mới";
export const KEEP_EMAIL_BUTTON = "Giữ email cũ";
export const EMAIL_REPLACE_HINT = "Để trống rồi lưu là xoá email của người này.";
export const EMAIL_MAX = 254;
export const REVEAL_PHONE_BUTTON = "Xem số đầy đủ";
export const REVEAL_PHONE_FAILED = "Chưa lấy được số. Thử lại.";
/**
 * NOT IN CHAPTER 16 — forced by the server: `PATCH /citizen-contacts/{id}` requires the FULL number and
 * refuses a mask (`domain.NormalizeCitizenPhone`). Said under the field while the number is still masked,
 * so the disabled `Lưu thay đổi` explains itself. Reported as a contract gap; remove when the server
 * accepts "phone unchanged".
 */
export const REVEAL_BEFORE_SAVE_HINT = "Bấm “Xem số đầy đủ” rồi mới lưu được — máy chủ chỉ nhận số đầy đủ.";
export const FIELD_UNIT = "Thôn, tổ dân phố";
export const UNIT_UNKNOWN = "Chưa rõ";
export const FIELD_ADDRESS = "Địa chỉ";
export const FIELD_TAGS = "Nhãn";
export const TAGS_HINT = "Cách nhau bằng dấu phẩy: hộ nghèo, tổ trưởng, hộ kinh doanh…";
/** ADR 0097 D4 — fixed under the tags field, never switched off. */
export const TAGS_WARNING = "Không tạo nhãn về sức khoẻ, dân tộc, tôn giáo, chính trị";
export const FIELD_GROUPS = "Nhóm";
export const NO_GROUPS_IN_DIALOG = "Xã chưa lập nhóm nào. Lập ở ngăn “Nhóm”.";
export const FIELD_NOTE = "Ghi chú";
export const FIELD_ACTIVE = "Đang liên hệ";
export const SUBMIT_ADD = "Thêm vào danh bạ";
export const SUBMIT_EDIT = "Lưu thay đổi";
export const ADDED_TOAST = "Đã thêm vào danh bạ.";
export const SAVED_TOAST = "Đã lưu.";
export const PHONE_TAKEN_TOAST = "Số điện thoại này đã có trong danh bạ.";
export const SAVE_FAILED_TOAST = "Chưa lưu được. Vui lòng thử lại.";
export const DETAIL_LOADING = "Đang đọc thông tin người dân…";

/** The prototype's minimum before `Thêm vào danh bạ` enables (`CitizenForm.tsx:56`); the server still normalises. */
export const PHONE_MIN_CHARS = 8;

/** The draft of the dialog — what the person typed, before any normalisation. */
export type ContactDraft = {
  readonly fullName: string;
  readonly phone: string;
  /** What is typed in the Email box. On edit it is sent only once "Nhập email mới" was pressed (`emailEdited`). */
  readonly email: string;
  readonly residentialUnitId: string;
  readonly address: string;
  readonly tagText: string;
  readonly groupIds: readonly string[];
  readonly note: string;
  readonly isActive: boolean;
};

export const EMPTY_CONTACT_DRAFT: ContactDraft = {
  fullName: "",
  phone: "",
  email: "",
  residentialUnitId: "",
  address: "",
  tagText: "",
  groupIds: [],
  note: "",
  isActive: true,
};

/** `a, b ,, c` → `["a", "b", "c"]` — the prototype's split (`CitizenForm.tsx:67-70`). */
export function parseTags(text: string): string[] {
  return text
    .split(",")
    .map((t) => t.trim())
    .filter((t) => t !== "");
}

/** Toggle one group id in the draft's list, keeping the order of ticking. */
export function toggleGroup(ids: readonly string[], id: string): string[] {
  return ids.includes(id) ? ids.filter((g) => g !== id) : [...ids, id];
}

/**
 * Whether the submit button is enabled: name and number present (§5, "vô hiệu khi thiếu ô bắt buộc").
 * In EDIT mode `phoneRevealed` must also hold — the hint above says why.
 */
export function canSubmitContact(draft: ContactDraft, editing: boolean, phoneRevealed: boolean): boolean {
  if (draft.fullName.trim() === "" || draft.phone.trim().length < PHONE_MIN_CHARS) return false;
  return !editing || phoneRevealed;
}

/**
 * The Email box's problem, or `null`. Empty is allowed (the field is optional). The shape check is the
 * loose one — something@something.tld, no spaces, at most 254 characters — the server's is the real one.
 */
export function emailProblem(email: string): string | null {
  const e = email.trim();
  if (e === "") return null;
  return e.length > EMAIL_MAX || !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(e) ? EMAIL_INVALID : null;
}

/** Which toast a refused save gets: the chapter's sentence for `phone_taken`, its other sentence for the rest. */
export function saveFailureToast(code: string | undefined): string {
  return code === "phone_taken" ? PHONE_TAKEN_TOAST : SAVE_FAILED_TOAST;
}

/* ---- groups tab (§6) ------------------------------------------------------------------------- */

export const NEW_GROUP_LABEL = "Lập nhóm mới";
export const NEW_GROUP_PLACEHOLDER = "Tổ trưởng dân phố, Hộ kinh doanh…";
export const NEW_GROUP_BUTTON = "Lập nhóm";
export const GROUP_CREATED_TOAST = "Đã lập nhóm.";
export const GROUP_CREATE_FAILED_TOAST = "Tên nhóm đã có, hoặc chưa lưu được.";
export const GROUP_REMOVED_TOAST = "Đã gỡ nhóm.";
export const EMPTY_GROUPS_TITLE = "Chưa có nhóm nào";
export const EMPTY_GROUPS_TEXT = "Nhóm giúp gửi thông báo cho đúng người: tổ trưởng dân phố, hộ kinh doanh, hội viên phụ nữ…";
export const GROUPS_TABLE_LABEL = "Bảng nhóm";
export const GROUPS_LOADING = "Đang đọc danh sách nhóm…";

export function memberCountText(n: number): string {
  return `${n.toLocaleString("vi-VN")} người`;
}

/** `Gỡ nhóm {tên}` — the trash icon's accessible name (§6). */
export function removeGroupLabel(name: string): string {
  return `Gỡ nhóm ${name}`;
}

export function removeGroupQuestion(name: string): string {
  return `Gỡ nhóm ${name}?`;
}
/** §6: "Người trong nhóm vẫn còn trong danh bạ". */
export const REMOVE_GROUP_EXPLANATION = "Người trong nhóm vẫn còn trong danh bạ. Chỉ nhóm được gỡ.";
export const GROUP_REASON_LABEL = "Lý do gỡ (không bắt buộc)";
export const REMOVE_GROUP_BUTTON = "Gỡ nhóm";

/* ---- Excel import (§7) ----------------------------------------------------------------------- */

export const IMPORT_TITLE = "Nhập danh bạ người dân từ Excel";
export const IMPORT_TEMPLATE_BUTTON = "Tải mẫu danh bạ người dân";
export const IMPORT_UNIT = "người dân";
export const IMPORT_FILE_INPUT_ID = "tep-nhap-danh-ba-nguoi-dan"; // vi-name-ok: DOM id, read by staff in the markup like its siblings (`tep-nhap-don-thu`)
export const IMPORT_TITLE_ID = "tieu-de-nhap-danh-ba-nguoi-dan"; // vi-name-ok: DOM id, same convention as `tieu-de-nhap-don-thu`
/** A preview that is neither valid nor names a row: nothing to fix, nothing to import. Said, not guessed. */
export const IMPORT_PREVIEW_UNEXPLAINED = "Máy chủ chưa nhận tệp này và không nêu dòng nào sai. Hãy chọn lại tệp rồi thử lại.";
export const IMPORT_EMPTY_FILE = "Tệp không có dòng nào để nhập.";

/** The shared import view's report shape (`TaskImportView`), filled from this import's preview. */
export type ImportReport = petitions_taskImportResultOut;

/**
 * A preview → the report the shared view draws, or a sentence when there is nothing to draw. A refused
 * preview keeps the server's `row_count` (it counts the rows it read) so the heading can say "e dòng sai
 * trên tổng số n dòng"; with no count it says only the errors.
 */
export function importPreviewReport(preview: { valid: boolean; rowCount: number; errors: readonly ImportError[] }): { report: ImportReport } | { problem: string } {
  if (!preview.valid) {
    if (preview.errors.length === 0) return { problem: IMPORT_PREVIEW_UNEXPLAINED };
    return { report: { total_rows: preview.rowCount, created: 0, committed: false, errors: [...preview.errors], codes: [] } };
  }
  if (preview.rowCount === 0) return { problem: IMPORT_EMPTY_FILE };
  return { report: { total_rows: preview.rowCount, created: 0, committed: false, errors: [], codes: [] } };
}

/** Rows named by the errors — row 0 (the whole file) is not a row. */
function wrongRows(errors: ImportReport["errors"]): number {
  return new Set(errors.filter((e) => e.row > 0).map((e) => e.row)).size;
}

/** `{n} dòng hợp lệ, sẵn sàng nhập` · `{e} dòng sai trên tổng số {n} dòng` · `{e} dòng sai` · `Tệp chưa nhập được`. */
export function importPreviewHeading(report: ImportReport): string {
  if (report.errors.length === 0) return `${report.total_rows} dòng hợp lệ, sẵn sàng nhập`;
  const rows = wrongRows(report.errors);
  if (rows === 0) return "Tệp chưa nhập được";
  return report.total_rows > 0 ? `${rows} dòng sai trên tổng số ${report.total_rows} dòng` : `${rows} dòng sai`;
}

/** `Nhập {n} người dân` once a clean check is read; `Nhập` before. */
export function importButtonLabel(report: ImportReport | null): string {
  return report !== null && report.errors.length === 0 && report.total_rows > 0 ? `Nhập ${report.total_rows} ${IMPORT_UNIT}` : "Nhập";
}

/** `Đã nhập {n} người dân.`; a replay (`null`) says the file is already in. */
export function importedToast(rowCount: number | null): string {
  return rowCount === null ? replayedText("") : `Đã nhập ${rowCount} ${IMPORT_UNIT}.`;
}
