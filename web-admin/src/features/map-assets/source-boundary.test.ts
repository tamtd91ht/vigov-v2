import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

/**
 * Source-level guards of the economic map, read straight from the files: none of these breaks a screen
 * on the author's machine, so no functional test would turn red.
 */
const DIR = fileURLToPath(new URL(".", import.meta.url));
const FILES = readdirSync(DIR)
  .filter((f) => /\.tsx?$/.test(f) && !/\.test\.tsx?$/.test(f))
  .map((f) => ({ name: f, src: readFileSync(join(DIR, f), "utf8") }));

describe("economic map — source boundaries", () => {
  it("scans something", () => {
    expect(FILES.length).toBeGreaterThan(5);
  });

  it("MapLibre is never imported as a VALUE at module level — only `import type` + dynamic import()", () => {
    for (const f of FILES) {
      expect(/^import\s+(?!type\b)[^;]*from\s+"maplibre-gl"/m.test(f.src), f.name).toBe(false);
    }
    const dynamic = FILES.filter((f) => f.src.includes('import("maplibre-gl")')).map((f) => f.name).sort();
    expect(dynamic).toEqual(["economic-map.tsx", "position-picker.tsx"]);
  });

  it("no NEXT_PUBLIC_, no popup HTML, no raw HTML", () => {
    for (const f of FILES) {
      expect(f.src.includes(["NEXT", "PUBLIC"].join("_")), f.name).toBe(false);
      expect(/\.setHTML\s*\(|\.innerHTML\s*=|dangerouslySetInnerHTML|new\s+\w*\.?Popup\(/.test(f.src), f.name).toBe(false);
    }
  });

  it("maxBounds is only ever a CONSTRUCTOR option — never set after the map exists", () => {
    for (const f of FILES) expect(/setMaxBounds\s*\(/.test(f.src), f.name).toBe(false);
  });

  it("every map is created only after setWorkerUrl(MAPLIBRE_WORKER_URL) — blank map under Turbopack otherwise", () => {
    const creators = FILES.filter((f) => /new\s+ml\.Map\(/.test(f.src));
    expect(creators.map((f) => f.name).sort()).toEqual(["economic-map.tsx", "position-picker.tsx"]);
    for (const f of creators) {
      const set = f.src.indexOf("ml.setWorkerUrl(MAPLIBRE_WORKER_URL)");
      expect(set, f.name).toBeGreaterThan(-1);
      expect(set, f.name).toBeLessThan(f.src.search(/new\s+ml\.Map\(/));
    }
  });

  it("no DOM marker per asset: the only Marker is the form's own pin", () => {
    const withMarker = FILES.filter((f) => /new\s+ml\.Marker\(/.test(f.src)).map((f) => f.name);
    expect(withMarker).toEqual(["position-picker.tsx"]);
  });
});
