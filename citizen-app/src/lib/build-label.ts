/**
 * THE BUILD LABEL — "<major>.<minor>" of `package.json`'s version, or `""` when it is missing or malformed
 * (`buildLabel` in scripts/cau-hinh.mjs). Shown once, on the commune app's Cá nhân footer
 * ("ViGov phiên bản 1.1", `PROTOTYPE.md` §6.6); `""` shows no line at all, never an invented version number.
 *
 * A compile-time constant: Vite's `define` replaces the name with a string literal. Public by nature.
 */
declare const __VIGOV_BUILD_LABEL__: string;

export const BUILD_LABEL: string = __VIGOV_BUILD_LABEL__;
