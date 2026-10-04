// Copies MapLibre's web worker into public/maplibre/ so it is served SAME-ORIGIN at
// /maplibre/maplibre-gl-worker.mjs (src/features/map-assets/map-logic.ts, MAPLIBRE_WORKER_URL).
//
// WHY: maplibre-gl 6 locates its worker from `import.meta.url`; Turbopack rewrites that to a
// `file:///ROOT/...` URL, MapLibre rejects anything not http(s), the worker URL becomes "" and the map
// stays blank ("Worker failed to load", /ban-do in production 04/10/2026). Turbopack also does not
// emit the worker file, so the copy has to be ours.
//
// BOTH files: the worker imports ./maplibre-gl-shared.mjs, so they must sit side by side.
// GENERATED, gitignored: copied from node_modules on every build (`prebuild`) and dev start (`predev`),
// so the worker always matches the installed maplibre-gl — a worker from another version breaks the map.
// Not `postinstall`: the Dockerfile's deps stage runs `npm ci` before scripts/ is copied in.
import { cpSync, mkdirSync } from "node:fs";
import { createRequire } from "node:module";
import path from "node:path";
import { fileURLToPath } from "node:url";

const require = createRequire(import.meta.url);
const dist = path.join(path.dirname(require.resolve("maplibre-gl/package.json")), "dist");
const out = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..", "public", "maplibre");

mkdirSync(out, { recursive: true });
for (const f of ["maplibre-gl-worker.mjs", "maplibre-gl-shared.mjs"]) {
  cpSync(path.join(dist, f), path.join(out, f));
}
console.log("[maplibre] worker copied to public/maplibre");
