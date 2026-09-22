/**
 * BỀ MẶT CỦA BA TÍNH NĂNG THẬT — quét danh thiếp · tìm văn phòng · đăng nhập bằng số Zalo.
 *
 * VÌ SAO Ở ĐÂY KHÔNG CÒN MỘT CỬA `bien-the/…` NÀO:
 *
 *   Trước đây ba màn quyền là một biến thể bản dựng riêng (`quyen`), gỡ được bằng `resolve.alias`
 *   vì chúng chỉ tồn tại để Zalo nhìn thấy ba lời gọi quyền. Nay ba quyền ấy thuộc về **chính
 *   ứng dụng sản phẩm**: quét danh thiếp, tìm văn phòng, đăng nhập là những việc app này
 *   làm, không phải ba màn trình diễn. Một biến thể để gỡ chúng đi sẽ là một biến thể gỡ mất
 *   nửa ứng dụng — nên nó không còn, và ba tính năng được nhập thẳng.
 *
 *   Hai cửa còn lại — `bien-the/kham-pha` và `bien-the/chan-doan` — mới là thứ phân biệt bản
 *   `goc` (bản nộp) với bản `day-du` (demo nội bộ). Xem `vite.config.ts`.
 */
import type { ComponentType } from "react";

import { ManDanhThiep } from "./ManDanhThiep";

export { KhoiDangNhap, TimVanPhong } from "./LienHeTinhNang";
export { KiemTraDuongTruyen } from "./KiemTraDuongTruyen";

/**
 * Màn "Danh thiếp" — KHÔNG CÒN LÀ MỘT TAB từ 21/09/2026 (khuya).
 *
 * ⚠ MẤT TAB KHÔNG PHẢI MẤT ĐƯỜNG TỚI, VÀ KHOẢNG CÁCH GIỮA HAI ĐIỀU ẤY LÀ CẢ Ý NGHĨA CỦA DÒNG NÀY.
 *
 *   Bản mẫu của PM vẽ bốn tab và không có tab nào cho danh thiếp; việc danh thiếp nằm trong menu
 *   nhanh của màn chủ. Màn này giữ nguyên chỗ trong sổ màn hình — nó vẫn là một màn thật, vẫn có
 *   tiêu đề riêng — chỉ là không có ô trên thanh tab.
 *
 * ⚠ TIÊU ĐỀ LÀ "Danh thiếp", KHÔNG CÒN LÀ "Quét danh thiếp" — ĐỔI 22/09/2026.
 *
 *   Từ lượt này màn không mở đầu bằng khối quét nữa: khối đứng đầu là "Thiếp số của chúng tôi"
 *   (`ManDanhThiep`), và có đúng MỘT ô menu dẫn vào đây, tên "Danh thiếp". Một thanh tiêu đề ghi
 *   "Quét danh thiếp" trên một màn mở ra tấm thiếp của chính công ty là thanh tiêu đề nói sai chỗ
 *   người dùng đang đứng — và với người lớn tuổi, "không biết mình đang ở đâu" là lúc họ đóng app.
 *
 *   ĐÂY LÀ MÀN DUY NHẤT DÙNG `scanQRCode`, `keepScreen` VÀ `downloadFile`. Ba quyền ấy được khai
 *   với Zalo, và một quyền đã khai mà màn dùng nó không tới được là đúng thứ người duyệt trả về.
 *   Nên `tabSangLen: "home"` không phải một mặc định cho có: menu nhanh của màn chủ là đường vào
 *   duy nhất, nên ô "Trang chủ" là ô nói đúng công dân đang ở nhánh nào.
 */
export const MAN_DANH_THIEP: {
  id: "danh-thiep";
  headerTitle: string;
  cho: { kieu: "ngoai-tab"; tabSangLen: "home" };
  component: ComponentType;
} = {
  id: "danh-thiep",
  headerTitle: "Danh thiếp",
  cho: { kieu: "ngoai-tab", tabSangLen: "home" },
  component: ManDanhThiep,
};
