/**
 * Câu chữ của tab "Phân quyền". Hàm thuần, không gọi mạng, không dựng DOM — nên kiểm được đúng
 * những ca dễ nói sai nhất.
 *
 * Cùng khuôn với `nhan-can-bo.ts`: chữ trên màn hình của một cơ quan nhà nước là thứ có người
 * phải trả lời, không phải chỗ để diễn đạt cho gọn.
 */

import type { identity_vaiTroCotRa } from "@/lib/api/schema.gen";

import type { ThieuTruc } from "./ma-tran-quyen";

/**
 * Đầu cột vai trò — MỘT số đếm, "{n} cán bộ", đúng câu prototype (`RolePermissionMatrix.tsx:147-149`).
 *
 * `staff_count` là mọi cán bộ đang giữ vai trò — đúng con số trang Người dùng đếm được bằng tay, để
 * hai màn hình không nói hai con số khác nhau về cùng một vai trò. Vế "… trong số đó có tài khoản
 * đang hoạt động" (`active_account_count`) đã bị bỏ theo quyết định của chủ đầu tư ngày 08/10/2026
 * (thẻ B, P13); hợp đồng vẫn phát trường ấy, màn hình này chỉ không in nó.
 */
export function nhanSoNguoiGiuVaiTro(cot: identity_vaiTroCotRa): string {
  return `${cot.staff_count} cán bộ`;
}

/** Nhãn phụ của vai trò lãnh đạo — `is_leader`. CHỈ là nhãn: xem `ma-tran-phan-quyen.tsx`. */
export const NHAN_LANH_DAO = "Lãnh đạo";

/**
 * Câu đọc được của một ô, dành cho trình đọc màn hình.
 *
 * Trên màn hình mỗi ô là một dấu hiệu (`✓` / `–`) vì 264 ô mà ô nào cũng có chữ thì không đọc nổi
 * ở bề rộng 320px. Nhưng dấu hiệu ấy phải có một câu đi kèm: HÌNH DẠNG chứ không phải màu là thứ
 * phân biệt hai trạng thái, và người dùng trình đọc màn hình cần nghe ra "Đã cấp" / "Chưa cấp"
 * thay vì một ký tự.
 */
export function nhanO(daCap: boolean): string {
  return daCap ? "Đã cấp" : "Chưa cấp";
}

/**
 * Câu giải thích khi ma trận không có trục để dựng — KHÔNG ĐỂ MỘT BẢNG RỖNG ĐỨNG ĐÓ.
 *
 * Ba câu khác nhau vì ba ca có ba người xử lý khác nhau, và cả ba đều nói việc kế tiếp phải làm
 * chứ không chỉ nói cái gì đang thiếu. Không câu nào gọi đây là lỗi: một xã vừa được khởi tạo mà
 * chưa có vai trò nào là một hệ thống đang chạy đúng.
 */
export function nhanChuaCauHinh(thieu: ThieuTruc): string {
  switch (thieu) {
    case "vaiTro":
      return (
        // No pointer to "Tạo tám vai trò mẫu": the owner removed that panel from this page (08/10/2026).
        "Đơn vị chưa có vai trò nào, nên ma trận chưa có cột nào để hiển thị. Liên hệ quản trị hệ " +
        "thống nếu đơn vị đã hoạt động mà danh sách vẫn trống."
      );
    case "danhMucQuyen":
      return (
        "Danh mục quyền chưa có khoá nào, nên ma trận chưa có hàng nào để hiển thị. Đây là danh mục " +
        "dùng chung của toàn hệ thống, không phải cấu hình riêng của đơn vị — báo cho quản trị hệ thống."
      );
    case "caHai":
      return (
        "Đơn vị chưa có vai trò nào và danh mục quyền cũng chưa có khoá nào, nên chưa dựng được ma " +
        "trận. Liên hệ quản trị hệ thống để kiểm tra việc khởi tạo đơn vị."
      );
  }
}

export const NUT_LUU = "Lưu";
export const NUT_HUY = "Huỷ";

/**
 * Tên đọc được của một ô bấm: "{tên vai trò} — {nhãn quyền}", vai trò trước như `title` của prototype
 * (`RolePermissionMatrix.tsx:231`). Trình đọc màn hình đọc riêng ô ấy, không kèm tiêu đề hàng và cột,
 * nên thiếu một nửa là cán bộ tick mà không biết mình đang cấp quyền gì cho ai.
 */
export function nhanOBatTat(nhanQuyen: string, tenVaiTro: string): string {
  return `${tenVaiTro} — ${nhanQuyen}`;
}

/** Toast after a column saved — the prototype's sentence (`RolePermissionMatrix.tsx:117`). */
export function savedToast(roleName: string): string {
  return `Đã lưu quyền của vai trò ${roleName}.`;
}

/** Tên đọc được của hai nút đầu cột — chữ "Lưu" đứng một mình thì tám cột nghe như nhau. */
export function nhanNutLuu(tenVaiTro: string): string {
  return `Lưu phân quyền của vai trò ${tenVaiTro}`;
}

export function nhanNutHuy(tenVaiTro: string): string {
  return `Huỷ thay đổi chưa lưu của vai trò ${tenVaiTro}`;
}

/**
 * Lý do cột của CHÍNH vai trò người đang đăng nhập không bấm được — câu #14 (open-questions),
 * cùng ý với câu máy chủ trả ở 403 `self_target_forbidden` (`service-identity/internal/http/quyen.go`).
 * Khoá cột chỉ là tiện dụng: máy chủ vẫn từ chối dù màn hình có khoá hay không.
 */
export const LY_DO_KHONG_TU_SUA =
  "Bạn đang giữ vai trò này nên không tự sửa phân quyền của nó được. Hãy nhờ một người quản trị khác của xã thực hiện.";

/** Chú thích bảng cho trình đọc màn hình — hai chế độ, hai câu. */
export const CHU_THICH_BANG_XEM =
  "Ma trận phân quyền của đơn vị: mỗi hàng là một quyền, mỗi cột là một vai trò. Bảng chỉ để xem.";
export const CHU_THICH_BANG_SUA =
  "Ma trận phân quyền của đơn vị: mỗi hàng là một quyền, mỗi cột là một vai trò. Mỗi cột được lưu riêng bằng nút Lưu ở đầu cột.";
