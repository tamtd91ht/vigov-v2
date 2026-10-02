/**
 * PostCSS runs exactly one plugin: Tailwind v4 — the same setup as web-admin (ADR 0068 §2).
 *
 * The CSS is compiled at build time into a static file served from this app's own host, so it is
 * covered by `style-src 'self'` of the per-request CSP (`src/lib/csp.ts`). Nothing here injects a
 * `<style>` tag or a `style` attribute at run time — either would be refused by that policy.
 *
 * Preflight is deliberately NOT imported (see `src/app/globals.css`).
 */
const config = {
  plugins: {
    "@tailwindcss/postcss": {},
  },
};

export default config;
