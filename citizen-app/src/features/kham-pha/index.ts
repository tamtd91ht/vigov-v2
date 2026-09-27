/**
 * Bề mặt của lớp khám phá — màn xác nhận xã và quy tắc đọc gợi ý.
 *
 * Không còn đứng sau `resolve.alias` (27/09/2026): bản dựng chỉ còn một, nên không có bản rỗng nào
 * để trỏ sang. Danh mục xã mẫu, bộ chọn xã và trang xã mẫu đã bị xoá khỏi mã (ADR 0044 câu 4 ·
 * ADR 0047) — tên xã chỉ còn một nguồn: máy chủ.
 */
export { GoiYXaScreen } from "./GoiYXaScreen";
export { phanGiaiGoiY, TIEU_DE_XAC_NHAN_XA, type XaGoiY } from "./goi-y";
