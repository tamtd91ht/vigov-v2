import type { ReactNode } from "react";

// Be Vietnam Pro, SELF-HOSTED (owner decision 02/10/2026): the four weights the spec uses, from
// `@fontsource/be-vietnam-pro`. Each file declares the vietnamese, latin-ext and latin faces with
// their `unicode-range`, so a browser downloads only the subsets a page actually uses. The fonts
// are bundled into this app's static assets and served from the commune's own host — NOT
// `next/font/google`, which would make the build (and a staff member's browser, on a cache miss)
// depend on a Google request.
import "@fontsource/be-vietnam-pro/400.css";
import "@fontsource/be-vietnam-pro/500.css";
import "@fontsource/be-vietnam-pro/600.css";
import "@fontsource/be-vietnam-pro/700.css";

import "./globals.css";

/**
 * Class that sets `--font-be-vietnam` (`globals.css`, layer `base`) — the same shape as a
 * `next/font` `variable`, so the body font is one CSS variable with a system fallback behind it.
 */
const FONT_VARIABLE_CLASS = "font-be-vietnam";

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
    <html lang="vi" className={FONT_VARIABLE_CLASS}>
      <body>{children}</body>
    </html>
  );
}
