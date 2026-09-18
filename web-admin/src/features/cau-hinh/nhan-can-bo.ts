/**
 * Câu chữ hiện trên một dòng danh bạ cán bộ. Hàm thuần, không gọi mạng, không dựng DOM —
 * nên kiểm được đúng những ca dễ hiện sai nhất.
 *
 * Nhãn lấy theo `docs/ui-ux/14-cau-hinh.md §3`, không tự đặt lại: chữ trên màn hình của một cơ
 * quan nhà nước là thứ có người phải trả lời, không phải chỗ để diễn đạt cho gọn.
 */

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
  return active ? "Đang hoạt động" : "Đã ngừng";
}

/** Nhãn cột "Tài khoản" — `has_account`. Xem chú thích của `nhanTrangThai`. */
export function nhanTaiKhoan(coTaiKhoan: boolean): string {
  return coTaiKhoan ? "Có tài khoản" : "Chỉ trong danh bạ";
}

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
