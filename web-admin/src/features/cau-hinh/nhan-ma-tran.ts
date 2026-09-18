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
 * Đầu cột vai trò — HAI SỐ ĐẾM, và số thứ hai là TẬP CON của số thứ nhất.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * VÌ SAO HAI SỐ CHỨ KHÔNG PHẢI MỘT (người dùng chốt, và hợp đồng phát ra đúng hai trường):
 *
 *   `staff_count`           mọi cán bộ đang giữ vai trò này — ĐÚNG con số tab Người dùng đếm
 *                           được bằng tay. Không lọc theo tài khoản, để hai màn hình cạnh nhau
 *                           không nói hai con số khác nhau về cùng một vai trò.
 *   `active_account_count`  trong số đó, ai có tài khoản và tài khoản đang mở. Đây là những
 *                           người mà một ô của cột này thật sự tác động tới.
 *
 * Một số thì luôn có một câu đố: "3 cán bộ" không cho biết một ô tích trên cột ấy chạm tới mấy
 * người, và một vai trò `3 · 0` là cột mà mọi dấu tích không tới được ai — đúng ca không nhìn
 * thấy được sau một con số duy nhất (`service-identity/internal/domain/quyen.go`).
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * CÂU CHỮ PHẢI NÓI RÕ QUAN HỆ TẬP CON, nên "1 TRONG SỐ ĐÓ" chứ không phải một nhãn đứng riêng:
 * hai con số đặt cạnh nhau mà không có chữ "trong số đó" thì đọc thành hai phép đo khác nhau, và
 * người đọc sẽ cộng chúng lại.
 *
 * TỪ VỰNG MƯỢN NGUYÊN TAB NGƯỜI DÙNG — "có tài khoản" (`has_account`) và "đang hoạt động"
 * (`active`) là đúng hai cột của bảng danh bạ (`nhan-can-bo.ts`), và đúng hai điều kiện máy chủ
 * đếm. Đặt ra một từ thứ ba ở đây là mời cán bộ đi tìm xem nó khác gì hai từ kia.
 */
export function nhanSoNguoiGiuVaiTro(cot: identity_vaiTroCotRa): string {
  // KHÔNG AI GIỮ VAI TRÒ NÀY: một câu, không phải "0 cán bộ · 0 trong số đó…". Số thứ hai là tập
  // con của một tập rỗng nên nó không nói thêm được gì, và câu ghép ấy đọc như tiếng máy.
  // Điều kiện đòi CẢ HAI số bằng 0: nếu có ngày máy chủ trả một tập con lớn hơn tập cha thì đó là
  // dữ liệu lệch, và nó phải hiện ra nguyên hai con số chứ không bị câu này nuốt mất.
  if (cot.staff_count === 0 && cot.active_account_count === 0) {
    return "Chưa có cán bộ nào giữ vai trò này";
  }
  return `${cot.staff_count} cán bộ · ${cot.active_account_count} trong số đó có tài khoản đang hoạt động`;
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
        "Đơn vị chưa có vai trò nào, nên ma trận chưa có cột nào để hiển thị. Vai trò được lập khi " +
        "đơn vị được khởi tạo — liên hệ quản trị hệ thống nếu đơn vị đã hoạt động mà danh sách vẫn trống."
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
