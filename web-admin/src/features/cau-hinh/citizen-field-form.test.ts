import { afterEach, describe, expect, it, vi } from "vitest";

import type { CitizenField } from "@/lib/api/citizen-fields";

import {
  CODE_INVALID,
  CODE_REQUIRED,
  CODE_TAKEN,
  CODE_TAKEN_RETIRED,
  FIELD_CODE_PATTERN,
  LABEL_REQUIRED,
  OPTION_LABEL_REQUIRED,
  OPTION_VALUE_DUPLICATE,
  OPTION_VALUE_INVALID,
  OPTION_VALUE_REQUIRED,
  OPTIONS_NONE,
  PLACEHOLDER_TOO_LONG,
  RETIRE_REASON_REQUIRED,
  RANDOM_CODE_LENGTH,
  addOption,
  freeRandomFieldCode,
  randomFieldCode,
  isCodeCollision,
  manualCodeError,
  buildCreateBody,
  buildUpdateBody,
  checkRetireReason,
  draftFromField,
  fieldTypeLabel,
  moveCode,
  newFieldDraft,
  orderAfterDrop,
  restoreOption,
  retireOrRemoveOption,
  setFieldType,
  setOptionLabel,
  setOptionValue,
  splitFields,
  validateFieldDraft,
  type CitizenFieldDraft,
} from "./citizen-field-form";

const CHOICE: CitizenField = {
  code: "hongheo1",
  label: "Phân loại hộ",
  field_type: "singleBox",
  placeholder: "",
  options: [
    { value: "ho-ngheo", label: "Hộ nghèo", retired: false },
    { value: "can-ngheo", label: "Cận nghèo", retired: false },
  ],
  number_kind: "",
  date_pattern: "",
  required: false,
  show_in_list: true,
  sort_order: 1,
  created_at: "2026-10-17T00:00:00Z",
  updated_at: "2026-10-17T00:00:00Z",
  retired_at: null,
};

const draft = (over: Partial<CitizenFieldDraft>): CitizenFieldDraft => ({ ...newFieldDraft(), code: "abcdefgh", label: "Tên", ...over });

describe("new field code (owner 17/10/2026: 8–12 chars, a–z 0–9, starts with a letter)", () => {
  it.each([
    ["abcdefgh", undefined],
    ["a1b2c3d4", undefined],
    ["a1b2c3d4e5f6", undefined],
    ["", CODE_REQUIRED],
    ["abcdefg", CODE_INVALID],
    ["abcdefghijklm", CODE_INVALID],
    ["1abcdefgh", CODE_INVALID],
    ["abcd-efgh", CODE_INVALID],
    ["Abcdefgh", CODE_INVALID],
  ])("%j → %s", (code, error) => {
    expect(validateFieldDraft(draft({ code }), true, []).code).toBe(error);
  });

  it("a code the commune already holds — a retired one included — is taken", () => {
    expect(validateFieldDraft(draft({ code: "hongheo1" }), true, ["hongheo1"]).code).toBe(CODE_TAKEN);
    expect(validateFieldDraft(draft({ code: "cucu0001" }), true, ["hongheo1", "cucu0001"], ["cucu0001"]).code).toBe(CODE_TAKEN_RETIRED);
  });

  it("on edit the code is not checked at all (it cannot change)", () => {
    expect(validateFieldDraft(draft({ code: "" }), false, []).code).toBeUndefined();
  });
});

describe("validation per type", () => {
  it("label required on every type", () => {
    expect(validateFieldDraft(draft({ label: "  " }), true, []).label).toBe(LABEL_REQUIRED);
  });

  it("placeholder only counts for singleText / multiText", () => {
    const long = "x".repeat(201);
    expect(validateFieldDraft(draft({ fieldType: "multiText", placeholder: long }), true, []).placeholder).toBe(PLACEHOLDER_TOO_LONG);
    expect(validateFieldDraft(draft({ fieldType: "number", placeholder: long }), true, []).placeholder).toBeUndefined();
  });

  it("combobox / singleBox need ≥1 live option; each new value is kebab-case, unique, labelled", () => {
    const d = draft({
      fieldType: "combobox",
      options: [
        { value: "", label: "A", retired: false, saved: false },
        { value: "Ho Ngheo", label: "B", retired: false, saved: false },
        { value: "ho-ngheo", label: "C", retired: false, saved: false },
        { value: "ho-ngheo", label: "D", retired: false, saved: false },
        { value: "can-ngheo", label: "", retired: false, saved: false },
      ],
    });
    expect(validateFieldDraft(d, true, []).optionRows).toEqual({
      0: OPTION_VALUE_REQUIRED,
      1: OPTION_VALUE_INVALID,
      3: OPTION_VALUE_DUPLICATE,
      4: OPTION_LABEL_REQUIRED,
    });
    const allRetired = draft({ fieldType: "singleBox", options: [{ value: "a", label: "A", retired: true, saved: true }] });
    expect(validateFieldDraft(allRetired, false, []).options).toBe(OPTIONS_NONE);
    expect(validateFieldDraft(draft({ fieldType: "singleBox", options: [] }), true, []).options).toBe(OPTIONS_NONE);
  });

  it("a clean number / dateTime / text draft has no error", () => {
    for (const fieldType of ["singleText", "multiText", "number", "dateTime"] as const) {
      expect(validateFieldDraft(draft({ fieldType }), true, [])).toEqual({});
    }
  });
});

