/**
 * Cấu hình › Thuộc tính danh bạ (ADR 0101) — the words and the pure decisions of the field form: the
 * draft, its validation per type, the option list, the order. No JSX, no network, so every rule here has
 * a test that does not need a browser.
 *
 * THE CLIENT CHECKS MIRROR WHAT THE OWNER FIXED (#1): a code of 6–8 characters `[a-z][a-z0-9]{5,7}`,
 * immutable; the type immutable; options `{value, label}` where a saved value is never deleted, only
 * retired. The server checks all of it again — a check here only saves a round trip, it protects nothing.
 *
 * LENGTH CAPS ARE THIS SCREEN'S ASSUMPTION (label 100, placeholder 200, option label 100, option value 40,
 * 100 options): ADR 0101 names none. A server with a tighter cap still refuses with its own sentence,
 * which the dialog shows verbatim.
 */

import type {
  CitizenField,
  CitizenFieldCreateBody,
  CitizenFieldNumberKind,
  CitizenFieldOption,
  CitizenFieldType,
  CitizenFieldUpdateBody,
} from "@/lib/api/citizen-fields";

/* ---- wording -------------------------------------------------------------------------------------- */

export const CITIZEN_FIELD_TAB_TITLE = "Thuộc tính danh bạ";
export const CITIZEN_FIELD_DESCRIPTION =
  "Thông tin riêng xã muốn ghi cho từng người dân, ngoài các ô có sẵn. Thứ tự ở đây là thứ tự trên biểu mẫu Thêm / Sửa người dân — kéo thả hoặc dùng nút lên / xuống để đổi.";
export const ADD_FIELD_BUTTON = "Thêm thuộc tính";
export const ADD_FIELD_TITLE = "Thêm thuộc tính";
export const EDIT_FIELD_TITLE = "Sửa thuộc tính";
export const NO_FIELDS = "Xã chưa khai thuộc tính nào. Bấm “Thêm thuộc tính” để bắt đầu.";
export const RETIRED_SECTION = "Đã ngừng dùng";
export const FIELD_SAVED_TOAST = "Đã lưu thuộc tính.";
export const FIELD_ADDED_TOAST = "Đã thêm thuộc tính.";
export const FIELD_RETIRED_TOAST = "Đã ngừng dùng thuộc tính.";
export const ORDER_SAVED_TOAST = "Đã lưu thứ tự.";
export const ORDER_FAILED_TOAST = "Chưa lưu được thứ tự. Thứ tự cũ được giữ.";
export const RETIRE_CONSEQUENCE =
  "Thuộc tính sẽ ẩn khỏi biểu mẫu, cột danh sách và bộ lọc. Giá trị đã nhập cho từng người vẫn được giữ, không xoá.";
export const RETIRE_REASON_LABEL = "Lý do ngừng dùng";
export const RETIRE_REASON_REQUIRED = "Nhập lý do trước khi ngừng dùng.";
export const RETIRE_REASON_MAX = 500;
export const NO_PERMISSION =
  "Tài khoản của bạn không có quyền cấu hình thuộc tính danh bạ người dân (admin.citizen_field), nên tab này không hiển thị.";
export const OPTIONS_HINT =
  "Mã giá trị đã lưu thì không đổi và không xoá được — chỉ sửa nhãn hoặc ngừng dùng. Người đã chọn giá trị ngừng dùng vẫn giữ giá trị ấy.";

/** `12/50 thuộc tính đang dùng` — the limit said before the click (ADR 0101 #1). */
export function fieldCountText(live: number, limit: number): string {
  return `${live}/${limit} thuộc tính đang dùng`;
}

export function limitReachedText(limit: number): string {
  return `Đã đủ ${limit} thuộc tính đang dùng. Ngừng dùng một thuộc tính trước khi thêm thuộc tính mới.`;
}

/* ---- the catalogue of types and patterns ---------------------------------------------------------- */

/** The six types in the owner's order, with the words a commune reads. */
export const FIELD_TYPES: readonly { readonly value: CitizenFieldType; readonly label: string }[] = [
  { value: "singleText", label: "Chữ một dòng" },
  { value: "multiText", label: "Chữ nhiều dòng" },
  { value: "combobox", label: "Chọn nhiều" },
  { value: "singleBox", label: "Chọn một" },
  { value: "number", label: "Số" },
  { value: "dateTime", label: "Ngày giờ" },
];

export function fieldTypeLabel(t: CitizenFieldType): string {
  return FIELD_TYPES.find((x) => x.value === t)?.label ?? t;
}

