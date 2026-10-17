/**
 * The commune's dynamic attributes on `/nguoi-dan` (ADR 0101) — the pure half: what a control holds, what
 * is sent, what a cell shows, what a filter sends. No JSX, no network.
 *
 * WHAT A CONTROL HOLDS IS NOT WHAT IS SENT. A number is typed as text ("12,5"), a date as the picker's
 * string ("2026-10-17"); the body carries a number and epoch MILLIS (TASK E-1). The conversion lives here,
 * once, so the form, the filter and the tests read the same rule.
 *
 * TIME IS VIETNAM TIME, NEVER THE MACHINE'S: a date typed in a commune means that day in Asia/Ho_Chi_Minh
 * (+07:00, no daylight saving), the zone every other screen of this app pins (`audit-log-view.ts`). A
 * browser left on another zone must not shift a birth date by a day.
 *
 * TEXT CAPS ARE THIS SCREEN'S ASSUMPTION (one line 255, several lines 2000): ADR 0101 names none. The
 * server's own refusal still shows if it is tighter.
 */

import type { CitizenAttributeFilter, CitizenAttributes, CitizenAttributeValue } from "@/lib/api/citizen-contacts";
import type { CitizenField } from "@/lib/api/citizen-fields";

/* ---- limits and words ------------------------------------------------------------------------------ */

export const SINGLE_TEXT_MAX = 255;
export const MULTI_TEXT_MAX = 2000;

export const REQUIRED_TEXT = "Ô này bắt buộc.";
export const INTEGER_INVALID = "Nhập số nguyên, ví dụ: 12.";
export const DECIMAL_INVALID = "Nhập số, ví dụ: 12,5.";
export const NUMBER_TOO_LARGE = "Số quá lớn.";
export const DATE_INVALID = "Ngày giờ chưa đúng.";
export const CHOICE_INVALID = "Lựa chọn không còn trong danh sách.";
export const RANGE_REVERSED = "“Từ” phải trước hoặc bằng “đến”.";

export function textTooLong(max: number): string {
  return `Tối đa ${max} ký tự.`;
}

/* ---- Vietnam time ---------------------------------------------------------------------------------- */

const VN_OFFSET_MS = 7 * 60 * 60 * 1000;
const pad = (n: number, w = 2) => String(n).padStart(w, "0");

/** The wall-clock parts of an instant, read in Vietnam time. */
function vnParts(ms: number): { y: number; mo: number; d: number; h: number; mi: number } {
  const t = new Date(ms + VN_OFFSET_MS);
  return { y: t.getUTCFullYear(), mo: t.getUTCMonth() + 1, d: t.getUTCDate(), h: t.getUTCHours(), mi: t.getUTCMinutes() };
}

/** The instant of a Vietnam wall-clock time; `null` when the parts are not a real date (31/02). */
function vnInstant(y: number, mo: number, d: number, h = 0, mi = 0): number | null {
  if (y < 1 || y > 9999 || mo < 1 || mo > 12 || d < 1 || h > 23 || mi > 59) return null;
  const ms = Date.UTC(y, mo - 1, d, h, mi) - VN_OFFSET_MS;
  const back = vnParts(ms);
  return back.y === y && back.mo === mo && back.d === d ? ms : null;
}

/** Which native picker a pattern uses, and what string it holds. */
export type DateInputKind = "date" | "datetime-local" | "month" | "year";

export function dateInputKind(pattern: string): DateInputKind {
  switch (pattern) {
    case "dd/MM/yyyy HH:mm":
      return "datetime-local";
    case "MM/yyyy":
      return "month";
    case "yyyy":
      return "year";
    default:
      return "date";
  }
}

/**
 * The picker's string → epoch millis, at the START of what it names (`2026-10` = 01/10/2026 00:00). `null`
 * for an empty or malformed string — never a guessed date.
 */
export function pickerToMillis(kind: DateInputKind, s: string): number | null {
  const v = s.trim();
  let m: RegExpExecArray | null;
  switch (kind) {
    case "date":
      m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(v);
      return m === null ? null : vnInstant(+m[1]!, +m[2]!, +m[3]!);
    case "datetime-local":
      m = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})(?::\d{2}(?:\.\d+)?)?$/.exec(v);
      return m === null ? null : vnInstant(+m[1]!, +m[2]!, +m[3]!, +m[4]!, +m[5]!);
    case "month":
      m = /^(\d{4})-(\d{2})$/.exec(v);
      return m === null ? null : vnInstant(+m[1]!, +m[2]!, 1);
    case "year":
      m = /^(\d{4})$/.exec(v);
      return m === null ? null : vnInstant(+m[1]!, 1, 1);
  }
}

