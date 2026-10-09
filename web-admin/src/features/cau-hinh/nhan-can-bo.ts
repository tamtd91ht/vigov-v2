/**
 * Câu chữ hiện trên một dòng danh bạ cán bộ. Hàm thuần, không gọi mạng, không dựng DOM —
 * nên kiểm được đúng những ca dễ hiện sai nhất.
 *
 * Nhãn lấy theo `docs/ui-ux/14-cau-hinh.md §3`, không tự đặt lại: chữ trên màn hình của một cơ
 * quan nhà nước là thứ có người phải trả lời, không phải chỗ để diễn đạt cho gọn.
 */

import { CHOOSE_XLSX_BUTTON, IMPORT_DIALOG_DESCRIPTION, IMPORT_SUBMIT } from "./excel-import-flow";
import type { KetTra } from "./tra-danh-muc";

/**
 * Múi giờ của toàn hệ thống: `Asia/Ho_Chi_Minh`
 * (`docs/ui-ux/00-tong-quan-he-thong.md:270`).
 *
 * GHIM MÚI GIỜ, KHÔNG DÙNG MÚI GIỜ CỦA MÁY: một mốc đăng nhập là dữ liệu hành chính. Để nó
 * theo cài đặt của từng máy thì hai cán bộ mở cùng một bản ghi đọc ra hai giờ khác nhau, và
 * khi có thanh tra thì không ai nói được giờ nào là giờ đúng. Đây là hằng số của cả nước, không
 * phải giá trị theo xã — nên nó nằm trong mã, không nằm trong cấu hình xã (luật 1, bất biến 10
 * nói về giá trị RIÊNG của xã).
 */
const MUI_GIO = "Asia/Ho_Chi_Minh";

const DINH_DANG_GIO = new Intl.DateTimeFormat("vi-VN", {
  timeZone: MUI_GIO,
  hour: "2-digit",
  minute: "2-digit",
  second: "2-digit",
  hour12: false,
});

const DINH_DANG_NGAY = new Intl.DateTimeFormat("vi-VN", {
  timeZone: MUI_GIO,
  day: "numeric",
  month: "numeric",
  year: "numeric",
});

/** `11:10:38 16/9/2026` — đúng thứ tự giờ trước, ngày sau như đặc tả §3. */
function mocThoiGian(giaTri: string): string | null {
  const luc = new Date(giaTri);
  if (Number.isNaN(luc.getTime())) return null;
  return `${DINH_DANG_GIO.format(luc)} ${DINH_DANG_NGAY.format(luc)}`;
}

/**
 * Nhãn cột "Đăng nhập gần nhất".
 *
 * `null` KHÔNG PHẢI MỘT LỖI, và không được gộp với lỗi. Hợp đồng khai `last_login_at` là
 * `string | null`, và `null` ở đây là một điều máy chủ **khẳng định**: người này chưa đăng nhập
 * lần nào. Đó chính là dòng người quản trị đi tìm trên màn hình này — tài khoản đã cấp mà chưa
 * ai dùng. Hiện "Chưa đăng nhập" theo đặc tả §3.
 *
 * Ca thứ ba — có giá trị nhưng không đọc được thành mốc thời gian — là hỏng hợp đồng, không
 * phải một trạng thái nghiệp vụ, nên nó nói ra bằng một câu KHÁC. Gộp nó vào "Chưa đăng nhập"
 * là báo sai một sự thật về một tài khoản.
 */
export function nhanDangNhapGanNhat(giaTri: string | null): string {
  if (giaTri === null) return "Chưa đăng nhập";
  return mocThoiGian(giaTri) ?? "Mốc thời gian không đọc được";
}

/** Nhãn cột "Ngày tạo" — cột sắp xếp được thứ hai, nên nó phải hiện ra để mũi tên có nghĩa. */
export function nhanNgayTao(giaTri: string): string {
  return mocThoiGian(giaTri) ?? "Mốc thời gian không đọc được";
}

/**
 * Nhãn chip "Trạng thái" — `active`.
 *
 * `active` và `has_account` là HAI câu hỏi khác nhau, và đọc cái này thay cho cái kia là báo
 * một dòng danh bạ thành một tài khoản đang hoạt động — một con số xã gửi lên cấp trên
 * (`service-identity/internal/http/can_bo.go`, chú thích trên `HasAccount`). Vì vậy hai nhãn,
 * hai hàm, không một hàm nào trộn cả hai.
 */
