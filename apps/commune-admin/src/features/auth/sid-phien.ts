/**
 * Giữ `sid` của phiên đang mở, phía trình duyệt.
 *
 * VÌ SAO PHẢI GIỮ Ở ĐÂY: hợp đồng chỉ có `DELETE /api/v1/sessions/{sid}` — muốn đăng xuất thì
 * phải biết `sid`, mà `sid` chỉ xuất hiện đúng một lần, trong phản hồi 201 của lần đăng nhập.
 * Không có route nào hỏi lại "phiên hiện tại của tôi là phiên nào". ĐÂY LÀ MỘT LỖ HỔNG CỦA HỢP
 * ĐỒNG, đã nêu trong báo cáo, không phải một lựa chọn thiết kế: mở tab mới hoặc đóng trình
 * duyệt rồi mở lại là mất `sid`, và người dùng không tự kết thúc phiên của mình được nữa.
 *
 * VÌ SAO `sid` ĐƯỢC PHÉP Ở TRONG JAVASCRIPT: bản thân `sid` không phải giấy thông hành. Máy chủ
 * chỉ nhận token đã ký bằng khoá của nó — token ấy nằm trong cookie httpOnly, JavaScript không
 * với tới. `services/identity/internal/http/handler.go` nói rõ điều này ngay trên trường `Sid`.
 *
 * VÌ SAO KHÔNG GIỮ GÌ THÊM: phản hồi đăng nhập còn có họ tên và chức vụ cán bộ. Họ tên là dữ
 * liệu cá nhân (nghị định 13/2023, luật 3); cất nó vào kho của trình duyệt là dựng thêm một nơi
 * lưu dữ liệu cá nhân nằm ngoài tầm kiểm soát của hệ thống, để đổi lấy một dòng chữ trên đầu
 * trang. Không đáng.
 *
 * VÌ SAO `sessionStorage` CHỨ KHÔNG PHẢI COOKIE: cookie là việc của máy chủ. Client tự đặt
 * cookie là client tự viết vào đúng cái mặt phẳng mang cách ly giữa các xã.
 */

const KHOA = "vigov_sid";

/** Mọi lời gọi đều bọc try/catch: chế độ riêng tư của trình duyệt có thể ném ngay khi ghi. */
export function luuSid(sid: string): void {
  try {
    window.sessionStorage.setItem(KHOA, sid);
  } catch {
    /* Không giữ được thì thôi — đăng xuất sẽ báo không rõ phiên, chứ không làm hỏng đăng nhập. */
  }
}

export function docSid(): string | null {
  try {
    return window.sessionStorage.getItem(KHOA);
  } catch {
    return null;
  }
}

export function xoaSid(): void {
  try {
    window.sessionStorage.removeItem(KHOA);
  } catch {
    /* như trên */
  }
}
