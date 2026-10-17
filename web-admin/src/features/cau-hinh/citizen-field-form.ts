/**
 * Cấu hình › Thuộc tính danh bạ (ADR 0101) — the words and the pure decisions of the field form: the
 * draft, its validation per type, the option list, the order. No JSX, no network, so every rule here has
 * a test that does not need a browser.
 *
 * THE CLIENT CHECKS MIRROR WHAT THE OWNER FIXED (#1, 17/10/2026): a code of 8–12 characters
 * `[a-z][a-z0-9]{7,11}`, drawn at random, immutable; the type immutable; options `{value, label}` where a saved value is never deleted, only
 * retired. The server checks all of it again — a check here only saves a round trip, it protects nothing.
 *
 * LENGTH CAPS ARE THIS SCREEN'S ASSUMPTION (label 100, placeholder 200, option label 100, option value 40,
 * 100 options): ADR 0101 names none. A server with a tighter cap still refuses with its own sentence,
 * which the dialog shows verbatim.
 */

import {
  displayModeOf,
  layoutWidthOf,
  type CitizenField,
  type CitizenFieldCreateBody,
  type CitizenFieldDisplayMode,
  type CitizenFieldLayoutWidth,
  type CitizenFieldNumberKind,
  type CitizenFieldOption,
  type CitizenFieldType,
  type CitizenFieldUpdateBody,
} from "@/lib/api/citizen-fields";

/* ---- wording -------------------------------------------------------------------------------------- */

export const CITIZEN_FIELD_TAB_TITLE = "Thuộc tính danh bạ";
export const CITIZEN_FIELD_DESCRIPTION =
  "Thông tin riêng xã muốn ghi cho từng người dân, ngoài các ô có sẵn. Thứ tự ở đây là thứ tự trên biểu mẫu Thêm / Sửa người dân — kéo thả hoặc dùng nút lên / xuống để đổi.";
export const ADD_FIELD_BUTTON = "Thêm thuộc tính";
export const ADD_FIELD_TITLE = "Thêm thuộc tính";
export const EDIT_FIELD_TITLE = "Sửa thuộc tính";
export const NO_FIELDS = "Xã chưa khai thuộc tính nào. Bấm “Thêm thuộc tính” để bắt đầu.";
export const FIELD_SAVED_TOAST = "Đã lưu thuộc tính.";
export const FIELD_ADDED_TOAST = "Đã thêm thuộc tính.";
export const FIELD_DELETED_TOAST = "Đã xoá thuộc tính";
export const ORDER_SAVED_TOAST = "Đã lưu thứ tự.";
export const ORDER_FAILED_TOAST = "Chưa lưu được thứ tự. Thứ tự cũ được giữ.";
export const DELETE_FIELD_BUTTON = "Xoá thuộc tính";
export const DELETE_REASON_LABEL = "Lý do xoá";
export const DELETE_REASON_REQUIRED = "Nhập lý do xoá.";
export const RETIRE_REASON_MAX = 500;

/** `Xoá thuộc tính “Hộ nghèo”?` — the specific question of the confirm box (spec v2 §7). */
export function deleteFieldTitle(label: string): string {
  return `Xoá thuộc tính “${label}”?`;
}

/**
 * What "Xoá" does (owner, 17/10/2026): only the definition goes; every value typed stays (rule 7) and the
 * code stays reserved, because ZNS / SMS templates map it and a new field reusing it would inherit them.
 */
export function deleteFieldConsequence(code: string): string {
  return `Thuộc tính sẽ không còn hiện ở biểu mẫu, danh sách và bộ lọc danh bạ, cũng không chọn được khi gửi tin ZNS/SMS. Dữ liệu đã nhập cho người dân được giữ nguyên. Mã ${code} sẽ không dùng lại được cho thuộc tính mới.`;
}
export const NO_PERMISSION =
  "Tài khoản của bạn không có quyền cấu hình thuộc tính danh bạ người dân (admin.citizen_field), nên tab này không hiển thị.";
export const OPTIONS_HINT =
  "Mã giá trị đã lưu thì không đổi và không xoá được — chỉ sửa nhãn hoặc ngừng dùng. Người đã chọn giá trị ngừng dùng vẫn giữ giá trị ấy.";

/** `12/50 thuộc tính đang dùng` — the limit said before the click (ADR 0101 #1). */
export function fieldCountText(live: number, limit: number): string {
  return `${live}/${limit} thuộc tính đang dùng`;
}