/** Epoch millis → the picker's string, in Vietnam time. */
export function millisToPicker(kind: DateInputKind, ms: number): string {
  const p = vnParts(ms);
  switch (kind) {
    case "date":
      return `${pad(p.y, 4)}-${pad(p.mo)}-${pad(p.d)}`;
    case "datetime-local":
      return `${pad(p.y, 4)}-${pad(p.mo)}-${pad(p.d)}T${pad(p.h)}:${pad(p.mi)}`;
    case "month":
      return `${pad(p.y, 4)}-${pad(p.mo)}`;
    case "year":
      return pad(p.y, 4);
  }
}

/**
 * The LAST millisecond of the period a picker string names — the inclusive upper bound of a "đến" filter:
 * "đến 17/10/2026" includes the whole of that day, "đến 10/2026" the whole month.
 */
export function pickerToEndMillis(kind: DateInputKind, s: string): number | null {
  const start = pickerToMillis(kind, s);
  if (start === null) return null;
  const p = vnParts(start);
  let next: number | null;
  switch (kind) {
    case "date":
      next = Date.UTC(p.y, p.mo - 1, p.d + 1) - VN_OFFSET_MS;
      break;
    case "datetime-local":
      next = start + 60_000;
      break;
    case "month":
      next = Date.UTC(p.y, p.mo, 1) - VN_OFFSET_MS;
      break;
    case "year":
      next = Date.UTC(p.y + 1, 0, 1) - VN_OFFSET_MS;
      break;
  }
  return next - 1;
}

/** Epoch millis shown per the field's pattern: `17/10/2026`, `17/10/2026 08:30`, `10/2026`, `2026`. */
export function formatDateByPattern(pattern: string, ms: number): string {
  const p = vnParts(ms);
  switch (pattern) {
    case "dd/MM/yyyy HH:mm":
      return `${pad(p.d)}/${pad(p.mo)}/${pad(p.y, 4)} ${pad(p.h)}:${pad(p.mi)}`;
    case "MM/yyyy":
      return `${pad(p.mo)}/${pad(p.y, 4)}`;
    case "yyyy":
      return pad(p.y, 4);
    default:
      return `${pad(p.d)}/${pad(p.mo)}/${pad(p.y, 4)}`;
  }
}

/* ---- numbers --------------------------------------------------------------------------------------- */

/**
 * Typed text → a number, or `null` when it is not one of the kind. A decimal accepts the Vietnamese comma
 * ("12,5") as well as a point; thousands separators are refused rather than guessed ("1.000" could be one
 * thousand or one point zero).
 */
export function parseNumberInput(kind: "integer" | "decimal", s: string): number | null {
  const v = s.trim();
  if (kind === "integer") {
    if (!/^-?\d+$/.test(v)) return null;
    const n = Number(v);
    return Number.isSafeInteger(n) ? n : null;
  }
  if (!/^-?\d+(?:[.,]\d+)?$/.test(v)) return null;
  const n = Number(v.replace(",", "."));
  return Number.isFinite(n) ? n : null;
}

/** A stored number as a commune reads it: the Vietnamese decimal comma, no grouping (it is a value, not an amount). */
export function formatNumber(n: number): string {
  return String(n).replace(".", ",");
}

/** A number field's kind; `""` (the server's "not set") reads as decimal, the kind that refuses no number. */
function numberKindOf(f: CitizenField): "integer" | "decimal" {
  return f.number_kind === "integer" ? "integer" : "decimal";
}

/* ---- the form's values ----------------------------------------------------------------------------- */

/** What one dynamic control holds: text for text, number and date controls; a list for "Chọn nhiều". */
export type AttributeInput = string | readonly string[];
export type AttributeInputs = Readonly<Record<string, AttributeInput>>;

function emptyInput(f: CitizenField): AttributeInput {
  return f.field_type === "combobox" ? [] : "";
}

/** The controls' starting values from a stored contact — empty for every live field the contact has no value for. */
export function inputsFromAttributes(fields: readonly CitizenField[], stored: CitizenAttributes | undefined): AttributeInputs {
  const out: Record<string, AttributeInput> = {};
  for (const f of fields) {
    const v = stored?.[f.code];
    out[f.code] = v === undefined ? emptyInput(f) : inputFromValue(f, v);
  }
  return out;
}