export function nhanTrangThai(active: boolean): string {
  // The prototype's two words (`UserTable`, ADR 0068 lần 5). "Tạm khoá", not "Đã ngừng": `active` is what
  // the lockout route writes, and the row keeps its "Mở khoá tài khoản" action.
  return active ? "Đang hoạt động" : "Tạm khoá";
}

/** Nhãn cột "Tài khoản" — `has_account`. Xem chú thích của `nhanTrangThai`. */
export function nhanTaiKhoan(coTaiKhoan: boolean): string {
  return coTaiKhoan ? "Có tài khoản" : "Chỉ trong danh bạ";
}

/** Marker for a staff row with no email (optional since 4cf87b6; the server returns `""`). */
export const NO_EMAIL_MARKER = "—";

/**
 * The muted sub-line under the name on `/nguoi-dung` when the row has no email (user 09/10/2026). The
 * list says it in words because the email is the login name: "no email" is why the row has no account.
 */
export const NO_EMAIL_SUBLINE = "Chưa có thư điện tử";

/** The `/nguoi-dung` sub-line under the name: the email, or `NO_EMAIL_SUBLINE`. */
export function emailSubline(email: string): string {
  return email.trim() === "" ? NO_EMAIL_SUBLINE : email;
}

/** The email as shown in the list and the detail panel. Blank never renders as an empty cell. */
export function emailLabel(email: string): string {
  return email.trim() === "" ? NO_EMAIL_MARKER : email;
}

/**
 * Whether "Cấp tài khoản" can succeed for this row. The email IS the login name, so the server
 * answers 409 `staff_has_no_email` without one. This is UX only — the server still decides.
 */
export function canIssueAccount(email: string): boolean {
  return email.trim() !== "";
}

/** Shown next to the disabled "Cấp tài khoản" button, and read by screen readers via `aria-describedby`. */
export const NO_EMAIL_ACCOUNT_REASON =
  "Chưa cấp được tài khoản: cán bộ này chưa có thư điện tử (thư điện tử là tên đăng nhập). " +
  "Hãy sửa hồ sơ và thêm thư điện tử trước.";

/**
 * Nhãn cột "Bộ phận" — tra `department_id` trong danh mục bộ phận của xã.
 *
 * NĂM CA, NĂM CÂU KHÁC NHAU, và không câu nào là ô trống. Đây là chỗ dễ nói sai nhất trên màn
 * hình này: "chưa phân bộ phận" và "id không tra được" cùng cho ra một ô trống nếu không ai
 * phân biệt, trong khi ca đầu là việc của người quản lý nhân sự (phân bộ phận cho cán bộ) và ca
 * sau là một dòng dữ liệu lệch không ai biết mà sửa. Xem `tra-danh-muc.ts` để biết năm ca ấy
 * đến từ đâu.
 *
 * "Bộ phận" là từ của `kb/00-foundation/ubiquitous-language.md` — cây này chứa cả Đảng uỷ,
 * HĐND và UBMTTQ, nên không gọi là "phòng ban".
 */
export function nhanBoPhan(ket: KetTra): string {
  switch (ket.loai) {
    case "dangDoc":
      return "Đang tải…";
    case "chuaGan":
      return "Chưa phân bộ phận";
    case "coTen":
      return ket.ten;
    case "khongTraDuoc":
      return "Không tra được trong danh mục";
    case "khongCoDanhMuc":
      return "Chưa đọc được danh mục bộ phận";
  }
}

/**
 * Nhãn cột "Vai trò" — tra `role_id` trong danh mục vai trò của xã.
 *
 * Cùng năm ca với cột Bộ phận nhưng KHÔNG dùng chung câu chữ: "Chưa gán vai trò" là một trạng
 * thái có hệ quả riêng — tài khoản ấy đăng nhập được nhưng không có quyền nào, và đó đúng là
 * dòng người quản trị đi tìm. Gộp hai cột vào một câu chung ("Chưa có") là xoá mất hệ quả ấy.
 *
 * CỘT NÀY KHÔNG HIỆN QUYỀN CỦA VAI TRÒ, chỉ hiện tên — đúng chủ ý đã ghi trong lý do phân quyền
 * của tuyến `GET /api/v1/roles`: danh mục vai trò là thứ mọi tài khoản đã đăng nhập đọc được, còn
 * ai đang giữ khoá nào thì đòi `admin.role`. Muốn xem quyền của từng vai trò thì sang tab Phân
 * quyền (`ma-tran-phan-quyen.tsx`, `GET /api/v1/role-permissions`) — hai tuyến, hai quyền, cố ý.
 */
