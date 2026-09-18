/**
 * Đọc danh sách quyền của phiên hiện tại để **ẩn/hiện** giao diện.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * LỚP NÀY LÀ TIỆN DỤNG, KHÔNG PHẢI BIỆN PHÁP.
 *
 * Kiểm quyền ở giao diện thay cho tầng dịch vụ là không kiểm gì cả — mã client sửa được, và
 * một cán bộ tò mò không cần sửa gì: chỉ cần gọi thẳng `/api/v1/staff` bằng chính cookie
 * phiên của mình (luật 5, cấm #1). Cái chặn thật là khai báo `RequirePermission("admin.user")`
 * trên tuyến, ở phía máy chủ, trên **từng** yêu cầu.
 *
 * Ẩn một tab vì vậy chỉ để cán bộ không phải bấm vào một thứ chắc chắn trả 403. Nếu có ngày
 * hàm này trả `true` sai, hậu quả là một màn hình hiện lỗi 403 — không phải một lần dữ liệu
 * rời khỏi máy chủ.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */

/**
 * Khoá quyền của tab "Người dùng" — `docs/ui-ux/14-cau-hinh.md §12.8`: tab nào thiếu quyền thì
 * ẩn tab đó. Cùng một chuỗi mà máy chủ đòi trên hai tuyến danh bạ (`x-vigov-permission` trong
 * `kb/20-contracts/openapi.json`), nên hai bên không thể lệch nhau mà không ai thấy.
 */
export const QUYEN_QUAN_LY_NGUOI_DUNG = "admin.user";

/**
 * Khoá quyền của tab "Phân quyền" — `docs/ui-ux/14-cau-hinh.md §12.8`, cùng một quy tắc: tab nào
 * thiếu quyền thì ẩn tab đó. Cùng chuỗi mà máy chủ đòi trên `GET /api/v1/role-permissions`
 * (`x-vigov-permission` trong `kb/20-contracts/openapi.json`).
 *
 * KHÔNG PHẢI `admin.user`, và hai khoá này không suy ra nhau: người quản lý danh bạ cán bộ chưa
 * chắc được xem ai đang giữ khoá nào, và ngược lại. Đó chính là lý do bộ quyền được liệt kê từng
 * khoá một (luật 5, bất biến 3b).
 */
export const QUYEN_PHAN_QUYEN = "admin.role";

/**
 * Có đúng khoá quyền này hay không. **So sánh chuỗi chính xác, không tiền tố, không ký tự thay
 * thế.**
 *
 * Một quyền là MỘT khoá phẳng `"<nhóm>.<việc>"` (luật 5, bất biến 3b), không phải một cây.
 * Nếu ở đây có một phép khớp kiểu `admin.*` thì `admin.audit` — quyền xem nhật ký hệ thống —
 * sẽ mở luôn tab quản lý người dùng, và bộ quyền của khách không phải tích Descartes: các khoá
 * ấy được liệt kê từng cái một chính vì `admin.audit` cố ý không phải `admin.user`.
 *
 * KHÔNG NHẮC MỘT CON SỐ TỔNG Ở ĐÂY: danh mục quyền nằm trong CSDL (`quyen`, do máy chủ phát ra
 * theo `GET /api/v1/role-permissions`), và một con số chép vào mã là con số sai vào ngày khách
 * chốt thêm một khoá — sai lặng lẽ, vì không bài test nào đọc lại nó.
 */
export function coQuyen(dsQuyen: readonly string[], khoa: string): boolean {
  return dsQuyen.includes(khoa);
}
