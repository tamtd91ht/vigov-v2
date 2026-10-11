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

/** Centre + radius + bounds, checked — the shape of both the effective frame and the platform default. */
export type FrameValues = {
  readonly centerLat: number;
  readonly centerLng: number;
  readonly radiusKm: number;
  readonly bounds: FrameBounds;
};

/**
 * Whose frame is in effect (ADR 0072 K3): the commune's own (`commune`) or the platform default set by
 * the vendor's operator (`default`). The commune's own always wins when it exists.
 */
export type FrameSource = "commune" | "default";

/** The frame in effect, checked. */
export type MapFrame = FrameValues & { readonly source: FrameSource };

function checkedValues(lat: unknown, lng: unknown, r: unknown, b: unknown): FrameValues | null {
  if (!Array.isArray(b) || b.length !== 4 || !b.every((n) => typeof n === "number" && Number.isFinite(n))) return null;
  const [minLng, minLat, maxLng, maxLat] = b as [number, number, number, number];
  if (!(minLng < maxLng && minLat < maxLat)) return null;
  if (minLng < -180 || maxLng > 180 || minLat < -90 || maxLat > 90) return null;
  if (typeof lat !== "number" || typeof lng !== "number" || typeof r !== "number") return null;
  return { centerLat: lat, centerLng: lng, radiusKm: r, bounds: [minLng, minLat, maxLng, maxLat] };
}

/**
 * The frame the map may be built in, or `null`. FAIL CLOSED: `configured:false`, a missing field, a
 * `source` that is neither `commune` nor `default`, or bounds that are not four finite, ordered,
 * in-range numbers all mean NO MAP — never a world view, never a guessed box (a world view shows the
 * archipelagos with OSM's names, ADR 0072 H3). An unknown source is refused rather than read as one of
 * the two: the screen would otherwise tell the officer whose frame this is, and say it wrongly.
 */
export function frameFromApi(out: comms_mapFrameOut): MapFrame | null {
  if (!out.configured) return null;
  if (out.source !== "commune" && out.source !== "default") return null;
  const v = checkedValues(out.center_lat, out.center_lng, out.radius_km, out.bounds);
  return v === null ? null : { ...v, source: out.source };
}

/**
 * The platform default carried by the reply (`default`), or `null` — absent, or malformed. Always present
 * with `source: "default"`; only best-effort with `source: "commune"` (the server may fail to read it),
 * so `null` there means "not known", never "the platform has none".
 */
export function frameDefaultFromApi(out: comms_mapFrameOut): FrameValues | null {
  const d = out.default;
  if (d === null || d === undefined) return null;
  return checkedValues(d.center_lat, d.center_lng, d.radius_km, d.bounds);
}

/** The form's hints and the notice version, as the SERVER sent them (ADR 0072 K2 open item #2: owner may tune). */
export type FrameHints = {
  /** `recommended_radius_km`, or `null` when not a positive number. */
  readonly recommendedKm: number | null;
  /** `usual_radius_km` as `[low, high]`, or `null` when not two ordered numbers — then no warning is shown. */
  readonly usualKm: readonly [number, number] | null;
  /** The server's current version of the K6 text; what an acknowledgement must carry. */
  readonly noticeVersion: string;
};

export function frameHintsFromApi(out: comms_mapFrameOut): FrameHints {
  const rec = out.recommended_radius_km;
  const u = out.usual_radius_km;
  const usual =
    Array.isArray(u) && u.length === 2 && u.every((n) => typeof n === "number" && Number.isFinite(n)) && u[0]! < u[1]!
      ? ([u[0]!, u[1]!] as const)
      : null;
  return {
    recommendedKm: typeof rec === "number" && Number.isFinite(rec) && rec > 0 ? rec : null,
    usualKm: usual,
    noticeVersion: typeof out.notice_version === "string" ? out.notice_version : "",
  };
}

