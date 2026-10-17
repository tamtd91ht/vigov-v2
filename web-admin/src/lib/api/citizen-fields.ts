/**
 * Dynamic attributes of the citizen directory (ADR 0101 #1–#5) — the commune's own field definitions,
 * owned by `service-identity`. Cấu hình › Thuộc tính danh bạ writes them; `/nguoi-dan` reads the live ones
 * to draw its form, its columns and its filters.
 *
 *   GET    /api/v1/citizen-fields[?include_retired=true]  citizen.read         the definitions, in sort order
 *   POST   /api/v1/citizen-fields                         admin.citizen_field  201 the field · 409 `citizen_field_code_taken` / `citizen_field_limit`
 *   PATCH  /api/v1/citizen-fields/{code}                  admin.citizen_field  200 · label · placeholder · options · required · show_in_list · date_pattern
 *   DELETE /api/v1/citizen-fields/{code}                  admin.citizen_field  204 · `{reason}` — RETIRES (hidden, values kept, ADR 0101 #4)
 *   PUT    /api/v1/citizen-field-order                    admin.citizen_field  204 · `{codes}` — the whole live order
 *
 * `code`, `field_type` and `number_kind` never change after creation (#1): the PATCH body sends them `null`.
 *
 * NO `tenant_id`, RELATIVE PATHS, `credentials: same-origin`: the three rules of `goi.ts`. A definition
 * holds no personal data, but nothing here logs a body anyway (rule 3 is cheaper kept than argued).
 */

import {
  docJSON, // vi-name-ok: existing export of goi.ts, imported not declared (rule 12 inv 3)
  goiGhi, // vi-name-ok: existing export of goi.ts, imported not declared (rule 12 inv 3)
} from "./goi";
import type { KetQua } from "./goi"; // vi-name-ok: existing type of goi.ts, imported not declared (rule 12 inv 3)
import type {
  identity_citizenFieldIn,
  identity_citizenFieldOptionWire,
  identity_citizenFieldOrderIn,
  identity_citizenFieldOut,
  identity_citizenFieldPatchIn,
  identity_citizenFieldRetireIn,
  identity_citizenFieldsOut,
  identity_delete_citizen_fields_by_code,
  identity_get_citizen_fields,
  identity_patch_citizen_fields_by_code,
  identity_post_citizen_fields,
  identity_put_citizen_field_order,
} from "./schema.gen";

// Every shape is `schema.gen.ts`'s (agent rule 6). The ONLY local additions narrow two server strings to
// the values ADR 0101 fixed — `field_type` and `number_kind` — so a screen can switch on them exhaustively.
// The server owns the catalogue; a value outside it would be a server change, and arrives here as a type
// the screens do not draw rather than a crash.

/** ADR 0101 #2 — the owner's names, verbatim (a deliberate deviation from ADR 0011's Vietnamese enums). */
export type CitizenFieldType = "singleText" | "multiText" | "combobox" | "singleBox" | "number" | "dateTime";

export type CitizenFieldNumberKind = "integer" | "decimal";

/** One choice of a `combobox` / `singleBox` field. A saved value is never deleted, only `retired` (#1). */
export type CitizenFieldOption = identity_citizenFieldOptionWire;

/**
 * One field. `number_kind` is `""` unless `field_type` is `number`, `date_pattern` is `""` unless it is
 * `dateTime`; `options` is `[]` for a type without options; `retired_at` is `null` for a live field.
 */
export type CitizenField = Omit<identity_citizenFieldOut, "field_type" | "number_kind"> & {
  field_type: CitizenFieldType;
  number_kind: CitizenFieldNumberKind | "";
};

export type CitizenFieldList = Omit<identity_citizenFieldsOut, "items"> & { items: CitizenField[] };

/** POST body: the whole definition; what does not apply to the type is `""` / `[]`. */
export type CitizenFieldCreateBody = Omit<identity_citizenFieldIn, "field_type" | "number_kind"> & {
  field_type: CitizenFieldType;
  number_kind: CitizenFieldNumberKind | "";
};

