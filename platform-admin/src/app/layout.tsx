import type { ReactNode } from "react";

// Be Vietnam Pro, SELF-HOSTED, the same four weights as web-admin (ADR 0068 §2). Each file declares
// the vietnamese, latin-ext and latin faces with their `unicode-range`; the font files are bundled
// into this app's static assets and served from this host, which `font-src 'self'` allows
// (`src/lib/csp.ts`). NOT `next/font/google`: that would make the build — and an operator's
// browser on a cache miss — depend on a Google request.
import "@fontsource/be-vietnam-pro/400.css";
import "@fontsource/be-vietnam-pro/500.css";
import "@fontsource/be-vietnam-pro/600.css";
import "@fontsource/be-vietnam-pro/700.css";

import "./globals.css";

/** Class that sets `--font-be-vietnam` (`globals.css`, layer `base`), as in web-admin. */
const FONT_VARIABLE_CLASS = "font-be-vietnam";

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
    <html lang="vi" className={FONT_VARIABLE_CLASS}>
      <body>{children}</body>
    </html>
  );
}
