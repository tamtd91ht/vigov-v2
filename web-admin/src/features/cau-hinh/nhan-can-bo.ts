/**
 * Câu chữ hiện trên một dòng danh bạ cán bộ. Hàm thuần, không gọi mạng, không dựng DOM —
 * nên kiểm được đúng những ca dễ hiện sai nhất.
 *
 * Nhãn lấy theo `docs/ui-ux/14-cau-hinh.md §3`, không tự đặt lại: chữ trên màn hình của một cơ
 * quan nhà nước là thứ có người phải trả lời, không phải chỗ để diễn đạt cho gọn.
 */

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
