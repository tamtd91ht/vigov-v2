/**
 * THE LOCATION FUNCTION `App.tsx` INJECTS INTO THE STATE HALF — built from two commercial-half pieces:
 * `requestLocationCodes` (`features/tinh-nang/`, the ONLY place allowed to call `zmp-sdk`) and
 * `exchangeLocation` (`goi-may-chu.ts`, the network file of this directory). Same shape and same reason
 * as `cau-vigov.ts`: the state half may import neither (`ranh-gioi-hai-nua.test.ts` §3a), so this file
 * only joins them, and `App.tsx` translates the result into the state half's type.
 *
 * NO SESSION OF ANY KIND IS READ OR SENT. The route is public (`vihat-miniapp` README): the commercial
 * ticket in `kho-phien.tsx` is not touched, and the ViGov session never leaves `cong-dan/`.
 *
 * ⚠ THE TWO CODES STOP HERE. They go into one exchange and nowhere else — no log, no state, not returned
 *   upward. What goes up is only the coordinates, or the branch that says why there are none.
 */
// vi-name-ok: importing the EXISTING result type `KetQuaXin` from zalo-api.ts — no new name
import { type KetQuaXin, type LocationCodes, requestLocationCodes } from "../tinh-nang/zalo-api";

import { exchangeLocation, type LocationExchangeResult } from "./goi-may-chu";

/** The exchange's branches plus the three of the Zalo step. */
export type CurrentLocationResult =
  | LocationExchangeResult
  | { kieu: "tu-choi" }
  | { kieu: "ngoai-zalo" }
  | { kieu: "khong-lay-duoc-ma" };

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
  if (codes.kieu === "ngoai-zalo") return { kieu: "ngoai-zalo" };
  if (codes.kieu === "tu-choi") return { kieu: "tu-choi" };
  if (codes.kieu !== "xong" || codes.du_lieu.access_token === "" || codes.du_lieu.location_token === "") {
    return { kieu: "khong-lay-duoc-ma" };
  }
  return exchange(codes.du_lieu);
}
