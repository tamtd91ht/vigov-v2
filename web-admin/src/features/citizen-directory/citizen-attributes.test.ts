import { describe, expect, it } from "vitest";

import type { CitizenField } from "@/lib/api/citizen-fields";

import {
  CHOICE_INVALID,
  DATE_INVALID,
  DECIMAL_INVALID,
  EMPTY_FILTER_DRAFT,
  INTEGER_INVALID,
  RANGE_REVERSED,
  REQUIRED_TEXT,
  advancedFilterCount,
  attributesFromInputs,
  dateInputKind,
  displayAttribute,
  filtersFromDraft,
  formatDateByPattern,
  inputsFromAttributes,
  millisToPicker,
  parseNumberInput,
  pickerToEndMillis,
  pickerToMillis,
  toggleChoice,
} from "./citizen-attributes";

const F = (over: Partial<CitizenField>): CitizenField => ({
  code: "abcdef",
  label: "X",
  field_type: "singleText",
  placeholder: "",
  options: [],
  number_kind: "",
  date_pattern: "",
  required: false,
  show_in_list: false,
  sort_order: 0,
  created_at: "2026-10-17T00:00:00Z",
  updated_at: "2026-10-17T00:00:00Z",
  retired_at: null,
  display_mode: "",
  layout_width: "half",
  ...over,
});

const OPTIONS = [
  { value: "ho-ngheo", label: "Hộ nghèo", retired: false },
  { value: "can-ngheo", label: "Cận nghèo", retired: false },
  { value: "cu-1", label: "Cũ", retired: true },
];
const TEXT = F({ code: "ghichu01", required: true });
const INT = F({ code: "sonhan01", field_type: "number", number_kind: "integer" });
const DEC = F({ code: "dientich", field_type: "number", number_kind: "decimal" });
const DAY = F({ code: "ngaysinh", field_type: "dateTime", date_pattern: "dd/MM/yyyy" });
const MINUTE = F({ code: "henluc01", field_type: "dateTime", date_pattern: "dd/MM/yyyy HH:mm" });
const MONTH = F({ code: "thangvao", field_type: "dateTime", date_pattern: "MM/yyyy" });
const YEAR = F({ code: "namsinh1", field_type: "dateTime", date_pattern: "yyyy" });
const ONE = F({ code: "hongheo1", field_type: "singleBox", options: OPTIONS });
const MANY = F({ code: "nhucau01", field_type: "combobox", options: [...OPTIONS, { value: "dien", label: "Điện", retired: false }] });

/** 17/10/2026 00:00 in Vietnam = 16/10/2026 17:00 UTC. */
const OCT17_VN = Date.UTC(2026, 9, 16, 17, 0);

