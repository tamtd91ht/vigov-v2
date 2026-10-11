"use client";

import { MapPin } from "lucide-react";
import type { Map as MapLibreMap, Marker } from "maplibre-gl";
import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";

import { registerPmtilesProtocol } from "@/lib/basemap/protocol";
import { buildBasemapStyle } from "@/lib/basemap/style";

import { PIN_HINT } from "./labels";
import { MAPLIBRE_WORKER_URL, insideFrame, round6, type MapFrame } from "./map-logic";

/** Prototype `LocationPicker.tsx`: typed coordinates fly to zoom 16 in 400ms. */
const PICK_ZOOM = 16;
const TYPED_FLY_MS = 400;

/**
 * The small map of the add / edit form (prototype `LocationPicker.tsx`, spec 07): click to place the
 * pin, drag it to adjust; a typed position flies there. The pin is the prototype's red lucide `MapPin`
 * (anchored at its tip), rendered by React into the marker's own element through a portal — no HTML
 * string anywhere (rule 13).
 *
 * SAME SELF-HOSTED BASEMAP as `EconomicMap` (style built in code, same origin, `/basemap/*`). The dialog
 * renders this only while the page's map is available, so an unconfigured basemap shows no picker; the
 * coordinates can still be typed.
 *
 * SAME FRAME, SAME CONSTRUCTOR RULE as `EconomicMap`: `bounds` + `maxBounds` are constructor options,
 * so the picker can never show — or place a pin — outside the commune. A click or a drag ending
 * outside the frame is refused (`onOutside`) and the pin goes back; the form refuses it again on submit.
 *
 * ONE marker, the form's own pin — not a marker per asset.
 */
export function PositionPicker({
  frame,
  lat,
  lng,
  onPick,
  onOutside,
}: {
  frame: MapFrame;
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
  /** The last position the MAP produced (click / drag): echoed back through the inputs, it must not fly. */
  const pickedRef = useRef<{ lat: number; lng: number } | null>(null);
  // The marker's element, made once; the pin icon is portalled into it.
  const [pinEl] = useState<HTMLDivElement | null>(() => (typeof document === "undefined" ? null : document.createElement("div")));
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
        style: buildBasemapStyle(window.location.origin),
        bounds,
        maxBounds: bounds,
        renderWorldCopies: false,
        // The style source's OpenStreetMap credit, compact on this small map.
        attributionControl: { compact: true },
        dragRotate: false,
        pitchWithRotate: false,
        touchPitch: false,
      });
      created = map;
      mapRef.current = map;
      map.getCanvas().style.cursor = "crosshair";
      map.addControl(new ml.NavigationControl({ showCompass: false }), "top-right");

      const marker = new ml.Marker({ element: pinEl ?? undefined, anchor: "bottom", draggable: true });
      markerRef.current = marker;
      const place = (pLng: number, pLat: number) => {
        marker.setLngLat([pLng, pLat]);
        if (marker.getElement().parentElement === null) marker.addTo(map);
      };
      const start = posRef.current;
      if (start.lat !== null && start.lng !== null && insideFrame(bounds, start.lng, start.lat)) {
        pickedRef.current = { lat: start.lat, lng: start.lng };
        place(start.lng, start.lat);
      }

      const pick = (pLat: number, pLng: number) => {
        const p = { lat: round6(pLat), lng: round6(pLng) };
        pickedRef.current = p;
        onPickRef.current(p.lat, p.lng);
      };
      map.on("click", (e) => {
        const { lng: cLng, lat: cLat } = e.lngLat;
        if (!insideFrame(bounds, cLng, cLat)) return onOutsideRef.current();
        place(cLng, cLat);
        pick(cLat, cLng);
      });
      marker.on("dragend", () => {
        const p = marker.getLngLat();
        if (!insideFrame(bounds, p.lng, p.lat)) {
          const back = posRef.current;
          if (back.lat !== null && back.lng !== null) marker.setLngLat([back.lng, back.lat]);
          return onOutsideRef.current();
        }
        pick(p.lat, p.lng);
      });
    })().catch(() => {
      // The MapLibre or PMTiles chunk did not load: no picker map; the lat / lng inputs still work.
    });
    return () => {
      cancelled = true;
      markerRef.current?.remove();
      markerRef.current = null;
      created?.remove();
      mapRef.current = null;
    };
  }, [minLng, minLat, maxLng, maxLat, pinEl]);

  // Typed coordinates move the pin and fly there — only inside the frame; the form shows why otherwise.
  useEffect(() => {
    const map = mapRef.current;
    const marker = markerRef.current;
    if (map === null || marker === null || lat === null || lng === null) return;
    if (!insideFrame([minLng, minLat, maxLng, maxLat], lng, lat)) return;
    marker.setLngLat([lng, lat]);
    if (marker.getElement().parentElement === null) marker.addTo(map);
    const picked = pickedRef.current;
    if (picked !== null && round6(picked.lat) === round6(lat) && round6(picked.lng) === round6(lng)) return;
    pickedRef.current = { lat, lng };
    map.flyTo({ center: [lng, lat], zoom: PICK_ZOOM, duration: TYPED_FLY_MS });
  }, [lat, lng, minLng, minLat, maxLng, maxLat]);

  return (
    <div data-picker-frame="" className="relative h-64 overflow-hidden rounded-[10px] border border-line">
      <div
        ref={containerRef}
        className="size-full cursor-crosshair"
        role="region"
        aria-label="Bản đồ chọn vị trí"
        data-testid="position-picker"
      />
      {(lat === null || lng === null) && (
        <p
          data-picker-hint=""
          className="pointer-events-none absolute inset-x-0 bottom-0 m-0 bg-white/90 px-3 py-1.5 text-center text-[11.5px] text-ink-muted"
        >
          {PIN_HINT}
        </p>
      )}
      {pinEl !== null &&
        createPortal(
          <MapPin aria-hidden="true" focusable="false" className="size-7 text-danger drop-shadow" fill="#E5484D" />,
          pinEl,
        )}
    </div>
  );
}