function inputFromValue(f: CitizenField, v: CitizenAttributeValue): AttributeInput {
  switch (f.field_type) {
    case "combobox":
      return Array.isArray(v) ? v : typeof v === "string" ? [v] : [];
    case "number":
      return typeof v === "number" ? formatNumber(v) : String(v);
    case "dateTime":
      return typeof v === "number" ? millisToPicker(dateInputKind(f.date_pattern), v) : "";
    default:
      return Array.isArray(v) ? v.join(", ") : String(v);
  }
}

function isEmptyInput(v: AttributeInput | undefined): boolean {
  return v === undefined || (typeof v === "string" ? v.trim() === "" : v.length === 0);
}

/** The values a choice field accepts: its live options, plus whatever the contact already holds (a retired option is kept, rule 7). */
function acceptedValues(f: CitizenField, stored: CitizenAttributeValue | undefined): Set<string> {
  const ok = new Set(f.options.filter((o) => !o.retired).map((o) => o.value));
  if (typeof stored === "string") ok.add(stored);
  if (Array.isArray(stored)) for (const s of stored) ok.add(s);
  return ok;
}

/**
 * The controls → the `attributes` of the body, or the sentence under each control that is wrong.
 *
 * VALUES OF FIELDS NOT ON THE FORM ARE CARRIED OVER UNTOUCHED. A retired field is not drawn (#4) but its
 * value is still the person's; the body is the whole map, so dropping it here would erase it on the first
 * save after retirement. An EMPTY live control removes its key — that is the officer clearing the value.
 */
export function attributesFromInputs(
  fields: readonly CitizenField[],
  inputs: AttributeInputs,
  stored: CitizenAttributes | undefined,
): { ok: true; attributes: CitizenAttributes } | { ok: false; errors: Record<string, string> } {
  const out: CitizenAttributes = {};
  const live = new Set(fields.map((f) => f.code));
  for (const [k, v] of Object.entries(stored ?? {})) if (!live.has(k)) out[k] = v;

  const errors: Record<string, string> = {};
  for (const f of fields) {
    const input = inputs[f.code];
    if (isEmptyInput(input)) {
      if (f.required) errors[f.code] = REQUIRED_TEXT;
      continue;
    }
    const r = valueFromInput(f, input!, stored?.[f.code]);
    if ("error" in r) errors[f.code] = r.error;
    else out[f.code] = r.value;
  }
  return Object.keys(errors).length > 0 ? { ok: false, errors } : { ok: true, attributes: out };
}

function valueFromInput(f: CitizenField, input: AttributeInput, stored: CitizenAttributeValue | undefined): { value: CitizenAttributeValue } | { error: string } {
  const text = typeof input === "string" ? input.trim() : "";
  switch (f.field_type) {
    case "singleText":
    case "multiText": {
      const max = f.field_type === "singleText" ? SINGLE_TEXT_MAX : MULTI_TEXT_MAX;
      return [...text].length > max ? { error: textTooLong(max) } : { value: text };
    }
    case "number": {
      const kind = numberKindOf(f);
      const n = parseNumberInput(kind, text);
      if (n === null) return { error: kind === "integer" && /^-?\d+$/.test(text) ? NUMBER_TOO_LARGE : kind === "integer" ? INTEGER_INVALID : DECIMAL_INVALID };
      return { value: n };
    }
    case "dateTime": {
      const ms = pickerToMillis(dateInputKind(f.date_pattern), text);
      return ms === null ? { error: DATE_INVALID } : { value: ms };
    }
    case "singleBox":
      return acceptedValues(f, stored).has(text) ? { value: text } : { error: CHOICE_INVALID };
    case "combobox": {
      const list = typeof input === "string" ? [input] : [...input];
      const ok = acceptedValues(f, stored);
      return list.every((v) => ok.has(v)) ? { value: list } : { error: CHOICE_INVALID };
    }
  }
}

/** Toggle one value in a "Chọn nhiều" control, keeping the options' order rather than the click order. */
export function toggleChoice(f: CitizenField, current: readonly string[], value: string): string[] {
  const next = current.includes(value) ? current.filter((v) => v !== value) : [...current, value];
  const order = f.options.map((o) => o.value);
  return [...next].sort((a, b) => order.indexOf(a) - order.indexOf(b));
}

/* ---- the list's cells ------------------------------------------------------------------------------ */

function optionLabel(f: CitizenField, value: string): string {
  return f.options.find((o) => o.value === value)?.label ?? value;
}