export function limitReachedText(limit: number): string {
  return `Đã đủ ${limit} thuộc tính đang dùng. Xoá một thuộc tính trước khi thêm thuộc tính mới.`;
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

/** "Kiểu hiển thị" of a choice type, `full` first: the default (owner, 17/10/2026). */
export const DISPLAY_MODE_LABEL = "Kiểu hiển thị";
export const DISPLAY_MODES: readonly { readonly value: CitizenFieldDisplayMode; readonly label: string }[] = [
  { value: "full", label: "Hiện đủ các lựa chọn" },
  { value: "dropdown", label: "Danh sách thả xuống" },
];

/** "Độ rộng trên biểu mẫu" (ADR 0101 #3c): one column of the contact form's 2-column grid, or the whole row. */
export const LAYOUT_WIDTH_LABEL = "Độ rộng trên biểu mẫu";
export const LAYOUT_WIDTH_HINT = "Hai thuộc tính “Nửa hàng” liền nhau nằm trái / phải; màn hình hẹp luôn xếp một cột.";
export const LAYOUT_WIDTHS: readonly { readonly value: CitizenFieldLayoutWidth; readonly label: string }[] = [
  { value: "half", label: "Nửa hàng" },
  { value: "full", label: "Cả hàng" },
];

/* ---- the layout preview (ADR 0101 #3c) ---- */

export const LIST_VIEW = "Danh sách";
export const LAYOUT_VIEW = "Bố cục";
export const LAYOUT_TITLE = "Bố cục biểu mẫu";
export const LAYOUT_STATIC_NOTE = "Các ô cố định (Họ tên, Số điện thoại, …) hiển thị phía trên theo mẫu.";
export const LAYOUT_NARROW_NOTE = "Màn hình hẹp (dưới 640px): biểu mẫu thật cũng xếp mỗi thuộc tính một hàng như dưới đây.";
export const EMPTY_SLOT_HINT = "Ô bên phải còn trống";
export const WIDTH_SAVED_TOAST = "Đã lưu độ rộng.";
export const WIDTH_FAILED_TOAST = "Chưa lưu được độ rộng. Độ rộng cũ được giữ.";

/**
 * The half fields left ALONE on their row, in the order given — what CSS grid's sparse auto-flow does with
 * `half` = 1 column and `full` = 2: a half followed by a full (or by nothing) leaves the right slot empty,
 * because the full one cannot fit beside it and the grid never back-fills. The preview marks that slot.
 */
export function loneHalfCodes(fields: readonly Pick<CitizenField, "code" | "layout_width" | "field_type">[]): ReadonlySet<string> {
  const lone = new Set<string>();
  let open: string | null = null; // a half sitting in the left column, its right slot still free
  for (const f of fields) {
    if (layoutWidthOf(f) === "full") {
      if (open !== null) lone.add(open);
      open = null;
    } else open = open === null ? f.code : null;
  }
  if (open !== null) lone.add(open);
  return lone;
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

/**
 * A NEW code (owner, 17/10/2026): 8–12 characters, a–z and 0–9, starting with a letter. Codes saved under
 * the earlier 6–8 rule stay as they are — the edit dialog shows them read-only and never checks them.
 */
export const FIELD_CODE_PATTERN = /^[a-z][a-z0-9]{7,11}$/;
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
  /** Only read for a type with options; kept across a type change inside the dialog. */
  readonly displayMode: CitizenFieldDisplayMode;
  /** `null` on add until the person picks one: the server then applies the type's default (#3c). */
  readonly layoutWidth: CitizenFieldLayoutWidth | null;
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
    displayMode: "full",
    layoutWidth: null,
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
    displayMode: displayModeOf(f),
    layoutWidth: layoutWidthOf(f),
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
export const CODE_INVALID = "Mã gồm 8–12 ký tự a–z, 0–9, bắt đầu bằng chữ";
export const CODE_TAKEN = "Mã này đã có trong xã";
export const CODE_TAKEN_DELETED = "Mã này đã dùng cho một thuộc tính đã xoá, không dùng lại được";
export const CODE_HINT = "Mã dùng để ghép biến khi gửi tin ZNS/SMS; không đổi được sau khi tạo.";
export const CODE_LOCKED_HINT = "Mã không đổi được sau khi tạo vì dữ liệu đã nhập và biến ZNS/SMS gắn với mã.";
export const CODE_EDIT_BUTTON = "Sửa";
export const CODE_REDRAW = "Sinh mã khác";
/** Said under Mã when the server refused a random code with 409 and a fresh one was drawn in its place. */
export const CODE_REDRAWN_AFTER_CONFLICT = "Mã vừa trùng trong xã, đã sinh mã khác — bấm Lưu lại";

/* ---- the random code (owner, 17/10/2026: "web tự gen random 1 mã có từ 8-12 ký tự, khi lưu thì có check
 * trùng theo tenant/xã và nó chính là thứ dùng để map biến trong các luồng liên quan như sms, zns") ---- */

const CODE_FIRST = "abcdefghijklmnopqrstuvwxyz";
const CODE_REST = "abcdefghijklmnopqrstuvwxyz0123456789";

/**
 * Length of a drawn code: 10, inside the owner's 8–12 (ADR 0101 #1b). service-identity accepts 8–12 for a new
 * code (`citizenFieldCodeShape`, domain/citizen_field.go) and the DB CHECK 6–12 (migration 0037).
 */
export const RANDOM_CODE_LENGTH = 10;

/**
 * One uniformly random character of `alphabet` from the CSPRNG (rule 13: never the non-cryptographic generator). Bytes at or
 * above the largest multiple of the alphabet size are thrown away, so no letter is favoured by the modulo.
 */
function randomChar(alphabet: string): string {
  const limit = 256 - (256 % alphabet.length);
  const buf = new Uint8Array(1);
  for (;;) {
    crypto.getRandomValues(buf);
    if (buf[0]! < limit) return alphabet[buf[0]! % alphabet.length]!;
  }
}

/** A fresh code: a letter, then a–z / 0–9, `length` characters in all. Always matches FIELD_CODE_PATTERN. */
export function randomFieldCode(length: number = RANDOM_CODE_LENGTH): string {
  let s = randomChar(CODE_FIRST);
  while (s.length < length) s += randomChar(CODE_REST);
  return s;
}

/**
 * A random code no field of the commune holds — `taken` is EVERY code, retired ones included (PRIMARY KEY
 * (tenant_id, code) covers both). A collision is redrawn silently; the space (26·36⁷) makes a second draw
 * already improbable, so the cap only guards a broken random source. `null` past it: the dialog then asks
 * for a hand-typed code rather than proposing one it knows is taken.
 */
export function freeRandomFieldCode(taken: ReadonlySet<string>): string | null {
  for (let i = 0; i < 20; i++) {
    const c = randomFieldCode();
    if (!taken.has(c)) return c;
  }
  return null;
}

/**
 * The sentence under a hand-typed code, live: pattern first, then whether the commune already holds it
 * (and says so when the holder was deleted — the screen hides deleted fields, so the person would look
 * for it in the list and not find it).
 */
export function manualCodeError(code: string, liveCodes: ReadonlySet<string>, retiredCodes: ReadonlySet<string>): string | undefined {
  const c = code.trim();
  if (c === "") return CODE_REQUIRED;
  if (!FIELD_CODE_PATTERN.test(c)) return CODE_INVALID;
  if (retiredCodes.has(c)) return CODE_TAKEN_DELETED;
  if (liveCodes.has(c)) return CODE_TAKEN;
  return undefined;
}

/** A collision is the one code error that disables Lưu while typing (the pattern errors still let Lưu show every other problem). */
export function isCodeCollision(error: string | undefined): boolean {
  return error === CODE_TAKEN || error === CODE_TAKEN_DELETED;
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
 * already holds, deleted (retired) fields included — a deleted code is still a code (rule 7, invariant 3).
 * `retiredCodes` (a subset of `takenCodes`) only changes the sentence.
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
    else if (retiredCodes.includes(code)) errors.code = CODE_TAKEN_DELETED;
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
    display_mode: hasOptions(d.fieldType) ? d.displayMode : "",
    // null unless the person picked a width: the server's default then follows the type (#3c).
    layout_width: d.layoutWidth,
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
    display_mode: null,
    layout_width: null,
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
  // Only when it changed: opening and saving a field never rewrites its drawing.
  if (hasOptions(f.field_type) && d.displayMode !== displayModeOf(f)) body.display_mode = d.displayMode;
  if (d.layoutWidth !== null && d.layoutWidth !== layoutWidthOf(f)) body.layout_width = d.layoutWidth;
  return Object.values(body).every((v) => v === null) ? null : body;
}

/** The PATCH of the preview's "Nửa hàng / Cả hàng": the width alone, every other key unchanged. */
export function layoutWidthBody(width: CitizenFieldLayoutWidth): CitizenFieldUpdateBody {
  return {
    code: null,
    label: null,
    field_type: null,
    placeholder: null,
    options: null,
    number_kind: null,
    date_pattern: null,
    required: null,
    show_in_list: null,
    display_mode: null,
    layout_width: width,
  };
}

/* ---- retire ---------------------------------------------------------------------------------------- */

export function checkRetireReason(reason: string): { ok: true; reason: string } | { ok: false; text: string } {
  const r = reason.trim();
  if (r === "") return { ok: false, text: DELETE_REASON_REQUIRED };
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
