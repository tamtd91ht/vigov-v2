/// <reference types="vite/client" />
/**
 * MapLibre GL, made ready ONCE per app run: the `pmtiles://` protocol registered and the worker in place. Imported
 * ONLY dynamically (`scene-map.tsx`), from inside the effect that draws a map — never at module load, so the tests
 * that render the send form (no DOM, no WebGL) never evaluate MapLibre.
 *
 * ⚠ NOT A SEPARATE CHUNK ON ZALO. The Mini App bundle is ONE classic `iife` script by necessity (`vite.config.ts`:
 * `zmp-cli sync-config` lists classic scripts only, and `iife` cannot split), so Rollup inlines this dynamic import.
 * What it still buys: MapLibre's code RUNS only on the send form. What it does not: the bytes ride in `app.js`.
 *
 * THE WORKER IS A BLOB OF THE BUNDLED WORKER SOURCE. MapLibre 6 finds its worker from `import.meta.url`, which an
 * `iife` bundle does not have (the build silences EMPTY_IMPORT_META), and a second file next to `app.js` would depend
 * on Zalo serving an unlisted `.mjs` with a JavaScript MIME type — unverifiable here. So the worker's source ships as
 * a string (`?raw`, MapLibre's own `dist/maplibre-gl-worker.mjs`, self-contained since 6.x) and starts from a Blob
 * URL, the way MapLibre 1–5 always started theirs. This is the library's fixed, bundled code — nothing built from
 * data, nothing fetched (rule 13 #3 forbids code built from strings of DATA; this string is a pinned dependency).
 * It is a MODULE worker (the file ends in `export`): Chrome 80+ / Safari 15+ webviews; older ones get no map and the
 * fallback sentence (the map's `error`/timeout path), never a blocked send.
 *
 * Why maplibre-gl 6 and not 5 (whose UMD build inlines a classic worker): every 5.x is under GHSA-jrc7-96c5-q579
 * (critical, fixed in 6.4.1) — rule 13 #6 forbids shipping it.
 */
import "maplibre-gl/dist/maplibre-gl.css";

import type * as MapLibre from "maplibre-gl";

let ready: Promise<typeof MapLibre> | null = null;

export function loadMapLibre(): Promise<typeof MapLibre> {
  ready ??= Promise.all([
    import("maplibre-gl"),
    import("pmtiles"),
    import("maplibre-gl/dist/maplibre-gl-worker.mjs?raw"),
  ]).then(
    ([ml, { Protocol }, worker]) => {
      // The source map comment would make devtools ask for a `.map` file that is not shipped.
      const source = worker.default.replace(/\n\/\/# sourceMappingURL=\S*\s*$/, "\n");
      // Kept for the app's lifetime, never revoked: MapLibre starts workers lazily and again after a map is removed.
      ml.setWorkerUrl(URL.createObjectURL(new Blob([source], { type: "text/javascript" })));
      // One Protocol instance: it caches the archive's header and directories; two would read them twice.
      const protocol = new Protocol();
      ml.addProtocol("pmtiles", protocol.tile as Parameters<typeof ml.addProtocol>[1]);
      return ml;
    },
    (err: unknown) => {
      // A failed load must not be cached: the next open of the form tries again.
      ready = null;
      throw err;
    },
  );
  return ready;
}
