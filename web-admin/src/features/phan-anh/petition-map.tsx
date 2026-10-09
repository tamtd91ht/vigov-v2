"use client";

import type { GeoJSONSource, Map as MapLibreMap } from "maplibre-gl";
import { CloudOff, MapPinOff, RefreshCw } from "lucide-react";
import { useEffect, useRef, useState, type ComponentType, type ReactNode } from "react";

import { Button } from "@/components/ui/button";
import { FRAME_LOAD_FAILED, FRAME_NOT_SET } from "@/features/map-assets/labels";
import { frameFromApi, insideFrame, MAPLIBRE_WORKER_URL, type MapFrame } from "@/features/map-assets/map-logic";
import { cn } from "@/lib/cn";
import type { KetQua } from "@/lib/api/goi";
import { getMapFrame } from "@/lib/api/map-frame";
import { listCitizenReportPoints, type CitizenReportFilter, type PointsResult } from "@/lib/api/phieu-phan-anh";
import type { comms_mapFrameOut, petitions_citizenReportPointOut } from "@/lib/api/schema.gen";
import { BASEMAP_MISSING_SENTENCE } from "@/lib/basemap/assets";
import { registerPmtilesProtocol } from "@/lib/basemap/protocol";
import { buildBasemapStyle } from "@/lib/basemap/style";

import {
  HEAT_MAP_EMPTY_HINT,
  HEAT_MAP_EMPTY_TITLE,
  HEAT_MAP_LOAD_FAILED,
  HEAT_MAP_LOADING,
  HEAT_MAP_REGION_LABEL,
  heatMapPointsCaption,
  MAP_LOAD_FAILED,
  MAP_LOADING,
  MINI_MAP_OUTSIDE_FRAME,
  MINI_MAP_REGION_LABEL,
} from "./nhan-phieu";
import { Glyph } from "./petition-ui";

/**
 * The two petition maps of `/phan-anh` — the `Bản đồ nhiệt` tab (prototype `FeedbackHeatmap.tsx`) and
 * the drawer's `Vị trí` mini-map (`FeedbackMiniMap.tsx`) — on the SELF-HOSTED basemap only.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * THE BASEMAP IS THE PERSONAL-DATA BOUNDARY (ADR 0072 §Sửa đổi + §Trả lời 09/10/2026). These maps draw
 * CITIZENS' coordinates, so the viewport around a citizen's home must never reach a third-party tile
 * server (rule 3 stop #2). The style is built in code by `buildBasemapStyle(window.location.origin)`:
 * every URL in it is `<this origin>/basemap/…`. NEVER `MAP_STYLE_URL` / OpenFreeMap (that is `/ban-do`,
 * H1), not even "until the file exists": basemap not configured (`BASEMAP_URL` unset, decided on the
 * SERVER page and passed down as a flag) → one sentence, no map, and no coordinates are even fetched.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * THE FRAME IS THE COMMUNE'S (`GET /api/v1/map-frame`, readable with `feedback.read` since ADR 0072
 * §Trả lời 09/10/2026) and goes into the CONSTRUCTOR as `maxBounds` — never set afterwards, so no view
 * ever shows anything outside the commune (the economic map's rule, `economic-map.tsx`). The prototype's
 * hard-coded Đà Nẵng centre is a per-commune value and is not used (rule 1 invariant 10). The frame is
 * read through `frameFromApi`, the economic map's own parser — one parser, not a fork.
 *
 * LIFECYCLE is `economic-map.tsx`'s: MapLibre and PMTiles are dynamic imports inside the effect (they
 * need `window` and are not in the bundle of a page that draws no map), the worker URL is set before
 * any map exists, the instance lives in a ref (never state), and `map.remove()` on unmount releases the
 * WebGL context. No popup and no HTML anywhere (rule 13 #3).
 */

/* ── the commune frame ──────────────────────────────────────────────────────────────────────── */

export type FrameState =
  | { readonly kind: "loading" }
  | { readonly kind: "ready"; readonly frame: MapFrame }
  | { readonly kind: "unset" }
  | { readonly kind: "error"; readonly message: string };

export type FrameLoader = () => Promise<KetQua<comms_mapFrameOut>>;

/** The frame in effect, read once when `enabled` (no basemap → no read at all). */
function useCommuneFrame(enabled: boolean, load: FrameLoader): FrameState {
  const [state, setState] = useState<FrameState>({ kind: "loading" });
  useEffect(() => {
    if (!enabled) return;
    let dropped = false;
    load().then((r) => {
      if (dropped) return;
      if (!r.ok) return setState({ kind: "error", message: r.thongBao });
      const frame = frameFromApi(r.duLieu);
      setState(frame === null ? { kind: "unset" } : { kind: "ready", frame });
    });
    return () => {
      dropped = true;
    };
  }, [enabled, load]);
  return state;
}

