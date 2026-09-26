/**
 * Thanh tab của màn Cấu hình (`docs/ui-ux/14-cau-hinh.md §0`) — phần quyết định, tách khỏi phần dựng.
 *
 * QUYẾT ĐỊNH CỦA NGƯỜI DÙNG (26/09): tài khoản mở được TỪ HAI tab trở lên thì hiện thanh tab; chỉ mở
 * được MỘT tab thì KHÔNG hiện thanh, nội dung tab ấy hiện thẳng. Một nút tab đứng một mình không
 * chọn được gì, chỉ chiếm chỗ.
 *
 * TAB NÀO HIỆN ĐI ĐÚNG THEO CỔNG ĐÃ CÓ, không theo một luật mới đặt ở đây. Hai tab có cổng —
 * Người dùng (`admin.user`) và Phân quyền (`admin.role`) — gọi đúng hai hàm của `quyen-tab.ts` mà
 * `TabNguoiDung` / `TabPhanQuyen` gọi, để chỉ có MỘT nguồn quyết định. Bốn tab kia KHÔNG có cổng
 * (`cong: null`) vì tuyến đọc của chúng mở cho mọi tài khoản đã đăng nhập; riêng `GET /sla` thì máy
 * chủ tự trả 403 và tab Thời hạn xử lý hiện nguyên câu ấy. Thêm cổng cho bốn tab này là để GIAO DIỆN
 * từ chối điều máy chủ không từ chối — luật 5, cấm #1.
 *
 * Đây vẫn là TIỆN DỤNG, không phải biện pháp: máy chủ kiểm quyền trên từng yêu cầu.
 */

import type { KetQua } from "@/lib/api/goi";
import type { identity_phienHienTaiRa } from "@/lib/api/schema.gen";

import { quyetDinhTabNguoiDung, quyetDinhTabPhanQuyen, type QuyetDinhTab } from "./quyen-tab";

export type MaTabCauHinh =
  | "so-do-to-chuc"
  | "thon-to-dan-pho"
  | "nguoi-dung"
  | "phan-quyen"
  | "danh-muc"
  | "thoi-han-xu-ly";

/** `null` là chưa đọc xong phiên — cùng ba trạng thái với `PhienDaDoc` của `PhienProvider`. */
export type PhienDoc = KetQua<identity_phienHienTaiRa> | null;

export type MoTaTab<M extends string = MaTabCauHinh> = {
  readonly ma: M;
  readonly nhan: string;
  /** `null` = tab không có cổng ẩn cả tab (tuyến đọc `any-authenticated`). */
  readonly cong: ((phien: KetQua<identity_phienHienTaiRa>) => QuyetDinhTab) | null;
};

/**
 * Sáu tab đã dựng, đúng thứ tự và đúng nhãn của §0. Bốn tab chưa dựng (Trường bản đồ, Lời hệ thống,
 * Tự động hoá, Máy chủ thư) KHÔNG có nút ở đây — một nút bấm vào không ra gì là một lời hứa suông;
 * chúng nằm ở `KhoiChuaDung` kèm lý do.
 */
export const TAB_CAU_HINH: readonly MoTaTab[] = [
  { ma: "so-do-to-chuc", nhan: "Sơ đồ tổ chức", cong: null },
  { ma: "thon-to-dan-pho", nhan: "Thôn / Tổ dân phố", cong: null },
  { ma: "nguoi-dung", nhan: "Người dùng", cong: quyetDinhTabNguoiDung },
  { ma: "phan-quyen", nhan: "Phân quyền", cong: quyetDinhTabPhanQuyen },
  { ma: "danh-muc", nhan: "Danh mục", cong: null },
  { ma: "thoi-han-xu-ly", nhan: "Thời hạn xử lý", cong: null },
];

/**
 * Các tab được hiện, giữ nguyên thứ tự của `ds`.
 *
 * FAIL CLOSED với tab có cổng: chưa đọc xong phiên (`null`) hoặc đọc hỏng thì tab ấy KHÔNG hiện —
 * "chưa rõ" không được hành xử như "có". Tab không cổng thì hiện ở mọi trạng thái, vì chúng không
 * phụ thuộc phiên.
 */
export function cacTabHien<M extends string>(
  ds: readonly MoTaTab<M>[],
  phien: PhienDoc,
): readonly MoTaTab<M>[] {
  return ds.filter((t) => t.cong === null || (phien !== null && t.cong(phien).hien));
}

/**
 * Có dựng thanh tab hay không.
 *
 * CHƯA ĐỌC XONG PHIÊN THÌ CHƯA DỰNG THANH: lúc ấy chưa biết tài khoản mở được mấy tab, và dựng thanh
 * rồi gỡ đi (hoặc dựng bốn nút rồi mọc thêm hai) là màn hình nhảy dưới tay cán bộ. Trong lúc chờ, tab
 * đầu tiên hiện thẳng; đọc xong thì thanh chỉ có thể XUẤT HIỆN, không bao giờ biến mất.
 */
export function coThanhTab(phien: PhienDoc, soTabHien: number): boolean {
  return phien !== null && soTabHien >= 2;
}

/**
 * Chỉ số tab kế tiếp theo phím, theo mẫu tabs của WAI-ARIA: ← / → đi vòng, Home về đầu, End về
 * cuối. Phím khác trả `null` để trình duyệt xử lý như thường (Tab vẫn rời khỏi thanh).
 */
export function tabKeTheoPhim(phim: string, hienTai: number, soTab: number): number | null {
  if (soTab <= 0) return null;
  switch (phim) {
    case "ArrowRight":
      return (hienTai + 1) % soTab;
    case "ArrowLeft":
      return (hienTai - 1 + soTab) % soTab;
    case "Home":
      return 0;
    case "End":
      return soTab - 1;
    default:
      return null;
  }
}
