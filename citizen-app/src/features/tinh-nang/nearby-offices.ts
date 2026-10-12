/**
 * "TÌM VĂN PHÒNG GẦN TÔI" — the ViHAT intro's one use of the location permission (owner 12/10/2026).
 *
 * The location is exchanged ON THE SERVER, by the SAME client the commune page uses (`features/dang-nhap/
 * current-location.ts` `getCurrentLocation` → `vihat-miniapp` `POST /api/v1/location`, shared-app body without
 * `appId`): `getLocation` only hands out a token, and only the server holds the app secret that turns it into
 * coordinates. Nothing here duplicates that contract.
 *
 * ⚠ THE PERSON'S COORDINATES LIVE IN ONE FUNCTION SCOPE: `locateNearbyOffices` receives them, computes a distance
 *   to each office ON THE PHONE, and returns only `{ index, km }` rows. They are never put in React state, never
 *   stored, never logged, never sent anywhere else — what the screen keeps is "office n is about X km away", which
 *   is what the person asked for and nothing more. A test pins that the result carries no coordinate.
 *
 * ⚠ ZALO IS ASKED ONLY AFTER THE APP'S OWN "CHO PHÉP" (Mini App policy 3.3.4), shown in the page (not an overlay,
 *   same reason as `features/yeu-cau/ConsultPhotos.tsx`). "Không" calls nothing.
 */
import { type Office, OFFICES } from "../../content/company-profile";
import { type CurrentLocationResult, getCurrentLocation } from "../dang-nhap/current-location";

import { NEARBY_OFFICES } from "./noi-dung";

/** Mean Earth radius (IUGG), km. The error of a sphere vs the ellipsoid is < 0.5 % — far below "cách khoảng". */
const EARTH_RADIUS_KM = 6371.0088;

type Point = { lat: number; lng: number };

/** Great-circle (haversine) distance between two WGS84 points, in km. PURE. */
export function distanceKm(a: Point, b: Point): number {
  const rad = Math.PI / 180;
  const dLat = (b.lat - a.lat) * rad;
  const dLng = (b.lng - a.lng) * rad;
  const h = Math.sin(dLat / 2) ** 2 + Math.cos(a.lat * rad) * Math.cos(b.lat * rad) * Math.sin(dLng / 2) ** 2;
  return 2 * EARTH_RADIUS_KM * Math.asin(Math.min(1, Math.sqrt(h)));
}

/**
 * Vietnamese number for "cách khoảng X km": under 10 km one decimal with a comma ("3,4"), otherwise a whole number
 * with "." between thousands ("1.140"). A straight-line distance is approximate, so more digits would claim a
 * precision the figure does not have.
 */
export function formatDistanceKm(km: number): string {
  // Decide on the ROUNDED value: 9.96 km must read "10", not "10,0" — the rule is about what is printed.
  const tenths = Math.round(km * 10) / 10;
  if (tenths < 10) return tenths.toFixed(1).replace(".", ",");
  return String(Math.round(km)).replace(/\B(?=(\d{3})+(?!\d))/g, ".");
}

/** One office and how far it is — the ONLY trace of the person's location the screen keeps. */
export type NearbyRow = { index: number; km: number };

/** Offices nearest first; ties keep the published order. PURE. */
export function sortByDistance(here: Point, offices: readonly Office[] = OFFICES): NearbyRow[] {
  return offices
    .map((office, index) => ({ index, km: distanceKm(here, office) }))
    .sort((x, y) => x.km - y.km || x.index - y.index);
}

/**
 * What the block shows:
 *   idle      the published order, the "Tìm văn phòng gần tôi" button
 *   asking    the purpose sentence with "Cho phép" / "Không" — Zalo not asked yet
 *   locating  waiting for Zalo and the exchange
 *   sorted    nearest first, each with its distance
 *   default   the published order and ONE sentence saying what to do next
 */
export type NearbyState =
  | { step: "idle" }
  | { step: "asking" }
  | { step: "locating" }
  | { step: "sorted"; rows: readonly NearbyRow[] }
  | { step: "default"; note: string };

/**
 * The sentence for a location that did not come. Table by WHAT THE PERSON DOES NEXT (as `App.tsx`
 * `toSceneLocationResult`), never a code on screen.
 */
export function failureNote(kind: Exclude<CurrentLocationResult["kieu"], "xong">): string {
  switch (kind) {
    case "tu-choi":
      return NEARBY_OFFICES.zalo_refused;
    case "ngoai-zalo":
      return NEARBY_OFFICES.outside_zalo;
    case "qua-nhieu-lan":
      return NEARBY_OFFICES.too_many;
    // Pressing again changes nothing: the route is not wired (503), the App ID unknown (422), or the build has no
    // server address.
    case "tam-ngung":
    case "chua-khai-host":
      return NEARBY_OFFICES.paused;
    // A fresh tap gets fresh codes: Zalo's ~2-minute single-use token expired (502), a body the server could not
    // read, the network, or a Zalo step that failed.
    case "zalo-khong-tra-loi":
    case "yeu-cau-hong":
    case "khong-goi-duoc":
    case "khong-lay-duoc-ma":
      return NEARBY_OFFICES.retry;
  }
}

/**
 * Ask Zalo + exchange, then sort. NEVER THROWS: a rejected promise here would leave the block on "Đang tìm…".
 * `getLocation` / `offices` exist only so a test can replace them.
 */
export async function locateNearbyOffices(
  getLocation: () => Promise<CurrentLocationResult> = getCurrentLocation,
  offices: readonly Office[] = OFFICES,
): Promise<Extract<NearbyState, { step: "sorted" | "default" }>> {
  let result: CurrentLocationResult;
  try {
    result = await getLocation();
  } catch {
    return { step: "default", note: NEARBY_OFFICES.retry };
  }
  if (result.kieu !== "xong") return { step: "default", note: failureNote(result.kieu) };
  // The coordinates go no further than this line.
  return {
    step: "sorted",
    rows: sortByDistance({ lat: result.location.latitude, lng: result.location.longitude }, offices),
  };
}

/**
 * The block's flow, without React — so "Zalo only after Cho phép" is testable without a DOM. `set` is the screen's
 * state setter. Once allowed, a later tap in the same screen opening goes straight to Zalo (Zalo still shows its own
 * dialog) — the same rule as the consultation photos.
 */
export function createNearbyFlow(
  set: (state: NearbyState) => void,
  locate: () => Promise<Extract<NearbyState, { step: "sorted" | "default" }>> = locateNearbyOffices,
) {
  let allowed = false;
  let busy = false;
  async function run() {
    if (busy) return;
    busy = true;
    set({ step: "locating" });
    set(await locate());
    busy = false;
  }
  return {
    find() {
      if (busy) return;
      if (allowed) void run();
      else set({ step: "asking" });
    },
    allow() {
      allowed = true;
      return run();
    },
    deny() {
      set({ step: "default", note: NEARBY_OFFICES.denied });
    },
  };
}
