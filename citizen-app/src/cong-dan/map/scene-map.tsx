/**
 * THE REAL MAP IN "NƠI XẢY RA" — ADR 0072 §Sửa đổi 11/10/2026 (Mini App) M1–M5. Replaces the SVG sketch: the owner
 * asked for a map "như các app bản đồ thông thường".
 *
 *   - LOCKED to the commune's frame (`maxBounds`, M4); opens on Zalo's point when it lies inside, else on the frame.
 *   - A DRAGGABLE pin; tapping the map moves it. Either sets the coordinates and `location_citizen_adjusted = true`
 *     (M3) — the screen holds that state, this file only reports the point (`onPlace`).
 *   - Self-hosted basemap through `service-comms` ONLY (`commune-basemap-style.ts`) — no other host, ever.
 *   - `name:vi` labels, disputed/maritime boundaries filtered, the OSM credit shown uncollapsed (M4).
 *   - ANY failure — no frame, no route, no WebGL, a style/tile error before the first draw, or no draw within
 *     `MAP_LOAD_TIMEOUT_MS` — removes the map and says ONE sentence. Never a grey box, never an error code, and the
 *     send button never waits for the map (M2 last row, ADR 0072 S1).
 *
 * ACCESSIBILITY: the map is a pointer surface; its text alternative is the coordinates line above it and the address
 * box, which say everything the pin says. Gestures are cooperative (two fingers move the map) so a one-finger scroll
 * of the form is never captured — the pin itself still drags with one finger. Zoom buttons are 48px.
 *
 * ⚠ M5: every tile request names the viewport — roughly where the citizen is. It reaches ViGov's `comms` only; it is
 *   declared in `content/ket-xuat-ho-so.ts`. Nothing here logs, stores, or puts coordinates in a URL or a key.
 */
import { useEffect, useRef, useState } from "react";
import type { Map as MapLibreMap, Marker } from "maplibre-gl";

import { communeMapFrame } from "../api/goi-vigov";
import type { SceneLocation } from "../api/hop-dong-phan-anh";
import { XA_PA } from "../man/noi-dung";

import { buildCommuneBasemapStyle, communeBasemapBase } from "./commune-basemap-style";
import {
  frameOutcome,
  initialMapView,
  isInsideFrame,
  mapNotice,
  type MapNotice,
  placedLocation,
  type SceneMapState,
} from "./scene-map-logic";

/** No first draw by then = the basemap is not coming (weak network, route missing): fall back rather than hang. */
export const MAP_LOAD_TIMEOUT_MS = 20_000;

/**
 * The frame read and the map's fate, for one send form. Reads the frame ONCE, the first time `wanted` is true with a
 * commune `domain` (the build's — a lookup key, never a commune reference), and keeps the answer for the form's life.
 * `fail()` is the map's way of saying it could not draw: from then on, no map on this form.
 */
export function useSceneMapState(domain: string | undefined, wanted: boolean): { state: SceneMapState; fail: () => void } {
  const [state, setState] = useState<SceneMapState>({ kind: "off" });
  /** The domain whose frame was asked for — so the read runs once per form, not once per render or per step. */
  const asked = useRef<string | null>(null);
  useEffect(() => {
    if (domain === undefined || domain === "" || !wanted || asked.current === domain) return;
    asked.current = domain;
    let live = true;
    let settled = false;
    setState({ kind: "loading" });
    const settle = (next: SceneMapState) => {
      if (!live) return;
      settled = true;
      setState(next);
    };
    communeMapFrame(domain).then(
      (result) => settle(frameOutcome(result, communeBasemapBase())),
      () => settle({ kind: "unavailable" }),
    );
    return () => {
      live = false;
      // Left before the answer (StrictMode's probe, or the citizen stepped back): the next time asks again.
      if (!settled && asked.current === domain) {
        asked.current = null;
        setState({ kind: "off" });
      }
    };
  }, [domain, wanted]);
  return { state, fail: () => setState({ kind: "unavailable" }) };
}

const NOTICE_TEXT: Readonly<Record<Exclude<MapNotice, null>, string>> = {
  hint: XA_PA.map_hint,
  loading: XA_PA.map_loading,
  unavailable: XA_PA.map_unavailable,
  outside: XA_PA.map_outside,
};

/**
 * The map area of the address block: nothing (`off`), one sentence (`loading`, `unavailable`), or the map and its
 * sentence (`ready`). The sentence is a `role="status"` line — words, never only a colour or an icon.
 */
export function SceneMapArea(props: {
  state: SceneMapState;
  location: SceneLocation | null;
  onPlace: (location: SceneLocation) => void;
  onFail: () => void;
}) {
  const { state } = props;
  if (state.kind === "off") return null;
  if (state.kind !== "ready") {
    return (
      <p className="xa-scene-map__note" role="status">
        {NOTICE_TEXT[state.kind]}
      </p>
    );
  }
  return <SceneMap state={state} location={props.location} onPlace={props.onPlace} onFail={props.onFail} />;
}

/** A 48×48 pin drawn with DOM calls (no markup strings — rule 13 #3); the tip is the element's bottom centre. */
function pinElement(): HTMLElement {
  const el = document.createElement("div");
  el.className = "xa-scene-map__pin";
  const ns = "http://www.w3.org/2000/svg";
  const svg = document.createElementNS(ns, "svg");
  svg.setAttribute("viewBox", "0 0 24 24");
  svg.setAttribute("width", "48");
  svg.setAttribute("height", "48");
  svg.setAttribute("aria-hidden", "true");
  svg.setAttribute("focusable", "false");
  const path = document.createElementNS(ns, "path");
  path.setAttribute("d", "M12,23s7.5,-7,7.5,-12A7.5,7.5,0,0,0,4.5,11C4.5,16,12,23,12,23z");
  path.setAttribute("fill", "#e5484d");
  path.setAttribute("stroke", "#9f1d22");
  path.setAttribute("stroke-width", "1.5");
  const dot = document.createElementNS(ns, "circle");
  dot.setAttribute("cx", "12");
  dot.setAttribute("cy", "11");
  dot.setAttribute("r", "2.8");
  dot.setAttribute("fill", "#ffffff");
  svg.append(path, dot);
  el.append(svg);
  return el;
}

