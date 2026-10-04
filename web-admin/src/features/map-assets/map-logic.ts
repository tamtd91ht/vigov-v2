/**
 * The decision half of `/ban-do` — pure, no network, no DOM, no MapLibre — so the rules a working
 * screen never shows breaking have tests:
 *
 *   1. THE FRAME IS HARD (ADR 0072 H3). The map exists only inside the commune's frame; nothing here
 *      widens it, and a point outside it is never flown to (`insideFrame`).
 *   2. COORDINATES ARE [lng, lat] (RFC 7946). A swapped pair draws every pin in the wrong ocean, so a
 *      point whose pair is not a valid [lng, lat] is skipped, never "repaired".
 *   3. LAYER TOGGLES ARE A FILTER, not a refetch.
 *
 * Directory named after the entity (`MapAsset`, kb/00-foundation/ubiquitous-language.md), not the
 * route: new identifiers and directories are English (rule 12); the route `/ban-do` stays Vietnamese.
 */

import type {
  comms_mapAssetFeatureOut,
  comms_mapAssetPointPropsOut,
  comms_mapAssetPointsOut,
  comms_mapFrameOut,
} from "@/lib/api/schema.gen";

/* ---- the frame -------------------------------------------------------------------------------- */

/** `[minLng, minLat, maxLng, maxLat]` — the server's `bounds`, the shape MapLibre's LngLatBoundsLike reads. */
export type FrameBounds = readonly [number, number, number, number];

/** A configured frame, checked. */
export type MapFrame = {
  readonly centerLat: number;
  readonly centerLng: number;
  readonly radiusKm: number;
  readonly bounds: FrameBounds;
};

/**
 * The frame the map may be built in, or `null`. FAIL CLOSED: `configured:false`, a missing field, or
 * bounds that are not four finite, ordered, in-range numbers all mean NO MAP — never a world view, never
 * a guessed box (a world view shows the archipelagos with OSM's names, ADR 0072 H3).
 */
export function frameFromApi(out: comms_mapFrameOut): MapFrame | null {
  if (!out.configured) return null;
  const b = out.bounds;
  if (!Array.isArray(b) || b.length !== 4 || !b.every((n) => typeof n === "number" && Number.isFinite(n))) return null;
  const [minLng, minLat, maxLng, maxLat] = b as [number, number, number, number];
  if (!(minLng < maxLng && minLat < maxLat)) return null;
  if (minLng < -180 || maxLng > 180 || minLat < -90 || maxLat > 90) return null;
  const { center_lat: lat, center_lng: lng, radius_km: r } = out;
  if (typeof lat !== "number" || typeof lng !== "number" || typeof r !== "number") return null;
  return { centerLat: lat, centerLng: lng, radiusKm: r, bounds: [minLng, minLat, maxLng, maxLat] };
}

/** True when `[lng, lat]` lies inside the frame (edges included). */
export function insideFrame(bounds: FrameBounds, lng: number, lat: number): boolean {
  if (!Number.isFinite(lng) || !Number.isFinite(lat)) return false;
  const [minLng, minLat, maxLng, maxLat] = bounds;
  return lng >= minLng && lng <= maxLng && lat >= minLat && lat <= maxLat;
}

/** Radius bounds of the frame form — proposal 1–30 km; the server's named constants are the authority (ADR 0072 H3, open #4). */
export const FRAME_RADIUS_MIN_KM = 1;
export const FRAME_RADIUS_MAX_KM = 30;

/* ---- points ----------------------------------------------------------------------------------- */

export type EconomicFeature = {
  readonly type: "Feature";
  readonly id: string;
  readonly geometry: { readonly type: "Point"; readonly coordinates: readonly [number, number] };
  readonly properties: comms_mapAssetPointPropsOut;
};

export type EconomicCollection = {
  readonly type: "FeatureCollection";
  readonly features: readonly EconomicFeature[];
};

export const EMPTY_COLLECTION: EconomicCollection = { type: "FeatureCollection", features: [] };

function validLngLat(c: readonly unknown[]): c is readonly [number, number] {
  const [lng, lat] = c;
  return (
    c.length === 2 &&
    typeof lng === "number" &&
    typeof lat === "number" &&
    Number.isFinite(lng) &&
    Number.isFinite(lat) &&
    lng >= -180 &&
    lng <= 180 &&
    lat >= -90 &&
    lat <= 90
  );
}

