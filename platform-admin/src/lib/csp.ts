/**
 * The Content-Security-Policy of every HTML document this console serves (rule 13, inv. 3 + 5).
 *
 * WHY PER REQUEST, WITH A NONCE, AND NOT A STATIC STRING IN next.config.ts: the App Router puts
 * inline `<script>` tags in every page (the RSC payload, `self.__next_f.push(...)`). A static
 * `script-src 'self'` blocks them and the page never hydrates; the only static policy that keeps
 * the page alive is `'unsafe-inline'`, which is exactly the XSS hole CSP exists to close — on the
 * most privileged surface in the system (ADR 0048 §Hệ quả). A nonce issued per request lets
 * Next's own scripts run and nothing else. Next reads the nonce from the `Content-Security-Policy`
 * REQUEST header the proxy sets, and stamps it on the scripts it renders.
 *
 * `next.config.ts` still sends a static policy on EVERY route (API answers and static assets
 * included). It carries only directives that cannot break a page — framing, `base-uri`,
 * `object-src`, `form-action` — so whether the browser receives one policy or both, a document is
 * governed by this one for scripts and styles. The two never have to agree on a script rule.
 *
 * WHAT DEVELOPMENT LOOSENS, AND ONLY THERE: React's dev build evaluates code for stack frames
 * (`'unsafe-eval'`), and Next dev injects CSS through `<style>` tags without a nonce. A production
 * build carries neither, and `buildDocumentCsp(nonce, false)` is what the tests pin.
 */
export function buildDocumentCsp(nonce: string, development: boolean): string {
  const scriptSrc = ["'self'", `'nonce-${nonce}'`, "'strict-dynamic'"];
  if (development) scriptSrc.push("'unsafe-eval'");
  const styleSrc = development ? ["'self'", "'unsafe-inline'"] : ["'self'", `'nonce-${nonce}'`];

  return [
    "default-src 'self'",
    `script-src ${scriptSrc.join(" ")}`,
    `style-src ${styleSrc.join(" ")}`,
    "img-src 'self' data:",
    "font-src 'self'",
    // Same origin only: the browser reaches service-platform through this app's `/api/v1/*`
    // gateway, never directly — there is no cross-origin API to allow.
    "connect-src 'self'",
    "object-src 'none'",
    "base-uri 'none'",
    "form-action 'self'",
    "frame-ancestors 'none'",
  ].join("; ");
}

/**
 * 128 bits from the platform CSPRNG (rule 13, inv. 2), base64. A nonce that can be predicted is
 * no nonce: `Math.random` is forbidden here for exactly that reason.
 */
export function createNonce(): string {
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  let binary = "";
  for (const b of bytes) binary += String.fromCharCode(b);
  return btoa(binary);
}
