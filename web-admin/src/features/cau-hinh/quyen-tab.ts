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
import { coQuyen, QUYEN_PHAN_QUYEN, QUYEN_QUAN_LY_NGUOI_DUNG } from "@/lib/quyen";

export type QuyetDinhTab =
  | { hien: true }
  /** Đọc được quyền, và tài khoản không có khoá của tab ấy. Đặc tả §12.8: ẩn tab đó. */
  | { hien: false; vi: "khong-du-quyen" }
  /** Không đọc được quyền — phiên hết hạn, mạng hỏng, máy chủ lỗi. Vẫn là ẩn. */
  | { hien: false; vi: "khong-doc-duoc"; thongBao: string };

export function quyetDinhTabNguoiDung(ketQua: KetQua<identity_phienHienTaiRa>): QuyetDinhTab {
  return theoKhoaQuyen(ketQua, QUYEN_QUAN_LY_NGUOI_DUNG);
}

/**
 * Tab "Phân quyền" — `admin.role`. Cùng quy tắc, cùng nhánh FAIL CLOSED, khác đúng một khoá.
 *
 * Tab này CHỈ ĐỌC ma trận; không có tuyến ghi nào phía sau nó, nên `admin.role` ở đây mở ra một
 * màn hình xem chứ không mở ra quyền cấp phát (xem `ma-tran-phan-quyen.tsx`).
 */
export function quyetDinhTabPhanQuyen(ketQua: KetQua<identity_phienHienTaiRa>): QuyetDinhTab {
  return theoKhoaQuyen(ketQua, QUYEN_PHAN_QUYEN);
}

/**
 * Phần chung của hai quyết định trên: đọc được quyền chưa, và có ĐÚNG khoá ấy không.
 *
 * Riêng tư, và nhận một khoá duy nhất chứ không nhận một danh sách: một hàm công khai nhận danh
 * sách khoá là hàm mời người gọi truyền hai khoá vào rồi mở tab khi có bất kỳ khoá nào.
 */
function theoKhoaQuyen(ketQua: KetQua<identity_phienHienTaiRa>, khoa: string): QuyetDinhTab {
  // FAIL CLOSED: không đọc được danh sách quyền thì coi như KHÔNG có quyền. "Chưa rõ" không
  // được hành xử như "có" — trên đường cách ly thì không có giá trị mặc định nào (luật 1).
  if (!ketQua.ok) return { hien: false, vi: "khong-doc-duoc", thongBao: ketQua.thongBao };

  return coQuyen(ketQua.duLieu.permissions, khoa)
    ? { hien: true }
    : { hien: false, vi: "khong-du-quyen" };
}