export const NUMBER_KINDS: readonly { readonly value: CitizenFieldNumberKind; readonly label: string }[] = [
  { value: "integer", label: "Nguyên" },
  { value: "decimal", label: "Thập phân" },
];

/** The four display patterns of a `dateTime` field. The pattern also decides the picker (`citizen-attributes.ts`). */
export const DATE_PATTERNS = ["dd/MM/yyyy", "dd/MM/yyyy HH:mm", "MM/yyyy", "yyyy"] as const;
export type DatePattern = (typeof DATE_PATTERNS)[number];

export function isDatePattern(p: string): p is DatePattern {
  return (DATE_PATTERNS as readonly string[]).includes(p);
}

export function hasOptions(t: CitizenFieldType): boolean {
  return t === "combobox" || t === "singleBox";
}

export function hasPlaceholder(t: CitizenFieldType): boolean {
  return t === "singleText" || t === "multiText";
}

/* ---- limits ---------------------------------------------------------------------------------------- */

/** ADR 0101 #1, verbatim: 6–8 characters, a–z and 0–9, starting with a letter. */
export const FIELD_CODE_PATTERN = /^[a-z][a-z0-9]{5,7}$/;
/** kebab-case: lowercase words of a–z / 0–9 joined by single hyphens (`ho-ngheo`, `can-ngheo`). */
export const OPTION_VALUE_PATTERN = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;
export const LABEL_MAX = 100;
export const PLACEHOLDER_MAX = 200;
export const OPTION_LABEL_MAX = 100;
export const OPTION_VALUE_MAX = 40;
export const OPTIONS_MAX = 100;

/* ---- the draft ------------------------------------------------------------------------------------- */

/** One option row. `saved` = it came from the server: its value is locked and it can only be retired. */
export type OptionDraft = {
  readonly value: string;
  readonly label: string;
  readonly retired: boolean;
  readonly saved: boolean;
};

export type CitizenFieldDraft = {
  readonly code: string;
  readonly label: string;
  readonly fieldType: CitizenFieldType;
  readonly placeholder: string;
  readonly options: readonly OptionDraft[];
  readonly numberKind: CitizenFieldNumberKind;
  readonly datePattern: DatePattern;
  readonly required: boolean;
  readonly showInList: boolean;
};

export function newFieldDraft(): CitizenFieldDraft {
  return {
    code: "",
    label: "",
    fieldType: "singleText",
    placeholder: "",
    options: [],
    numberKind: "integer",
    datePattern: "dd/MM/yyyy",
    required: false,
    showInList: false,
  };
}

export function draftFromField(f: CitizenField): CitizenFieldDraft {
  return {
    code: f.code,
    label: f.label,
    fieldType: f.field_type,
    placeholder: f.placeholder,
    options: f.options.map((o) => ({ value: o.value, label: o.label, retired: o.retired, saved: true })),
    numberKind: f.number_kind === "" ? "integer" : f.number_kind,
    datePattern: isDatePattern(f.date_pattern) ? f.date_pattern : "dd/MM/yyyy",
    required: f.required,
    showInList: f.show_in_list,
  };
}

/** Choosing a type with options on a draft that has none starts it with one empty row to fill. */
export function setFieldType(d: CitizenFieldDraft, t: CitizenFieldType): CitizenFieldDraft {
  const options = hasOptions(t) && d.options.length === 0 ? [emptyOption()] : d.options;
  return { ...d, fieldType: t, options };
}

/* ---- options --------------------------------------------------------------------------------------- */

function emptyOption(): OptionDraft {
  return { value: "", label: "", retired: false, saved: false };
}

export function addOption(d: CitizenFieldDraft): CitizenFieldDraft {
  return { ...d, options: [...d.options, emptyOption()] };
}

/** A saved option's value is the stored key on every person who chose it: it never changes (#1). */
export function setOptionValue(d: CitizenFieldDraft, i: number, value: string): CitizenFieldDraft {
  return { ...d, options: d.options.map((o, k) => (k === i && !o.saved ? { ...o, value } : o)) };
}

export function setOptionLabel(d: CitizenFieldDraft, i: number, label: string): CitizenFieldDraft {
  return { ...d, options: d.options.map((o, k) => (k === i ? { ...o, label } : o)) };
}

/**
 * "Ngừng dùng" on a saved option marks it retired (kept, hidden from new choices); "Bỏ" on an option
 * typed in this dialog and never saved simply removes the row — nothing stores it yet.
 */
export function retireOrRemoveOption(d: CitizenFieldDraft, i: number): CitizenFieldDraft {
  const o = d.options[i];
  if (o === undefined) return d;
  if (o.saved) return { ...d, options: d.options.map((x, k) => (k === i ? { ...x, retired: true } : x)) };
  return { ...d, options: d.options.filter((_, k) => k !== i) };
}