/** A stored value as a cell shows it: option labels, the date per the pattern, the decimal comma. `""` = no value. */
export function displayAttribute(f: CitizenField, v: CitizenAttributeValue | undefined): string {
  if (v === undefined) return "";
  switch (f.field_type) {
    case "combobox":
    case "singleBox":
      return (Array.isArray(v) ? v : [String(v)]).map((x) => optionLabel(f, x)).join(", ");
    case "number":
      return typeof v === "number" ? formatNumber(v) : String(v);
    case "dateTime":
      return typeof v === "number" ? formatDateByPattern(f.date_pattern, v) : String(v);
    default:
      return Array.isArray(v) ? v.join(", ") : String(v);
  }
}

/* ---- filters --------------------------------------------------------------------------------------- */

/** Which live fields have a filter (ADR 0101 #10): choices by value, numbers and dates by range. Text has none. */
export function isFilterable(f: CitizenField): boolean {
  return f.field_type === "combobox" || f.field_type === "singleBox" || f.field_type === "number" || f.field_type === "dateTime";
}

/** What one filter control holds before it is applied. */
export type FilterInput = { readonly values: readonly string[] } | { readonly from: string; readonly to: string };
export type FilterInputs = Readonly<Record<string, FilterInput>>;

/** The panel's whole draft: the creation-date range (picker strings) and one entry per filterable field. */
export type FilterDraft = {
  readonly createdFrom: string;
  readonly createdTo: string;
  readonly fields: FilterInputs;
};

export const EMPTY_FILTER_DRAFT: FilterDraft = { createdFrom: "", createdTo: "", fields: {} };

/**
 * The draft → what the search sends: `created_from` / `created_to` in millis and one filter per field that
 * says something. A range needs only one bound (#10); an untouched control sends nothing. A reversed range
 * or a malformed bound is refused HERE, with the sentence under the control, and nothing is sent.
 */
export function filtersFromDraft(
  fields: readonly CitizenField[],
  d: FilterDraft,
): { ok: true; createdFrom: number | null; createdTo: number | null; filters: CitizenAttributeFilter[] } | { ok: false; errors: Record<string, string> } {
  const errors: Record<string, string> = {};
  const range = (kind: DateInputKind, from: string, to: string, key: string): { from?: number; to?: number } => {
    const out: { from?: number; to?: number } = {};
    if (from.trim() !== "") {
      const v = pickerToMillis(kind, from);
      if (v === null) errors[key] = DATE_INVALID;
      else out.from = v;
    }
    if (to.trim() !== "") {
      const v = pickerToEndMillis(kind, to);
      if (v === null) errors[key] = DATE_INVALID;
      else out.to = v;
    }
    if (errors[key] === undefined && out.from !== undefined && out.to !== undefined && out.from > out.to) errors[key] = RANGE_REVERSED;
    return out;
  };

  const created = range("date", d.createdFrom, d.createdTo, "created");
  const filters: CitizenAttributeFilter[] = [];
  for (const f of fields) {
    if (!isFilterable(f)) continue;
    const input = d.fields[f.code];
    if (input === undefined) continue;
    if ("values" in input) {
      if (input.values.length > 0) filters.push({ code: f.code, values: [...input.values], from: null, to: null });
      continue;
    }
    let r: { from?: number; to?: number };
    if (f.field_type === "number") {
      r = {};
      const kind = numberKindOf(f);
      for (const side of ["from", "to"] as const) {
        const raw = input[side];
        if (raw.trim() === "") continue;
        const n = parseNumberInput(kind, raw);
        if (n === null) errors[f.code] = kind === "integer" ? INTEGER_INVALID : DECIMAL_INVALID;
        else r[side] = n;
      }
      if (errors[f.code] === undefined && r.from !== undefined && r.to !== undefined && r.from > r.to) errors[f.code] = RANGE_REVERSED;
    } else {
      r = range(dateInputKind(f.date_pattern), input.from, input.to, f.code);
    }
    if (errors[f.code] === undefined && (r.from !== undefined || r.to !== undefined)) filters.push({ code: f.code, values: [], from: r.from ?? null, to: r.to ?? null });
  }
  if (Object.keys(errors).length > 0) return { ok: false, errors };
  return { ok: true, createdFrom: created.from ?? null, createdTo: created.to ?? null, filters };
}

/** How many conditions the applied advanced filter holds — the figure on the "Bộ lọc" button. */
export function advancedFilterCount(createdFrom: number | null, createdTo: number | null, filters: readonly CitizenAttributeFilter[]): number {
  return (createdFrom !== null || createdTo !== null ? 1 : 0) + filters.length;
}
