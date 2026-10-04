"use client";

import type { Map as MapLibreMap, Marker } from "maplibre-gl";
import { useEffect, useRef } from "react";

import { OPENFREEMAP_ATTRIBUTION } from "./labels";
import { MAPLIBRE_WORKER_URL, insideFrame, round6, type MapFrame } from "./map-logic";

/**
 * The small map of the add / edit form (spec §8.1): click to place the pin, drag it to adjust.
 *
 * SAME FRAME, SAME CONSTRUCTOR RULE as `EconomicMap`: `bounds` + `maxBounds` are constructor options,
 * so the picker can never show — or place a pin — outside the commune. A click or a drag ending
 * outside the frame is refused (`onOutside`) and the pin goes back; the form refuses it again on submit.
 *
 * ONE marker, the form's own pin — not a marker per asset.
 */
export function PositionPicker({
  styleUrl,
  frame,
  lat,
  lng,
  onPick,
  onOutside,
  expanded = false,
}: {
  styleUrl: string;
  frame: MapFrame;
  /** "Mở rộng bản đồ" inside the form's dialog: a taller map to place the pin precisely. */
  expanded?: boolean;
  /** Current position from the inputs; `null` when empty or invalid. */
  lat: number | null;
  lng: number | null;
  onPick: (lat: number, lng: number) => void;
  onOutside: () => void;
}) {
  const containerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<MapLibreMap | null>(null);
  const markerRef = useRef<Marker | null>(null);
  const onPickRef = useRef(onPick);
  const onOutsideRef = useRef(onOutside);
  const posRef = useRef<{ lat: number | null; lng: number | null }>({ lat, lng });
  useEffect(() => {
    onPickRef.current = onPick;
    onOutsideRef.current = onOutside;
    posRef.current = { lat, lng };
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
        dragRotate: false,
        pitchWithRotate: false,
        touchPitch: false,
      });
      created = map;
      mapRef.current = map;
      map.addControl(new ml.AttributionControl({ compact: true, customAttribution: OPENFREEMAP_ATTRIBUTION }));
      map.addControl(new ml.NavigationControl({ showCompass: false }), "top-right");

      const marker = new ml.Marker({ draggable: true });
      markerRef.current = marker;
      const place = (pLng: number, pLat: number) => {
        marker.setLngLat([pLng, pLat]);
        if (marker.getElement().parentElement === null) marker.addTo(map);
      };
      const start = posRef.current;
      if (start.lat !== null && start.lng !== null && insideFrame(bounds, start.lng, start.lat)) place(start.lng, start.lat);

      map.on("click", (e) => {
        const { lng: cLng, lat: cLat } = e.lngLat;
        if (!insideFrame(bounds, cLng, cLat)) return onOutsideRef.current();
        place(cLng, cLat);
        onPickRef.current(round6(cLat), round6(cLng));
      });
      marker.on("dragend", () => {
        const p = marker.getLngLat();
        if (!insideFrame(bounds, p.lng, p.lat)) {
          const back = posRef.current;
          if (back.lat !== null && back.lng !== null) marker.setLngLat([back.lng, back.lat]);
          return onOutsideRef.current();
        }
        onPickRef.current(round6(p.lat), round6(p.lng));
      });
    });
    return () => {
      cancelled = true;
      markerRef.current?.remove();
      markerRef.current = null;
      created?.remove();
      mapRef.current = null;
    };
  }, [styleUrl, minLng, minLat, maxLng, maxLat]);

  // Typed coordinates move the pin — only inside the frame; the form shows why otherwise.
  useEffect(() => {
    const map = mapRef.current;
    const marker = markerRef.current;
    if (map === null || marker === null || lat === null || lng === null) return;
    if (!insideFrame([minLng, minLat, maxLng, maxLat], lng, lat)) return;
    marker.setLngLat([lng, lat]);
    if (marker.getElement().parentElement === null) marker.addTo(map);
  }, [lat, lng, minLng, minLat, maxLng, maxLat]);

  // A new height needs a resize, or MapLibre keeps drawing at the old canvas size.
  useEffect(() => {
    const map = mapRef.current;
    if (map === null) return;
    const id = requestAnimationFrame(() => map.resize());
    return () => cancelAnimationFrame(id);
  }, [expanded]);

  return (
    <div
      ref={containerRef}
      id="position-picker-map"
      className={`${expanded ? "h-[70vh]" : "h-64"} w-full overflow-hidden rounded-xl border border-line`}
      data-expanded={expanded ? "" : undefined}
      role="region"
      aria-label="Bản đồ chọn vị trí"
      data-testid="position-picker"
    />
  );
}
