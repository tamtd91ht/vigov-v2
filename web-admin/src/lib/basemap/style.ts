import { layers, namedFlavor } from "@protomaps/basemaps";
import type { LayerSpecification, StyleSpecification } from "maplibre-gl";

import { BASEMAP_FILE, BASEMAP_ROUTE, BASEMAP_SPRITE } from "./assets";

/**
 * The MapLibre style of the petition maps (heat-map tab, scene map in the ticket drawer) — built HERE,
 * in code, never fetched: ADR 0072 §Sửa đổi 09/10/2026 forbids any host outside ViGov for style, tiles,
 * glyphs or sprites, and §Trả lời 09/10/2026 fixes what the style must do. Every URL it contains is
 * `<page origin>/basemap/...`; `style.test.ts` walks the whole object and fails on any other.
 *
 * This is NOT the economic map's style. `/ban-do` stays on OpenFreeMap (`MAP_STYLE_URL`, H1), and the
 * petition maps must never fall back to it — not even "until the file exists" (§Sửa đổi 09/10/2026).
 *
 * Three rewrites of the stock Protomaps `light` layers, each one an owner decision:
 *   1. Labels read `name:vi`, then `name` — nothing else (§Trả lời: "name:vi trước, không có thì name").
 *      The stock Vietnamese layers fall back to `name:en` and stack a second line; that is replaced.
 *   2. Boundary lines: disputed and maritime ones are filtered out (§Trả lời, and the reason there:
 *      OSM draws them in a way that does not match Vietnam's position).
 *   3. Attribution `© OpenStreetMap contributors` is carried on the source (ODbL; ADR 0072 §Dựng step 6).
 */

/** Source id inside the style. Not a URL, never shown. */
export const BASEMAP_SOURCE_ID = "protomaps";

/** `name:vi` first, then the local `name` (owner, 09/10/2026). No English fallback, no second line. */
export const BASEMAP_TEXT_FIELD = ["coalesce", ["get", "name:vi"], ["get", "name"]] as const;

/**
 * Plain text, no link: a link would put a third-party URL inside the style, and the test that keeps the
 * style same-origin would have to make an exception. ODbL asks for the credit, which this is.
 */
export const BASEMAP_ATTRIBUTION = "Protomaps © OpenStreetMap contributors";

/**
 * Boundary kinds that are drawn — an ALLOW-list over the `kind` property of the Protomaps `boundaries`
 * layer. Everything else (`unrecognized_*` claim lines, `lease_limit`, `overlay_limit`, `map_unit`)
 * is a class of contested or non-administrative line; dropping it is the same reasoning as dropping
 * `disputed` — cheaper and safer than judging each stroke.
 */
const DRAWN_BOUNDARY_KINDS = ["country", "macroregion", "region", "county", "locality"];

/**
 * Hides a boundary line that is disputed or maritime. `to-boolean` because the tile carries `disputed`
 * as `true` or absent today (Protomaps `Boundaries.java`), and an integer 1 tomorrow must hide too.
 * Protomaps drops OSM `maritime=yes` ways at build time; the `maritime` guard costs nothing and keeps
 * that true if the tile builder ever stops doing it.
 */
const BOUNDARY_GUARD = [
  "all",
  ["!", ["to-boolean", ["get", "disputed"]]],
  ["!", ["to-boolean", ["get", "maritime"]]],
  ["in", ["get", "kind"], ["literal", DRAWN_BOUNDARY_KINDS]],
] as const;

const LEGACY_COMPARISON = new Set(["==", "!=", "<", "<=", ">", ">="]);

/**
 * The stock boundary filters are LEGACY syntax (`["<=", "kind_detail", 2]`). MapLibre treats an `all`
 * that mixes legacy and expression children as legacy, so the guard above would be misread. Only the
 * one shape Protomaps uses is converted; any other shape returns `null` and the layer is DROPPED —
 * fail closed: a missing boundary line is safe, an unfiltered one is not.
 */
function toExpression(filter: unknown): unknown[] | null {
  if (filter === undefined) return ["literal", true];
  if (!Array.isArray(filter) || filter.length !== 3) return null;
  const [op, key, value] = filter as [unknown, unknown, unknown];
  if (typeof op !== "string" || !LEGACY_COMPARISON.has(op) || typeof key !== "string") return null;
  if (typeof value !== "number" && typeof value !== "string" && typeof value !== "boolean") return null;
  return [op, ["get", key], value];
}

/** True when a `text-field` shows a place NAME (any `name…` key). Shields (`shield_text`) and house numbers keep theirs. */
function showsName(textField: unknown): boolean {
  return /"(?:pgf:)?name/.test(JSON.stringify(textField));
}

function rewrite(layer: LayerSpecification): LayerSpecification | null {
  if ("source-layer" in layer && layer["source-layer"] === "boundaries") {
    const original = toExpression("filter" in layer ? layer.filter : undefined);
    if (original === null) return null;
    // `unknown` casts: the style-spec types do not model a guard built as data; the shape is checked by
    // `style.test.ts`, and MapLibre validates the whole style when the map loads it.
    return { ...layer, filter: ["all", original, BOUNDARY_GUARD] } as unknown as LayerSpecification;
  }
  if (layer.type === "symbol" && layer.layout !== undefined && showsName(layer.layout["text-field"])) {
    return { ...layer, layout: { ...layer.layout, "text-field": BASEMAP_TEXT_FIELD } } as unknown as LayerSpecification;
  }
  return layer;
}

/**
 * The page's own origin, checked: `https://host[:port]` (http allowed for `next dev` on localhost),
 * nothing after it. Absolute URLs because MapLibre hands parts of the style to its web worker, where a
 * relative URL would resolve against the worker script's location rather than the page; built from the
 * origin the browser is already on (`window.location.origin`), never from configuration.
 */
function checkedOrigin(origin: string): string {
  let u: URL;
  try {
    u = new URL(origin);
  } catch {
    throw new Error("basemap style: origin is not a URL");
  }
  if ((u.protocol !== "https:" && u.protocol !== "http:") || u.origin !== origin) {
    throw new Error("basemap style: origin must be scheme://host[:port] and nothing else");
  }
  return u.origin;
}

/**
 * The complete style. Pass `window.location.origin`. Register the `pmtiles://` protocol first
 * (`./protocol.ts`), or the source never loads.
 */
export function buildBasemapStyle(origin: string): StyleSpecification {
  const base = `${checkedOrigin(origin)}${BASEMAP_ROUTE}`;
  const out: LayerSpecification[] = [];
  for (const l of layers(BASEMAP_SOURCE_ID, namedFlavor("light"), { lang: "vi" }) as LayerSpecification[]) {
    const r = rewrite(l);
    if (r !== null) out.push(r);
  }
  return {
    version: 8,
    glyphs: `${base}/fonts/{fontstack}/{range}.pbf`,
    sprite: `${base}/sprites/${BASEMAP_SPRITE}`,
    sources: {
      [BASEMAP_SOURCE_ID]: {
        type: "vector",
        url: `pmtiles://${base}/${BASEMAP_FILE}`,
        attribution: BASEMAP_ATTRIBUTION,
      },
    },
    layers: out,
  };
}