export function nhanVaiTro(ket: KetTra): string {
  switch (ket.loai) {
    case "dangDoc":
      return "Đang tải…";
    case "chuaGan":
      return "Chưa gán vai trò";
    case "coTen":
      return ket.ten;
    case "khongTraDuoc":
      return "Không tra được trong danh mục";
    case "khongCoDanhMuc":
      return "Chưa đọc được danh mục vai trò";
  }
}

/* ---- `/nguoi-dung` in the prototype's shape (owner decisions 08/10/2026, fix-web-admin card A) ---- */

/** Search box name and placeholder — the prototype's words verbatim (`UserTable.tsx`). They are TRUE
 * since 4b0b9ce3: the server matches email and unit name as well as name, position and the phones.
 * A placeholder promising a field the server does not search sends an officer to an empty list and the
 * wrong conclusion, so the two move together. */
export const SEARCH_LABEL = "Tìm cán bộ";
export const SEARCH_PLACEHOLDER = "Tìm theo tên, thư điện tử, bộ phận…";

/** One sentence for both "no staff" and "no match" — the prototype's (`UserTable.tsx`). */
export const EMPTY_STAFF_LIST = "Không có cán bộ nào khớp điều kiện tìm kiếm.";

/** Row buttons' hover titles (the person goes in `aria-label`). */
export const EDIT_ACCOUNT_BUTTON = "Sửa tài khoản";
export const DELETE_ACCOUNT_BUTTON = "Xoá tài khoản";

/**
 * Why Trash2 is disabled on a row holding a sign-in account. The server refuses it anyway (409
 * `staff_has_account`, #10): retirement or transfer is a LOCK, so the name stays readable on every
 * record the person handled.
 */
export const DELETE_BLOCKED_REASON = "Đang có tài khoản đăng nhập nên không xoá được — hãy khoá tài khoản.";

/** The page's refusal for an account without `admin.user` (prototype `AccountWorkspace.tsx`, `Denied`). */
export const USERS_DENIED =
  "Tài khoản của bạn không có quyền quản lý tài khoản người dùng. Liên hệ Chánh Văn phòng hoặc quản " +
  "trị viên của đơn vị nếu cần.";

/**
 * The line under the table. `total` is the commune's register size (`GET /api/v1/staff-counts`) when no
 * search is applied, the search's match count (`POST /api/v1/staff-count-queries`) while one is; `null`
 * when that read failed or has not answered yet — the line then says only what is on screen.
 */
export function staffCountLine(shown: number, total: number | null): string {
  return total === null ? `Hiển thị ${shown} cán bộ.` : `Hiển thị ${shown}/${total} cán bộ.`;
}

/* ---- `/nguoi-dung` after the user's decisions of 09/10/2026 ---------------------------------------- */

/**
 * The `Điện thoại` cell (user 09/10/2026, prototype's single column): the personal mobile when there is
 * one, else the office phone, else "—". VALUES EXACTLY AS THE SERVER SENT THEM — whatever masking it
 * applied per field stays (Decree 13, open question #16); nothing here masks or unmasks. `kind` says
 * which of the two numbers is shown, so the cell can still name its legal status (#16) to whoever
 * hovers or reads it with a screen reader.
 */
export function phoneCell(mobile: string, office: string): { text: string; kind: string | null } {
  if (mobile.trim() !== "") return { text: mobile, kind: "Di động cá nhân" };
  if (office.trim() !== "") return { text: office, kind: "Máy bàn cơ quan" };
  return { text: "—", kind: null };
}

/**
 * Long reason of the account badge, in its `title` and in visually hidden text. The badge itself says
 * only `nhanTaiKhoan(has_account)`, so the row stays one line (spec P0 #2: rows ≤ 56px).
 */
export function accountBadgeReason(hasAccount: boolean, email: string): string {
  if (hasAccount) return "Đã cấp tài khoản đăng nhập; thư điện tử là tên đăng nhập.";
  if (!canIssueAccount(email)) return NO_EMAIL_ACCOUNT_REASON;
  return "Chưa cấp tài khoản đăng nhập. Mở menu ⋯ ở cuối dòng và chọn Cấp tài khoản.";
}