/* ── the MapLibre canvas ────────────────────────────────────────────────────────────────────── */

export type MapContent =
  | { readonly kind: "heat"; readonly points: readonly petitions_citizenReportPointOut[] }
  | { readonly kind: "pin"; readonly lat: number; readonly lng: number };

export type PetitionMapCanvasProps = {
  readonly frame: MapFrame;
  readonly content: MapContent;
  readonly label: string;
  readonly className?: string;
};

export const POINTS_SOURCE_ID = "petition-points";
export const LAYER_HEAT = "petition-heat";
export const LAYER_DOTS = "petition-dots";
/** The mini-map's zoom on the pin (prototype `FeedbackMiniMap.tsx:143`). */
export const PIN_ZOOM = 15;
const PIN_COLOUR = "#E5484D";

function pointCollection(points: readonly petitions_citizenReportPointOut[]) {
  return {
    type: "FeatureCollection" as const,
    features: points.map((p) => ({
      type: "Feature" as const,
      // NOTHING but the position: the route sends lat · lng · status and the map needs only the first two.
      properties: {},
      geometry: { type: "Point" as const, coordinates: [p.lng, p.lat] },
    })),
  };
}

/**
 * One MapLibre map on the self-hosted basemap, locked to the commune frame. The parent renders it only
 * once the frame is known; a changed frame remounts it (`key`).
 */
export function PetitionMapCanvas({ frame, content, label, className }: PetitionMapCanvasProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<MapLibreMap | null>(null);
  const [ready, setReady] = useState(false);
  const [failed, setFailed] = useState(false);
  const contentRef = useRef(content);
  useEffect(() => {
    contentRef.current = content;
  });

  const [minLng, minLat, maxLng, maxLat] = frame.bounds;
  const pinLng = content.kind === "pin" ? content.lng : null;
  const pinLat = content.kind === "pin" ? content.lat : null;

  useEffect(() => {
    let cancelled = false;
    let created: MapLibreMap | null = null;
    const bounds: [number, number, number, number] = [minLng, minLat, maxLng, maxLat];

    void (async () => {
      const ml = await import("maplibre-gl");
      if (cancelled || containerRef.current === null) return;
      // Before ANY map exists — see MAPLIBRE_WORKER_URL for why the default URL is empty under Turbopack.
      ml.setWorkerUrl(MAPLIBRE_WORKER_URL);
      // `pmtiles://` must be known BEFORE the map reads the style, or the source fails silently.
      await registerPmtilesProtocol(ml);
      if (cancelled || containerRef.current === null) return;
      const pin = pinLng !== null && pinLat !== null ? ([pinLng, pinLat] as [number, number]) : null;
      const map = new ml.Map({
        container: containerRef.current,
        // Same origin, built in code — never a fetched or configured style URL.
        style: buildBasemapStyle(window.location.origin),
        // The pin's map opens ON the pin; the heat map on the whole frame. Both are bounded by the frame.
        ...(pin !== null ? { center: pin, zoom: PIN_ZOOM } : { bounds }),
        maxBounds: bounds,
        renderWorldCopies: false,
        // The ODbL credit the style's source carries, compact (the "i" button) on a small map.
        attributionControl: { compact: true },
        // No rotation: a rotated view of a bounded box exposes corners outside it.
        dragRotate: false,
        pitchWithRotate: false,
        touchPitch: false,
      });
      created = map;
      mapRef.current = map;
      map.touchZoomRotate.disableRotation();
      map.addControl(new ml.NavigationControl({ showCompass: false }), "top-right");

      let loaded = false;
      map.on("error", () => {
        // A tile failing after load is a gap; only a style that never loads is a broken map.
        if (!loaded && !cancelled) setFailed(true);
      });

      if (pin !== null) {
        // MapLibre's own default pin element — no HTML of ours, no popup.
        new ml.Marker({ color: PIN_COLOUR }).setLngLat(pin).addTo(map);
      }

      map.on("load", () => {
        loaded = true;
        const c = contentRef.current;
        if (c.kind !== "heat") return setReady(true);
        // The fitted frame is the furthest anyone may zoom out (maxBounds already holds the edges).
        map.setMinZoom(map.getZoom());
        map.addSource(POINTS_SOURCE_ID, {
          type: "geojson",
          data: pointCollection(c.points) as unknown as GeoJSON.FeatureCollection,
        });
        // Prototype `FeedbackHeatmap.tsx:81-116`. EVERY POINT WEIGHS THE SAME: the prototype weighs an
        // overdue petition 3, but the points route carries no deadline (lat · lng · status only, rule 3),
        // and overdue is DERIVED from a stored deadline (rule 10 inv. 3) — never guessed from a status.
        map.addLayer({
          id: LAYER_HEAT,
          type: "heatmap",
          source: POINTS_SOURCE_ID,
          paint: {
            "heatmap-weight": 1,
            "heatmap-intensity": 1.1,
            "heatmap-radius": 34,
            "heatmap-opacity": 0.75,
            "heatmap-color": [
              "interpolate",
              ["linear"],
              ["heatmap-density"],
              0,
              "rgba(47,177,249,0)",
              0.2,
              "rgba(47,177,249,0.45)",
              0.45,
              "rgba(134,185,64,0.55)",
              0.7,
              "rgba(255,122,26,0.7)",
              1,
              "rgba(229,72,77,0.85)",
            ],
          },
        });
        map.addLayer({
          id: LAYER_DOTS,
          type: "circle",
          source: POINTS_SOURCE_ID,
          paint: {
            "circle-radius": 4,
            "circle-color": "#102B43",
            "circle-opacity": 0.65,
            "circle-stroke-width": 1.5,
            "circle-stroke-color": "#FFFFFF",
          },
        });
        setReady(true);
      });
    })().catch(() => {
      // The MapLibre or PMTiles chunk did not load (network): say so, draw nothing.
      if (!cancelled) setFailed(true);
    });

    return () => {
      cancelled = true;
      created?.remove();
      mapRef.current = null;
      setReady(false);
    };
  }, [minLng, minLat, maxLng, maxLat, pinLng, pinLat]);

  // New points for the same frame (a filter changed): new source data, no new map.
  const points = content.kind === "heat" ? content.points : null;
  useEffect(() => {
    const map = mapRef.current;
    if (!ready || map === null || points === null) return;
    const source = map.getSource(POINTS_SOURCE_ID) as GeoJSONSource | undefined;
    source?.setData(pointCollection(points) as unknown as GeoJSON.FeatureCollection);
  }, [ready, points]);

  if (failed) return <MapNote className={className}>{MAP_LOAD_FAILED}</MapNote>;
  return (
    <div
      ref={containerRef}
      role="region"
      aria-label={label}
      data-testid="petition-map"
      className={cn("w-full overflow-hidden rounded-[10px] border border-solid border-line", className)}
    />
  );
}

