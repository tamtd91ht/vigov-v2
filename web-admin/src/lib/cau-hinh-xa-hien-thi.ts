import type { TenantConfig } from "./tenant-config";

/**
 * Phần cấu hình xã được phép đi xuống trình duyệt: **chỉ những gì để hiển thị**.
 *
 * `tenantId` CỐ Ý KHÔNG có ở đây, dù máy chủ đã biết nó. Một giá trị mà client không bao giờ
 * cầm là một giá trị client không thể gửi ngược lên — mà client tự khai xã chính là client tự
 * cấp quyền (luật 1, cấm #2). `host` cũng không: trình duyệt đã ở trên host ấy rồi, mang thêm
 * một bản sao chỉ tạo chỗ cho hai bản lệch nhau.
 */
export type CauHinhXaHienThi = {
  /** Tên xã tại thời điểm này. Một thuộc tính, không phải định danh. */
  displayName: string;
  /** Cơ quan cấp trên — hiện ở dòng dưới tên xã trên đầu trang. */
  parentAuthority: string;
  /**
   * Logo của xã (ADR 0069) — URL ảnh công khai, "" = chưa có (vẽ biểu tượng toà nhà). Chỉ để hiển
   * thị: không định danh ai, không cấp quyền gì. Bắt buộc, không tuỳ chọn: thiếu khoá thì `tsc` đỏ
   * ở chỗ dựng, thay vì một ô logo lặng lẽ rỗng.
   */
  logoUrl: string;
  /** Banner web-admin (ADR 0069 #5) — "" = không có dải dưới thanh trên cùng. */
  webAdminBannerUrl: string;
};

/**
 * Rút phần hiển thị ra khỏi cấu hình đầy đủ. Gọi ở máy chủ, ngay trước khi dựng provider.
 *
 * VÌ SAO NẰM Ở ĐÂY MÀ KHÔNG Ở `components/cau-hinh-xa.tsx` cạnh provider: tệp ấy mang
 * `"use client"`, và MỌI thứ nó xuất ra — cả một hàm thuần — thành tham chiếu client. Trang
 * (server component) gọi nó thì Next hỏng lúc chạy: "Attempted to call phanHienThi() from the
 * server" — build vẫn xanh, trang đăng nhập prod trắng (26/09/2026). Tệp này không có chỉ thị
 * nào nên dùng được ở cả hai phía.
 */
export function phanHienThi(cauHinh: TenantConfig): CauHinhXaHienThi {
  return {
    displayName: cauHinh.displayName,
    parentAuthority: cauHinh.parentAuthority,
    logoUrl: cauHinh.logoUrl,
    webAdminBannerUrl: cauHinh.webAdminBannerUrl,
  };
}
