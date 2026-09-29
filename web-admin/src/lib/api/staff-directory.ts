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

import { QUYEN_DUYET_GIA_HAN } from "@/lib/permissions";

import { docJSON, thamSoTheoHopDong, type KetQua } from "./request";
import type { identity_danhBaChonNguoiRa, identity_get_staff_directory } from "./schema.gen";

/**
 * Khoá được phép gửi vào `permission` — ĐÚNG MỘT, và kiểu hẹp hơn kiểu hợp đồng (`string`) có chủ ý.
 *
 * Máy chủ chỉ nhận `task.extend` (`service-identity/internal/http/danh_ba_chon_nguoi.go:95-109`,
 * `quyenLanhDaoGiaoViec`); mọi khoá khác là 400. Lý do nằm ở phía máy chủ: một bộ lọc nhận khoá bất
 * kỳ là một tuyến `AnyAuthenticated` liệt kê ai cầm `admin.user` trong xã — đúng thứ
 * `GET /api/v1/role-permissions` giữ sau `admin.role`. Để `string` ở đây thì một màn hình gửi khoá
 * khác chỉ biết mình sai khi ô chọn hiện câu 400; `tsc` chặn được sớm hơn thế.
 */
export type QuyenLocDanhBa = typeof QUYEN_DUYET_GIA_HAN;

/**
 * Dựng đường dẫn. Tách khỏi lời gọi mạng để kiểm được mà không cần thay `fetch`.
 *
 * `unit` và `permission` đi qua `thamSoTheoHopDong`, nên máy chủ đổi tên tham số là `tsc` đỏ ở đây.
 * Không có bộ phận thì tham số VẮNG MẶT HẲN — `unit=` rỗng không có nghĩa "mọi bộ phận" ở mọi tuyến.
 * Không lọc quyền thì `permission` cũng vắng mặt: máy chủ TỪ CHỐI `permission=` rỗng thay vì đọc nó
 * thành "không lọc" (`danh_ba_chon_nguoi.go:115-118`).
 */
export function duongDanDanhBaChonNguoi(boPhanID?: string, quyen?: QuyenLocDanhBa): string {
  const duongDan: identity_get_staff_directory["duongDan"] = "/api/v1/staff-directory";
  const truyVan = new URLSearchParams();
  const dat = thamSoTheoHopDong<identity_get_staff_directory["truyVan"]>(truyVan);
  dat("unit", boPhanID?.trim());
  dat("permission", quyen);
  const chuoi = truyVan.toString();
  return chuoi === "" ? duongDan : `${duongDan}?${chuoi}`;
}

/**
 * GET /api/v1/staff-directory — cả xã, hoặc chỉ những người thuộc đúng một bộ phận, hoặc chỉ những
 * người đang cầm một khoá quyền trong xã.
 *
 * Lọc theo `unit` là so KHỚP ĐÚNG `department_id`: người chưa thuộc bộ phận nào không nằm trong
 * danh sách lọc của bộ phận nào cả.
 *
 * `quyen` CHỈ DÀNH CHO Ô `Lãnh đạo giao việc`: người được ghi ở đó là người DUYỆT đề nghị lùi hạn
 * (ADR 0038), nên gợi một người không cầm `task.extend` là gợi một người không bao giờ duyệt được. Các
 * ô chọn khác (người thực hiện, chuyên viên theo dõi) không truyền nó — ai trong xã cũng nhận việc được.
 */
export function layDanhBaChonNguoi(
  boPhanID?: string,
  quyen?: QuyenLocDanhBa,
): Promise<KetQua<identity_danhBaChonNguoiRa>> {
  return docJSON<identity_danhBaChonNguoiRa>(duongDanDanhBaChonNguoi(boPhanID, quyen));
}