/** "Dùng lại" — a retired saved option back in use. */
export function restoreOption(d: CitizenFieldDraft, i: number): CitizenFieldDraft {
  return { ...d, options: d.options.map((o, k) => (k === i ? { ...o, retired: false } : o)) };
}

/* ---- validation ------------------------------------------------------------------------------------ */

export type FieldFormErrors = Partial<Record<"code" | "label" | "placeholder" | "options", string>> & {
  /** Per option row, by index: the sentence under that row. */
  readonly optionRows?: Readonly<Record<number, string>>;
};

export const CODE_REQUIRED = "Nhập mã thuộc tính.";
export const CODE_INVALID = "Mã gồm 6–8 ký tự a–z, 0–9 và bắt đầu bằng chữ cái, ví dụ: hongheo1.";
export const CODE_TAKEN = "Mã này đã có trong xã.";
export const CODE_TAKEN_RETIRED = "Mã này đã có trong xã (đã ngừng dùng).";
export const CODE_AUTO_HINT = "Tự sinh từ tên hiển thị. Không đổi được sau khi tạo.";
export const CODE_MANUAL_HINT = "6–8 ký tự a–z, 0–9, bắt đầu bằng chữ cái. Không đổi được sau khi tạo.";
export const CODE_LOCKED_HINT = "Mã không đổi được sau khi tạo vì dữ liệu đã nhập gắn với mã.";
export const CODE_EDIT_BUTTON = "Sửa";
export const CODE_REGENERATE = "Tự sinh lại";

/** What the dialog says when the server took the auto code meanwhile and the next free one was put in. */
export function codeRegeneratedText(taken: string, next: string): string {
  return `Mã ${taken} vừa được dùng trong xã. Đã đổi sang mã ${next} — bấm Lưu để thử lại.`;
}

/* ---- the generated code (owner, 17/10/2026: "mã thuộc tính hãy tự gen, cho phép sửa, chống trùng trong xã") ---- */

/** Digits appended to a folded label shorter than 6: `ho` → `ho2345`. Starts at 2 so `…1` never reads as `…l`. */
const PAD_DIGITS = "234567890";
const CODE_MIN = 6;
const CODE_MAX = 8;

/** `Hộ nghèo 2026` → `hongheo2026`: diacritics folded (đ → d), lower case, only a–z / 0–9 kept. */
export function foldLabel(label: string): string {
  return label
    .normalize("NFD")
    .replace(/[̀-ͯ]/g, "")
    .replace(/[đĐ]/g, "d")
    .toLowerCase()
    .replace(/[^a-z0-9]/g, "");
}

/**
 * The code a label suggests, before any collision: folded, `tt` in front when it would not start with a
 * letter (or is empty), cut to 8, padded with digits to 6. Always matches FIELD_CODE_PATTERN.
 */
export function baseCodeFromLabel(label: string): string {
  let s = foldLabel(label);
  if (!/^[a-z]/.test(s)) s = `tt${s}`;
  s = s.slice(0, CODE_MAX);
  for (let i = 0; s.length < CODE_MIN; i++) s += PAD_DIGITS[i % PAD_DIGITS.length];
  return s;
}

/**
 * The first code for `label` that no field of the commune holds — `taken` is EVERY code, retired ones
 * included: PRIMARY KEY (tenant_id, code) covers both, so a retired code proposed here is a 409 later.
 * On collision the tail becomes a counter at the base's length (`hongheo` → `hongh01` → `hongh02`), 2
 * digits, then 3 once 99 are gone. `null` only when all 999 are taken — the dialog then asks for a hand-typed code.
 */
export function generateFieldCode(label: string, taken: ReadonlySet<string>): string | null {
  const base = baseCodeFromLabel(label);
  if (!taken.has(base)) return base;
  for (let n = 1; n <= 999; n++) {
    const tail = String(n).padStart(n < 100 ? 2 : 3, "0");
    const candidate = base.slice(0, base.length - tail.length) + tail;
    if (!taken.has(candidate)) return candidate;
  }
  return null;
}

/**
 * The sentence under a hand-typed code, live: pattern first, then whether the commune already holds it
 * (and says so when the holder is retired — the person may look for it in the list and not find it).
 */
export function manualCodeError(code: string, liveCodes: ReadonlySet<string>, retiredCodes: ReadonlySet<string>): string | undefined {
  const c = code.trim();
  if (c === "") return CODE_REQUIRED;
  if (!FIELD_CODE_PATTERN.test(c)) return CODE_INVALID;
  if (retiredCodes.has(c)) return CODE_TAKEN_RETIRED;
  if (liveCodes.has(c)) return CODE_TAKEN;
  return undefined;
}