/**
 * The server's FeatureCollection, minus any point whose coordinates are not a valid `[lng, lat]`.
 * `skipped` is the count, so the screen logs ONCE (count only — never a name or an id: rule 3 keeps
 * logs free of anything that could name a household business).
 */
export function toEconomicCollection(out: comms_mapAssetPointsOut): { collection: EconomicCollection; skipped: number } {
  const features: EconomicFeature[] = [];
  let skipped = 0;
  for (const f of out.features ?? []) {
    const c = f?.geometry?.coordinates;
    if (!Array.isArray(c) || !validLngLat(c)) {
      skipped += 1;
      continue;
    }
    features.push({
      type: "Feature",
      id: f.id,
      geometry: { type: "Point", coordinates: [c[0], c[1]] },
      properties: { ...f.properties },
    });
  }
  return { collection: { type: "FeatureCollection", features }, skipped };
}

/** Client-side narrowing to the visible groups — the source data for clusters (counts must match what is shown). */
export function visibleCollection(c: EconomicCollection, visible: ReadonlySet<string>): EconomicCollection {
  return { type: "FeatureCollection", features: c.features.filter((f) => visible.has(f.properties.asset_type_code)) };
}

/** Arithmetic mean of the points — the "Lấy tâm từ các đối tượng đã có" button. `null` when there is none. */
export function meanCentre(
  features: readonly (comms_mapAssetFeatureOut | EconomicFeature)[],
): { lat: number; lng: number } | null {
  let n = 0;
  let sumLng = 0;
  let sumLat = 0;
  for (const f of features) {
    const c = f.geometry.coordinates;
    if (!validLngLat(c)) continue;
    sumLng += c[0];
    sumLat += c[1];
    n += 1;
  }
  if (n === 0) return null;
  return { lat: round6(sumLat / n), lng: round6(sumLng / n) };
}

export function round6(x: number): number {
  return Math.round(x * 1e6) / 1e6;
}

/* ---- worker ----------------------------------------------------------------------------------- */

/**
 * Where MapLibre's web worker is served — a SAME-ORIGIN static copy made by
 * `scripts/copy-maplibre-worker.mjs` (run by `prebuild` / `predev`) into `public/maplibre/`.
 *
 * WHY IT MUST BE SET: maplibre-gl 6 finds its worker from `import.meta.url`, and Turbopack rewrites that
 * to `file:///ROOT/node_modules/…`, which fails MapLibre's `^https?:` check → worker URL "" →
 * `new Worker("")` → "Worker failed to load" and a map that never loads its style: a blank frame with
 * only the controls (production, 04/10/2026). Turbopack does not emit the worker file either, so
 * guessing the URL would 404. Same origin also keeps the future CSP at `worker-src 'self'`, no `blob:`.
 * Call `setWorkerUrl(MAPLIBRE_WORKER_URL)` BEFORE every `new Map`.
 */
export const MAPLIBRE_WORKER_URL = "/maplibre/maplibre-gl-worker.mjs";

/* ---- layers ----------------------------------------------------------------------------------- */

export const SOURCE_ID = "economic-places";
export const LAYER_CLUSTERS = "economic-clusters";
export const LAYER_CLUSTER_COUNT = "economic-cluster-count";
export const LAYER_POINTS = "economic-points";
export const LAYER_LABELS = "economic-labels";
export const LAYER_SELECTED = "economic-selected-point";
export const CLUSTER_MAX_ZOOM = 14;
export const CLUSTER_RADIUS = 50;
export const LABEL_MIN_ZOOM = 14;

/** Fallback colour of a group the commune added itself (not one of the eleven). */
export const OTHER_GROUP_COLOUR = "#64748b";

/**
 * One colour per default group (ADR 0072 §3 codes, spec §3 hues). All dark enough to carry a white
 * outline on the light basemap; the legend and the detail panel always show the group's NAME too —
 * colour is never the only signal.
 */
