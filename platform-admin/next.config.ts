import type { NextConfig } from "next";

/**
 * Build config of the ViHAT operator console (ADR 0048 §Sửa 27/09: a separate app, served on the
 * one operator host only).
 *
 * WHAT IS DELIBERATELY ABSENT: any `env` block and any `NEXT_PUBLIC_*` variable. A public
 * variable is baked into the browser bundle at build time (rule 8, inv. 4), and turning the
 * operator area on or pointing it somewhere by a build constant is ADR 0048 STOP CONDITION #5.
 * The one server-side setting this app reads, `PLATFORM_HTTP_ADDR`, is read at REQUEST time by
 * `src/lib/server/platform-origin.ts` and never reaches the browser.
 *
 * SECURITY HEADERS (rule 13, inv. 5; TCVN 14423 §5.17.2.3) are sent on EVERY route from here —
 * pages, `/api/v1/*` answers, static assets. The CSP below is the static FLOOR: directives that
 * can never break a page. Each HTML document additionally gets the per-request nonce policy built
 * in `src/lib/csp.ts` by `src/proxy.ts`; why it cannot live here is written there.
 */

/** Exported for the test that pins it (`src/security-headers.test.ts`). */
export const SECURITY_HEADERS: { key: string; value: string }[] = [
  {
    key: "Content-Security-Policy",
    value: "base-uri 'none'; object-src 'none'; form-action 'self'; frame-ancestors 'none'",
  },
  // Two years, subdomains included. The operator host lives under the platform root domain, and
  // nothing under it is meant to be reachable over plain http.
  { key: "Strict-Transport-Security", value: "max-age=63072000; includeSubDomains" },
  { key: "X-Content-Type-Options", value: "nosniff" },
  // No path or query of this console leaves for another origin: they name communes and domains.
  { key: "Referrer-Policy", value: "no-referrer" },
  // `frame-ancestors 'none'` is the modern refusal; X-Frame-Options covers browsers that predate it.
  { key: "X-Frame-Options", value: "DENY" },
];

const nextConfig: NextConfig = {
  // Only the dependencies that really run, so the image needs no node_modules (fewer CVEs to patch).
  output: "standalone",
  // Do not announce the Next version to whoever is probing.
  poweredByHeader: false,
  reactStrictMode: true,
  async headers() {
    return [{ source: "/:path*", headers: SECURITY_HEADERS }];
  },
};

export default nextConfig;
