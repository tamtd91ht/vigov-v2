// @vitest-environment jsdom
//
// `display_mode` of a choice field on the contact form (owner, 17/10/2026): `full` (or missing / "") keeps the
// checkbox / radio group; `dropdown` draws a native <select> for singleBox and a popover of checkboxes for
// combobox. The value handed back is the same in both modes.

import { act, useState } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it } from "vitest";

import type { CitizenField } from "@/lib/api/citizen-fields";

import { attributeInputId, CitizenAttributeInputs, NOTHING_CHOSEN } from "./citizen-attribute-inputs";
import type { AttributeInput, AttributeInputs } from "./citizen-attributes";

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  // Radix Popper measures its anchor; jsdom has no ResizeObserver.
  if (!("ResizeObserver" in globalThis)) {
    (globalThis as unknown as { ResizeObserver: unknown }).ResizeObserver = class {
      observe() {}
      unobserve() {}
      disconnect() {}
    };
  }
});

const base = (over: Partial<CitizenField>): CitizenField => ({
  code: "abcdefgh",
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

const MULTI = (mode?: CitizenField["display_mode"]) =>
  base({
    code: "nhucau01",
    label: "Nhu cầu hỗ trợ",
    field_type: "combobox",
    options: [
      { value: "dien", label: "Điện", retired: false },
      { value: "nuoc", label: "Nước", retired: false },
      { value: "cu-1", label: "Cũ", retired: true },
    ],
    ...(mode === undefined ? {} : { display_mode: mode }),
  });
const SINGLE = (mode?: CitizenField["display_mode"]) =>
  base({
    code: "hongheo1",
    label: "Phân loại hộ",
    field_type: "singleBox",
    options: [
      { value: "ho-ngheo", label: "Hộ nghèo", retired: false },
      { value: "can-ngheo", label: "Cận nghèo", retired: false },
    ],
    ...(mode === undefined ? {} : { display_mode: mode }),
  });

let root: Root | null = null;
let host: HTMLDivElement | null = null;
let last: AttributeInputs = {};

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  last = {};
});

function Harness({ fields, initial }: { fields: CitizenField[]; initial: AttributeInputs }) {
  const [inputs, setInputs] = useState<AttributeInputs>(initial);
  return (
    <CitizenAttributeInputs
      fields={fields}
      inputs={inputs}
      stored={undefined}
      errors={{}}
      onChange={(code: string, v: AttributeInput) => {
        const next = { ...inputs, [code]: v };
        last = next;
        setInputs(next);
      }}
    />
  );
}

function mount(fields: CitizenField[], initial: AttributeInputs = {}): HTMLDivElement {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() => r.render(<Harness fields={fields} initial={initial} />));
  return host;
}

describe("display_mode full — today's groups", () => {
  it.each([[""], ["full"]] as const)("display_mode %s: combobox = checkboxes, singleBox = radios", (mode) => {
    const el = mount([MULTI(mode), SINGLE(mode)]);
    expect(el.querySelectorAll("fieldset[data-attribute='nhucau01'] input[type=checkbox]")).toHaveLength(2);
    expect(el.querySelectorAll("fieldset[data-attribute='hongheo1'] input[type=radio]")).toHaveLength(2);
    expect(el.querySelector("select")).toBeNull();
    expect(el.querySelector("button[aria-haspopup]")).toBeNull();
  });
});

