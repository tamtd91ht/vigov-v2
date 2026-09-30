/**
 * DEMO BUILD — `true` only in a commune's own app built with `scripts/deploy.mjs --vao-thang --demo`
 * (owner's decision 30/09/2026, for the period before the app is submitted to Zalo; the owner drops the
 * flag at submission). Every other build and every test run: `false`.
 *
 * A COMPILE-TIME CONSTANT: Vite's `define` replaces `__VIGOV_DEMO__` with the literal `false` or `true`, so
 * this line becomes `export const DEMO_BUILD = false`, which the bundler inlines into every importer
 * (rolldown `inlineConst`, on by default). Every `if (DEMO_BUILD)` branch then folds away and the demo words
 * and sample data never reach a normal bundle — `bundle-for-zalo.test.ts` measures exactly that.
 *
 * ⚠ KEEP THE RIGHT-HAND SIDE A BARE `__VIGOV_DEMO__`. Any expression around it (`=== true`, a `typeof`
 * guard, `&& XA_CO_DINH !== null`) is no longer a literal, is NOT inlined — measured on 30/09/2026 — and the
 * demo strings ship inside the real app, unreachable but present. The build step is the guard instead:
 * `demoBuild` in scripts/cau-hinh.mjs only yields `true` with a commune baked in (`--vao-thang`).
 *
 * Never read from storage, a URL or a launch parameter: a demo switch a citizen could flip is a switch that
 * shows a fake number on a real app. CLIENT-SIDE ONLY — nothing here opens a session, and the server is
 * unchanged: a real ViGov session still needs Zalo's phone token.
 */
declare const __VIGOV_DEMO__: boolean;

export const DEMO_BUILD: boolean = __VIGOV_DEMO__;
