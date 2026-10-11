"use client";

import type {
  ExpressionSpecification,
  FilterSpecification,
  GeoJSONSource,
  HeatmapLayerSpecification,
  Map as MapLibreMap,
  MapGeoJSONFeature,
  MapLayerMouseEvent,
} from "maplibre-gl";
import { useEffect, useRef, useState } from "react";

import { registerPmtilesProtocol } from "@/lib/basemap/protocol";
import { cn } from "@/lib/cn";
import { buildBasemapStyle } from "@/lib/basemap/style";

import {
  CENTRE_COLOUR,
  CENTRE_SOURCE_ID,
  LAYER_CENTRE,
  LAYER_CENTRE_HALO,
  LAYER_CENTRE_LABEL,
  centreCollection,
  CLUSTER_MAX_ZOOM,
  CLUSTER_RADIUS,
  HEAT_PAINT,
  HEAT_SOURCE_ID,
  LABEL_COLOUR,
  LABEL_MIN_ZOOM,
  LAYER_CLUSTER_COUNT,
  LAYER_CLUSTERS,
  LAYER_HALO,
  LAYER_HEAT,
  LAYER_LABELS,
  LAYER_POINTS,
  MAPLIBRE_WORKER_URL,
  SOURCE_ID,
  colourExpression,
  heatCollection,
  insideFrame,
  pointSizes,
  pointsFilter,
  visibleCollection,
  type EconomicCollection,
  type HeatPoint,
  type MapFrame,
} from "./map-logic";

/**
 * The MapLibre map of `/ban-do` — the ONLY component that touches the map instance.
 *
 * THE BASEMAP IS THE SELF-HOSTED ONE (owner 11/10/2026, the same as `/phan-anh`): the style is built in
 * code by `buildBasemapStyle(window.location.origin)`, every URL in it is `<this origin>/basemap/…`, and
 * nothing is fetched from a third-party host. The parent draws this component only when the server page
 * found `BASEMAP_URL` set; unset → one sentence and no map, never a fallback provider. The map also
 * carries citizens' petition points (the heat layer), which is exactly why no viewport may leave the
 * origin (rule 3 stop #2). Vietnamese place names come from the style (`name:vi`, then `name`).
 *
 * LIFECYCLE (ADR 0072; the user's engineering guide §17–18):
 *   - MapLibre and PMTiles are loaded with a DYNAMIC import inside the effect: they need `window` and
 *     WebGL, and the page is server-rendered. Neither is imported at module level.
 *   - The map is created ONCE per frame and kept in a `useRef` — never React state: a map in
 *     state re-renders the tree on every camera move, and a re-render must never rebuild the map.
 *   - `map.remove()` on unmount releases the WebGL context; browsers cap live contexts, so a leaked one
 *     per visit ends in a blank map.
 *
 * THE FRAME IS IN THE CONSTRUCTOR (ADR 0072 H3; security review 04/10/2026). `bounds` and `maxBounds`
 * are constructor options, never set afterwards: a map built first and bounded second draws one frame
 * at the default world view — which shows the archipelagos. The parent renders this component only
 * once the frame is known; a changed frame remounts it (`key`), so maxBounds is never mutated.
 *
 * NO DOM MARKER PER ASSET. One GeoJSON source, clustered in the browser, drawn by WebGL layers.
 * Details never go into a MapLibre popup (`setHTML` would be raw HTML, rule 13 #3): a click hands the
 * id to the parent, which opens a React panel.
 */
export type FlyRequest = { readonly lng: number; readonly lat: number; readonly seq: number };

export type EconomicMapProps = {
  frame: MapFrame;
  /** Name drawn beside the saved frame centre (the commune's display name). */
  centreLabel: string;
  /** Grown into the page overlay ("Mở rộng bản đồ"): fill the region instead of the fixed height. */
  expanded?: boolean;
  /**
   * "Trình chiếu" (spec 03): bigger pins and labels, no scale bar. The SAME map instance — switching
   * repaints, it never rebuilds (a rebuild would refit the frame and drop the officer's view).
   */
  presentation?: boolean;
  collection: EconomicCollection;
  visible: ReadonlySet<string>;
  /** Fly to a point — honoured ONLY inside the frame (the parent checks too). `seq` makes a repeat fly again. */
  flyTo: FlyRequest | null;
  /**
   * Petition points of the heat layer, or `null` = layer hidden. The parent passes points only while the
   * switch is on AND the points route answered; any refusal or error is `null` — never part of a set.
   */
  heatPoints?: readonly HeatPoint[] | null;
  onPointClick: (id: string) => void;
  /** The style could not be loaded (before the first `load`). */
  onLoadError: () => void;
};