/** True when the radius lies outside the usual range — a WARNING, never a refusal (K2: the officer decides). */
export function radiusUnusual(radiusKm: number, usualKm: readonly [number, number] | null): boolean {
  return usualKm !== null && (radiusKm < usualKm[0] || radiusKm > usualKm[1]);
}

/** True when `[lng, lat]` lies inside the frame (edges included). */
export function insideFrame(bounds: FrameBounds, lng: number, lat: number): boolean {
  if (!Number.isFinite(lng) || !Number.isFinite(lat)) return false;
  const [minLng, minLat, maxLng, maxLat] = bounds;
  return lng >= minLng && lng <= maxLng && lat >= minLat && lat <= maxLat;
}

/**
 * Hard radius bounds of the frame form — "> 0 and ≤ 50 km" (ADR 0072 K2), and > 0 is ≥ 0.1 in practice
 * because the column is `numeric(4,1)`. The SAME for every commune (a platform rule, not a per-commune
 * value), and only a convenience: the server re-checks and its sentence wins. Raising 50 is a stop
 * condition of the ADR, not an edit here.
 */
export const FRAME_RADIUS_MIN_KM = 0.1;
export const FRAME_RADIUS_MAX_KM = 50;

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

/* ---- commune centre --------------------------------------------------------------------------- */

/**
 * The SAVED frame centre (map-frame `center_lat`/`center_lng`), drawn as a fixed landmark so staff
 * see where the commune is the moment the map opens (owner, 04/10/2026). Its own source and layers,
 * BENEATH the asset layers: it never covers an asset nor takes its clicks, and it is not an asset.
 */
export const CENTRE_SOURCE_ID = "commune-centre";
export const LAYER_CENTRE_HALO = "commune-centre-halo";
export const LAYER_CENTRE = "commune-centre-point";
export const LAYER_CENTRE_LABEL = "commune-centre-label";
/** Prototype's red (`asset-display.ts` "Cơ sở y tế", spec 08 B "Điểm tâm xã"). */
export const CENTRE_COLOUR = "#E5484D";
/** Navy of the prototype's pin labels (`AssetMap.tsx:249`) — also the centre's label (spec 08 B). */
export const LABEL_COLOUR = "#102B43";

/** One Point at the frame centre, `[lng, lat]` (GeoJSON order), labelled with the commune's name. */
export function centreCollection(frame: FrameValues, label: string) {
  return {
    type: "FeatureCollection" as const,
    features: [
      {
        type: "Feature" as const,
        geometry: { type: "Point" as const, coordinates: [frame.centerLng, frame.centerLat] as [number, number] },
        properties: { label },
      },
    ],
  };
}

/* ---- layers ----------------------------------------------------------------------------------- */

export const SOURCE_ID = "economic-places";
export const LAYER_CLUSTERS = "economic-clusters";
export const LAYER_CLUSTER_COUNT = "economic-cluster-count";
export const LAYER_POINTS = "economic-points";
export const LAYER_LABELS = "economic-labels";
/** The faint disc under each pin (prototype `AssetMap.tsx:203-212`). Not clickable — the pin is. */
export const LAYER_HALO = "economic-points-halo";
export const CLUSTER_MAX_ZOOM = 14;
export const CLUSTER_RADIUS = 50;
export const LABEL_MIN_ZOOM = 14;

/** Fallback colour of a group the commune added itself (not one of the eleven) — the prototype's "no key" grey. */
export const OTHER_GROUP_COLOUR = "#8AA2B8";

/**
 * One colour per default group: the prototype's `ASSET_LAYERS` (`vigov-require/apps/admin/src/lib/
 * asset-display.ts:18-28`, spec 04) mapped onto OUR codes (ADR 0072 §3) — their `enterprise` is our
 * `doanh-nghiep`, and so on in the same order. The legend and the detail panel always show the group's
 * NAME too — colour is never the only signal.
 */
