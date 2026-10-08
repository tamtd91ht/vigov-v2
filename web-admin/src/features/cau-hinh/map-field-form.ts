/**
 * The decision half of the "Trường bản đồ" tab (`docs/ui-ux/14-cau-hinh.md §6`). Pure — no network,
 * no DOM — so the one rule a working screen never shows breaking has a test:
 *
 *   AN OPTION VALUE ALREADY ISSUED IS LOCKED. On edit, existing `chon` options can be relabelled and
 *   new ones appended; an existing VALUE can be neither changed nor removed. Assets will store the
 *   value, so changing it orphans every record filed under the old one — the server refuses with 409
 *   `option_removed` (`service-comms/internal/domain/map_field_schema.go`, `ErrOptionRemoved`). The
 *   lock here spares staff that round-trip; it does not replace the refusal.
 *
 * `field_code`, `asset_type_code` and `value_type` are equally fixed after create, and the PATCH
 * body built here never names them (the server refuses a body that does, even unchanged).
 */

import type {
  comms_createMapFieldSchemaIn,
  comms_fieldOptionIn,
  comms_mapFieldSchemaOut,
} from "@/lib/api/schema.gen";
import type { UpdateMapFieldIn } from "@/lib/api/map-field-schemas";

/**
 * The six value types: VALUE as stored (ADR 0011, Vietnamese without diacritics — the server's
 * constants, `service-comms/internal/domain/map_field_schema.go`) and the LABEL of spec Cấu hình 06
 * (ADR 0079; prototype `AssetFieldTable.tsx:29-36`). Only the labels follow the spec: its values
 * (`text`, `integer`, …) are not what the server stores (spec 12 "giữ backend, chỉ map lại").
 */
export const VALUE_TYPES: readonly { readonly value: string; readonly label: string }[] = [
  { value: "van-ban", label: "Chữ" },
  { value: "so-nguyen", label: "Số nguyên" },
  { value: "so-thap-phan", label: "Số thập phân" },
  { value: "dung-sai", label: "Có / Không" },
  { value: "ngay", label: "Ngày" },
  { value: "chon", label: "Chọn trong danh sách" },
];

/** The server's `FieldCodeMaxLen` — a longer generated code would be refused as "too long". */
const FIELD_CODE_MAX = 64;

/**
 * The field code generated from the label (spec 06; prototype `AssetFieldTable.tsx:281-288`): strip
 * Vietnamese diacritics, `đ` → `d`, lowercase, every other run of characters → one `_`, trimmed.
 *
 * ADAPTED TO THE SERVER'S SHAPE `^[a-z][a-z0-9_]*$`, 1–64 (`NormalizeFieldCode`): the prototype's
 * slug of "3 tầng" is `3_tang`, which the server refuses for starting with a digit — such a code gets
 * the prefix `f_`. Cut at 64 characters, then trimmed of a trailing `_` again. Empty = the label has
 * no letter or digit, and nothing can be sent.
 */
export function fieldCodeFromLabel(label: string): string {
  let code = label
    .normalize("NFD")
    .replace(/[̀-ͯ]/g, "")
    .replace(/[đĐ]/g, "d")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "_")
    .replace(/^_+|_+$/g, "");
  if (code === "") return "";
  if (!/^[a-z]/.test(code)) code = `f_${code}`;
  return code.slice(0, FIELD_CODE_MAX).replace(/_+$/, "");
}

export const LABEL_NEEDS_ALNUM = "Nhãn phải có ít nhất một chữ cái hoặc chữ số.";

export const CHOICE_TYPE = "chon";

/** A value the server sent that is not one of the six is shown AS SENT, never relabelled as a guess. */
export function valueTypeLabel(value: string): string {
  return VALUE_TYPES.find((t) => t.value === value)?.label ?? value;
}

/** One option being edited. `locked`: the value came from the server and may not change. */
export type OptionDraft = { readonly value: string; readonly label: string; readonly locked: boolean };

/** The form as typed. Strings throughout — inputs return strings. */
export type MapFieldDraft = {
  readonly assetTypeCode: string;
  readonly fieldCode: string;
  readonly label: string;
  readonly valueType: string;
  readonly options: readonly OptionDraft[];
  readonly isRequired: boolean;
  readonly sortOrder: string;
};

export function newDraft(assetTypeCode: string): MapFieldDraft {
  return {
    assetTypeCode,
    fieldCode: "",
    label: "",
    valueType: "van-ban",
    options: [],
    isRequired: false,
    sortOrder: "",
  };
}

