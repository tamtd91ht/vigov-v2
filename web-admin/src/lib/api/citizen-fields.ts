/**
 * Dynamic attributes of the citizen directory (ADR 0101 #1–#5) — the commune's own field definitions,
 * owned by `service-identity`. Cấu hình › Thuộc tính danh bạ writes them; `/nguoi-dan` reads the live ones
 * to draw its form, its columns and its filters.
 *
 *   GET    /api/v1/citizen-fields[?include_retired=true]  citizen.read         the definitions, in sort order
 *   POST   /api/v1/citizen-fields                         admin.citizen_field  one new field
 *   PATCH  /api/v1/citizen-fields/{code}                  admin.citizen_field  label · placeholder · options · required · show_in_list · date_pattern
 *   DELETE /api/v1/citizen-fields/{code}                  admin.citizen_field  `{reason}` — RETIRES (hidden, values kept, ADR 0101 #4)
 *   PUT    /api/v1/citizen-field-order                    admin.citizen_field  `{codes}` — the whole live order
 *
 * `code` and `field_type` never change after creation (#1), so neither is in the PATCH body.
 *
 * NO `tenant_id`, RELATIVE PATHS, `credentials: same-origin`: the three rules of `goi.ts`. A definition
 * holds no personal data, but nothing here logs a body anyway (rule 3 is cheaper kept than argued).
 */

import {
  docJSON, // vi-name-ok: existing export of goi.ts, imported not declared (rule 12 inv 3)
  goiGhi, // vi-name-ok: existing export of goi.ts, imported not declared (rule 12 inv 3)
} from "./goi";
import type { KetQua } from "./goi"; // vi-name-ok: existing type of goi.ts, imported not declared (rule 12 inv 3)

// ---- UNTIL REGEN (TASK E-1) ------------------------------------------------------------------------
// `schema.gen.ts` has no citizen-field route yet: the backend half of ADR 0101 is built in parallel. These
// shapes are the contract TASK E-1 fixed (17/10/2026), written by hand ONLY until `make kb` regenerates the
// schema — then every type below is replaced by its `identity_*` twin and every path gets its
// `satisfies <route>["duongDan"]`, exactly as `citizen-contacts.ts` does (rule 2, invariant 7; agent rule 6).

/** ADR 0101 #2 — the owner's names, verbatim (a deliberate deviation from ADR 0011's Vietnamese enums). */
export type CitizenFieldType = "singleText" | "multiText" | "combobox" | "singleBox" | "number" | "dateTime";

/** One choice of a `combobox` / `singleBox` field. A saved value is never deleted, only `retired` (#1). */
export type CitizenFieldOption = { value: string; label: string; retired: boolean };

export type CitizenFieldNumberKind = "integer" | "decimal";

export type CitizenField = {
  code: string;
  label: string;
  field_type: CitizenFieldType;
  placeholder: string;
  options: CitizenFieldOption[];
  number_kind: CitizenFieldNumberKind | null;
  date_pattern: string | null;
  required: boolean;
  show_in_list: boolean;
  sort_order: number;
  retired_at: string | null;
};

export type CitizenFieldList = { items: CitizenField[] };

/** POST body: the field without what the server assigns (`sort_order`) or sets later (`retired_at`). */
export type CitizenFieldCreateBody = Omit<CitizenField, "retired_at" | "sort_order">;

/** PATCH body: only what may change after creation; an absent key is left as it is. */
export type CitizenFieldUpdateBody = Partial<
  Pick<CitizenField, "label" | "placeholder" | "options" | "required" | "show_in_list" | "date_pattern">
>;

export type CitizenFieldRetireBody = { reason: string };
export type CitizenFieldOrderBody = { codes: string[] };

const FIELDS_PATH = "/api/v1/citizen-fields";
const FIELD_PATH = "/api/v1/citizen-fields/{code}";
/** The reorder route. Its final name was left open by TASK E-1 — this constant is the one line to change. */
export const CITIZEN_FIELD_ORDER_PATH = "/api/v1/citizen-field-order";

/**
 * Success statuses are not published yet: create answers 201 (or 200 with the field), the other writes
 * 204 (or 200). Narrow each to the contract's one status at regen — `goiGhi` asks for that.
 */
const CREATED = [200, 201] as const;
const DONE = [200, 204] as const;
// ---- end UNTIL REGEN -------------------------------------------------------------------------------

/** The most fields a commune keeps (ADR 0101 #1). The server enforces it; the tab says it before the click. */
export const CITIZEN_FIELD_LIMIT = 50;

/** `/api/v1/citizen-fields`, with `?include_retired=true` only when asked — the tab asks, the directory does not. */
export function citizenFieldsPath(includeRetired: boolean): string {
  return includeRetired ? `${FIELDS_PATH}?include_retired=true` : FIELDS_PATH;
}

/** The code percent-encoded: `[a-z][a-z0-9]{5,7}` has nothing to escape, but a raw splice is a path that can walk. */
export function citizenFieldPath(code: string): string {
  return FIELD_PATH.replace("{code}", encodeURIComponent(code));
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
  const kq = await goiGhi(FIELDS_PATH, "POST", body, CREATED, { "Idempotency-Key": idempotencyKey }, { withCode: true });
  return kq.ok ? { ok: true, duLieu: null } : kq;
}

/** `PATCH /api/v1/citizen-fields/{code}` — only the keys present change. */
export async function updateCitizenField(code: string, body: CitizenFieldUpdateBody): Promise<KetQua<null>> {
  const kq = await goiGhi(citizenFieldPath(code), "PATCH", body, DONE, undefined, { withCode: true });
  return kq.ok ? { ok: true, duLieu: null } : kq;
}

/** `DELETE /api/v1/citizen-fields/{code}` with `{reason}` — retires the field; every value typed is kept. */
export async function retireCitizenField(code: string, reason: string): Promise<KetQua<null>> {
  const body: CitizenFieldRetireBody = { reason };
  const kq = await goiGhi(citizenFieldPath(code), "DELETE", body, DONE);
  return kq.ok ? { ok: true, duLieu: null } : kq;
}

/** The whole live order, every code once. A partial list is the server's to refuse, not this file's to pad. */
export async function reorderCitizenFields(codes: readonly string[]): Promise<KetQua<null>> {
  const body: CitizenFieldOrderBody = { codes: [...codes] };
  const kq = await goiGhi(CITIZEN_FIELD_ORDER_PATH, "PUT", body, DONE);
  return kq.ok ? { ok: true, duLieu: null } : kq;
}
