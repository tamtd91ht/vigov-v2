import type { ReactNode } from "react";

// Inter, SELF-HOSTED (spec 00 §1; ADR 0068 lần 6 #6): the five weights the spec's classes use —
// font-normal/medium/semibold/bold/extrabold = 400/500/600/700/800 — from `@fontsource/inter`.
// Import the COMBINED per-weight file, never the per-subset ones (`vietnamese-400.css`): only the
// combined file gives each face its `unicode-range`. Without it the last-declared subset face claims
// every character, and letters it lacks fall through to the system font mid-word. With it a browser
// downloads only the subsets a page uses — vietnamese, latin-ext, latin here.
// The fonts are bundled into this app's static assets and served from the commune's own host —
// NOT `next/font/google` as the spec writes, which would make the build (and a staff member's
// browser, on a cache miss) depend on a Google request.
import "@fontsource/inter/400.css";
import "@fontsource/inter/500.css";
import "@fontsource/inter/600.css";
import "@fontsource/inter/700.css";
import "@fontsource/inter/800.css";

import "./globals.css";

import { Toaster } from "@/components/ui/toaster";

/**
 * Class that sets `--font-inter` (`globals.css`, layer `base`) — the same shape as a
 * `next/font` `variable`, so the body font is one CSS variable with a system fallback behind it.
 */
const FONT_VARIABLE_CLASS = "font-inter";

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
  // A platform constant, not a per-commune value: this layout cannot resolve the commune (see
  // above), and a commune name is never baked into the build (rule 1, invariant 10). It carries
  // no product name either — staff see the commune, never "ViGov" (ADR 0068 §13, owner wording).
  // Every commune screen overrides it with "<screen> · <commune>" via `communePageMetadata`;
  // this is what the sign-in and 404 pages inherit.
  title: "Hệ thống điều hành số cấp xã",
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="vi" className={FONT_VARIABLE_CLASS}>
      <body>
        {children}
        <Toaster />
      </body>
    </html>
  );
}