/**
 * PATCH body: every key present, `null` = unchanged. `code`, `field_type` and `number_kind` are immutable
 * (#1) and always sent `null` by this screen; `options`, when set, is the WHOLE list.
 */
export type CitizenFieldUpdateBody = identity_citizenFieldPatchIn;

/** The 409 of a code the commune already holds, retired fields included. */
export const CITIZEN_FIELD_CODE_TAKEN = "citizen_field_code_taken" satisfies identity_post_citizen_fields["errorCodes"][409];

const FIELDS_PATH = "/api/v1/citizen-fields" satisfies identity_get_citizen_fields["duongDan"];
const CREATE_PATH = "/api/v1/citizen-fields" satisfies identity_post_citizen_fields["duongDan"];
const UPDATE_PATH = "/api/v1/citizen-fields/{code}" satisfies identity_patch_citizen_fields_by_code["duongDan"];
const RETIRE_PATH = "/api/v1/citizen-fields/{code}" satisfies identity_delete_citizen_fields_by_code["duongDan"];
/** The reorder route. */
export const CITIZEN_FIELD_ORDER_PATH = "/api/v1/citizen-field-order" satisfies identity_put_citizen_field_order["duongDan"];

/** The most fields a commune keeps (ADR 0101 #1). The server enforces it; the tab says it before the click. */
export const CITIZEN_FIELD_LIMIT = 50;

/** `/api/v1/citizen-fields`, with `?include_retired=true` only when asked — the tab asks, the directory does not. */
export function citizenFieldsPath(includeRetired: boolean): string {
  return includeRetired ? `${FIELDS_PATH}?include_retired=true` : FIELDS_PATH;
}

/** The code percent-encoded: `[a-z][a-z0-9]{5,7}` has nothing to escape, but a raw splice is a path that can walk. */
export function citizenFieldPath(code: string): string {
  return UPDATE_PATH.replace("{code}", encodeURIComponent(code));
}

function retirePath(code: string): string {
  return RETIRE_PATH.replace("{code}", encodeURIComponent(code));
}

/** `GET /api/v1/citizen-fields` — the definitions. Live only unless `includeRetired`. */
export function listCitizenFields(includeRetired = false): Promise<KetQua<CitizenFieldList>> {
  return docJSON<CitizenFieldList>(citizenFieldsPath(includeRetired));
}

/**
 * `POST /api/v1/citizen-fields`. The key is minted when the dialog OPENS and kept across retries of that
 * one attempt, like every create of this app. `withCode`: a taken code comes back with its `code`, so the
 * dialog can put the server's sentence under "Mã" rather than at the bottom of the form.
 */
export async function createCitizenField(body: CitizenFieldCreateBody, idempotencyKey: string): Promise<KetQua<null>> {
  const kq = await goiGhi(CREATE_PATH, "POST", body, 201, { "Idempotency-Key": idempotencyKey }, { withCode: true });
  return kq.ok ? { ok: true, duLieu: null } : kq;
}

/** `PATCH /api/v1/citizen-fields/{code}` — only the keys present change. */
export async function updateCitizenField(code: string, body: CitizenFieldUpdateBody): Promise<KetQua<null>> {
  const kq = await goiGhi(citizenFieldPath(code), "PATCH", body, 200, undefined, { withCode: true });
  return kq.ok ? { ok: true, duLieu: null } : kq;
}

/** `DELETE /api/v1/citizen-fields/{code}` with `{reason}` — retires the field; every value typed is kept. */
export async function retireCitizenField(code: string, reason: string): Promise<KetQua<null>> {
  const body: identity_citizenFieldRetireIn = { reason };
  const kq = await goiGhi(retirePath(code), "DELETE", body, 204);
  return kq.ok ? { ok: true, duLieu: null } : kq;
}

/** The whole live order, every code once. A partial list is the server's to refuse, not this file's to pad. */
export async function reorderCitizenFields(codes: readonly string[]): Promise<KetQua<null>> {
  const body: identity_citizenFieldOrderIn = { codes: [...codes] };
  const kq = await goiGhi(CITIZEN_FIELD_ORDER_PATH, "PUT", body, 204);
  return kq.ok ? { ok: true, duLieu: null } : kq;
}
