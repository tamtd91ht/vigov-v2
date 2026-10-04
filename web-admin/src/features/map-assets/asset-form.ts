/**
 * The decision half of the add / edit form (spec §8). Pure, so the rules have tests:
 *
 *   - REQUIRED: group, name, position. A position outside the commune's frame is refused here (the
 *     frame is hard, ADR 0072 H3) — the server accepts any world coordinate, so this is the only place
 *     that keeps a pin inside the commune.
 *   - A PATCH CARRIES ONLY WHAT CHANGED. A masked value (`masked: true`) is never sent back: sending
 *     "09****000" up would overwrite a real number with its mask.
 *   - CUSTOM VALUES are typed by the field's `value_type`; an emptied field is sent as `null` (the
 *     server clears that key), an untouched one is not sent.
 *   - `verified` is never in a body (400): confirmation is its own route.
 */

import type { CreateMapAssetBody, UpdateMapAssetBody } from "@/lib/api/map-assets";
import type { comms_mapAssetOut, comms_mapFieldSchemaOut, JsonValue } from "@/lib/api/schema.gen";

import {
  BAD_COORDINATE,
  DEFAULT_STATUS,
  PIN_OUTSIDE_FRAME,
  REQUIRED_GROUP,
  REQUIRED_NAME,
  REQUIRED_POSITION,
} from "./labels";
import { insideFrame, round6, type FrameBounds } from "./map-logic";

/** The form as typed — strings throughout (inputs return strings). */
export type AssetDraft = {
  readonly assetTypeCode: string;
  readonly status: string;
  readonly name: string;
  readonly address: string;
  readonly residentialUnitId: string;
  readonly lat: string;
  readonly lng: string;
  readonly representative: string;
  readonly phone: string;
  readonly description: string;
  readonly taxCode: string;
  readonly industryCode: string;
  readonly employeeCount: string;
  readonly establishedOn: string;
  /** Custom values as typed, keyed by `field_code`. `dung-sai` uses "" / "true" / "false". */
  readonly custom: Readonly<Record<string, string>>;
};

export function formatCoordinate(x: number): string {
  return round6(x).toFixed(6);
}

/**
 * A new draft. The position starts at the FRAME's centre when there is one — never a commune's
 * coordinates written in code (spec §8.1's `15.730507 / 108.378110` is one commune; rule 1 inv. 10).
 */
export function newAssetDraft(assetTypeCode: string, centre: { lat: number; lng: number } | null): AssetDraft {
  return {
    assetTypeCode,
    status: DEFAULT_STATUS,
    name: "",
    address: "",
    residentialUnitId: "",
    lat: centre === null ? "" : formatCoordinate(centre.lat),
    lng: centre === null ? "" : formatCoordinate(centre.lng),
    representative: "",
    phone: "",
    description: "",
    taxCode: "",
    industryCode: "",
    employeeCount: "",
    establishedOn: "",
    custom: {},
  };
}

function customToText(v: JsonValue | undefined): string {
  if (v === undefined || v === null) return "";
  if (typeof v === "boolean") return v ? "true" : "false";
  if (typeof v === "number" || typeof v === "string") return String(v);
  return "";
}

/** Draft from a saved asset. Masked fields start EMPTY and are locked by the form (`masked`). */
export function draftFromAsset(a: comms_mapAssetOut): AssetDraft {
  const custom: Record<string, string> = {};
  for (const [k, v] of Object.entries(a.custom_values ?? {})) custom[k] = customToText(v);
  return {
    assetTypeCode: a.asset_type_code,
    status: a.status,
    name: a.name,
    address: a.address ?? "",
    residentialUnitId: a.residential_unit_id ?? "",
    lat: formatCoordinate(a.lat),
    lng: formatCoordinate(a.lng),
    representative: a.masked ? "" : (a.representative ?? ""),
    phone: a.masked ? "" : (a.phone ?? ""),
    description: a.description ?? "",
    taxCode: a.masked ? "" : (a.tax_code ?? ""),
    industryCode: a.industry_code ?? "",
    employeeCount: a.employee_count === null || a.employee_count === undefined ? "" : String(a.employee_count),
    establishedOn: a.established_on ?? "",
    custom,
  };
}