const POINT_COLOUR = colourExpression() as unknown as ExpressionSpecification;

export function EconomicMap({
  frame,
  centreLabel,
  expanded = false,
  presentation = false,
  collection,
  visible,
  flyTo,
  heatPoints = null,
  onPointClick,
  onLoadError,
}: EconomicMapProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<MapLibreMap | null>(null);
  const [ready, setReady] = useState(false);

  // Latest callbacks and data for handlers registered once at creation.
  const onPointClickRef = useRef(onPointClick);
  const onLoadErrorRef = useRef(onLoadError);
  const dataRef = useRef({ collection, visible, heatPoints, presentation });
  // The saved centre, read when the map loads; a changed frame remounts this component (`key`), so a
  // stale centre cannot survive. The label may change (name corrected) and is pushed by an effect below.
  const centreRef = useRef(centreCollection(frame, centreLabel));
  useEffect(() => {
    onPointClickRef.current = onPointClick;
    onLoadErrorRef.current = onLoadError;
    dataRef.current = { collection, visible, heatPoints, presentation };
    centreRef.current = centreCollection(frame, centreLabel);
  });

  const [minLng, minLat, maxLng, maxLat] = frame.bounds;

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
      const map = new ml.Map({
        container: containerRef.current,
        // Same origin, built in code — never a fetched or configured style URL.
        style: buildBasemapStyle(window.location.origin),
        bounds,
        maxBounds: bounds,
        renderWorldCopies: false,
        // The ODbL credit the style's source carries (OpenStreetMap), compact — the "i" button — as on
        // the `/phan-anh` maps. No provider string of our own.
        attributionControl: { compact: true },
        // No rotation: a rotated view of a bounded box exposes corners outside it, and staff read a
        // north-up map.
        dragRotate: false,
        pitchWithRotate: false,
        touchPitch: false,
      });
      created = map;
      mapRef.current = map;
      map.touchZoomRotate.disableRotation();
      map.addControl(new ml.NavigationControl({ showCompass: false }), "top-right");
      map.addControl(new ml.ScaleControl({ unit: "metric" }), "bottom-right");

      let loaded = false;
      map.on("error", () => {
        // A tile that fails after load is a gap, not a broken page; only a style that never loads is.
        if (!loaded) onLoadErrorRef.current();
      });

      map.on("load", () => {
        loaded = true;
        // The fitted frame is the furthest anyone may zoom out (maxBounds already holds the edges).
        map.setMinZoom(map.getZoom());

        const d = dataRef.current;

        // "Bản đồ nhiệt phản ánh" — the LOWEST of our layers: a density wash beneath the centre and every
        // asset pin, so it never covers a pin nor takes its click. Created hidden and empty; the effect
        // below fills and shows it only while the parent passes points.
        map.addSource(HEAT_SOURCE_ID, {
          type: "geojson",
          data: heatCollection(d.heatPoints ?? []) as unknown as GeoJSON.FeatureCollection,
        });
        map.addLayer({
          id: LAYER_HEAT,
          type: "heatmap",
          source: HEAT_SOURCE_ID,
          layout: { visibility: d.heatPoints === null ? "none" : "visible" },
          paint: HEAT_PAINT as unknown as HeatmapLayerSpecification["paint"],
        });

        // The saved commune centre — added before the asset layers so every one draws above it and keeps its clicks.
        map.addSource(CENTRE_SOURCE_ID, {
          type: "geojson",
          data: centreRef.current as unknown as GeoJSON.FeatureCollection,
        });
        map.addLayer({
          id: LAYER_CENTRE_HALO,
          type: "circle",
          source: CENTRE_SOURCE_ID,
          paint: { "circle-radius": 16, "circle-color": CENTRE_COLOUR, "circle-opacity": 0.18 },
        });
        map.addLayer({
          id: LAYER_CENTRE,
          type: "circle",
          source: CENTRE_SOURCE_ID,
          paint: {
            "circle-radius": 7,
            "circle-color": CENTRE_COLOUR,
            "circle-stroke-width": 3,
            "circle-stroke-color": "#ffffff",
          },
        });
        map.addLayer({
          id: LAYER_CENTRE_LABEL,
          type: "symbol",
          source: CENTRE_SOURCE_ID,
          layout: {
            "text-field": ["get", "label"],
            // A stack `/basemap/fonts/` serves (`BASEMAP_FONTS`); any other draws no text, with no error.
            "text-font": ["Noto Sans Medium"],
            "text-size": 14,
            "text-offset": [0, 1.4],
            "text-anchor": "top",
            // Always shown: it is the one landmark that answers "where is the commune".
            "text-allow-overlap": true,
            "text-ignore-placement": true,
          },
          // Navy with a white halo, as the asset labels (spec 08 B) — the red is the dot's, not the name's.
          paint: { "text-color": LABEL_COLOUR, "text-halo-color": "#ffffff", "text-halo-width": 1.4 },
        });

        map.addSource(SOURCE_ID, {
          type: "geojson",
          data: visibleCollection(d.collection, d.visible) as unknown as GeoJSON.FeatureCollection,
          cluster: true,
          clusterMaxZoom: CLUSTER_MAX_ZOOM,
          clusterRadius: CLUSTER_RADIUS,
        });
        map.addLayer({
          id: LAYER_CLUSTERS,
          type: "circle",
          source: SOURCE_ID,
          filter: ["has", "point_count"],
          paint: {
            "circle-color": "#334155",
            "circle-radius": ["step", ["get", "point_count"], 16, 10, 20, 50, 26],
            "circle-stroke-width": 2,
            "circle-stroke-color": "#ffffff",
          },
        });
        map.addLayer({
          id: LAYER_CLUSTER_COUNT,
          type: "symbol",
          source: SOURCE_ID,
          filter: ["has", "point_count"],
          layout: {
            "text-field": ["get", "point_count_abbreviated"],
            // A stack `/basemap/fonts/` serves (`BASEMAP_FONTS`).
            "text-font": ["Noto Sans Medium"],
            "text-size": 13,
            "text-allow-overlap": true,
          },
          paint: { "text-color": "#ffffff" },
        });
        // Prototype `AssetMap.tsx:200-251`: a faint halo beneath each pin, the pin, its name from z14.
        // No selection ring — the prototype has none; the detail panel names the chosen place.
        const size = pointSizes(d.presentation);
        map.addLayer({
          id: LAYER_HALO,
          type: "circle",
          source: SOURCE_ID,
          filter: pointsFilter(d.visible) as unknown as FilterSpecification,
          paint: { "circle-color": POINT_COLOUR, "circle-radius": size.halo, "circle-opacity": 0.18 },
        });
        map.addLayer({
          id: LAYER_POINTS,
          type: "circle",
          source: SOURCE_ID,
          filter: pointsFilter(d.visible) as unknown as FilterSpecification,
          paint: {
            "circle-color": POINT_COLOUR,
            "circle-radius": size.pin,
            "circle-stroke-width": 1.5,
            "circle-stroke-color": "#ffffff",
            // Unverified drawn LIGHTER (spec §10 `da_xac_minh`); the panel says it in words.
            "circle-opacity": ["case", ["==", ["get", "verified"], true], 1, 0.55],
          },
        });
        map.addLayer({
          id: LAYER_LABELS,
          type: "symbol",
          source: SOURCE_ID,
          minzoom: LABEL_MIN_ZOOM,
          filter: pointsFilter(d.visible) as unknown as FilterSpecification,
          layout: {
            "text-field": ["get", "name"],
            "text-font": ["Noto Sans Regular"],
            "text-size": size.label,
            "text-offset": [0, 1.4],
            "text-anchor": "top",
            "text-max-width": 12,
          },
          paint: { "text-color": LABEL_COLOUR, "text-halo-color": "#ffffff", "text-halo-width": 1.4 },
        });

        map.on("click", LAYER_CLUSTERS, (e: MapLayerMouseEvent) => {
          const f: MapGeoJSONFeature | undefined = e.features?.[0];
          const clusterId = f?.properties?.cluster_id;
          if (typeof clusterId !== "number" || f === undefined || f.geometry.type !== "Point") return;
          const [lng, lat] = f.geometry.coordinates as [number, number];
          const source = map.getSource(SOURCE_ID) as GeoJSONSource | undefined;
          void source?.getClusterExpansionZoom(clusterId).then((zoom) => {
            // Zoom INTO the frame only; a cluster centre outside it zooms at the click instead.
            const center: [number, number] = insideFrame(bounds, lng, lat) ? [lng, lat] : [e.lngLat.lng, e.lngLat.lat];
            map.easeTo({ center, zoom });
          });
        });
        map.on("click", LAYER_POINTS, (e: MapLayerMouseEvent) => {
          const id = e.features?.[0]?.properties?.id;
          if (typeof id === "string") onPointClickRef.current(id);
        });
        for (const layer of [LAYER_CLUSTERS, LAYER_POINTS]) {
          map.on("mouseenter", layer, () => {
            map.getCanvas().style.cursor = "pointer";
          });
          map.on("mouseleave", layer, () => {
            map.getCanvas().style.cursor = "";
          });
        }
        setReady(true);
      });
    })().catch(() => {
      // The MapLibre or PMTiles chunk did not load (network): the same state as a style that never loads.
      if (!cancelled) onLoadErrorRef.current();
    });

    return () => {
      cancelled = true;
      created?.remove();
      mapRef.current = null;
      setReady(false);
    };
  }, [minLng, minLat, maxLng, maxLat]);

  // Heat layer: points → fill and show; `null` → empty and hide. Emptied as well as hidden, so a refused
  // or failed read never leaves the previous set on the GPU looking like the current one.
  useEffect(() => {
    const map = mapRef.current;
    if (!ready || map === null) return;
    const source = map.getSource(HEAT_SOURCE_ID) as GeoJSONSource | undefined;
    source?.setData(heatCollection(heatPoints ?? []) as unknown as GeoJSON.FeatureCollection);
    map.setLayoutProperty(LAYER_HEAT, "visibility", heatPoints === null ? "none" : "visible");
  }, [ready, heatPoints]);

  // Data or visible groups changed: new source data (clusters must count only what is shown) and the
  // layer filter — NO network call here; a refetch happens only when a server filter changes.
  useEffect(() => {
    const map = mapRef.current;
    if (!ready || map === null) return;
    const source = map.getSource(SOURCE_ID) as GeoJSONSource | undefined;
    source?.setData(visibleCollection(collection, visible) as unknown as GeoJSON.FeatureCollection);
    const filter = pointsFilter(visible) as unknown as FilterSpecification;
    map.setFilter(LAYER_HALO, filter);
    map.setFilter(LAYER_POINTS, filter);
    map.setFilter(LAYER_LABELS, filter);
  }, [ready, collection, visible]);

  // "Trình chiếu" on/off: repaint the sizes in place (the layers were created with the size in effect).
  useEffect(() => {
    const map = mapRef.current;
    if (!ready || map === null) return;
    const size = pointSizes(presentation);
    map.setPaintProperty(LAYER_HALO, "circle-radius", size.halo);
    map.setPaintProperty(LAYER_POINTS, "circle-radius", size.pin);
    map.setLayoutProperty(LAYER_LABELS, "text-size", size.label);
    // The overlay is a different size from the page column.
    const id = requestAnimationFrame(() => map.resize());
    return () => cancelAnimationFrame(id);
  }, [ready, presentation]);

  useEffect(() => {
    const map = mapRef.current;
    if (!ready || map === null) return;
    const source = map.getSource(CENTRE_SOURCE_ID) as GeoJSONSource | undefined;
    source?.setData(centreRef.current as unknown as GeoJSON.FeatureCollection);
  }, [ready, centreLabel]);

  // Expand / collapse changes the canvas size: tell MapLibre after the layout has settled, so the
  // view is redrawn at the new size instead of stretched. maxBounds keeps the frame at any size.
  useEffect(() => {
    const map = mapRef.current;
    if (!ready || map === null) return;
    const id = requestAnimationFrame(() => map.resize());
    return () => cancelAnimationFrame(id);
  }, [ready, expanded]);

  useEffect(() => {
    const map = mapRef.current;
    if (!ready || map === null || flyTo === null) return;
    // Never move the camera to a point outside the frame (security review 04/10/2026, #2).
    if (!insideFrame([minLng, minLat, maxLng, maxLat], flyTo.lng, flyTo.lat)) return;
    map.flyTo({ center: [flyTo.lng, flyTo.lat], zoom: Math.max(map.getZoom(), 16) });
  }, [ready, flyTo, minLng, minLat, maxLng, maxLat]);

  return (
    <div
      ref={containerRef}
      className={cn(
        expanded || presentation
          ? "economic-map min-h-0 w-full flex-1 overflow-hidden"
          : // Flush to the column edges (prototype `MapWorkspace.tsx`). From lg the workspace has a fixed
            // height and the map takes all of it; stacked below lg, it has its own height.
            "economic-map h-[60vh] min-h-[360px] w-full overflow-hidden lg:h-auto lg:min-h-0 lg:flex-1",
        // No scale bar in "Trình chiếu" (spec 03). Hidden, not removed: the control is created once with the map.
        presentation && "[&_.maplibregl-ctrl-scale]:hidden",
      )}
      data-expanded={expanded ? "" : undefined}
      role="region"
      aria-label="Bản đồ các đối tượng kinh tế của xã"
      data-testid="economic-map"
    />
  );
}