describe("options — add, relabel, retire, never delete a saved value", () => {
  it.each(["combobox", "singleBox"] as const)("choosing %s starts with one empty row; with none left Lưu is refused", (t) => {
    const d = setFieldType(newFieldDraft(), t);
    expect(d.options).toEqual([{ value: "", label: "", retired: false, saved: false }]);
    expect(validateFieldDraft({ ...d, code: "abcdefgh", label: "Tên" }, true, []).optionRows).toEqual({ 0: OPTION_VALUE_REQUIRED });
    expect(validateFieldDraft({ ...retireOrRemoveOption(d, 0), code: "abcdefgh", label: "Tên" }, true, []).options).toBe(OPTIONS_NONE);
  });

  it("a saved value is locked; its label changes; it retires instead of disappearing, and comes back", () => {
    let d = draftFromField(CHOICE);
    d = setOptionValue(d, 0, "doi-ma");
    expect(d.options[0]!.value).toBe("ho-ngheo");
    d = setOptionLabel(d, 0, "Hộ nghèo (2026)");
    expect(d.options[0]!.label).toBe("Hộ nghèo (2026)");
    d = retireOrRemoveOption(d, 1);
    expect(d.options).toHaveLength(2);
    expect(d.options[1]).toEqual({ value: "can-ngheo", label: "Cận nghèo", retired: true, saved: true });
    d = restoreOption(d, 1);
    expect(d.options[1]!.retired).toBe(false);
  });

  it("a new row typed in this dialog can be edited and removed outright", () => {
    let d = addOption(draftFromField(CHOICE));
    d = setOptionValue(d, 2, "khac");
    expect(d.options[2]!.value).toBe("khac");
    d = retireOrRemoveOption(d, 2);
    expect(d.options).toHaveLength(2);
  });
});

describe("bodies", () => {
  it("create: what does not belong to the type is sent empty", () => {
    const d = setFieldType(draft({ code: "sonhan01", label: " Số nhân khẩu ", placeholder: "bỏ đi", numberKind: "decimal" }), "number");
    expect(buildCreateBody(d)).toEqual({
      code: "sonhan01",
      label: "Số nhân khẩu",
      field_type: "number",
      placeholder: "",
      options: [],
      number_kind: "decimal",
      date_pattern: "",
      required: false,
      show_in_list: false,
    });
    expect(buildCreateBody(draft({ fieldType: "dateTime", datePattern: "MM/yyyy" }))).toMatchObject({ date_pattern: "MM/yyyy", number_kind: "" });
  });

  it("update: only what changed, every other key null; nothing changed → null", () => {
    expect(buildUpdateBody(CHOICE, draftFromField(CHOICE))).toBeNull();
    const d = retireOrRemoveOption({ ...draftFromField(CHOICE), label: "Hộ", required: true }, 1);
    expect(buildUpdateBody(CHOICE, d)).toEqual({
      code: null,
      field_type: null,
      placeholder: null,
      number_kind: null,
      date_pattern: null,
      show_in_list: null,
      label: "Hộ",
      required: true,
      options: [
        { value: "ho-ngheo", label: "Hộ nghèo", retired: false },
        { value: "can-ngheo", label: "Cận nghèo", retired: true },
      ],
    });
  });

  it("retire reason is required", () => {
    expect(checkRetireReason("  ")).toEqual({ ok: false, text: RETIRE_REASON_REQUIRED });
    expect(checkRetireReason(" Thôi dùng ")).toEqual({ ok: true, reason: "Thôi dùng" });
  });
});

