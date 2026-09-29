/**
 * THE LOCATION FUNCTION `App.tsx` INJECTS INTO THE STATE HALF — built from two commercial-half pieces:
 * `requestLocationCodes` (`features/tinh-nang/`, the ONLY place allowed to call `zmp-sdk`) and
 * `exchangeLocation` (`server-calls.ts`, the network file of this directory). Same shape and same reason
 * as `vigov-bridge.ts`: the state half may import neither (`two-halves-boundary.test.ts` §3a), so this file
 * only joins them, and `App.tsx` translates the result into the state half's type.
 *
 * NO SESSION OF ANY KIND IS READ OR SENT. The route is public (`vihat-miniapp` README): the commercial
 * ticket in `session-store.tsx` is not touched, and the ViGov session never leaves `citizen/`.
 *
 * ⚠ THE TWO CODES STOP HERE. They go into one exchange and nowhere else — no log, no state, not returned
 *   upward. What goes up is only the coordinates, or the branch that says why there are none.
 */
// vi-name-ok: importing the EXISTING result type `KetQuaXin` from zalo-api.ts — no new name
import { type KetQuaXin, type LocationCodes, readRuntimeAppId, requestLocationCodes } from "../tinh-nang/zalo-api";

import { exchangeLocation, type LocationExchangeResult } from "./server-calls";

/** The exchange's branches plus the three of the Zalo step. */
export type CurrentLocationResult =
  | LocationExchangeResult
  | { kind: "tu-choi" }
  | { kind: "ngoai-zalo" }
  | { kind: "khong-lay-duoc-ma" };

/**
 * Codes, then the exchange — right away, because the location token is single-use and lives about two
 * minutes. `getCodes` / `exchange` exist only so a test can replace the two pieces.
 *
 * REFUSED = NO EXCHANGE. An empty code (the platform's answer in a dev environment) does not call the
 * server either: sending an empty token is a 400 the citizen can do nothing about.
 */
export async function getCurrentLocation(
  getCodes: () => Promise<KetQuaXin<LocationCodes>> = requestLocationCodes,
  exchange: (codes: LocationCodes) => Promise<LocationExchangeResult> = exchangeLocation,
): Promise<CurrentLocationResult> {
  const codes = await getCodes();
  if (codes.kieu === "ngoai-zalo") return { kind: "ngoai-zalo" };
  if (codes.kieu === "tu-choi") return { kind: "tu-choi" };
  if (codes.kieu !== "xong" || codes.du_lieu.access_token === "" || codes.du_lieu.location_token === "") {
    return { kind: "khong-lay-duoc-ma" };
  }
  return exchange(codes.du_lieu);
}

/**
 * THE COMMUNE APP'S LOCATION — the same two steps, with the App ID of the running app in the exchange body,
 * so `vihat-miniapp` uses THIS app's secret (without it Zalo answers 502 for a commune app's token).
 *
 * App ID unknown → `tam-ngung` BEFORE Zalo is asked: sending without it is exactly the 502 above, after the
 * citizen had already agreed to share where they stand. `readAppId` / `getCodes` / `exchange` exist only so
 * a test can replace the three pieces.
 */
export async function getCommuneAppLocation(
  readAppId: () => string | null = readRuntimeAppId,
  getCodes: () => Promise<KetQuaXin<LocationCodes>> = requestLocationCodes,
  exchange: (codes: LocationCodes, appId: string) => Promise<LocationExchangeResult> = (codes, appId) =>
    exchangeLocation(codes, undefined, appId),
): Promise<CurrentLocationResult> {
  const appId = readAppId();
  if (appId === null || appId === "") return { kind: "tam-ngung" };
  return getCurrentLocation(getCodes, (codes) => exchange(codes, appId));
}