const COORD = /^-?\d{1,3}(\.\d{1,6})?$/;

/** Parse one coordinate as typed: at most 6 decimals, within ±limit. `null` when invalid. */
export function parseCoordinate(text: string, limit: 90 | 180): number | null {
  const t = text.trim().replace(",", ".");
  if (!COORD.test(t)) return null;
  const x = Number(t);
  return Number.isFinite(x) && Math.abs(x) <= limit ? x : null;
}

export type PositionCheck = { ok: true; lat: number; lng: number } | { ok: false; message: string };

/** Position of the draft, checked against the frame when there is one. */
export function checkPosition(latText: string, lngText: string, bounds: FrameBounds | null): PositionCheck {
  if (latText.trim() === "" || lngText.trim() === "") return { ok: false, message: REQUIRED_POSITION };
  const lat = parseCoordinate(latText, 90);
  const lng = parseCoordinate(lngText, 180);
  if (lat === null || lng === null) return { ok: false, message: BAD_COORDINATE };
  if (bounds !== null && !insideFrame(bounds, lng, lat)) return { ok: false, message: PIN_OUTSIDE_FRAME };
  return { ok: true, lat, lng };
}

/** Active fields of the group, in their order. A disabled field is not on the form (spec §10 note). */
export function formFields(fields: readonly comms_mapFieldSchemaOut[]): comms_mapFieldSchemaOut[] {
  return fields.filter((f) => f.is_active).sort((a, b) => a.sort_order - b.sort_order);
}

/** One typed custom value, `null` for empty, or an error sentence. */
function typedCustom(f: comms_mapFieldSchemaOut, text: string): { ok: true; value: JsonValue } | { ok: false; message: string } {
  const t = text.trim();
  if (t === "") return { ok: true, value: null };
  switch (f.value_type) {
    case "so-nguyen":
      return /^-?\d+$/.test(t) ? { ok: true, value: Number(t) } : { ok: false, message: `${f.label}: hãy nhập một số nguyên.` };
    case "so-thap-phan": {
      const x = Number(t.replace(",", "."));
      return Number.isFinite(x) ? { ok: true, value: x } : { ok: false, message: `${f.label}: hãy nhập một số.` };
    }
    case "dung-sai":
      return { ok: true, value: t === "true" };
    default:
      // van-ban, ngay (YYYY-MM-DD from <input type=date>), chon (an option value)
      return { ok: true, value: t };
  }
}

export type BuildResult<T> = { kind: "send"; body: T } | { kind: "error"; message: string } | { kind: "unchanged" };

function optional(s: string): string | undefined {
  const t = s.trim();
  return t === "" ? undefined : t;
}

function integerOrNull(s: string): number | null | "bad" {
  const t = s.trim();
  if (t === "") return null;
  return /^\d+$/.test(t) ? Number(t) : "bad";
}

