import type { PhienDaDoc } from "./phien-hien-tai";

/**
 * Khối người dùng trên đầu trang (`docs/ui-ux/15-phu-luc-giao-dien-chung.md §3`, vùng Phải).
 *
 * TÁCH KHỎI PHẦN DỰNG GIAO DIỆN vì quyết định ở đây có một ca không nhìn thấy bằng mắt: phiên
 * đọc không được. Nằm lẫn trong một component `.tsx` thì ca ấy không có bài test nào chạm
 * tới — mà đó đúng là ca sẽ xảy ra khi phiên vừa hết hạn, tức lúc người dùng đang làm việc.
 */
export type KhoiNguoiDung =
  /** Chưa đọc xong. Đầu trang chừa chỗ, không đoán. */
  | { hien: false; vi: "dang-doc" }
  /** Đọc xong và hỏng — phiên hết hạn, mạng lỗi, máy chủ lỗi. KHÔNG hiện tên nào. */
  | { hien: false; vi: "khong-doc-duoc" }
  | { hien: true; hoTen: string; chucVu: string };

/**
 * KHÔNG CÓ TÊN DỰ PHÒNG, và đó là toàn bộ điểm của hàm này.
 *
 * Đầu trang của một cơ quan nhà nước hiện sai tên người đang đăng nhập là một sự cố có người
 * phải trả lời — nặng hơn hẳn một khoảng trống. Nên khi không đọc được phiên, khối này biến
 * mất, chứ không hiện "Người dùng", không hiện tên cũ còn sót trong bộ nhớ, và không hiện một
 * chuỗi rỗng trông như đang tải.
 *
 * `canBoGon` cố ý KHÔNG có thư điện tử, dù bản prototype hiện nó dưới tên. Địa chỉ thư công vụ
 * là dữ liệu cá nhân (luật 3), và nó không cần thiết để một người nhận ra chính mình trên đầu
 * trang mình vừa đăng nhập.
 */
export function khoiNguoiDung(phien: PhienDaDoc): KhoiNguoiDung {
  if (phien === null) return { hien: false, vi: "dang-doc" };
  if (!phien.ok) return { hien: false, vi: "khong-doc-duoc" };

  const cb = phien.duLieu.staff;
  return { hien: true, hoTen: cb.full_name, chucVu: cb.position };
}
