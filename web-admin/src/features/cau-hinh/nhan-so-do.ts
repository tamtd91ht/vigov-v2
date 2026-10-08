/**
 * Câu chữ của tab "Sơ đồ tổ chức" — spec Cấu hình `03-so-do-to-chuc.md` (ADR 0079). Hằng thuần,
 * không gọi mạng, không dựng DOM.
 *
 * Chữ trên màn hình của một cơ quan nhà nước là thứ có người phải trả lời: nhãn, tiêu đề và câu báo
 * lấy NGUYÊN VĂN theo spec, không tự đặt lại.
 *
 * `NUT_LUU`, `NUT_HUY`, `LOI_THU_TU`, `CHUA_CO_THAY_DOI` còn được tab Thôn / Tổ dân phố đọc
 * (`tab-thon-to-dan-pho.tsx`, `residential-unit-form.ts`) — đổi giá trị là đổi cả màn ấy.
 */

export const TIEU_DE_SO_DO = "Sơ đồ tổ chức";

// No leading symbol: the screen draws a lucide `Plus` beside the word (ADR 0068 §2).
export const NUT_THEM_BO_PHAN = "Thêm bộ phận";
export const ADD_BUTTON = "Thêm";
export const NUT_LUU = "Lưu";
export const NUT_HUY = "Huỷ";

/**
 * Hover titles of the three icon buttons on a card — the spec's words, verbatim. The accessible name
 * (`aria-label`) is a separate, longer sentence that names the unit: a screen reader reads a button
 * apart from its card, and ten buttons all called "Sửa bộ phận" say nothing about which unit.
 */
export const TITLE_ADD_CHILD = "Thêm bộ phận con";
export const TITLE_EDIT = "Sửa bộ phận";
export const TITLE_DELETE = "Xoá bộ phận";

export function nhanNutThemCon(tenBoPhan: string): string {
  return `Thêm bộ phận con của ${tenBoPhan}`;
}

export function nhanNutSua(tenBoPhan: string): string {
  return `Sửa bộ phận ${tenBoPhan}`;
}

/**
 * Số cán bộ của một bộ phận — `staff_count` của máy chủ (ADR 0079 "Số cán bộ"). `0 cán bộ` được in
 * ra, không bỏ trống: một bộ phận chưa có ai là một sự thật cán bộ cần thấy.
 */
export function nhanSoCanBo(soCanBo: number): string {
  return `${soCanBo} cán bộ`;
}

/** Dialog titles. "+ con" opens the same "Thêm bộ phận" dialog with the parent preselected. */
export const ADD_TITLE = "Thêm bộ phận";
export const EDIT_TITLE = "Sửa bộ phận";

/** The dialog's description line. True: the server derives the code from the name when none is sent. */
export const DIALOG_DESCRIPTION = "Mã bộ phận do hệ thống tự sinh từ tên.";

export const O_TEN = "Tên bộ phận";
export const NAME_PLACEHOLDER = "Ví dụ: Bộ phận Một cửa";
export const O_CHA = "Trực thuộc";
/** Kept although spec 03 drops it: owner decision 3 of ADR 0079 ("ô Thứ tự" — giữ). */
export const O_THU_TU = "Thứ tự";

/** The "no parent" choice of `Trực thuộc` — a unit at the top level, directly under the committee. */
export const CHON_KHONG_CO_CHA = "— Trực thuộc Uỷ ban —";

/** In-place error when `Tên` is blank. The server never sees that body. */
export const NAME_REQUIRED = "Vui lòng nhập tên bộ phận.";

/** Lỗi tại chỗ khi ô `Thứ tự` không phải một số nguyên. Máy chủ không bao giờ thấy thân ấy. */
export const LOI_THU_TU = "Thứ tự phải là một số nguyên, ví dụ: 3.";

/** Biểu mẫu sửa được gửi mà không đổi gì — không gửi yêu cầu nào. */
export const CHUA_CO_THAY_DOI = "Chưa có thay đổi nào để lưu.";

/** Toasts after a write (spec 03). */
export const ADDED_TOAST = "Đã thêm bộ phận mới.";
export const SAVED_TOAST = "Đã lưu bộ phận.";

export const DANG_TAI = "Đang tải sơ đồ tổ chức của đơn vị…";

/**
 * Empty tree — one sentence for every account (spec 03). A read-only account sees no "Thêm bộ phận"
 * button above it, so the sentence promises nothing it cannot do.
 */
export const EMPTY_TREE = "Chưa khai báo bộ phận nào.";
