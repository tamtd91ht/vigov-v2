/**
 * `GET /api/v1/staff-directory` — danh bạ CHỌN NGƯỜI NHẬN VIỆC của xã (dịch vụ `identity`).
 *
 * KHÁC HẲN `GET /api/v1/staff` (`lib/api/can-bo.ts`): tuyến ấy là SỔ QUẢN TRỊ, đòi `admin.user` —
 * một khoá cấu hình hệ thống mà gần như không cán bộ đang trực nào có. Tuyến này là
 * `AnyAuthenticated`: mọi cán bộ đã đăng nhập của CHÍNH xã ấy đọc được, và đổi lại nó chỉ trả bốn
 * trường — mã cán bộ · họ tên · chức vụ · bộ phận. Không `id`, không số điện thoại, không email
 * (`service-identity/internal/http/danh_ba_chon_nguoi.go`). Chỉ người có tài khoản đang hoạt động.
 *
 * `code` LÀ THỨ GỬI LẠI MÁY CHỦ khi giao việc — mã NGHIỆP VỤ (`CB-00123`), không phải ULID nội bộ.
 * Luật nắm giữ ở `service-petitions` so `can_bo_xu_ly_id` với `Principal.Ma`
 * (`service-petitions/internal/app/xu_ly_phan_anh.go:184-212`); gửi một id nội bộ thì phép so ấy
 * không bao giờ khớp và người được giao không tiến được phiếu của chính mình — lặng lẽ.
 *
 * KHÔNG PHÂN TRANG: cả danh sách hoặc một lời từ chối (máy chủ từ chối thay vì cắt bớt, vì một ô
 * chọn thiếu một đồng nghiệp là việc giao nhầm người mà không gì trên màn hình cho thấy).
 */

import { docJSON, thamSoTheoHopDong, type KetQua } from "./goi";
import type { identity_danhBaChonNguoiRa, identity_get_staff_directory } from "./schema.gen";

/**
 * Dựng đường dẫn. Tách khỏi lời gọi mạng để kiểm được mà không cần thay `fetch`.
 *
 * `unit` đi qua `thamSoTheoHopDong`, nên máy chủ đổi tên tham số là `tsc` đỏ ở đây. Không có bộ
 * phận thì tham số VẮNG MẶT HẲN — `unit=` rỗng không có nghĩa "mọi bộ phận" ở mọi tuyến.
 */
export function duongDanDanhBaChonNguoi(boPhanID?: string): string {
  const duongDan: identity_get_staff_directory["duongDan"] = "/api/v1/staff-directory";
  const truyVan = new URLSearchParams();
  thamSoTheoHopDong<identity_get_staff_directory["truyVan"]>(truyVan)("unit", boPhanID?.trim());
  const chuoi = truyVan.toString();
  return chuoi === "" ? duongDan : `${duongDan}?${chuoi}`;
}

/**
 * GET /api/v1/staff-directory — cả xã, hoặc chỉ những người thuộc đúng một bộ phận.
 *
 * Lọc theo `unit` là so KHỚP ĐÚNG `department_id`: người chưa thuộc bộ phận nào không nằm trong
 * danh sách lọc của bộ phận nào cả.
 */
export function layDanhBaChonNguoi(boPhanID?: string): Promise<KetQua<identity_danhBaChonNguoiRa>> {
  return docJSON<identity_danhBaChonNguoiRa>(duongDanDanhBaChonNguoi(boPhanID));
}