describe("display_mode dropdown", () => {
  it("singleBox: a native select with an empty first option; choosing hands back the value, the empty one clears", () => {
    const el = mount([SINGLE("dropdown")]);
    const sel = el.querySelector<HTMLSelectElement>(`select#${attributeInputId("hongheo1")}`)!;
    expect(sel).not.toBeNull();
    expect(el.querySelector("input[type=radio]")).toBeNull();
    const opts = Array.from(sel.options).map((o) => [o.value, o.textContent]);
    expect(opts).toEqual([
      ["", NOTHING_CHOSEN],
      ["ho-ngheo", "Hộ nghèo"],
      ["can-ngheo", "Cận nghèo"],
    ]);
    // The label names the select.
    expect(el.querySelector(`label[for='${attributeInputId("hongheo1")}']`)!.textContent).toContain("Phân loại hộ");
    const setter = Object.getOwnPropertyDescriptor(HTMLSelectElement.prototype, "value")!.set!;
    act(() => {
      setter.call(sel, "can-ngheo");
      sel.dispatchEvent(new Event("change", { bubbles: true }));
    });
    expect(last).toEqual({ hongheo1: "can-ngheo" });
    act(() => {
      setter.call(sel, "");
      sel.dispatchEvent(new Event("change", { bubbles: true }));
    });
    expect(last).toEqual({ hongheo1: "" });
  });

  it("combobox: a trigger summarising the chosen labels, opening checkboxes; Esc closes and refocuses the trigger", () => {
    const el = mount([MULTI("dropdown")], { nhucau01: ["nuoc"] });
    const trigger = el.querySelector<HTMLButtonElement>(`button#${attributeInputId("nhucau01")}`)!;
    expect(trigger).not.toBeNull();
    expect(el.querySelector("fieldset input[type=checkbox]")).toBeNull();
    expect(trigger.textContent).toBe("Nước");
    expect(trigger.getAttribute("aria-expanded")).toBe("false");
    expect(el.querySelector(`label[for='${attributeInputId("nhucau01")}']`)!.textContent).toContain("Nhu cầu hỗ trợ");

    act(() => trigger.click());
    expect(trigger.getAttribute("aria-expanded")).toBe("true");
    const list = el.querySelector<HTMLElement>("[data-choice-list='nhucau01']")!;
    // Inside the dialog's DOM, not portalled to <body> (a showModal dialog makes <body> inert).
    expect(host!.contains(list)).toBe(true);
    const boxes = Array.from(list.querySelectorAll<HTMLInputElement>("input[type=checkbox]"));
    expect(boxes.map((b) => [b.value, b.checked])).toEqual([
      ["dien", false],
      ["nuoc", true],
    ]);
    act(() => boxes[0]!.click());
    expect(last).toEqual({ nhucau01: ["dien", "nuoc"] });
    expect(trigger.textContent).toBe("Điện, Nước");

    act(() => {
      list.closest("[role=dialog]")!.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));
    });
    expect(trigger.getAttribute("aria-expanded")).toBe("false");
    expect(el.querySelector("[data-choice-list]")).toBeNull();
  });

  it("combobox with nothing chosen says so on the trigger", () => {
    const el = mount([MULTI("dropdown")]);
    expect(el.querySelector<HTMLButtonElement>(`button#${attributeInputId("nhucau01")}`)!.textContent).toBe(NOTHING_CHOSEN);
  });
});

describe("layout_width — the 2-column grid of the dynamic section (ADR 0101 #3c)", () => {
  it("one own grid, 1 column under sm and 2 from sm; full spans both, half one; sort order kept", () => {
    const el = mount([
      base({ code: "aaaa0001", label: "A", layout_width: "half" }),
      base({ code: "aaaa0002", label: "B", field_type: "multiText", layout_width: "full" }),
      { ...SINGLE("dropdown"), layout_width: "full" },
      { ...MULTI(), layout_width: "half" },
      base({ code: "aaaa0005", label: "E", field_type: "multiText", layout_width: "half" }),
    ]);
    const grid = el.querySelector<HTMLElement>("[data-attribute-grid]")!;
    expect(grid.className).toContain("grid-cols-1");
    expect(grid.className).toContain("sm:grid-cols-2");
    expect(grid.className).toContain("sm:col-span-2");
    const cells = Array.from(grid.children) as HTMLElement[];
    expect(cells).toHaveLength(5);
    const spans = cells.map((c) => c.className.includes("sm:col-span-2"));
    // The width is the field's setting, not its type: a half multiText stays half, a full singleBox spans.
    expect(spans).toEqual([false, true, true, false, false]);
    expect(cells[0]!.querySelector("#" + attributeInputId("aaaa0001"))).not.toBeNull();
    expect(cells[2]!.querySelector("#" + attributeInputId("hongheo1"))).not.toBeNull();
  });

  it("a width the server did not send falls back to the type's default (multiText full, others half)", () => {
    const el = mount([base({ code: "aaaa0001", layout_width: "" }), base({ code: "aaaa0002", field_type: "multiText", layout_width: "" })]);
    const cells = Array.from(el.querySelector("[data-attribute-grid]")!.children) as HTMLElement[];
    expect(cells.map((c) => c.className.includes("sm:col-span-2"))).toEqual([false, true]);
  });

  it("no live field: no empty grid", () => {
    const el = mount([]);
    expect(el.querySelector("[data-attribute-grid]")).toBeNull();
  });
});
