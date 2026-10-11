/**
 * THE SEND FORM'S MAP — every decision that is not drawing, as pure functions (ADR 0072 §Sửa đổi 11/10/2026 (Mini App)).
 * `scene-map.tsx` only wires these to MapLibre; the tests read them here, without a map, a DOM or a network.
 *
 * ⚠ NOTHING HERE LOGS, STORES OR PUTS COORDINATES IN A URL OR A KEY (rule 3), as `man/scene-location.tsx`.
 */
import type { NewsReadResult } from "../api/goi-vigov";
import type { CommuneMapFrame, CommuneMapFrameRead } from "../api/hop-dong-cong-khai";
import { isSceneLocation, type SceneLocation } from "../api/hop-dong-phan-anh";

/**
 * Where the map stands:
 *
 *   `off`          no map on this form at all — the accountless path (it cannot send coordinates), outside Zalo
 *                  (no location bridge), or no commune domain: today's form, no sentence
 *   `loading`      the frame is being read
 *   `unavailable`  frame not set, frame read failed, basemap route missing, or the map itself failed —
 *                  no map, ONE sentence, and the petition still goes with whatever Zalo gave (M2 last row)
 *   `ready`        draw it, locked to `frame`, tiles from `base`
 */
export type SceneMapState =
  | { readonly kind: "off" }
  | { readonly kind: "loading" }
  | { readonly kind: "unavailable" }
  | { readonly kind: "ready"; readonly frame: CommuneMapFrame; readonly base: string };

/**
 * One frame read → the map's state. PURE. Every branch but "a frame AND a basemap address" is `unavailable`: a 429, a
 * 503, a malformed body, `configured: false` and a host table with no `comms` line all mean the same thing to the
 * citizen — no map, send anyway. Never a guessed frame (ADR 0072 K3).
 */
export function frameOutcome(result: NewsReadResult<{ readonly frame: CommuneMapFrameRead }>, base: string): SceneMapState {
  if (result.kieu === "xong" && result.gia_tri.frame !== null && base !== "") {
    return { kind: "ready", frame: result.gia_tri.frame, base };
  }
  return { kind: "unavailable" };
}

export function isInsideFrame(location: SceneLocation, frame: CommuneMapFrame): boolean {
  const [west, south, east, north] = frame.bounds;
  return location.lng >= west && location.lng <= east && location.lat >= south && location.lat <= north;
}

/** Street level: close enough to tell one side of a road from the other, as the citizen needs to. */
export const PIN_ZOOM = 16;

/**
 * Where the map opens: on the location when it lies INSIDE the commune's frame, else on the frame itself (centre,
 * whole frame in view). A point outside the frame cannot be shown — the camera is locked to the frame — so it is
 * never centred on, and the screen says so in words (`mapNotice`).
 */
export function initialMapView(
  frame: CommuneMapFrame,
  location: SceneLocation | null,
): { readonly center: SceneLocation; readonly zoom: number } | { readonly center: SceneLocation; readonly bounds: CommuneMapFrame["bounds"] } {
  if (location !== null && isSceneLocation(location) && isInsideFrame(location, frame)) {
    return { center: { lat: location.lat, lng: location.lng }, zoom: PIN_ZOOM };
  }
  return { center: frame.center, bounds: frame.bounds };
}

/**
 * A point the citizen put on the map (drag end or tap) → the location to keep, or `null` to ignore it. Only a valid
 * pair INSIDE the frame is kept: the camera cannot leave the frame, so anything else is an artefact, not a choice.
 */
export function placedLocation(point: { readonly lng: number; readonly lat: number }, frame: CommuneMapFrame): SceneLocation | null {
  const candidate = { lat: point.lat, lng: point.lng };
  return isSceneLocation(candidate) && isInsideFrame(candidate, frame) ? candidate : null;
}

/**
 * The `location_citizen_adjusted` value for the send (M3). PURE.
 *
 *   no coordinates          → `null`  (the key is omitted — `true` without a pair is a 422)
 *   Zalo's point, untouched → `false`
 *   pin moved or map tapped → `true`
 */
export function locationAdjustedFlag(location: SceneLocation | null, movedByCitizen: boolean): boolean | null {
  return location === null ? null : movedByCitizen;
}

/** Which one sentence sits under the map, if any. PURE. */
export type MapNotice = "hint" | "loading" | "unavailable" | "outside" | null;

export function mapNotice(state: SceneMapState, location: SceneLocation | null, drawn: boolean): MapNotice {
  switch (state.kind) {
    case "off":
      return null;
    case "loading":
      return "loading";
    case "unavailable":
      return "unavailable";
    case "ready":
      if (!drawn) return "loading";
      return location !== null && !isInsideFrame(location, state.frame) ? "outside" : "hint";
  }
}
