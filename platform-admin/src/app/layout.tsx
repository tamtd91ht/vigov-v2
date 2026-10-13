import type { ReactNode } from "react";

// Inter, SELF-HOSTED, the same five weights as web-admin (spec 00 §1; ADR 0068 lần 6 #6):
// font-normal/medium/semibold/bold/extrabold = 400/500/600/700/800. The COMBINED per-weight file, never
// the per-subset ones: only it gives each face its `unicode-range`, so Vietnamese letters never fall
// through to the system font mid-word. The font files are bundled into this app's static assets and
// served from this host, which `font-src 'self'` allows (`src/lib/csp.ts`). NOT `next/font/google`:
// that would make the build — and an operator's browser on a cache miss — depend on a Google request.
import "@fontsource/inter/400.css";
import "@fontsource/inter/500.css";
import "@fontsource/inter/600.css";
import "@fontsource/inter/700.css";
import "@fontsource/inter/800.css";

import "./globals.css";

/** Class that sets `--font-inter` (`globals.css`, layer `base`), as in web-admin. */
const FONT_VARIABLE_CLASS = "font-inter";

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
