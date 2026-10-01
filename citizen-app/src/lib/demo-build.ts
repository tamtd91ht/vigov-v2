/**
 * `--demo` BUILD — `true` only in a commune's own app built with `scripts/deploy.mjs --vao-thang --demo`
 * (owner's decision 01/10/2026, ADR 0047 §6 — it REPLACES the 30/09 design; the owner drops the flag
 * before submitting to Zalo). Every other build and every test run: `false`.
 *
 * WHAT IT CHANGES — THE IDENTITY SOURCE, NOTHING ELSE:
 *   · `getUserInfo` is never called; the name is `DEMO_CITIZEN_NAME`;
 *   · `getPhoneNumber` and `getAccessToken` are never called either — an app Zalo has not approved is
 *     refused all three. The session is opened with App ID + `demoIdentity: true` only
 *     (`features/dang-nhap/hop-dong.ts`), and the server — only for an App ID it lists as a demo app —
 *     hands ViGov the number `DEMO_CITIZEN_PHONE`;
 *   · the phone field of the send form is pre-filled with that number.
 * Everything after the session is REAL: real server calls, real petitions in the commune's register.
 *
 * WHAT IT NEVER DOES: show any word of its own. The owner (01/10/2026): no "demo" line and no notice
 * anywhere in the app, in any build — `bundle-for-zalo.test.ts` measures both builds for it.
 *
 * A COMPILE-TIME CONSTANT: Vite's `define` replaces `__VIGOV_DEMO__` with the literal `false` or `true`, so
 * this line becomes `export const DEMO_BUILD = false`, which the bundler inlines into every importer
 * (rolldown `inlineConst`, on by default). Every `DEMO_BUILD ? … : …` then folds away and the fixed identity
 * never reaches a normal bundle — `bundle-for-zalo.test.ts` measures exactly that.
 *
 * ⚠ KEEP THE RIGHT-HAND SIDE A BARE `__VIGOV_DEMO__`. Any expression around it (`=== true`, a `typeof`
 * guard, `&& XA_CO_DINH !== null`) is no longer a literal, is NOT inlined — measured on 30/09/2026 — and the
 * demo branches ship inside the real app, unreachable but present. The build step is the guard instead:
 * `demoBuild` in scripts/cau-hinh.mjs only yields `true` with a commune baked in (`--vao-thang`).
 *
 * Never read from storage, a URL or a launch parameter: a switch a citizen could flip is a switch that
 * replaces a real identity with a fixed one on a real app.
 */
declare const __VIGOV_DEMO__: boolean;

export const DEMO_BUILD: boolean = __VIGOV_DEMO__;

/** The fixed name of the `--demo` build, given by the owner. Pre-fill and greeting only — it grants nothing. */
export const DEMO_CITIZEN_NAME = "Nguyễn Văn Hùng";

/**
 * The fixed number of the `--demo` build — the repo's agreed fake number (rule 3 #5). Pre-fills the send form;
 * the SESSION's number is the one `vihat-miniapp` sends ViGov, never this constant (rule 4 #2).
 */
export const DEMO_CITIZEN_PHONE = "0900000000";
