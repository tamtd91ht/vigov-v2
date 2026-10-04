/**
 * The economic map's asset register — service-comms (ADR 0072; `docs/ui-ux/10-ban-do-kinh-te-so.md`).
 *
 *   GET    /api/v1/map-asset-points             asset.read    GeoJSON, [lng, lat], light properties
 *   GET    /api/v1/map-assets                   asset.read    `Sổ địa điểm`, one page, ALWAYS masked
 *   GET    /api/v1/map-assets/{id}              asset.read    one asset; unmasked only for asset.update
 *   GET    /api/v1/map-asset-summary            asset.read    counts per group + verified ratio
 *   POST   /api/v1/map-assets                   asset.update  Idempotency-Key required
 *   PATCH  /api/v1/map-assets/{id}              asset.update  only the fields sent
 *   DELETE /api/v1/map-assets/{id}              asset.update  `{ reason }`, 204
 *   POST   /api/v1/map-assets/{id}/confirmation asset.update  `{ verified }`
 *   POST   /api/v1/map-asset-types/defaults     admin.lookup  sow the eleven groups
 *
 * PERSONAL DATA (rule 3): `representative`, `phone`, `tax_code` arrive masked unless the server decided
 * otherwise; this file never unmasks, logs or caches anything. Masked values are DISPLAYED as returned
 * and NEVER sent back up — a PATCH carries only fields the officer changed (`features/ban-do/asset-form.ts`).
 *
 * `verified` IS NEVER SENT ON CREATE OR PATCH: the server refuses it there (400) — confirmation is its
 * own route with its own trail.
 */

import { docJSON, docThanLoiGoi, goiGhi, thamSoTheoHopDong } from "./goi";
import type { KetQua } from "./goi";
import type {
  comms_createMapAssetIn,
  comms_delete_map_assets_by_id,
  comms_deleteMapAssetIn,
  comms_get_map_asset_points,
  comms_get_map_asset_summary,
  comms_get_map_assets,
  comms_get_map_assets_by_id,
  comms_mapAssetConfirmationIn,
  comms_mapAssetOut,
  comms_mapAssetPointsOut,
  comms_mapAssetSummaryOut,
  comms_patch_map_assets_by_id,
  comms_post_map_asset_types_defaults,
  comms_post_map_assets,
  comms_post_map_assets_by_id_confirmation,
  comms_seedMapAssetTypesOut,
  comms_updateMapAssetIn,
  page_Result_comms_mapAssetRowOut,
} from "./schema.gen";

const POINTS_PATH = "/api/v1/map-asset-points" satisfies comms_get_map_asset_points["duongDan"];
const LIST_PATH = "/api/v1/map-assets" satisfies comms_get_map_assets["duongDan"] & comms_post_map_assets["duongDan"];
const ITEM_PATH = "/api/v1/map-assets/{id}" satisfies comms_get_map_assets_by_id["duongDan"] &
  comms_patch_map_assets_by_id["duongDan"] &
  comms_delete_map_assets_by_id["duongDan"];
const CONFIRM_PATH = "/api/v1/map-assets/{id}/confirmation" satisfies comms_post_map_assets_by_id_confirmation["duongDan"];
const SUMMARY_PATH = "/api/v1/map-asset-summary" satisfies comms_get_map_asset_summary["duongDan"];
const DEFAULTS_PATH = "/api/v1/map-asset-types/defaults" satisfies comms_post_map_asset_types_defaults["duongDan"];

/** The filters shared by the points and the register. Empty string = not filtered. */
export type MapAssetFilter = {
  readonly q: string;
  readonly industryCode: string;
  readonly residentialUnitId: string;
  readonly status: string;
};

export const NO_FILTER: MapAssetFilter = { q: "", industryCode: "", residentialUnitId: "", status: "" };

function itemPath(template: string, id: string): string {
  return template.replace("{id}", encodeURIComponent(id));
}

/**
 * The four shared filters. Names checked against the POINTS contract; the register route declares the
 * same four names (`comms_get_map_assets["truyVan"]`), and `tsc` goes red at `listMapAssets` the day
 * the two drift.
 */
function filterQuery(f: MapAssetFilter): URLSearchParams {
  const query = new URLSearchParams();
  const set = thamSoTheoHopDong<comms_get_map_asset_points["truyVan"] & comms_get_map_assets["truyVan"]>(query);
  set("q", f.q.trim());
  set("industry_code", f.industryCode);
  set("residential_unit_id", f.residentialUnitId);
  set("status", f.status);
  return query;
}

function withQuery(path: string, query: URLSearchParams): string {
  const s = query.toString();
  return s === "" ? path : `${path}?${s}`;
}