describe("dateTime ↔ epoch millis, pinned to Vietnam time (+07:00)", () => {
  it("each pattern picks its picker", () => {
    expect(dateInputKind("dd/MM/yyyy")).toBe("date");
    expect(dateInputKind("dd/MM/yyyy HH:mm")).toBe("datetime-local");
    expect(dateInputKind("MM/yyyy")).toBe("month");
    expect(dateInputKind("yyyy")).toBe("year");
    expect(dateInputKind("")).toBe("date");
  });

  it("picker → millis at the START of the period, whatever the machine's zone", () => {
    expect(pickerToMillis("date", "2026-10-17")).toBe(OCT17_VN);
    expect(pickerToMillis("datetime-local", "2026-10-17T08:30")).toBe(OCT17_VN + 8.5 * 3600_000);
    expect(pickerToMillis("month", "2026-10")).toBe(Date.UTC(2026, 8, 30, 17, 0));
    expect(pickerToMillis("year", "2026")).toBe(Date.UTC(2025, 11, 31, 17, 0));
  });

  it("malformed or impossible → null, never a guessed date", () => {
    expect(pickerToMillis("date", "2026-02-31")).toBeNull();
    expect(pickerToMillis("date", "17/10/2026")).toBeNull();
    expect(pickerToMillis("year", "26")).toBeNull();
    expect(pickerToMillis("date", "")).toBeNull();
  });

  it("round trip millis → picker → millis", () => {
    for (const kind of ["date", "datetime-local", "month", "year"] as const) {
      const ms = pickerToMillis(kind, millisToPicker(kind, OCT17_VN + 8.5 * 3600_000))!;
      expect(millisToPicker(kind, ms)).toBe(millisToPicker(kind, OCT17_VN + 8.5 * 3600_000));
    }
  });

  it("end of period = the last millisecond (inclusive 'đến')", () => {
    expect(pickerToEndMillis("date", "2026-10-17")).toBe(OCT17_VN + 86_400_000 - 1);
    expect(pickerToEndMillis("month", "2026-10")).toBe(Date.UTC(2026, 9, 31, 17, 0) - 1);
    expect(pickerToEndMillis("year", "2026")).toBe(Date.UTC(2026, 11, 31, 17, 0) - 1);
    expect(pickerToEndMillis("datetime-local", "2026-10-17T08:30")).toBe(OCT17_VN + 8.5 * 3600_000 + 59_999);
  });

  it("display per pattern", () => {
    const ms = OCT17_VN + 8.5 * 3600_000;
    expect(formatDateByPattern("dd/MM/yyyy", ms)).toBe("17/10/2026");
    expect(formatDateByPattern("dd/MM/yyyy HH:mm", ms)).toBe("17/10/2026 08:30");
    expect(formatDateByPattern("MM/yyyy", ms)).toBe("10/2026");
    expect(formatDateByPattern("yyyy", ms)).toBe("2026");
  });
});

describe("numbers", () => {
  it("integer: digits only, safe range", () => {
    expect(parseNumberInput("integer", " 12 ")).toBe(12);
    expect(parseNumberInput("integer", "-3")).toBe(-3);
    expect(parseNumberInput("integer", "1,5")).toBeNull();
    expect(parseNumberInput("integer", "99999999999999999")).toBeNull();
  });

  it("decimal: the Vietnamese comma or a point; no thousands separators guessed", () => {
    expect(parseNumberInput("decimal", "12,5")).toBe(12.5);
    expect(parseNumberInput("decimal", "12.5")).toBe(12.5);
    expect(parseNumberInput("decimal", "7")).toBe(7);
    expect(parseNumberInput("decimal", "1.000,5")).toBeNull();
    expect(parseNumberInput("decimal", "abc")).toBeNull();
  });
});

describe("form values → attributes", () => {
  const fields = [TEXT, INT, DEC, DAY, ONE, MANY];

  it("converts each type: text trimmed, number typed, date → millis, choices as values", () => {
    const r = attributesFromInputs(
      fields,
      { ghichu01: " Có ghi chú ", sonhan01: "4", dientich: "12,5", ngaysinh: "2026-10-17", hongheo1: "ho-ngheo", nhucau01: ["dien", "ho-ngheo"] },
      undefined,
    );
    expect(r).toEqual({
      ok: true,
      attributes: { ghichu01: "Có ghi chú", sonhan01: 4, dientich: 12.5, ngaysinh: OCT17_VN, hongheo1: "ho-ngheo", nhucau01: ["dien", "ho-ngheo"] },
    });
  });

  it("errors per control: required, integer, decimal, date, choice no longer offered", () => {
    const r = attributesFromInputs(
      fields,
      { ghichu01: " ", sonhan01: "4,5", dientich: "x", ngaysinh: "2026-02-31", hongheo1: "cu-1", nhucau01: ["cu-1"] },
      undefined,
    );
    expect(r).toEqual({
      ok: false,
      errors: { ghichu01: REQUIRED_TEXT, sonhan01: INTEGER_INVALID, dientich: DECIMAL_INVALID, ngaysinh: DATE_INVALID, hongheo1: CHOICE_INVALID, nhucau01: CHOICE_INVALID },
    });
  });

  it("a retired option the person ALREADY holds is kept; values of retired FIELDS are carried over; an emptied control drops its key", () => {
    const stored = { hongheo1: "cu-1", cucu0001: "giữ nguyên", sonhan01: 3 };
    const r = attributesFromInputs([TEXT, INT, ONE], { ghichu01: "a", sonhan01: "", hongheo1: "cu-1" }, stored);
    expect(r).toEqual({ ok: true, attributes: { cucu0001: "giữ nguyên", ghichu01: "a", hongheo1: "cu-1" } });
  });

  it("stored attributes → control values (edit dialog)", () => {
    expect(
      inputsFromAttributes([INT, DEC, DAY, MANY, ONE, TEXT], { sonhan01: 4, dientich: 12.5, ngaysinh: OCT17_VN, nhucau01: ["dien"], hongheo1: "ho-ngheo" }),
    ).toEqual({ sonhan01: "4", dientich: "12,5", ngaysinh: "2026-10-17", nhucau01: ["dien"], hongheo1: "ho-ngheo", ghichu01: "" });
    expect(inputsFromAttributes([MINUTE, MONTH, YEAR], { henluc01: OCT17_VN + 8.5 * 3600_000, thangvao: OCT17_VN, namsinh1: OCT17_VN })).toEqual({
      henluc01: "2026-10-17T08:30",
      thangvao: "2026-10",
      namsinh1: "2026",
    });
  });

  it("toggleChoice keeps the options' order", () => {
    expect(toggleChoice(MANY, ["dien"], "ho-ngheo")).toEqual(["ho-ngheo", "dien"]);
    expect(toggleChoice(MANY, ["ho-ngheo", "dien"], "ho-ngheo")).toEqual(["dien"]);
  });
});