export const GROUP_COLOURS: Readonly<Record<string, string>> = {
  "doanh-nghiep": "#1d4ed8", // xanh dương
  "ho-kinh-doanh": "#0f766e", // xanh ngọc
  "hop-tac-xa": "#7e22ce", // tím
  cho: "#c2410c", // cam
  "truong-hoc": "#15803d", // xanh lá
  "co-so-y-te": "#b91c1c", // đỏ
  "di-tich": "#a16207", // vàng nâu
  "du-lich-lang-nghe": "#0284c7", // xanh dương nhạt
  ocop: "#9f1239", // nâu đỏ
  "ha-tang": "#1e3a5f", // xanh đen
  "cong-trinh-dau-tu-cong": "#111827", // đen
};

export function groupColour(code: string): string {
  return GROUP_COLOURS[code] ?? OTHER_GROUP_COLOUR;
}

/**
 * The same colours as Tailwind classes, for the legend swatches. Written out literally (Tailwind only
 * emits classes it can read in source), and a class rather than an inline `style` so a future CSP need
 * not allow inline styles. `map-logic.test.ts` holds the two tables equal.
 */
export const GROUP_SWATCH_CLASS: Readonly<Record<string, string>> = {
  "doanh-nghiep": "bg-[#1d4ed8]",
  "ho-kinh-doanh": "bg-[#0f766e]",
  "hop-tac-xa": "bg-[#7e22ce]",
  cho: "bg-[#c2410c]",
  "truong-hoc": "bg-[#15803d]",
  "co-so-y-te": "bg-[#b91c1c]",
  "di-tich": "bg-[#a16207]",
  "du-lich-lang-nghe": "bg-[#0284c7]",
  ocop: "bg-[#9f1239]",
  "ha-tang": "bg-[#1e3a5f]",
  "cong-trinh-dau-tu-cong": "bg-[#111827]",
};

export function groupSwatchClass(code: string): string {
  return GROUP_SWATCH_CLASS[code] ?? "bg-[#64748b]";
}

/** MapLibre `match` expression: colour by `asset_type_code`. */
export function colourExpression(): unknown[] {
  const pairs = Object.entries(GROUP_COLOURS).flat();
  return ["match", ["get", "asset_type_code"], ...pairs, OTHER_GROUP_COLOUR];
}

/** Filter of the individual-point and label layers: not a cluster, and a visible group. */
export function pointsFilter(visible: ReadonlySet<string>): unknown[] {
  return ["all", ["!", ["has", "point_count"]], ["in", ["get", "asset_type_code"], ["literal", [...visible].sort()]]];
}

/** Filter of the highlight layer: exactly the selected id, or nothing. */
export function selectedFilter(id: string | null): unknown[] {
  return ["all", ["!", ["has", "point_count"]], ["==", ["get", "id"], id ?? ""]];
}

/* ---- Vietnamese basemap labels (ADR 0072 H1) ------------------------------------------------- */

/** `name:vi` first, then the local `name` — the `liberty` style prefers `name_en`. */
export const VIETNAMESE_TEXT_FIELD: readonly unknown[] = ["coalesce", ["get", "name:vi"], ["get", "name"]];

type StyleLayerLike = { readonly id: string; readonly type: string; readonly layout?: Record<string, unknown> };

/**
 * Ids of the basemap symbol layers whose `text-field` shows a place NAME (references `name…`). Road
 * shields (`{ref}`) and house numbers keep their own field: rewriting those to a name would print a
 * street name on a highway shield.
 */
export function nameLabelLayers(layers: readonly StyleLayerLike[]): string[] {
  return layers
    .filter((l) => l.type === "symbol" && l.layout !== undefined && l.layout["text-field"] !== undefined)
    .filter((l) => /"name|\{name/.test(JSON.stringify(l.layout!["text-field"])))
    .map((l) => l.id);
}

/* ---- formatting ------------------------------------------------------------------------------ */

/** "26 đối tượng · 42,3% đã xác minh" — ratio 0..1 from the server, Vietnamese decimal comma. */
export function summaryLine(total: number, ratio: number): string {
  const pct = new Intl.NumberFormat("vi-VN", { maximumFractionDigits: 1 }).format(ratio * 100);
  return `${new Intl.NumberFormat("vi-VN").format(total)} đối tượng · ${pct}% đã xác minh`;
}
