import { describe, expect, it } from "vitest";

import type { CitizenField } from "@/lib/api/citizen-fields";

import {
  CODE_INVALID,
  CODE_REQUIRED,
  CODE_TAKEN,
  LABEL_REQUIRED,
  OPTION_LABEL_REQUIRED,
  OPTION_VALUE_DUPLICATE,
  OPTION_VALUE_INVALID,
  OPTION_VALUE_REQUIRED,
  OPTIONS_NONE,
  PLACEHOLDER_TOO_LONG,
  RETIRE_REASON_REQUIRED,
  addOption,
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
  number_kind: null,
  date_pattern: null,
  required: false,
  show_in_list: true,
  sort_order: 1,
  retired_at: null,
};

const draft = (over: Partial<CitizenFieldDraft>): CitizenFieldDraft => ({ ...newFieldDraft(), code: "abcdef", label: "Tên", ...over });

describe("field code (ADR 0101 #1: 6–8 chars, a–z 0–9, starts with a letter)", () => {
  it.each([
    ["abcdef", undefined],
    ["a1b2c3d4", undefined],
    ["", CODE_REQUIRED],
    ["abcde", CODE_INVALID],
    ["abcdefghi", CODE_INVALID],
    ["1abcdef", CODE_INVALID],
    ["abc-def", CODE_INVALID],
    ["Abcdef", CODE_INVALID],
  ])("%j → %s", (code, error) => {
    expect(validateFieldDraft(draft({ code }), true, []).code).toBe(error);
  });

  it("a code the commune already holds — a retired one included — is taken", () => {
    expect(validateFieldDraft(draft({ code: "hongheo1" }), true, ["hongheo1"]).code).toBe(CODE_TAKEN);
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
  it("choosing a choice type starts with one empty row", () => {
    expect(setFieldType(newFieldDraft(), "combobox").options).toEqual([{ value: "", label: "", retired: false, saved: false }]);
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
      date_pattern: null,
      required: false,
      show_in_list: false,
    });
    expect(buildCreateBody(draft({ fieldType: "dateTime", datePattern: "MM/yyyy" }))).toMatchObject({ date_pattern: "MM/yyyy", number_kind: null });
  });

  it("update: only what changed; nothing changed → null", () => {
    expect(buildUpdateBody(CHOICE, draftFromField(CHOICE))).toBeNull();
    const d = retireOrRemoveOption({ ...draftFromField(CHOICE), label: "Hộ", required: true }, 1);
    expect(buildUpdateBody(CHOICE, d)).toEqual({
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