describe("list cells", () => {
  it("option labels, date per pattern, decimal comma, empty = ''", () => {
    expect(displayAttribute(MANY, ["ho-ngheo", "dien"])).toBe("Hộ nghèo, Điện");
    expect(displayAttribute(ONE, "cu-1")).toBe("Cũ");
    expect(displayAttribute(ONE, "khong-ro")).toBe("khong-ro");
    expect(displayAttribute(DAY, OCT17_VN)).toBe("17/10/2026");
    expect(displayAttribute(DEC, 12.5)).toBe("12,5");
    expect(displayAttribute(TEXT, undefined)).toBe("");
  });
});

describe("filter body building (ADR 0101 #10)", () => {
  const fields = [TEXT, INT, DAY, ONE, MANY];

  it("empty draft → nothing", () => {
    expect(filtersFromDraft(fields, EMPTY_FILTER_DRAFT)).toEqual({ ok: true, createdFrom: null, createdTo: null, filters: [] });
  });

  it("choices → values; ranges → one bound is enough; date 'đến' is the end of the day; text fields are never filters", () => {
    const r = filtersFromDraft(fields, {
      createdFrom: "2026-10-17",
      createdTo: "",
      fields: {
        hongheo1: { values: ["ho-ngheo", "can-ngheo"] },
        nhucau01: { values: [] },
        sonhan01: { from: "", to: "5" },
        ngaysinh: { from: "", to: "2026-10-17" },
        ghichu01: { values: ["x"] },
      },
    });
    expect(r).toEqual({
      ok: true,
      createdFrom: OCT17_VN,
      createdTo: null,
      filters: [
        { code: "sonhan01", values: [], from: null, to: 5 },
        { code: "ngaysinh", values: [], from: null, to: OCT17_VN + 86_400_000 - 1 },
        { code: "hongheo1", values: ["ho-ngheo", "can-ngheo"], from: null, to: null },
      ],
    });
  });

  it("reversed or malformed ranges are refused under their control, nothing is built", () => {
    const r = filtersFromDraft(fields, {
      createdFrom: "2026-10-18",
      createdTo: "2026-10-17",
      fields: { sonhan01: { from: "9", to: "1" }, ngaysinh: { from: "x", to: "" } },
    });
    expect(r).toEqual({ ok: false, errors: { created: RANGE_REVERSED, sonhan01: RANGE_REVERSED, ngaysinh: DATE_INVALID } });
  });

  it("the toggle's figure counts the date range once and each attribute filter", () => {
    expect(advancedFilterCount(null, null, [])).toBe(0);
    expect(advancedFilterCount(1, 2, [{ code: "a", values: ["x"], from: null, to: null }])).toBe(2);
  });
});