export const GROUP_COLOURS: Readonly<Record<string, string>> = {
  "doanh-nghiep": "#2FB1F9",
  "ho-kinh-doanh": "#12B5C9",
  "hop-tac-xa": "#6E59E8",
  cho: "#FF7A1A",
  "truong-hoc": "#86B940",
  "co-so-y-te": "#E5484D",
  "di-tich": "#B8860B",
  "du-lich-lang-nghe": "#DB7093",
  ocop: "#8B5A2B",
  "ha-tang": "#43607B",
  "cong-trinh-dau-tu-cong": "#102B43",
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
  "doanh-nghiep": "bg-[#2FB1F9]",
  "ho-kinh-doanh": "bg-[#12B5C9]",
  "hop-tac-xa": "bg-[#6E59E8]",
  cho: "bg-[#FF7A1A]",
  "truong-hoc": "bg-[#86B940]",
  "co-so-y-te": "bg-[#E5484D]",
  "di-tich": "bg-[#B8860B]",
  "du-lich-lang-nghe": "bg-[#DB7093]",
  ocop: "bg-[#8B5A2B]",
  "ha-tang": "bg-[#43607B]",
  "cong-trinh-dau-tu-cong": "bg-[#102B43]",
};

export function groupSwatchClass(code: string): string {
  return GROUP_SWATCH_CLASS[code] ?? "bg-[#8AA2B8]";
}

/**
 * Pin, halo and label sizes (prototype `AssetMap.tsx:208,220,243`): larger in "Trình chiếu" so a room
 * reads the map off a projector (spec 03).
 */
export function pointSizes(presentation: boolean): { readonly pin: number; readonly halo: number; readonly label: number } {
  return presentation ? { pin: 8, halo: 13, label: 14 } : { pin: 6, halo: 10, label: 11 };
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

/* ---- "Bản đồ nhiệt phản ánh" (spec 02, owner 11/10/2026) ------------------------------------- */

/** GeoJSON source of the petition points. Separate from the asset source: never clustered, never clicked. */
export const HEAT_SOURCE_ID = "feedback-heat-points";
export const LAYER_HEAT = "feedback-heat";

/**
 * Spec `02-cong-nghe-ban-do.md`, "Bản đồ nhiệt phản ánh" — verbatim. The prototype fed this paint from
 * the ASSET tiles; the spec says to keep the paint and use real petition data when the backend has it,
 * which it does (`citizen-report-points`, the route `/phan-anh` draws from).
 */
export const HEAT_PAINT = {
  "heatmap-radius": 28,
  "heatmap-opacity": 0.45,
  "heatmap-color": [
    "interpolate",
    ["linear"],
    ["heatmap-density"],
    0,
    "rgba(47,177,249,0)",
    0.4,
    "rgba(47,177,249,0.5)",
    0.7,
    "rgba(255,122,26,0.6)",
    1,
    "rgba(229,72,77,0.8)",
  ],
} as const;

export type HeatPoint = { readonly lat: number; readonly lng: number };

/**
 * Petition points as GeoJSON — POSITION ONLY. The route also sends a status; it is dropped here so no
 * attribute of a citizen's petition reaches the map's worker or a feature query (rule 3).
 */
export function heatCollection(points: readonly HeatPoint[]) {
  return {
    type: "FeatureCollection" as const,
    features: points.map((p) => ({
      type: "Feature" as const,
      properties: {},
      geometry: { type: "Point" as const, coordinates: [p.lng, p.lat] },
    })),
  };
}

/* ---- formatting ------------------------------------------------------------------------------ */

/** "26 đối tượng · 42,3% đã xác minh" — ratio 0..1 from the server, Vietnamese decimal comma. */
export function summaryLine(total: number, ratio: number): string {
  const pct = new Intl.NumberFormat("vi-VN", { maximumFractionDigits: 1 }).format(ratio * 100);
  return `${new Intl.NumberFormat("vi-VN").format(total)} đối tượng · ${pct}% đã xác minh`;
}