/** GET the points of the filter. Groups are NOT filtered here: the layer toggles filter on the map, without a refetch. */
export function listMapAssetPoints(f: MapAssetFilter): Promise<KetQua<comms_mapAssetPointsOut>> {
  return docJSON<comms_mapAssetPointsOut>(withQuery(POINTS_PATH, filterQuery(f)));
}

/** GET one page of the register. `limit` is capped by the server (100). */
export function listMapAssets(
  f: MapAssetFilter,
  cursor: string,
  limit: number,
): Promise<KetQua<page_Result_comms_mapAssetRowOut>> {
  const query = filterQuery(f);
  const set = thamSoTheoHopDong<comms_get_map_assets["truyVan"]>(query);
  set("limit", limit);
  set("cursor", cursor);
  return docJSON<page_Result_comms_mapAssetRowOut>(withQuery(LIST_PATH, query));
}

export function getMapAsset(id: string): Promise<KetQua<comms_mapAssetOut>> {
  return docJSON<comms_mapAssetOut>(itemPath(ITEM_PATH, id));
}

export function getMapAssetSummary(): Promise<KetQua<comms_mapAssetSummaryOut>> {
  return docJSON<comms_mapAssetSummaryOut>(SUMMARY_PATH);
}

/** Create body: the contract's type WITHOUT `verified` (400 on create). */
export type CreateMapAssetBody = Omit<comms_createMapAssetIn, "verified">;
/** Patch body: the contract's type WITHOUT `verified` (400 on patch). */
export type UpdateMapAssetBody = Omit<comms_updateMapAssetIn, "verified">;

/**
 * POST — `idempotencyKey` is minted when the form OPENS and kept for every retry of it: the server's
 * only guard against a double-submitted form putting the same place on the map twice is that key.
 * Built field by field so nothing the caller did not mean to send goes up.
 */
export function createMapAsset(body: CreateMapAssetBody, idempotencyKey: string): Promise<KetQua<comms_mapAssetOut>> {
  const sent: CreateMapAssetBody = {
    asset_type_code: body.asset_type_code,
    name: body.name,
    lat: body.lat,
    lng: body.lng,
    address: body.address,
    residential_unit_id: body.residential_unit_id,
    representative: body.representative,
    phone: body.phone,
    status: body.status,
    tax_code: body.tax_code,
    industry_code: body.industry_code,
    employee_count: body.employee_count,
    established_on: body.established_on,
    description: body.description,
    custom_values: body.custom_values,
  };
  return docThanLoiGoi<comms_mapAssetOut>(goiGhi(LIST_PATH, "POST", sent, 201, { "Idempotency-Key": idempotencyKey }));
}

/** PATCH — only the fields present in `body`; `undefined` stays absent (`JSON.stringify` drops it). */
export function updateMapAsset(id: string, body: UpdateMapAssetBody): Promise<KetQua<comms_mapAssetOut>> {
  const sent: UpdateMapAssetBody = {
    asset_type_code: body.asset_type_code,
    name: body.name,
    lat: body.lat,
    lng: body.lng,
    address: body.address,
    residential_unit_id: body.residential_unit_id,
    representative: body.representative,
    phone: body.phone,
    status: body.status,
    tax_code: body.tax_code,
    industry_code: body.industry_code,
    employee_count: body.employee_count,
    established_on: body.established_on,
    description: body.description,
    custom_values: body.custom_values,
  };
  return docThanLoiGoi<comms_mapAssetOut>(goiGhi(itemPath(ITEM_PATH, id), "PATCH", sent, 200));
}

/** DELETE — soft delete, reason required, 204 with no body. */
export async function deleteMapAsset(id: string, reason: string): Promise<KetQua<null>> {
  const body: comms_deleteMapAssetIn = { reason };
  const res = await goiGhi(itemPath(ITEM_PATH, id), "DELETE", body, 204);
  return res.ok ? { ok: true, duLieu: null } : res;
}

/** POST confirmation — "Xác minh" (true) / "Bỏ xác minh" (false). */
export function setMapAssetConfirmation(id: string, verified: boolean): Promise<KetQua<comms_mapAssetOut>> {
  const body: comms_mapAssetConfirmationIn = { verified };
  return docThanLoiGoi<comms_mapAssetOut>(goiGhi(itemPath(CONFIRM_PATH, id), "POST", body, 200));
}

/** POST defaults — sow the eleven groups of ADR 0072 §3 into this commune's catalogue. No body. */
export function seedMapAssetTypeDefaults(): Promise<KetQua<comms_seedMapAssetTypesOut>> {
  return docThanLoiGoi<comms_seedMapAssetTypesOut>(goiGhi(DEFAULTS_PATH, "POST", undefined, 200));
}