/** A collision is the one code error that disables Lưu while typing (the pattern errors still let Lưu show every other problem). */
export function isCodeCollision(error: string | undefined): boolean {
  return error === CODE_TAKEN || error === CODE_TAKEN_RETIRED;
}
export const LABEL_REQUIRED = "Nhập tên hiển thị.";
export const LABEL_TOO_LONG = `Tên hiển thị tối đa ${LABEL_MAX} ký tự.`;
export const PLACEHOLDER_TOO_LONG = `Gợi ý nhập tối đa ${PLACEHOLDER_MAX} ký tự.`;
export const OPTIONS_NONE = "Cần ít nhất một lựa chọn đang dùng.";
export const OPTIONS_TOO_MANY = `Tối đa ${OPTIONS_MAX} lựa chọn.`;
export const OPTION_VALUE_REQUIRED = "Nhập mã giá trị.";
export const OPTION_VALUE_INVALID = "Mã giá trị chỉ gồm a–z, 0–9 nối bằng dấu gạch ngang, ví dụ: ho-ngheo.";
export const OPTION_VALUE_TOO_LONG = `Mã giá trị tối đa ${OPTION_VALUE_MAX} ký tự.`;
export const OPTION_VALUE_DUPLICATE = "Mã giá trị bị trùng.";
export const OPTION_LABEL_REQUIRED = "Nhập nhãn.";
export const OPTION_LABEL_TOO_LONG = `Nhãn tối đa ${OPTION_LABEL_MAX} ký tự.`;

/** Characters as a person counts them (an emoji or a combined Vietnamese letter is one), like the server's rune count. */
function charCount(s: string): number {
  return [...s].length;
}

/**
 * Every problem of the draft, keyed by the control it belongs under. `takenCodes` are the codes the commune
 * already holds, retired fields included — a retired code is still a code (rule 7, invariant 3).
 * `retiredCodes` (a subset of `takenCodes`) only changes the sentence: "(đã ngừng dùng)".
 */
export function validateFieldDraft(
  d: CitizenFieldDraft,
  creating: boolean,
  takenCodes: readonly string[],
  retiredCodes: readonly string[] = [],
): FieldFormErrors {
  const errors: { -readonly [K in keyof FieldFormErrors]: FieldFormErrors[K] } = {};
  if (creating) {
    const code = d.code.trim();
    if (code === "") errors.code = CODE_REQUIRED;
    else if (!FIELD_CODE_PATTERN.test(code)) errors.code = CODE_INVALID;
    else if (retiredCodes.includes(code)) errors.code = CODE_TAKEN_RETIRED;
    else if (takenCodes.includes(code)) errors.code = CODE_TAKEN;
  }
  const label = d.label.trim();
  if (label === "") errors.label = LABEL_REQUIRED;
  else if (charCount(label) > LABEL_MAX) errors.label = LABEL_TOO_LONG;
  if (hasPlaceholder(d.fieldType) && charCount(d.placeholder.trim()) > PLACEHOLDER_MAX) errors.placeholder = PLACEHOLDER_TOO_LONG;

  if (hasOptions(d.fieldType)) {
    const rows: Record<number, string> = {};
    const seen = new Set<string>();
    d.options.forEach((o, i) => {
      const value = o.value.trim();
      const optLabel = o.label.trim();
      if (!o.saved) {
        if (value === "") rows[i] = OPTION_VALUE_REQUIRED;
        else if (charCount(value) > OPTION_VALUE_MAX) rows[i] = OPTION_VALUE_TOO_LONG;
        else if (!OPTION_VALUE_PATTERN.test(value)) rows[i] = OPTION_VALUE_INVALID;
      }
      if (rows[i] === undefined && seen.has(value)) rows[i] = OPTION_VALUE_DUPLICATE;
      seen.add(value);
      if (rows[i] === undefined && optLabel === "") rows[i] = OPTION_LABEL_REQUIRED;
      else if (rows[i] === undefined && charCount(optLabel) > OPTION_LABEL_MAX) rows[i] = OPTION_LABEL_TOO_LONG;
    });
    if (Object.keys(rows).length > 0) errors.optionRows = rows;
    if (d.options.length > OPTIONS_MAX) errors.options = OPTIONS_TOO_MANY;
    else if (!d.options.some((o) => !o.retired)) errors.options = OPTIONS_NONE;
  }
  return errors;
}