/** A map-sized box holding one sentence instead of a map (prototype empty box: dashed, centred, 12.5px). */
function MapNote({
  className,
  icon = MapPinOff,
  role,
  children,
}: {
  className?: string;
  icon?: typeof MapPinOff;
  role?: "status" | "alert";
  children: ReactNode;
}) {
  return (
    <div
      className={cn(
        "grid w-full place-items-center rounded-[10px] border border-dashed border-line px-4 text-center text-[12.5px] text-ink-muted",
        className,
      )}
    >
      <p className="m-0 flex max-w-sm flex-col items-center gap-1.5" role={role}>
        <Glyph icon={icon} className="size-5 opacity-60" />
        <span>{children}</span>
      </p>
    </div>
  );
}

/** The frame's non-ready states, in the map's own box. */
function FrameNote({ state, className }: { state: Exclude<FrameState, { kind: "ready" }>; className: string }) {
  if (state.kind === "loading") return <MapNote className={className} role="status">{MAP_LOADING}</MapNote>;
  if (state.kind === "unset") return <MapNote className={className}>{FRAME_NOT_SET}</MapNote>;
  return (
    <MapNote className={className} icon={CloudOff} role="alert">
      {FRAME_LOAD_FAILED}: {state.message}
    </MapNote>
  );
}

/* ── the drawer's mini-map ──────────────────────────────────────────────────────────────────── */

const MINI_MAP_BOX = "h-52";

/**
 * `Vị trí` (prototype `FeedbackMiniMap.tsx`): 208px, one pin at the petition's coordinates, locked to
 * the commune frame. A point outside the frame is not shown (the map cannot be moved there) — the box
 * says so instead.
 */
export function PetitionMiniMap({
  lat,
  lng,
  basemapAvailable,
  loadFrame = getMapFrame,
  Canvas = PetitionMapCanvas,
}: {
  lat: number;
  lng: number;
  /** `basemapConfigured()`, decided on the server page. `false` → the sentence, no map, no frame read. */
  basemapAvailable: boolean;
  /** Injected only by tests. */
  loadFrame?: FrameLoader;
  Canvas?: ComponentType<PetitionMapCanvasProps>;
}) {
  const frame = useCommuneFrame(basemapAvailable, loadFrame);
  if (!basemapAvailable) return <MapNote className={MINI_MAP_BOX}>{BASEMAP_MISSING_SENTENCE}</MapNote>;
  if (frame.kind !== "ready") return <FrameNote state={frame} className={MINI_MAP_BOX} />;
  if (!insideFrame(frame.frame.bounds, lng, lat)) return <MapNote className={MINI_MAP_BOX}>{MINI_MAP_OUTSIDE_FRAME}</MapNote>;
  return (
    <Canvas
      key={frame.frame.bounds.join(",")}
      frame={frame.frame}
      content={{ kind: "pin", lat, lng }}
      label={MINI_MAP_REGION_LABEL}
      className={MINI_MAP_BOX}
    />
  );
}