/** POST body, or the first sentence that stops it. */
export function createBody(
  d: AssetDraft,
  fields: readonly comms_mapFieldSchemaOut[],
  bounds: FrameBounds | null,
): BuildResult<CreateMapAssetBody> {
  if (d.assetTypeCode === "") return { kind: "error", message: REQUIRED_GROUP };
  if (d.name.trim() === "") return { kind: "error", message: REQUIRED_NAME };
  const pos = checkPosition(d.lat, d.lng, bounds);
  if (!pos.ok) return { kind: "error", message: pos.message };
  const employees = integerOrNull(d.employeeCount);
  if (employees === "bad") return { kind: "error", message: "Số lao động phải là một số nguyên không âm." };

  const custom: Record<string, JsonValue> = {};
  for (const f of formFields(fields)) {
    const r = typedCustom(f, d.custom[f.field_code] ?? "");
    if (!r.ok) return { kind: "error", message: r.message };
    if (r.value === null) {
      if (f.is_required) return { kind: "error", message: `Hãy nhập ${f.label}.` };
      continue;
    }
    custom[f.field_code] = r.value;
  }

  return {
    kind: "send",
    body: {
      asset_type_code: d.assetTypeCode,
      name: d.name.trim(),
      lat: pos.lat,
      lng: pos.lng,
      status: d.status === "" ? undefined : d.status,
      address: optional(d.address),
      residential_unit_id: optional(d.residentialUnitId),
      representative: optional(d.representative),
      phone: optional(d.phone),
      tax_code: optional(d.taxCode),
      industry_code: optional(d.industryCode),
      employee_count: employees,
      established_on: optional(d.establishedOn),
      description: optional(d.description),
      custom_values: Object.keys(custom).length === 0 ? undefined : custom,
    },
  };
}

/**
 * PATCH body: ONLY fields whose value differs from `original`. An emptied text field goes up as `""`
 * would be a value — so it goes up as `null` (clear). Masked fields are never compared nor sent.
 */
export function updateBody(
  original: comms_mapAssetOut,
  d: AssetDraft,
  fields: readonly comms_mapFieldSchemaOut[],
  bounds: FrameBounds | null,
): BuildResult<UpdateMapAssetBody> {
  if (d.assetTypeCode === "") return { kind: "error", message: REQUIRED_GROUP };
  if (d.name.trim() === "") return { kind: "error", message: REQUIRED_NAME };
  const pos = checkPosition(d.lat, d.lng, bounds);
  if (!pos.ok) return { kind: "error", message: pos.message };
  const employees = integerOrNull(d.employeeCount);
  if (employees === "bad") return { kind: "error", message: "Số lao động phải là một số nguyên không âm." };

  const body: { -readonly [K in keyof UpdateMapAssetBody]: UpdateMapAssetBody[K] } = {};
  const text = (now: string, was: string | undefined): string | null | undefined => {
    const n = now.trim();
    const w = (was ?? "").trim();
    if (n === w) return undefined;
    return n === "" ? null : n;
  };

  if (d.assetTypeCode !== original.asset_type_code) body.asset_type_code = d.assetTypeCode;
  if (d.name.trim() !== original.name) body.name = d.name.trim();
  if (round6(pos.lat) !== round6(original.lat)) body.lat = pos.lat;
  if (round6(pos.lng) !== round6(original.lng)) body.lng = pos.lng;
  if (d.status !== original.status) body.status = d.status;
  body.address = text(d.address, original.address);
  body.residential_unit_id = text(d.residentialUnitId, original.residential_unit_id);
  if (!original.masked) {
    body.representative = text(d.representative, original.representative);
    body.phone = text(d.phone, original.phone);
    body.tax_code = text(d.taxCode, original.tax_code);
  }
  body.industry_code = text(d.industryCode, original.industry_code);
  body.established_on = text(d.establishedOn, original.established_on);
  body.description = text(d.description, original.description);
  if (employees !== (original.employee_count ?? null)) body.employee_count = employees;

  const custom: Record<string, JsonValue> = {};
  for (const f of formFields(fields)) {
    const now = (d.custom[f.field_code] ?? "").trim();
    const was = customToText(original.custom_values?.[f.field_code]).trim();
    const r = typedCustom(f, now);
    if (!r.ok) return { kind: "error", message: r.message };
    if (r.value === null && f.is_required) return { kind: "error", message: `Hãy nhập ${f.label}.` };
    if (now === was) continue;
    custom[f.field_code] = r.value;
  }
  if (Object.keys(custom).length > 0) body.custom_values = custom;

  const changed = Object.values(body).some((v) => v !== undefined);
  return changed ? { kind: "send", body } : { kind: "unchanged" };
}