function SceneMap(props: {
  state: Extract<SceneMapState, { kind: "ready" }>;
  location: SceneLocation | null;
  onPlace: (location: SceneLocation) => void;
  onFail: () => void;
}) {
  const { frame, base } = props.state;
  const container = useRef<HTMLDivElement | null>(null);
  const mapRef = useRef<MapLibreMap | null>(null);
  const markerRef = useRef<Marker | null>(null);
  const mlRef = useRef<typeof import("maplibre-gl") | null>(null);
  const [drawn, setDrawn] = useState(false);
  // Latest props for handlers registered once per map — re-creating the map on every keystroke would refetch tiles.
  const latest = useRef(props);
  latest.current = props;

  useEffect(() => {
    let cancelled = false;
    let map: MapLibreMap | null = null;
    let loaded = false;
    const giveUp = () => {
      if (cancelled || loaded) return;
      cancelled = true;
      latest.current.onFail();
    };
    const timer = setTimeout(giveUp, MAP_LOAD_TIMEOUT_MS);
    import("./maplibre-runtime")
      .then(({ loadMapLibre }) => loadMapLibre())
      .then((ml) => {
        if (cancelled || container.current === null) return;
        const view = initialMapView(frame, latest.current.location);
        const camera =
          "zoom" in view
            ? { center: [view.center.lng, view.center.lat] as [number, number], zoom: view.zoom }
            : { bounds: [...view.bounds] as [number, number, number, number], fitBoundsOptions: { padding: 8 } };
        try {
          map = new ml.Map({
            container: container.current,
            style: buildCommuneBasemapStyle(base),
            maxBounds: [...frame.bounds] as [number, number, number, number],
            ...camera,
            maxZoom: 19,
            // Plain 2-D: rotation and tilt only disorient, and a rotated map makes "which side of the road" harder.
            dragRotate: false,
            pitchWithRotate: false,
            touchPitch: false,
            cooperativeGestures: true,
            attributionControl: { compact: false },
            locale: {
              "Map.Title": XA_PA.map_label,
              "Marker.Title": XA_PA.map_pin,
              "NavigationControl.ZoomIn": XA_PA.map_zoom_in,
              "NavigationControl.ZoomOut": XA_PA.map_zoom_out,
              "AttributionControl.ToggleAttribution": XA_PA.map_credit,
              "CooperativeGesturesHandler.MobileHelpText": XA_PA.map_two_fingers,
              "CooperativeGesturesHandler.WindowsHelpText": XA_PA.map_two_fingers,
              "CooperativeGesturesHandler.MacHelpText": XA_PA.map_two_fingers,
            },
          });
        } catch {
          // No WebGL, or a style the builder refused (non-https base): no map, one sentence.
          giveUp();
          return;
        }
        mlRef.current = ml;
        mapRef.current = map;
        map.touchZoomRotate.disableRotation();
        map.addControl(new ml.NavigationControl({ showCompass: false }), "top-right");
        // Before the first draw any error (archive header, style, sprite, glyphs) means the map is not coming.
        // After it, a single missed tile is left alone: the map is usable and the pin still works.
        map.on("error", () => giveUp());
        map.once("load", () => {
          if (cancelled) return;
          loaded = true;
          clearTimeout(timer);
          setDrawn(true);
        });
        map.on("click", (e) => {
          const placed = placedLocation(e.lngLat, frame);
          if (placed !== null) latest.current.onPlace(placed);
        });
      })
      .catch(() => giveUp());
    return () => {
      cancelled = true;
      clearTimeout(timer);
      markerRef.current?.remove();
      markerRef.current = null;
      map?.remove();
      mapRef.current = null;
    };
  }, [frame, base]);

  // The pin follows the location the screen holds — Zalo's, or the citizen's last placement.
  useEffect(() => {
    const map = mapRef.current;
    const ml = mlRef.current;
    if (!drawn || map === null || ml === null) return;
    const location = props.location;
    if (location === null || !isInsideFrame(location, frame)) {
      markerRef.current?.remove();
      markerRef.current = null;
      return;
    }
    let marker = markerRef.current;
    if (marker === null) {
      marker = new ml.Marker({ element: pinElement(), anchor: "bottom", draggable: true });
      marker.on("dragend", () => {
        const placed = placedLocation(marker!.getLngLat(), frame);
        const current = latest.current.location;
        if (placed !== null) latest.current.onPlace(placed);
        else if (current !== null) marker!.setLngLat([current.lng, current.lat]);
      });
      markerRef.current = marker;
      marker.setLngLat([location.lng, location.lat]).addTo(map);
    } else {
      marker.setLngLat([location.lng, location.lat]);
    }
    // A new point off-screen (a fresh Zalo location) is brought into view; a point the citizen just placed is.
    if (!map.getBounds().contains([location.lng, location.lat])) {
      map.easeTo({ center: [location.lng, location.lat] });
    }
  }, [drawn, props.location, frame]);

  const notice = mapNotice(props.state, props.location, drawn);
  return (
    <div className="xa-scene-map">
      <div ref={container} className="xa-scene-map__canvas" role="region" aria-label={XA_PA.map_label} />
      {notice !== null && (
        <p className="xa-scene-map__note" role="status">
          {NOTICE_TEXT[notice]}
        </p>
      )}
    </div>
  );
}