/** Accessible name and tooltip of the row's `⋯` menu. */
export function moreActionsLabel(fullName: string): string {
  return `Thao tác khác: ${fullName}`;
}

/** Second line of the disabled `Cấp tài khoản` menu item — why it cannot be chosen. */
export const NO_EMAIL_MENU_HINT = "Chưa có thư điện tử — sửa hồ sơ để thêm.";

/** Add dialog — the prototype's words (`UserFormDialog.tsx`), user 09/10/2026. */
export const ADD_ACCOUNT_TITLE = "Thêm tài khoản cán bộ";
export const ADD_ACCOUNT_DESCRIPTION = "Cán bộ dùng thư điện tử này để đăng nhập.";
export const ADD_ACCOUNT_BUTTON = "Thêm tài khoản";

/** Edit dialog: the staff code as the sub-line under the title (#15: shown, never an input). */
export function staffCodeLine(code: string): string {
  return `Mã cán bộ: ${code}`;
}

/**
 * Added with an email, but the server's answer was a REPLAY of an earlier add (no row in it), so the
 * account could not be issued in the same step. Says what to do next; nothing was lost.
 */
export const ADD_GRANT_NOT_CHAINED =
  "Đã thêm vào danh bạ nhưng chưa cấp tài khoản. Mở menu ⋯ ở dòng của cán bộ và chọn Cấp tài khoản.";

/** Field labels of the account form that differ from the shared directory form. */
export const MOBILE_LABEL = "Điện thoại (di động)";
export const OFFICE_PHONE_LABEL = "Máy bàn cơ quan";
export const ROLE_LABEL = "Vai trò";
/** `""` is a real value of `PUT .../role` — somebody can hold no role — so it is a choice of its own. */
export const NO_ROLE_OPTION = "Không giữ vai trò nào";
export const MISSING_ROLE_OPTION = "Giá trị đang lưu — không còn trong danh mục";
export const NO_ROLES_YET = "Chưa có vai trò nào để gán.";

/** Under the disabled email of a row that holds an account: the email is the login name. */
export const EMAIL_LOCKED_HINT = "Không đổi được thư điện tử đăng nhập.";

/**
 * Under the disabled role choice of the signed-in officer's own row (#14). UX only: the server refuses
 * the change anyway (403 `self_target_forbidden`).
 */
export const OWN_ROLE_HINT = "Bạn không tự đổi vai trò của chính mình được. Hãy nhờ một người quản trị khác.";

/**
 * The profile was saved but the role change that followed was refused. The server's sentence is kept
 * verbatim after the prefix (#13/#14 reasons) — the prefix only says the first half already happened,
 * so the officer does not retype the profile.
 */
export function roleChangeFailed(serverSentence: string): string {
  return `Đã lưu hồ sơ, nhưng chưa đổi được vai trò: ${serverSentence}`;
}

/* ---- staff Excel import dialog (user 09/10/2026, prototype `ExcelImportDialog`) -------------------- */

export const STAFF_IMPORT_TITLE = "Nhập người dùng từ Excel";
// The staff dialog is now the shared one (`excel-import-dialog.tsx`); its three fixed words live there.
export const STAFF_IMPORT_DESCRIPTION = IMPORT_DIALOG_DESCRIPTION;
export const STAFF_IMPORT_CHOOSE_FILE = CHOOSE_XLSX_BUTTON;
export const STAFF_IMPORT_SUBMIT = IMPORT_SUBMIT;
export const STAFF_IMPORT_ERRORS_HEADING = "Tệp có lỗi — chưa cán bộ nào được tạo. Sửa các dòng dưới đây rồi nhập lại.";

/** Empty text cells read "—" (prototype), never an empty cell. */
export function orDash(text: string): string {
  return text.trim() === "" ? "—" : text;
}

/**
 * The `Bộ phận` cell: "—" for a row with no unit (prototype), every other case as `nhanBoPhan` says
 * it — a lookup failure must still read differently from "no unit".
 */
export function unitCellLabel(lookup: KetTra): string {
  return lookup.loai === "chuaGan" ? "—" : nhanBoPhan(lookup);
}