describe("order", () => {
  it("live by sort_order, retired apart", () => {
    const a = { ...CHOICE, code: "aaaaaa", sort_order: 2 };
    const b = { ...CHOICE, code: "bbbbbb", sort_order: 1 };
    const r = { ...CHOICE, code: "rrrrrr", sort_order: 0, retired_at: "2026-10-17T00:00:00Z" };
    const s = splitFields([a, r, b]);
    expect(s.live.map((f) => f.code)).toEqual(["bbbbbb", "aaaaaa"]);
    expect(s.retired.map((f) => f.code)).toEqual(["rrrrrr"]);
  });

  it("moveCode (the up / down buttons): one step; out of range returns the SAME list (no write)", () => {
    const codes = ["a", "b", "c"];
    expect(moveCode(codes, 2, 1)).toEqual(["a", "c", "b"]);
    expect(moveCode(codes, 0, 1)).toEqual(["b", "a", "c"]);
    expect(moveCode(codes, 0, -1)).toBe(codes);
    expect(moveCode(codes, 2, 3)).toBe(codes);
  });

  it("orderAfterDrop (drag): the dragged row takes the target's place; dropped nowhere or on itself → same list", () => {
    const codes = ["a", "b", "c", "d"];
    expect(orderAfterDrop(codes, "a", "c")).toEqual(["b", "c", "a", "d"]);
    expect(orderAfterDrop(codes, "d", "a")).toEqual(["d", "a", "b", "c"]);
    expect(orderAfterDrop(codes, "b", null)).toBe(codes);
    expect(orderAfterDrop(codes, "b", "b")).toBe(codes);
  });

  it("type labels are the owner's words", () => {
    expect(fieldTypeLabel("combobox")).toBe("Chọn nhiều");
    expect(fieldTypeLabel("singleBox")).toBe("Chọn một");
  });
});

/** Feeds `getRandomValues` the given bytes, in order, across calls (each call fills the whole buffer). */
function feedBytes(bytes: number[]) {
  let k = 0;
  return vi.spyOn(crypto, "getRandomValues").mockImplementation(<T extends ArrayBufferView | null>(buf: T): T => {
    const a = buf as unknown as Uint8Array;
    for (let i = 0; i < a.length; i++) a[i] = bytes[k++ % bytes.length]!;
    return buf;
  });
}

describe("random code (owner 17/10/2026: web tự gen random, chống trùng theo xã)", () => {
  afterEach(() => vi.restoreAllMocks());

  it("length, charset and first letter hold over many draws", () => {
    for (let i = 0; i < 500; i++) {
      const c = randomFieldCode();
      expect(c).toHaveLength(RANDOM_CODE_LENGTH);
      expect(c).toMatch(/^[a-z][a-z0-9]+$/);
      expect(FIELD_CODE_PATTERN.test(c)).toBe(true);
    }
    expect(randomFieldCode(12)).toHaveLength(12);
  });

  it("draws from crypto.getRandomValues, never Math.random; biased bytes are thrown away", () => {
    const math = vi.spyOn(Math, "random");
    // 255 ≥ 256-(256%26)=234 and ≥ 256-(256%36)=252: rejected. 0 → 'a', 25 → 'z', 26 → '0' (rest alphabet).
    const spy = feedBytes([255, 0, 25, 26, 35, 1, 2, 3, 4]);
    const c = randomFieldCode(8);
    expect(spy).toHaveBeenCalled();
    expect(math).not.toHaveBeenCalled();
    expect(c[0]).toBe("a");
    expect(c.slice(1, 4)).toBe("z09");
  });

  it("a code the commune holds (live or retired) is redrawn silently", () => {
    // First draw: all bytes 0 → "aaaaaaaa"; second draw: all bytes 1 → "bbbbbbbb".
    let call = 0;
    vi.spyOn(crypto, "getRandomValues").mockImplementation(<T extends ArrayBufferView | null>(buf: T): T => {
      const a = buf as unknown as Uint8Array;
      a.fill(call++ < RANDOM_CODE_LENGTH ? 0 : 1);
      return buf;
    });
    expect(freeRandomFieldCode(new Set(["aaaaaaaa"]))).toBe("bbbbbbbb");
  });

  it("a source that only ever repeats a taken code gives null (the dialog then asks for a typed code)", () => {
    feedBytes([0]);
    expect(freeRandomFieldCode(new Set(["aaaaaaaa"]))).toBeNull();
  });

  it("manual code: pattern 8–12, then live collision, then retired collision with its own sentence", () => {
    const live = new Set(["hongheo1"]);
    const retired = new Set(["cucu0001"]);
    expect(manualCodeError("", live, retired)).toBe(CODE_REQUIRED);
    expect(manualCodeError("1abcdefg", live, retired)).toBe(CODE_INVALID);
    expect(manualCodeError("abcdefg", live, retired)).toBe(CODE_INVALID);
    expect(manualCodeError("abcdefghijklm", live, retired)).toBe(CODE_INVALID);
    expect(manualCodeError("hongheo1", live, retired)).toBe(CODE_TAKEN);
    expect(manualCodeError("cucu0001", live, retired)).toBe(CODE_TAKEN_RETIRED);
    expect(manualCodeError("moi123456789", live, retired)).toBeUndefined();
    expect(isCodeCollision(CODE_TAKEN)).toBe(true);
    expect(isCodeCollision(CODE_TAKEN_RETIRED)).toBe(true);
    expect(isCodeCollision(CODE_INVALID)).toBe(false);
    expect(isCodeCollision(undefined)).toBe(false);
  });
});
