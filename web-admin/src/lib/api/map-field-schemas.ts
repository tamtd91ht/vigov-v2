/**
 * The four routes behind "Cấu hình → Trường bản đồ" (`docs/ui-ux/14-cau-hinh.md §6`):
 * GET · POST /api/v1/map-field-schemas, PATCH · DELETE /api/v1/map-field-schemas/{id}.
 *
 * READ IS `asset.read`, WRITE IS `admin.lookup` — two keys (`x-vigov-permission` in the contract).
 * The screen hides by them; the server refuses by them on every request (rule 5, #1).
 *
 * `asset_type_code`, `field_code` AND `value_type` NEVER GO UP IN A PATCH. All three are immutable,
 * and the server refuses the WHOLE request (400) when the body even names them — a stored value is
 * addressed by `asset_type_code/field_code` and was written as `value_type`
 * (`service-comms/internal/http/map_field_schema.go`). `UpdateMapFieldIn` omits them at compile
 * time; the body is built field by field so a row read from the server cannot carry them up.
 *
 * `options` ON A PATCH IS THE WHOLE NEW LIST, not a delta, and it must keep every value the row has
 * (relabel and append only; 409 `option_removed` otherwise). The screen locks existing values
 * (`features/cau-hinh/map-field-form.ts`); the server is the one that refuses.
 */

import { docJSON, docThanLoiGoi, goiGhi, thamSoTheoHopDong } from "./goi";
import type { KetQua } from "./goi";
import type {
  comms_createMapFieldSchemaIn,
  comms_delete_map_field_schemas_by_id,
  comms_deleteMapFieldSchemaIn,
  comms_get_map_field_schemas,
  comms_mapFieldSchemaListOut,
  comms_mapFieldSchemaOut,
  comms_patch_map_field_schemas_by_id,
  comms_post_map_field_schemas,
  comms_updateMapFieldSchemaIn,
} from "./schema.gen";

const LIST_PATH = "/api/v1/map-field-schemas" satisfies comms_get_map_field_schemas["duongDan"] &
  comms_post_map_field_schemas["duongDan"];
const ITEM_PATH = "/api/v1/map-field-schemas/{id}" satisfies comms_patch_map_field_schemas_by_id["duongDan"] &
  comms_delete_map_field_schemas_by_id["duongDan"];

/** PATCH body the screen may send — the contract's type MINUS the three immutable fields. */
export type UpdateMapFieldIn = Omit<comms_updateMapFieldSchemaIn, "asset_type_code" | "field_code" | "value_type">;

function itemPath(id: string): string {
  return ITEM_PATH.replace("{id}", encodeURIComponent(id));
}

/**
 * GET the fields of ONE asset type, inactive ones included (`Tắt` rows stay listed).
 *
 * ONE TYPE PER CALL because the contract declares `asset_type_code` REQUIRED. The handler also
 * answers without it (every type), but that is behaviour the contract does not promise.
 */
export function listMapFields(assetTypeCode: string): Promise<KetQua<comms_mapFieldSchemaListOut>> {
  const query = new URLSearchParams();
  thamSoTheoHopDong<comms_get_map_field_schemas["truyVan"]>(query)("asset_type_code", assetTypeCode);
  return docJSON<comms_mapFieldSchemaListOut>(`${LIST_PATH}?${query.toString()}`);
}

/**
 * POST — add a field. `idempotencyKey` is minted when the form OPENS and kept for every retry of
 * that form (same reasoning as `themBoPhan` in `so-do-to-chuc.ts`).
 *
 * `options` is sent ONLY for `chon`: the server refuses options on any other type (400).
 */
export function createMapField(
  body: comms_createMapFieldSchemaIn,
  idempotencyKey: string,
): Promise<KetQua<comms_mapFieldSchemaOut>> {
  const sent: comms_createMapFieldSchemaIn = {
    asset_type_code: body.asset_type_code,
    field_code: body.field_code,
    label: body.label,
    value_type: body.value_type,
    options: body.value_type === "chon" ? body.options?.map((o) => ({ value: o.value, label: o.label })) : undefined,
    is_required: body.is_required,
    sort_order: body.sort_order,
  };
  return docThanLoiGoi<comms_mapFieldSchemaOut>(
    goiGhi(LIST_PATH, "POST", sent, 201, { "Idempotency-Key": idempotencyKey }),
  );
}

/** PATCH — only the fields passed; absent means unchanged. `Tắt`/`Bật` sends `is_active` alone. */
export function updateMapField(id: string, body: UpdateMapFieldIn): Promise<KetQua<comms_mapFieldSchemaOut>> {
  const sent: UpdateMapFieldIn = {
    label: body.label ?? undefined,
    options: body.options?.map((o) => ({ value: o.value, label: o.label })) ?? undefined,
    is_required: body.is_required ?? undefined,
    sort_order: body.sort_order ?? undefined,
    is_active: body.is_active ?? undefined,
  };
  return docThanLoiGoi<comms_mapFieldSchemaOut>(goiGhi(itemPath(id), "PATCH", sent, 200));
}

/**
 * DELETE — soft delete with a required reason, 204. The field code is NEVER issued again in that
 * asset type (409 `field_code_retired` on a later create), and values already stored under it are
 * kept (rule 7, invariant 3).
 */
export async function deleteMapField(id: string, reason: string): Promise<KetQua<null>> {
  const body: comms_deleteMapFieldSchemaIn = { reason };
  const res = await goiGhi(itemPath(id), "DELETE", body, 204);
  return res.ok ? { ok: true, duLieu: null } : res;
}
