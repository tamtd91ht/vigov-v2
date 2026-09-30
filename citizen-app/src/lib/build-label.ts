/**
 * THE BUILD LABEL — "<short commit> · <dd/MM/yyyy>" of the build this bundle came from, or `""` when the
 * build had no git (`buildLabel` in scripts/cau-hinh.mjs). Shown once, on the commune app's Cá nhân footer
 * ("ViGov phiên bản …", `PROTOTYPE.md` §6.6); `""` shows no line at all, never an invented version number.
 *
 * A compile-time constant, like `DEMO_BUILD`: Vite's `define` replaces the name with a string literal.
 * Public by nature — a commit hash and a date.
 */
declare const __VIGOV_BUILD_LABEL__: string;

export const BUILD_LABEL: string = __VIGOV_BUILD_LABEL__;
