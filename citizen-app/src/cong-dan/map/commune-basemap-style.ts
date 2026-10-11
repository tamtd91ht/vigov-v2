/**
 * THE BASEMAP OF THE SEND FORM'S MAP — the same self-hosted Vietnam PMTiles archive, glyphs and sprite as web-admin's
 * petition maps and `/ban-do`, served to the Mini App by `service-comms` (ADR 0072 §Sửa đổi 11/10/2026 (Mini App) M1).
 *
 * WHY NOT web-admin's `/basemap/*`: the Mini App has no commune domain, so that same-origin route is not same-origin
 * here (M1 row 3). `comms` is a host Zalo already lets this app call.
 *
 * NO THIRD-PARTY HOST, EVER — not for the style, tiles, glyphs or sprite, not even "until the route exists" (ADR 0072
 * stop points 1 and 4, M1 row 4). Every URL below is built from `communeBasemapBase()` (`COMMUNE_BASEMAP_PATH` on
 * comms' host, `api/hop-dong-cong-khai.ts`); `commune-basemap-style.test.ts`
 * walks the whole style and fails on any other host. A missing route means NO MAP and one sentence, never another host.
 *
 * THE STYLE IS A PORT of `web-admin/src/lib/basemap/style.ts` (the two apps are separate bundles; neither may import
 * the other). The three owner rewrites are the same, and the test pins them the same way:
 *   1. labels read `name:vi`, then `name` — nothing else;
 *   2. disputed and maritime boundary lines are filtered out (allow-list of drawn kinds);
 *   3. the OSM credit rides on the source and is shown uncollapsed (`scene-map.tsx`).
 * Change one, change both.
 */
import { layers, namedFlavor } from "@protomaps/basemaps";
import type { LayerSpecification, StyleSpecification } from "maplibre-gl";

import { communeBasemapBase } from "../api/hop-dong-cong-khai";

export { communeBasemapBase };

/** The archive's fixed name — web-admin's `BASEMAP_FILE`: a refresh changes the storage prefix, never this name. */
export const BASEMAP_FILE = "vn-mainland.pmtiles";

/** The font stacks the Protomaps `light` layers name — web-admin's `BASEMAP_FONTS`. */
export const BASEMAP_FONTS = ["Noto Sans Regular", "Noto Sans Medium", "Noto Sans Italic"] as const;

/** Sprite sheet (Protomaps `light`); MapLibre appends `.json`, `.png`, `@2x.json`, `@2x.png`. */
export const BASEMAP_SPRITE = "light";

export const BASEMAP_SOURCE_ID = "protomaps";

/** `name:vi` first, then the local `name` (owner, 09/10/2026). No English fallback, no second line. */
export const BASEMAP_TEXT_FIELD = ["coalesce", ["get", "name:vi"], ["get", "name"]] as const;

/** Plain text, no link — a link would put a third-party URL inside the style. ODbL asks for the credit. */
export const BASEMAP_ATTRIBUTION = "Protomaps © OpenStreetMap contributors";

/** Boundary kinds that are drawn — an ALLOW-list; every contested or non-administrative class is dropped. */
const DRAWN_BOUNDARY_KINDS = ["country", "macroregion", "region", "county", "locality"];

const BOUNDARY_GUARD = [
  "all",
  ["!", ["to-boolean", ["get", "disputed"]]],
  ["!", ["to-boolean", ["get", "maritime"]]],
  ["in", ["get", "kind"], ["literal", DRAWN_BOUNDARY_KINDS]],
] as const;

const LEGACY_COMPARISON = new Set(["==", "!=", "<", "<=", ">", ">="]);

/**
 * The stock boundary filters are LEGACY syntax; mixed with the expression guard MapLibre would misread the `all`.
 * Only Protomaps' one shape is converted; anything else returns `null` and the layer is DROPPED — fail closed: a
 * missing boundary line is safe, an unfiltered one is not.
 */
function toExpression(filter: unknown): unknown[] | null {
  if (filter === undefined) return ["literal", true];
  if (!Array.isArray(filter) || filter.length !== 3) return null;
  const [op, key, value] = filter as [unknown, unknown, unknown];
  if (typeof op !== "string" || !LEGACY_COMPARISON.has(op) || typeof key !== "string") return null;
  if (typeof value !== "number" && typeof value !== "string" && typeof value !== "boolean") return null;
  return [op, ["get", key], value];
}

/** True when a `text-field` shows a place NAME. Shields and house numbers keep theirs. */
function showsName(textField: unknown): boolean {
  return /"(?:pgf:)?name/.test(JSON.stringify(textField));
}

function rewrite(layer: LayerSpecification): LayerSpecification | null {
  if ("source-layer" in layer && layer["source-layer"] === "boundaries") {
    const original = toExpression("filter" in layer ? layer.filter : undefined);
    if (original === null) return null;
    // `unknown` casts: the style-spec types do not model a guard built as data; MapLibre validates on load.
    return { ...layer, filter: ["all", original, BOUNDARY_GUARD] } as unknown as LayerSpecification;
  }
  if (layer.type === "symbol" && layer.layout !== undefined && showsName(layer.layout["text-field"])) {
    return { ...layer, layout: { ...layer.layout, "text-field": BASEMAP_TEXT_FIELD } } as unknown as LayerSpecification;
  }
  return layer;
}

/**
 * The complete style over `base` (from `communeBasemapBase`). HTTPS only: a base that is not one is refused, so a
 * mis-generated host table can never point the map at a plain-text channel. Register `pmtiles://` first.
 *
 * `domain` is the build's commune domain, sent as `?host=` on EVERY file — the same lookup key as the frame read.
 * comms answers 400 without it: the 6,000/min ceiling is counted per (commune domain, client network) (M1 row
 * "Trần tần suất"), and an unknown domain gets nothing. MapLibre (`normalizeSpriteURL`) and pmtiles keep the query
 * when they append `.json`/`.png` or `/{z}/{x}/{y}`.
 */
export function buildCommuneBasemapStyle(base: string, domain: string): StyleSpecification {
  let u: URL;
  try {
    u = new URL(base);
  } catch {
    throw new Error("commune basemap: base is not a URL");
  }
  if (u.protocol !== "https:" || u.search !== "" || u.hash !== "" || u.username !== "" || u.password !== "") {
    throw new Error("commune basemap: base must be https with no query, fragment or credentials");
  }
  if (domain === "") throw new Error("commune basemap: no commune domain");
  const root = `${u.origin}${u.pathname.replace(/\/+$/, "")}`;
  const query = `?${new URLSearchParams({ host: domain }).toString()}`;
  const out: LayerSpecification[] = [];
  for (const l of layers(BASEMAP_SOURCE_ID, namedFlavor("light"), { lang: "vi" }) as LayerSpecification[]) {
    const r = rewrite(l);
    if (r !== null) out.push(r);
  }
  return {
    version: 8,
    glyphs: `${root}/fonts/{fontstack}/{range}.pbf${query}`,
    sprite: `${root}/sprites/${BASEMAP_SPRITE}${query}`,
    sources: {
      [BASEMAP_SOURCE_ID]: {
        type: "vector",
        url: `pmtiles://${root}/${BASEMAP_FILE}${query}`,
        attribution: BASEMAP_ATTRIBUTION,
      },
    },
    layers: out,
  };
}
