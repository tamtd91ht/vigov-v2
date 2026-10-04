"use client";

import type {
  ExpressionSpecification,
  FilterSpecification,
  GeoJSONSource,
  Map as MapLibreMap,
  MapGeoJSONFeature,
  MapLayerMouseEvent,
} from "maplibre-gl";
import { useEffect, useRef, useState } from "react";

import { OPENFREEMAP_ATTRIBUTION } from "./labels";
import {
  CLUSTER_MAX_ZOOM,
  CLUSTER_RADIUS,
  LABEL_MIN_ZOOM,
  LAYER_CLUSTER_COUNT,
  LAYER_CLUSTERS,
  LAYER_LABELS,
  LAYER_POINTS,
  LAYER_SELECTED,
  MAPLIBRE_WORKER_URL,
  SOURCE_ID,
  VIETNAMESE_TEXT_FIELD,
  colourExpression,
  insideFrame,
  nameLabelLayers,
  pointsFilter,
  selectedFilter,
  visibleCollection,
  type EconomicCollection,
  type MapFrame,
} from "./map-logic";

/**
 * The MapLibre map of `/ban-do` — the ONLY component that touches the map instance.
 *
 * LIFECYCLE (ADR 0072; the user's engineering guide §17–18):
 *   - MapLibre is loaded with a DYNAMIC import inside the effect: it needs `window` and WebGL, and the
 *     page is server-rendered. It is never imported at module level.
 *   - The map is created ONCE per (style, frame) and kept in a `useRef` — never React state: a map in
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
  styleUrl: string;
  frame: MapFrame;
  collection: EconomicCollection;
  visible: ReadonlySet<string>;
  selectedId: string | null;
  /** Fly to a point — honoured ONLY inside the frame (the parent checks too). `seq` makes a repeat fly again. */
  flyTo: FlyRequest | null;
  onPointClick: (id: string) => void;
  /** The style could not be loaded (before the first `load`). */
  onLoadError: () => void;
};

const POINT_COLOUR = colourExpression() as unknown as ExpressionSpecification;

export function EconomicMap({
  styleUrl,
  frame,
  collection,
  visible,
  selectedId,
  flyTo,
  onPointClick,
  onLoadError,
}: EconomicMapProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<MapLibreMap | null>(null);
  const [ready, setReady] = useState(false);

  // Latest callbacks and data for handlers registered once at creation.
  const onPointClickRef = useRef(onPointClick);
  const onLoadErrorRef = useRef(onLoadError);
  const dataRef = useRef({ collection, visible, selectedId });
  useEffect(() => {
    onPointClickRef.current = onPointClick;
    onLoadErrorRef.current = onLoadError;
    dataRef.current = { collection, visible, selectedId };
  });

  const [minLng, minLat, maxLng, maxLat] = frame.bounds;

  useEffect(() => {
    let cancelled = false;
    let created: MapLibreMap | null = null;
    const bounds: [number, number, number, number] = [minLng, minLat, maxLng, maxLat];

    void import("maplibre-gl").then((ml) => {
      if (cancelled || containerRef.current === null) return;
      // Before ANY map exists — see MAPLIBRE_WORKER_URL for why the default URL is empty under Turbopack.
      ml.setWorkerUrl(MAPLIBRE_WORKER_URL);
      const map = new ml.Map({
        container: containerRef.current,
        style: styleUrl,
        bounds,
        maxBounds: bounds,
        renderWorldCopies: false,
        attributionControl: false,
        // No rotation: a rotated view of a bounded box exposes corners outside it, and staff read a
        // north-up map.
        dragRotate: false,
        pitchWithRotate: false,
        touchPitch: false,
      });
      created = map;
      mapRef.current = map;
      map.touchZoomRotate.disableRotation();
      // Default responsive behaviour: expanded on a wide map, compact (the "i" button) under 640px.
      map.addControl(new ml.AttributionControl({ customAttribution: OPENFREEMAP_ATTRIBUTION }));
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

        // Vietnamese names on the basemap (ADR 0072 H1) — BEFORE our layers exist, so ours keep theirs.
        for (const id of nameLabelLayers(map.getStyle().layers ?? [])) {
          map.setLayoutProperty(id, "text-field", VIETNAMESE_TEXT_FIELD as unknown as ExpressionSpecification);
        }

        const d = dataRef.current;
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
            // A font the `liberty` style's glyph server carries.
            "text-font": ["Noto Sans Bold"],
            "text-size": 13,
            "text-allow-overlap": true,
          },
          paint: { "text-color": "#ffffff" },
        });
        map.addLayer({
          id: LAYER_POINTS,
          type: "circle",
          source: SOURCE_ID,
          filter: pointsFilter(d.visible) as unknown as FilterSpecification,
          paint: {
            "circle-color": POINT_COLOUR,
            "circle-radius": 8,
            "circle-stroke-width": 2,
            "circle-stroke-color": "#ffffff",
            // Unverified drawn LIGHTER (spec §10 `da_xac_minh`); the panel says it in words.
            "circle-opacity": ["case", ["==", ["get", "verified"], true], 1, 0.45],
          },
        });
        map.addLayer({
          id: LAYER_SELECTED,
          type: "circle",
          source: SOURCE_ID,
          filter: selectedFilter(d.selectedId) as unknown as FilterSpecification,
          paint: {
            "circle-radius": 13,
            "circle-color": "rgba(0,0,0,0)",
            "circle-stroke-width": 3,
            "circle-stroke-color": "#f59e0b",
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
            "text-size": 12,
            "text-offset": [0, 1.1],
            "text-anchor": "top",
            "text-max-width": 10,
          },
          paint: { "text-color": "#1f2937", "text-halo-color": "#ffffff", "text-halo-width": 1.5 },
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
    });

    return () => {
      cancelled = true;
      created?.remove();
      mapRef.current = null;
      setReady(false);
    };
  }, [styleUrl, minLng, minLat, maxLng, maxLat]);

  // Data or visible groups changed: new source data (clusters must count only what is shown) and the
  // layer filter — NO network call here; a refetch happens only when a server filter changes.
  useEffect(() => {
    const map = mapRef.current;
    if (!ready || map === null) return;
    const source = map.getSource(SOURCE_ID) as GeoJSONSource | undefined;
    source?.setData(visibleCollection(collection, visible) as unknown as GeoJSON.FeatureCollection);
    const filter = pointsFilter(visible) as unknown as FilterSpecification;
    map.setFilter(LAYER_POINTS, filter);
    map.setFilter(LAYER_LABELS, filter);
  }, [ready, collection, visible]);

  useEffect(() => {
    const map = mapRef.current;
    if (!ready || map === null) return;
    map.setFilter(LAYER_SELECTED, selectedFilter(selectedId) as unknown as FilterSpecification);
  }, [ready, selectedId]);

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
      className="economic-map h-[60vh] min-h-[360px] w-full overflow-hidden rounded-xl border border-line md:h-[calc(100vh-260px)] md:min-h-[480px]"
      role="region"
      aria-label="Bản đồ các đối tượng kinh tế của xã"
      data-testid="economic-map"
    />
  );
}