/* ── the `Bản đồ nhiệt` tab ─────────────────────────────────────────────────────────────────── */

const HEAT_MAP_BOX = "h-80";

type PointsState = { readonly key: string; readonly result: PointsResult };

/**
 * The heat-map tab: EVERY located petition the list's filters match (`citizen-report-points`, same
 * query builder as the list), drawn as a heat layer with a dot per petition over it.
 *
 * 422 `too_many_points` (more than 5000): the server's sentence, and NOTHING drawn — a heat map of part
 * of the points looks like the commune's density and is not (ADR 0072 §Trả lời 09/10/2026). No
 * `Tải lại` there: the same filters get the same answer; narrowing them is the way out.
 */
export function PetitionHeatMap({
  filter,
  basemapAvailable,
  loadPoints = listCitizenReportPoints,
  loadFrame = getMapFrame,
  Canvas = PetitionMapCanvas,
}: {
  filter: CitizenReportFilter;
  basemapAvailable: boolean;
  /** Injected only by tests. */
  loadPoints?: (filter: CitizenReportFilter) => Promise<PointsResult>;
  loadFrame?: FrameLoader;
  Canvas?: ComponentType<PetitionMapCanvasProps>;
}) {
  const [reloads, setReloads] = useState(0);
  const [loaded, setLoaded] = useState<PointsState | null>(null);
  const key = `${JSON.stringify(filter)}|${reloads}`;
  const frame = useCommuneFrame(basemapAvailable, loadFrame);

  useEffect(() => {
    // No basemap → nothing will be drawn, so no citizen coordinate is even asked for.
    if (!basemapAvailable) return;
    let dropped = false;
    loadPoints(filter).then((result) => {
      if (!dropped) setLoaded({ key, result });
    });
    return () => {
      dropped = true;
    };
    // `key` carries the filter (by value) and the reload counter.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key, basemapAvailable, loadPoints]);

  if (!basemapAvailable) return <MapNote className={HEAT_MAP_BOX}>{BASEMAP_MISSING_SENTENCE}</MapNote>;

  const current = loaded !== null && loaded.key === key ? loaded.result : null;
  if (current === null) return <MapNote className={HEAT_MAP_BOX} role="status">{HEAT_MAP_LOADING}</MapNote>;

  if (!current.ok) {
    if (current.tooManyPoints) {
      return (
        <MapNote className={HEAT_MAP_BOX} role="status">
          {current.message}
        </MapNote>
      );
    }
    return (
      <div className={cn("grid w-full place-items-center rounded-[10px] border border-dashed border-line px-4", HEAT_MAP_BOX)}>
        <div className="flex max-w-sm flex-col items-center gap-2 text-center text-[12.5px] [&>p]:m-0">
          <Glyph icon={CloudOff} className="size-5 text-danger-600" />
          <p className="font-semibold text-navy">{HEAT_MAP_LOAD_FAILED}</p>
          <p className="text-danger-600" role="alert">
            {current.message}
          </p>
          <Button
            type="button"
            variant="secondary"
            size="sm"
            icon={<Glyph icon={RefreshCw} />}
            onClick={() => setReloads((n) => n + 1)}
          >
            Tải lại
          </Button>
        </div>
      </div>
    );
  }

  const points = current.data.items;
  if (points.length === 0) {
    // Prototype `FeedbackHeatmap.tsx:58-69`.
    return (
      <div className={cn("grid w-full place-items-center rounded-[10px] border border-dashed border-line text-center text-[12.5px] text-ink-muted", HEAT_MAP_BOX)}>
        <div className="max-w-xs [&>p]:m-0">
          <p>{HEAT_MAP_EMPTY_TITLE}</p>
          <p className="mt-1 text-[11.5px]">{HEAT_MAP_EMPTY_HINT}</p>
        </div>
      </div>
    );
  }

  if (frame.kind !== "ready") return <FrameNote state={frame} className={HEAT_MAP_BOX} />;

  return (
    <div className="flex min-w-0 flex-col gap-1.5 [&>p]:m-0">
      <Canvas
        key={frame.frame.bounds.join(",")}
        frame={frame.frame}
        content={{ kind: "heat", points }}
        label={HEAT_MAP_REGION_LABEL}
        className={HEAT_MAP_BOX}
      />
      <p className="text-[11.5px] text-ink-muted" role="status">
        {heatMapPointsCaption(points.length)}
      </p>
    </div>
  );
}
