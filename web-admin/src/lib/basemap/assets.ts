/**
 * The self-hosted Vietnam basemap of the petition maps — WHICH files exist under `/basemap/*` and how
 * each one is served (ADR 0072 §Sửa đổi 09/10/2026, §Trả lời 09/10/2026).
 *
 * Shared by the server route (`app/basemap/[...path]/route.ts`, which refuses everything not listed
 * here) and the style builder (`./style.ts`, which may only point at what is listed here). One list, so
 * the style can never ask for a file the route will not serve — a mismatch draws a blank map with no
 * error anywhere but the browser console.
 *
 * WHY AN EXACT ALLOW-LIST AND NOT A PREFIX: the route forwards to the platform's object storage. A
 * prefix match ("anything under fonts/") is one decoding quirk away from `..` reaching another object
 * in the same bucket; an exact match has no such edge — a traversal attempt is simply not on the list.
 *
 * No `server-only`: the style builder runs in the browser. Nothing here is a secret or per commune.
 */

/** The same-origin mount point. Never an absolute host: ADR 0072 forbids any host outside ViGov. */
export const BASEMAP_ROUTE = "/basemap";

/**
 * The one tile archive: mainland Vietnam + near-shore islands, bbox 101.6–110.0°E, 7.9–23.9°N (owner,
 * 09/10/2026). A FIXED name on purpose: a refresh uploads under a NEW dated prefix and changes
 * `BASEMAP_URL` (runbook `kb/40-runbooks/basemap-pmtiles.md`), so the name never has to change and an
 * archive being read is never overwritten in place.
 */
export const BASEMAP_FILE = "vn-mainland.pmtiles";

/**
 * Font stacks the style uses — exactly the ones the Protomaps `light` layers name in `text-font`, and
 * exactly the ones the upload script copies. `style.test.ts` fails if the style asks for another.
 */
export const BASEMAP_FONTS = ["Noto Sans Regular", "Noto Sans Medium", "Noto Sans Italic"] as const;

/** Sprite sheet name (Protomaps `light` flavour); MapLibre appends `.json`, `.png`, `@2x.json`, `@2x.png`. */
export const BASEMAP_SPRITE = "light";

/** The one sentence shown instead of a map when the basemap is not configured (ADR 0072 §Trả lời: "một câu"). */
export const BASEMAP_MISSING_SENTENCE = "Chưa cấu hình bản đồ nền.";

export type BasemapAsset = {
  /** Object key relative to `BASEMAP_URL`, each segment percent-encoded. */
  readonly key: string;
  /** Set by US, never copied from storage: an object uploaded as `text/html` must not render as a page. */
  readonly contentType: string;
  readonly cacheControl: string;
};

/**
 * Glyph ranges are 256 code points, `<start>-<start+255>.pbf`, up to U+FFFF — the layout MapLibre asks
 * for and the Protomaps assets repo ships.
 */
const GLYPH_RANGE = /^(\d{1,5})-(\d{1,5})\.pbf$/;

const SPRITE_FILES = new Set([
  `${BASEMAP_SPRITE}.json`,
  `${BASEMAP_SPRITE}.png`,
  `${BASEMAP_SPRITE}@2x.json`,
  `${BASEMAP_SPRITE}@2x.png`,
]);

/**
 * `no-cache` = revalidate every time (the ETag still saves the body). The archive is read in byte
 * ranges; a stale cached range from the PREVIOUS archive mixed with fresh ranges of the new one is a
 * corrupted tile, so the browser must ask each time. `private`: behind the session gate, never for a
 * shared cache.
 */
const ARCHIVE_CACHE = "private, no-cache";
/** Glyphs and sprites are immutable per refresh and tiny; a day of staleness after a refresh is harmless. */
const STATIC_CACHE = "private, max-age=86400";

/**
 * The asset named by the DECODED path segments after `/basemap/`, or `null` — not on the list, which the
 * route answers with 404. Every check is an exact comparison; nothing is normalised first.
 */
export function basemapAsset(segments: readonly string[]): BasemapAsset | null {
  if (segments.length === 1 && segments[0] === BASEMAP_FILE) {
    return { key: BASEMAP_FILE, contentType: "application/octet-stream", cacheControl: ARCHIVE_CACHE };
  }
  if (segments.length === 3 && segments[0] === "fonts") {
    const [, stack, range] = segments as [string, string, string];
    if (!(BASEMAP_FONTS as readonly string[]).includes(stack)) return null;
    const m = GLYPH_RANGE.exec(range);
    if (!m) return null;
    const start = Number(m[1]);
    const end = Number(m[2]);
    if (start % 256 !== 0 || end !== start + 255 || end > 65535) return null;
    return {
      key: `fonts/${encodeURIComponent(stack)}/${start}-${end}.pbf`,
      contentType: "application/x-protobuf",
      cacheControl: STATIC_CACHE,
    };
  }
  if (segments.length === 2 && segments[0] === "sprites" && SPRITE_FILES.has(segments[1]!)) {
    const file = segments[1]!;
    return {
      key: `sprites/${encodeURIComponent(file)}`,
      contentType: file.endsWith(".png") ? "image/png" : "application/json",
      cacheControl: STATIC_CACHE,
    };
  }
  return null;
}
