import type { ReactNode } from "react";

import "./globals.css";

/**
 * Bố cục gốc. Cố ý MỎNG: nó không suy ra xã.
 *
 * VÌ SAO KHÔNG SUY RA XÃ Ở ĐÂY dù nghe có vẻ đúng chỗ: `Host` không khớp xã nào phải trả 404,
 * mà trang 404 lại nằm bên trong chính bố cục gốc — suy ra xã ở đây là gọi 404 từ trong thứ
 * dựng nên trang 404. Nên việc suy ra xã nằm ở từng trang (`layCauHinhXa`), nơi `notFound()`
 * có một biên để dừng lại.
 *
 * `lang="vi"` không phải chi tiết nhỏ: nó quyết định trình đọc màn hình đọc tiếng Việt như
 * tiếng Việt, và quyết định trình duyệt ngắt dòng, kiểm chính tả theo tiếng Việt.
 */
export const metadata = {
  // Hằng số của sản phẩm, không phải giá trị theo xã — tên xã không bao giờ nằm trong thứ
  // được nung lúc build (luật 1, bất biến 10). Tên xã hiện trên đầu trang, đọc lúc chạy.
  title: "ViGov — Điều hành số cấp xã",
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="vi">
      <body>{children}</body>
    </html>
  );
}
