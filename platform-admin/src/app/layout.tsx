import type { ReactNode } from "react";

import "./globals.css";

/**
 * Root layout of the operator console.
 *
 * `lang="vi"`: screen readers read Vietnamese as Vietnamese, and the browser breaks lines and
 * spell-checks accordingly.
 *
 * `force-dynamic` IS A SECURITY SETTING HERE, NOT A PERFORMANCE ONE: the CSP nonce is issued per
 * request (`src/proxy.ts`), and a page prerendered at build time carries scripts with no nonce —
 * which the policy then blocks, or which tempts someone into `'unsafe-inline'`. A handful of
 * operators on one host never needed a static cache.
 */
export const dynamic = "force-dynamic";

export const metadata = {
  // A product constant. This console serves no commune, so no commune value exists to bake in.
  title: "ViGov — Khu vận hành nền tảng",
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="vi">
      <body>{children}</body>
    </html>
  );
}
