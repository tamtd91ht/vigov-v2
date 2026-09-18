/**
 * BỀ MẶT CỦA BA TÍNH NĂNG THẬT — quét danh thiếp · tìm văn phòng · đăng ký nhận tư vấn.
 *
 * VÌ SAO Ở ĐÂY KHÔNG CÒN MỘT CỬA `bien-the/…` NÀO:
 *
 *   Trước đây ba màn quyền là một biến thể bản dựng riêng (`quyen`), gỡ được bằng `resolve.alias`
 *   vì chúng chỉ tồn tại để Zalo nhìn thấy ba lời gọi quyền. Nay ba quyền ấy thuộc về **chính
 *   ứng dụng sản phẩm**: quét danh thiếp, tìm văn phòng, đăng ký tư vấn là những việc app này
 *   làm, không phải ba màn trình diễn. Một biến thể để gỡ chúng đi sẽ là một biến thể gỡ mất
 *   nửa ứng dụng — nên nó không còn, và ba tính năng được nhập thẳng.
 *
 *   Hai cửa còn lại — `bien-the/kham-pha` và `bien-the/chan-doan` — mới là thứ phân biệt bản
 *   `goc` (bản nộp) với bản `day-du` (demo nội bộ). Xem `vite.config.ts`.
 */
import type { ComponentType } from "react";

import { ManDanhThiep } from "./ManDanhThiep";

export { DangKyTuVan, TimVanPhong } from "./LienHeTinhNang";

/**
 * Tab "Danh thiếp".
 *
 * Nhãn ngắn có chủ đích: thanh tab có năm tab, và trên máy rộng 320px mỗi tab chỉ còn khoảng
 * 56px chữ. `screens.test.tsx` đo điều đó thay vì tin vào mắt.
 */
export const MAN_DANH_THIEP: {
  id: "danh-thiep";
  tabLabel: string;
  headerTitle: string;
  component: ComponentType;
} = {
  id: "danh-thiep",
  tabLabel: "Danh thiếp",
  headerTitle: "Quét danh thiếp",
  component: ManDanhThiep,
};
