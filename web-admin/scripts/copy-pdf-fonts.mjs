// Copies the Roboto TTFs the Tổng quan PDF export embeds into public/pdf-fonts/, served SAME-ORIGIN at
// /pdf-fonts/Roboto-Regular.ttf and /pdf-fonts/Roboto-Bold.ttf (src/features/dashboard/export-files.ts,
// PDF_FONT_URLS).
//
// WHY A TTF AT ALL: jsPDF's built-in fonts are WinAnsi and cannot draw Vietnamese ("Tổng" → "T?ng");
// a PDF must carry its font inside the file. jsPDF reads TTF only — @fontsource/roboto (the UI's
// fonts) ships WOFF/WOFF2 split by subset, so the TTF comes from @expo-google-fonts/roboto (Google
// Fonts' Roboto, SIL OFL 1.1). The licence travels with the copied fonts, as the OFL requires.
//
// WHY COPIED, NOT IMPORTED: Turbopack rewrites `new URL(..., import.meta.url)` to a file:// URL in
// production (the MapLibre worker hit it, scripts/copy-maplibre-worker.mjs), and inlining two 160 KB
// fonts into a JS chunk would make every reader of the chunk pay for them.
//
// GENERATED, gitignored: copied from node_modules on every build (`prebuild`) and dev start (`predev`),
// so the fonts always match the installed package. The Dockerfile already ships `public/`.
import { cpSync, mkdirSync } from "node:fs";
import { createRequire } from "node:module";
import path from "node:path";
import { fileURLToPath } from "node:url";

const require = createRequire(import.meta.url);
const pkg = path.dirname(require.resolve("@expo-google-fonts/roboto/package.json"));
const out = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..", "public", "pdf-fonts");

mkdirSync(out, { recursive: true });
cpSync(path.join(pkg, "400Regular", "Roboto_400Regular.ttf"), path.join(out, "Roboto-Regular.ttf"));
cpSync(path.join(pkg, "700Bold", "Roboto_700Bold.ttf"), path.join(out, "Roboto-Bold.ttf"));
cpSync(path.join(pkg, "LICENSE_FONT"), path.join(out, "OFL.txt"));
console.log("[pdf-fonts] Roboto TTF copied to public/pdf-fonts");
