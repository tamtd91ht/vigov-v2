/**
 * Chọn một NĂM để xem — dùng chung cho màn giải ngân (năm ngân sách) và cho lịch làm việc của xã
 * (năm nghỉ lễ, năm làm bù).
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * VÌ SAO WEB PHẢI NÊU NĂM, VÀ VÌ SAO KHÔNG ĐƯỢC NÊU NÓ Ở CHỖ NGƯỜI DÙNG KHÔNG THẤY.
 *
 * Cả ba tuyến đọc theo năm đều BẮT BUỘC tham số `year` và CỐ Ý không lấy năm hiện tại làm mặc
 * định ở máy chủ — lý do ghi thẳng trên mã: một mặc định ở đó quyết định "báo cáo tiền của năm
 * nào" một cách vô hình, và đúng 0h ngày 01/01 màn hình lặng lẽ đổi năm mà không dòng mã nào
 * thay đổi (`service-finance/internal/http/du_an.go`, `service-identity/.../ngay_nghi_le.go`).
 *
 * Nên năm phải do phía gọi nêu ra — và phía gọi ở đây là MÀN HÌNH, chỗ năm ấy hiện ra thành chữ
 * và đổi được bằng một ô chọn. Đó là khác biệt duy nhất giữa "một mặc định vô hình" và "một giá
 * trị ban đầu người dùng đang nhìn thấy".
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * KHÔNG CHÉP LẠI CẬN NĂM CỦA MÁY CHỦ (2000–2100). Hai hằng ấy nằm trong mã Go và không có trong
 * hợp đồng, nên chép sang đây là dựng một bản sao không ai canh. Thay vào đó ô chọn chỉ phát ra
 * một cửa sổ hẹp quanh năm nay — luôn nằm sâu bên trong mọi cận hợp lý — và máy chủ vẫn là chỗ
 * từ chối một năm nó không nhận.
 */

/** Số năm quá khứ đứng trong ô chọn, cùng độ dài với ví dụ của đặc tả `06-giai-ngan §2`. */
const SO_NAM_QUA_KHU = 3;

/**
 * Năm theo đồng hồ của MÁY NGƯỜI DÙNG, chỉ để làm giá trị ban đầu cho ô chọn.
 *
 * Tách thành hàm để mọi phép kiểm truyền vào một năm cố định thay vì phụ thuộc vào ngày chạy
 * test — một ca test đọc đồng hồ thật là một ca đổi màu theo ngày.
 *
 * KHÔNG MỘT PHÉP TÍNH HẠN NÀO DỰA VÀO ĐỒNG HỒ NÀY. Nó chọn cửa sổ để XEM; mọi hạn xử lý đều do
 * máy chủ tính theo giờ làm việc của xã (ADR 0007, luật 10 bất biến 4).
 */
export function namTheoDongHoMay(): number {
  return new Date().getFullYear();
}

/**
 * Các năm ô chọn đưa ra, **giảm dần**, năm sau đứng đầu.
 *
 * Năm sau có mặt vì kế hoạch vốn và lịch nghỉ lễ đều được lập TRƯỚC khi năm ấy tới: thông báo
 * nghỉ lễ của năm sau thường về vào quý IV, và cán bộ phải mở xem được nó.
 */
export function danhSachNam(nam: number): number[] {
  const ra: number[] = [];
  for (let n = nam + 1; n >= nam - SO_NAM_QUA_KHU; n--) ra.push(n);
  return ra;
}