/** Edit draft from a server row — every existing option LOCKED. */
export function editDraft(row: comms_mapFieldSchemaOut): MapFieldDraft {
  return {
    assetTypeCode: row.asset_type_code,
    fieldCode: row.field_code,
    label: row.label,
    valueType: row.value_type,
    options: row.options.map((o) => ({ value: o.value, label: o.label, locked: true })),
    isRequired: row.is_required,
    sortOrder: String(row.sort_order),
  };
}

export function addOption(d: MapFieldDraft): MapFieldDraft {
  return { ...d, options: [...d.options, { value: "", label: "", locked: false }] };
}

/** Change an option's VALUE — a no-op on a locked option. */
export function setOptionValue(d: MapFieldDraft, index: number, value: string): MapFieldDraft {
  const o = d.options[index];
  if (o === undefined || o.locked) return d;
  return { ...d, options: d.options.map((x, i) => (i === index ? { ...x, value } : x)) };
}

/** Relabel an option — allowed on every option, locked or not. */
export function setOptionLabel(d: MapFieldDraft, index: number, label: string): MapFieldDraft {
  if (d.options[index] === undefined) return d;
  return { ...d, options: d.options.map((x, i) => (i === index ? { ...x, label } : x)) };
}

/** Remove an option — a no-op on a locked option (only one added in this form can go). */
export function removeOption(d: MapFieldDraft, index: number): MapFieldDraft {
  const o = d.options[index];
  if (o === undefined || o.locked) return d;
  return { ...d, options: d.options.filter((_, i) => i !== index) };
}

export const SORT_ORDER_ERROR = "Thứ tự phải là một số nguyên, ví dụ: 3.";
export const NO_CHANGE = "Chưa có thay đổi nào để lưu.";

export type Built<T> =
  | { readonly kind: "send"; readonly body: T }
  | { readonly kind: "unchanged" }
  | { readonly kind: "error"; readonly message: string };

/** Blank = not given; an integer = that; anything else a LOCAL error (no `parseInt("3 chữ")`). */
function readSortOrder(s: string): { ok: true; value: number | undefined } | { ok: false } {
  const t = s.trim();
  if (t === "") return { ok: true, value: undefined };
  const n = Number(t);
  return Number.isInteger(n) ? { ok: true, value: n } : { ok: false };
}

function optionsIn(options: readonly OptionDraft[]): comms_fieldOptionIn[] {
  return options.map((o) => ({ value: o.value.trim(), label: o.label }));
}

/**
 * POST body. The code is GENERATED from the label (`fieldCodeFromLabel`) — `d.fieldCode` is the edit
 * draft's read-only copy and is ignored here. The one local check is the one the generation needs: a
 * label with no letter or digit yields no code. Every other shape (label length, at least one option
 * for `chon`) is checked by the SERVER, which names what it refuses; a second copy would drift (rule 9).
 */
export function createBody(d: MapFieldDraft): Built<comms_createMapFieldSchemaIn> {
  const code = fieldCodeFromLabel(d.label);
  if (code === "") return { kind: "error", message: LABEL_NEEDS_ALNUM };
  const order = readSortOrder(d.sortOrder);
  if (!order.ok) return { kind: "error", message: SORT_ORDER_ERROR };
  return {
    kind: "send",
    body: {
      asset_type_code: d.assetTypeCode,
      field_code: code,
      label: d.label.trim(),
      value_type: d.valueType,
      options: d.valueType === CHOICE_TYPE ? optionsIn(d.options) : undefined,
      is_required: d.isRequired,
      sort_order: order.value,
    },
  };
}

/**
 * PATCH body — ONLY what changed against the row. `options`, when it changed, is the WHOLE list:
 * every locked option first in its original order, then the new ones (the server wants the full
 * list, not a delta). A blanked `Thứ tự` means unchanged, not 0.
 */
export function updateBody(row: comms_mapFieldSchemaOut, d: MapFieldDraft): Built<UpdateMapFieldIn> {
  const order = readSortOrder(d.sortOrder);
  if (!order.ok) return { kind: "error", message: SORT_ORDER_ERROR };

  const body: UpdateMapFieldIn = {};
  if (d.label !== row.label) body.label = d.label;
  if (d.isRequired !== row.is_required) body.is_required = d.isRequired;
  if (order.value !== undefined && order.value !== row.sort_order) body.sort_order = order.value;

  if (row.value_type === CHOICE_TYPE) {
    const next = optionsIn(d.options);
    const same =
      next.length === row.options.length &&
      next.every((o, i) => o.value === row.options[i]?.value && o.label === row.options[i]?.label);
    if (!same) body.options = next;
  }

  return Object.keys(body).length === 0 ? { kind: "unchanged" } : { kind: "send", body };
}
