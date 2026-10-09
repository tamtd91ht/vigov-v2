import { describe, expect, it } from "vitest";

import { BASEMAP_FILE, BASEMAP_FONTS, BASEMAP_ROUTE, basemapAsset } from "./assets";
import { BASEMAP_ATTRIBUTION, BASEMAP_SOURCE_ID, BASEMAP_TEXT_FIELD, buildBasemapStyle } from "./style";

const ORIGIN = "https://xa-thu.example.test";

/** Every string anywhere inside a JSON-like value. */
function strings(v: unknown, out: string[] = []): string[] {
  if (typeof v === "string") out.push(v);
  else if (Array.isArray(v)) for (const x of v) strings(x, out);
  else if (v !== null && typeof v === "object") for (const x of Object.values(v)) strings(x, out);
  return out;
}

type AnyLayer = { id: string; type: string; filter?: unknown; layout?: Record<string, unknown>; "source-layer"?: string };

const style = buildBasemapStyle(ORIGIN);
const layers = style.layers as unknown as AnyLayer[];

describe("basemap style — nothing leaves the origin (ADR 0072 §Sửa đổi 09/10/2026)", () => {
  it("builds a non-trivial style", () => {
    expect(layers.length).toBeGreaterThan(30);
  });

  it("every URL-like string in the whole style is under <origin>/basemap/", () => {
    const urls = strings(style).filter((s) => /:\/\/|^\/\/|^www\./i.test(s));
    expect(urls.length).toBeGreaterThanOrEqual(3); // glyphs, sprite, source
    for (const u of urls) {
      const plain = u.replace(/^pmtiles:\/\//, "");
      expect(plain.startsWith(`${ORIGIN}${BASEMAP_ROUTE}/`), u).toBe(true);
    }
  });

  it("names no third-party host anywhere, not even in the attribution", () => {
    const all = JSON.stringify(style);
    for (const host of ["openfreemap", "protomaps.com", "openstreetmap.org", "api.protomaps", "tiles."]) {
      expect(all.includes(host), host).toBe(false);
    }
  });

  it("source, glyphs and sprite point at files the /basemap route actually serves", () => {
    const src = style.sources[BASEMAP_SOURCE_ID] as { url: string; attribution: string };
    expect(src.url).toBe(`pmtiles://${ORIGIN}/basemap/${BASEMAP_FILE}`);
    expect(basemapAsset([BASEMAP_FILE])).not.toBeNull();
    expect(style.glyphs).toBe(`${ORIGIN}/basemap/fonts/{fontstack}/{range}.pbf`);
    for (const f of BASEMAP_FONTS) expect(basemapAsset(["fonts", f, "0-255.pbf"]), f).not.toBeNull();
    // MapLibre appends these four suffixes to `sprite`.
    const sprite = String(style.sprite).slice(`${ORIGIN}/basemap/`.length).split("/");
    expect(sprite).toHaveLength(2);
    for (const suffix of [".json", ".png", "@2x.json", "@2x.png"]) {
      expect(basemapAsset([sprite[0]!, sprite[1]! + suffix]), suffix).not.toBeNull();
    }
  });

  it("every font the style names is one the route serves", () => {
    const named = new Set(strings(style.layers).filter((s) => /^Noto /.test(s)));
    expect(named.size).toBeGreaterThan(0);
    for (const f of named) expect((BASEMAP_FONTS as readonly string[]).includes(f), f).toBe(true);
  });

  it("carries the OpenStreetMap credit (ODbL)", () => {
    expect((style.sources[BASEMAP_SOURCE_ID] as { attribution: string }).attribution).toBe(BASEMAP_ATTRIBUTION);
    expect(BASEMAP_ATTRIBUTION).toContain("© OpenStreetMap contributors");
  });

  it("refuses an origin that is not scheme://host[:port]", () => {
    for (const bad of ["", "xa.example.test", `${ORIGIN}/`, `${ORIGIN}/x`, "javascript:alert(1)", "ftp://a.test"]) {
      expect(() => buildBasemapStyle(bad), bad).toThrow();
    }
  });
});

describe("basemap style — labels read name:vi, then name (owner, 09/10/2026)", () => {
  const named = layers.filter((l) => l.type === "symbol" && /"(?:pgf:)?name/.test(JSON.stringify(l.layout?.["text-field"])));

  it("every name label uses exactly coalesce(name:vi, name)", () => {
    expect(named.length).toBeGreaterThan(5);
    for (const l of named) expect(l.layout!["text-field"], l.id).toEqual(BASEMAP_TEXT_FIELD);
  });

  it("no label anywhere falls back to English", () => {
    for (const l of layers) {
      expect(JSON.stringify(l.layout?.["text-field"] ?? "").includes("name:en"), l.id).toBe(false);
    }
  });

  it("road shields and house numbers keep their own field", () => {
    const fields = layers.map((l) => JSON.stringify(l.layout?.["text-field"] ?? ""));
    expect(fields.some((f) => f.includes("shield_text"))).toBe(true);
    expect(fields.some((f) => f.includes("addr_housenumber"))).toBe(true);
  });
});

describe("basemap style — disputed and maritime boundaries are filtered (owner, 09/10/2026)", () => {
  const boundaries = layers.filter((l) => l["source-layer"] === "boundaries");

  it("there are boundary layers, and every one carries the guard", () => {
    expect(boundaries.length).toBeGreaterThan(0);
    for (const l of boundaries) {
      const f = JSON.stringify(l.filter);
      expect(f, l.id).toContain('["!",["to-boolean",["get","disputed"]]]');
      expect(f, l.id).toContain('["!",["to-boolean",["get","maritime"]]]');
      expect(f, l.id).toContain('["in",["get","kind"],["literal",["country","macroregion","region","county","locality"]]]');
    }
  });

  it("boundary filters are pure expressions — no legacy clause that would make MapLibre misread the guard", () => {
    for (const l of boundaries) {
      expect(JSON.stringify(l.filter), l.id).not.toMatch(/\["(?:<=|>=|<|>|==|!=)","kind_detail"/);
    }
  });

  it("no other layer reads the boundaries source-layer unguarded", () => {
    const unguarded = layers.filter(
      (l) => l["source-layer"] === "boundaries" && !JSON.stringify(l.filter).includes('"disputed"'),
    );
    expect(unguarded).toEqual([]);
  });
});
