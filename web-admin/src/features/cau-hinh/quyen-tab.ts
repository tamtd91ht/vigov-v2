/**
 * Quyết định một tab của màn Cấu hình có hiện hay không, tách khỏi phần dựng giao diện.
 *
 * VÌ SAO TÁCH: đây là chỗ duy nhất trong ứng dụng đọc `permissions` để ẩn/hiện, và nó phải
 * **đóng khi không chắc**. Nằm lẫn trong một component `.tsx` thì ca "không đọc được quyền"
 * không có bài test nào chạm tới — mà đó đúng là ca sẽ xảy ra khi phiên vừa hết hạn.
 *
 * NHẮC LẠI CHO NGƯỜI SỬA TỆP NÀY: các hàm ở đây là TIỆN DỤNG, không phải biện pháp. Một hàm trả
 * `true` sai thì hậu quả là một màn hình hiện lỗi 403 — dữ liệu không rời khỏi máy chủ, vì
 * `GET /api/v1/staff` kiểm `admin.user` và `GET /api/v1/role-permissions` kiểm `admin.role` trên
 * TỪNG yêu cầu ở phía máy chủ (luật 5, cấm #1).
 *
 * MỘT TAB, MỘT KHOÁ, MỘT HÀM — không có một hàm chung nhận danh sách khoá rồi "có khoá nào cũng
 * được". Quyền vào từng tab là từng khoá riêng (`docs/ui-ux/14-cau-hinh.md §12.8`), và một phép
 * hợp ở đây là đúng chỗ `admin.audit` mở được tab Phân quyền mà không ai thấy.
 */

import type { KetQua } from "@/lib/api/goi";
import type { identity_phienHienTaiRa } from "@/lib/api/schema.gen";
import {
  QUYEN_PHAN_QUYEN,
  QUYEN_QUAN_LY_NGUOI_DUNG,
  quyetDinhTheoKhoa,
  type QuyetDinhHien,
} from "@/lib/quyen";

/**
 * Cùng một hình dạng quyết định với mọi phần giao diện có cổng quyền — nay là một tên gọi khác
 * của `QuyetDinhHien` chứ không còn là một khai báo thứ hai. Giữ tên `QuyetDinhTab` để hai tab
 * hiện có không phải đổi, và vì ở màn Cấu hình thì "phần giao diện" đúng là một TAB (§12.8).
 */
export type QuyetDinhTab = QuyetDinhHien;

export function quyetDinhTabNguoiDung(ketQua: KetQua<identity_phienHienTaiRa>): QuyetDinhTab {
  return theoKhoaQuyen(ketQua, QUYEN_QUAN_LY_NGUOI_DUNG);
}

/**
 * Tab "Phân quyền" — `admin.role`. Cùng quy tắc, cùng nhánh FAIL CLOSED, khác đúng một khoá.
 *
 * Cùng một khoá mở cả việc XEM lẫn việc SỬA: máy chủ khai `admin.role` cho cả
 * `GET /api/v1/role-permissions` lẫn `PUT /api/v1/roles/{id}/permissions` (xem `coTheSuaPhanQuyen`).
 */
export function quyetDinhTabPhanQuyen(ketQua: KetQua<identity_phienHienTaiRa>): QuyetDinhTab {
  return theoKhoaQuyen(ketQua, QUYEN_PHAN_QUYEN);
}

/**
 * Phần chung của hai quyết định trên — nay nằm ở `lib/quyen.ts` vì nó có người dùng thứ ba và
 * thứ tư (`/giai-ngan`, `/phan-anh`). Đây chỉ còn là chỗ buộc mỗi tab vào ĐÚNG MỘT khoá.
 */
function theoKhoaQuyen(ketQua: KetQua<identity_phienHienTaiRa>, khoa: string): QuyetDinhTab {
  return quyetDinhTheoKhoa(ketQua, khoa);
}

/**
 * Ô ma trận có thành ô bấm được không — đúng khoá `admin.role`, tức khoá máy chủ kiểm ở `PUT`.
 *
 * TIỆN DỤNG, KHÔNG PHẢI BIỆN PHÁP (luật 5, cấm #1): trả `true` sai thì cán bộ bấm Lưu và nhận nguyên
 * câu 403 của máy chủ. Không đọc được phiên thì `false` — đóng khi không chắc.
 */
export function coTheSuaPhanQuyen(ketQua: KetQua<identity_phienHienTaiRa>): boolean {
  return quyetDinhTheoKhoa(ketQua, QUYEN_PHAN_QUYEN).hien;
}

/**
 * Mã vai trò (`vai_tro.ma`) của người đang đăng nhập, hoặc `null`.
 *
 * Phiên không phát id vai trò, chỉ phát `role.code`. Mã ấy là `UNIQUE (tenant_id, ma)`
 * (`service-identity/migrations/0001_init.sql:120`) và phiên với ma trận là của cùng một xã, nên
 * so mã với `code` của cột là so đúng một vai trò, không phải đoán. Dùng để khoá sẵn cột của chính
 * mình (#14) — máy chủ vẫn là bên từ chối (403 `self_target_forbidden`).
 */
export function maVaiTroCuaToi(ketQua: KetQua<identity_phienHienTaiRa>): string | null {
  if (!ketQua.ok) return null;
  return ketQua.duLieu.role?.code ?? null;
}