export function hasErrors(e: FieldFormErrors): boolean {
  return Object.keys(e).length > 0;
}

/* ---- bodies ---------------------------------------------------------------------------------------- */

function optionsOut(d: CitizenFieldDraft): CitizenFieldOption[] {
  return d.options.map((o) => ({ value: o.value.trim(), label: o.label.trim(), retired: o.retired }));
}

/**
 * The POST body. What does not apply to the type is sent EMPTY, never left over from a type chosen and
 * abandoned in the same dialog: a `number` field with options typed under `combobox` is not a combobox.
 */
export function buildCreateBody(d: CitizenFieldDraft): CitizenFieldCreateBody {
  return {
    code: d.code.trim(),
    label: d.label.trim(),
    field_type: d.fieldType,
    placeholder: hasPlaceholder(d.fieldType) ? d.placeholder.trim() : "",
    options: hasOptions(d.fieldType) ? optionsOut(d) : [],
    number_kind: d.fieldType === "number" ? d.numberKind : "",
    date_pattern: d.fieldType === "dateTime" ? d.datePattern : "",
    required: d.required,
    show_in_list: d.showInList,
  };
}

function sameOptions(a: readonly CitizenFieldOption[], b: readonly CitizenFieldOption[]): boolean {
  return a.length === b.length && a.every((o, i) => o.value === b[i]!.value && o.label === b[i]!.label && o.retired === b[i]!.retired);
}

/**
 * The PATCH body — every key the contract names, `null` for what did not change (the immutable `code`,
 * `field_type` and `number_kind` always `null`); `null` as a whole when nothing did (Lưu then just
 * closes, nothing is written).
 */
export function buildUpdateBody(f: CitizenField, d: CitizenFieldDraft): CitizenFieldUpdateBody | null {
  const body: CitizenFieldUpdateBody = {
    code: null,
    label: null,
    field_type: null,
    placeholder: null,
    options: null,
    number_kind: null,
    date_pattern: null,
    required: null,
    show_in_list: null,
  };
  const label = d.label.trim();
  if (label !== f.label) body.label = label;
  if (hasPlaceholder(f.field_type) && d.placeholder.trim() !== f.placeholder) body.placeholder = d.placeholder.trim();
  if (hasOptions(f.field_type)) {
    const options = optionsOut(d);
    if (!sameOptions(options, f.options)) body.options = options;
  }
  if (f.field_type === "dateTime" && d.datePattern !== f.date_pattern) body.date_pattern = d.datePattern;
  if (d.required !== f.required) body.required = d.required;
  if (d.showInList !== f.show_in_list) body.show_in_list = d.showInList;
  return Object.values(body).every((v) => v === null) ? null : body;
}

/* ---- retire ---------------------------------------------------------------------------------------- */

export function checkRetireReason(reason: string): { ok: true; reason: string } | { ok: false; text: string } {
  const r = reason.trim();
  if (r === "") return { ok: false, text: RETIRE_REASON_REQUIRED };
  if (charCount(r) > RETIRE_REASON_MAX) return { ok: false, text: `Lý do tối đa ${RETIRE_REASON_MAX} ký tự.` };
  return { ok: true, reason: r };
}

/* ---- order ----------------------------------------------------------------------------------------- */

/** Live fields in the order the form shows them; retired ones apart. Ties keep the server's order. */
export function splitFields(items: readonly CitizenField[]): { live: CitizenField[]; retired: CitizenField[] } {
  const sorted = [...items].sort((a, b) => a.sort_order - b.sort_order);
  return { live: sorted.filter((f) => f.retired_at === null), retired: sorted.filter((f) => f.retired_at !== null) };
}

/** `codes` with the item at `from` moved to `to`. Out of range or no move → the same list (no write follows). */
export function moveCode(codes: readonly string[], from: number, to: number): readonly string[] {
  if (from === to || from < 0 || to < 0 || from >= codes.length || to >= codes.length) return codes;
  const next = [...codes];
  const [item] = next.splice(from, 1);
  next.splice(to, 0, item!);
  return next;
}

/** The order after a drag of `active` dropped on `over` — the dragged row takes the target's place. */
export function orderAfterDrop(codes: readonly string[], active: string, over: string | null): readonly string[] {
  if (over === null) return codes;
  return moveCode(codes, codes.indexOf(active), codes.indexOf(over));
}

/** `Thuộc tính Hộ nghèo, vị trí 2 trên 5` — what a screen reader hears on a row's handle. */
export function positionText(label: string, index: number, total: number): string {
  return `Thuộc tính ${label}, vị trí ${index + 1} trên ${total}`;
}
