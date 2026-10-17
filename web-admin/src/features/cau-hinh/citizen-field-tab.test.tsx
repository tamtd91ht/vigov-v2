// @vitest-environment jsdom
//
// Cấu hình → Thuộc tính danh bạ (ADR 0101 #1–#5): the `admin.citizen_field` gate (denied case first), the
// ordered list with Lên / Xuống (the keyboard path of the reorder) and its rollback on refusal, the add
// dialog's validation and body, the edit dialog's read-only code and type, and "Xoá" (soft delete) with a
// warning and a reason — deleted fields hidden but their codes still reserved.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import type { PhienDaDoc } from "@/features/phien/phien-hien-tai";
import type { CitizenField } from "@/lib/api/citizen-fields";

const H = vi.hoisted(() => ({
  phien: null as PhienDaDoc,
  toast: { success: vi.fn(), error: vi.fn() },
}));
vi.mock("@/features/phien/phien-hien-tai", () => ({ usePhien: () => H.phien }));
vi.mock("sonner", () => ({ toast: H.toast }));

const form = await import("./citizen-field-form");
const { CitizenFieldTab } = await import("./citizen-field-tab");

const BASE: CitizenField = {
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
  display_mode: "",
  layout_width: "full",
};
const NUMBER: CitizenField = { ...BASE, code: "sonhan01", label: "Số nhân khẩu", field_type: "number", options: [], number_kind: "integer", sort_order: 2 };
const DATE: CitizenField = { ...BASE, code: "ngaysinh", label: "Ngày sinh chủ hộ", field_type: "dateTime", options: [], date_pattern: "dd/MM/yyyy", sort_order: 3 };
const RETIRED: CitizenField = { ...BASE, code: "cucu0001", label: "Thuộc tính cũ", field_type: "singleText", options: [], sort_order: 0, retired_at: "2026-10-17T01:00:00Z" };

/** The PATCH body with nothing changed: every key the contract names, null. */
const UNCHANGED = { code: null, label: null, field_type: null, placeholder: null, options: null, number_kind: null, date_pattern: null, required: null, show_in_list: null, display_mode: null, layout_width: null };

function session(permissions: string[]): PhienDaDoc {
  return { ok: true, duLieu: { staff: { full_name: "A", position: "B" }, role: null, permissions } } as unknown as PhienDaDoc;
}

type Call = { method: string; url: string; body?: string; headers: Record<string, string> };
let calls: Call[] = [];
let items: CitizenField[] = [];
let answers: Record<string, { status: number; body: unknown }> = {};

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

beforeEach(() => {
  calls = [];
  items = [BASE, NUMBER, DATE, RETIRED];
  answers = {};
  H.toast.success.mockReset();
  H.toast.error.mockReset();
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init: RequestInit) => {
      const method = String(init.method ?? "GET");
      calls.push({ method, url, body: typeof init.body === "string" ? init.body : undefined, headers: (init.headers ?? {}) as Record<string, string> });
      const json = (status: number, b: unknown) => new Response(JSON.stringify(b), { status, headers: { "Content-Type": "application/json" } });
      if (method === "GET" && url === "/api/v1/citizen-fields?include_retired=true") return json(200, { items });
      const a = answers[`${method} ${url}`];
      if (a !== undefined) return a.status === 204 ? new Response(null, { status: 204 }) : json(a.status, a.body);
      return json(500, { code: "internal", message: "no stub" });
    }),
  );
});

let root: Root | null = null;
let host: HTMLDivElement | null = null;

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  H.phien = null;
  vi.unstubAllGlobals();
});

async function settle() {
  for (let i = 0; i < 8; i++) await act(async () => {});
}

async function mount(): Promise<HTMLDivElement> {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() => r.render(<CitizenFieldTab />));
  await settle();
  return host;
}

const byLabel = (el: ParentNode, label: string) => el.querySelector<HTMLButtonElement>(`button[aria-label="${label}"]`)!;
function button(el: ParentNode, text: string): HTMLButtonElement {
  const found = Array.from(el.querySelectorAll<HTMLButtonElement>("button")).filter((b) => b.textContent?.trim() === text);
  expect(found.length, `button "${text}"`).toBeGreaterThan(0);
  return found.at(-1)!;
}
function typeInto(el: HTMLInputElement, value: string) {
  const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!;
  act(() => {
    setter.call(el, value);
    el.dispatchEvent(new Event("input", { bubbles: true }));
  });
}
function selectValue(el: HTMLSelectElement, value: string) {
  const setter = Object.getOwnPropertyDescriptor(HTMLSelectElement.prototype, "value")!.set!;
  act(() => {
    setter.call(el, value);
    el.dispatchEvent(new Event("change", { bubbles: true }));
  });
}
// The order as drawn — by the designer (the default view) or by the list.
const liveOrder = (el: HTMLElement) =>
  Array.from(el.querySelectorAll<HTMLElement>("[data-layout-card], [data-field-row]")).map((r) => r.dataset.layoutCard ?? r.dataset.fieldRow);
const writes = () => calls.filter((c) => c.method !== "GET");
const widthRadios = (dlg: ParentNode) => Array.from(dlg.querySelectorAll<HTMLInputElement>("[data-layout-width] input[type=radio]"));

describe("gate — admin.citizen_field, denied case first", () => {
  it("DENIED: citizen.read / admin.zalo / look-alikes do not open it, and NOTHING is requested", async () => {
    H.phien = session(["citizen.read", "citizen.update", "admin.zalo", "admin.citizen_fields", "ADMIN.CITIZEN_FIELD"]);
    const el = await mount();
    expect(el.textContent).toContain(form.NO_PERMISSION);
    expect(calls).toHaveLength(0);
  });

  it("session unreadable: the server's sentence, nothing requested", async () => {
    H.phien = { ok: false, thongBao: "Phiên làm việc đã hết hạn." } as PhienDaDoc;
    const el = await mount();
    expect(el.textContent).toContain("Phiên làm việc đã hết hạn.");
    expect(calls).toHaveLength(0);
  });

  it("ALLOWED: reads the definitions WITH the deleted ones, draws only the live ones in sort order", async () => {
    H.phien = session(["admin.citizen_field"]);
    const el = await mount();
    expect(calls.map((c) => `${c.method} ${c.url}`)).toEqual(["GET /api/v1/citizen-fields?include_retired=true"]);
    expect(liveOrder(el)).toEqual(["hongheo1", "sonhan01", "ngaysinh"]);
    expect(el.querySelector("[data-field-count]")!.textContent).toBe("3/50 thuộc tính đang dùng");
    // No "Đã ngừng dùng" section any more: a deleted field is nowhere on the screen.
    expect(el.textContent).not.toContain("Thuộc tính cũ");
    expect(el.textContent).not.toContain("cucu0001");
    expect(el.textContent).not.toContain("ngừng dùng");
    expect(el.querySelector("[data-retired-fields]")).toBeNull();
  });
});

describe("reorder — Lên / Xuống (keyboard-reachable buttons) and rollback", () => {
  it("Xuống on the first row PUTs the whole live order; Lên is disabled on the first row, Xuống on the last", async () => {
    H.phien = session(["admin.citizen_field"]);
    answers["PUT /api/v1/citizen-field-order"] = { status: 204, body: null };
    const el = await mount();
    expect(byLabel(el, "Đưa Phân loại hộ lên").disabled).toBe(true);
    expect(byLabel(el, "Đưa Ngày sinh chủ hộ xuống").disabled).toBe(true);
    // Focus + Enter on a native button is a click: the keyboard path is the same handler.
    byLabel(el, "Đưa Phân loại hộ xuống").focus();
    act(() => byLabel(el, "Đưa Phân loại hộ xuống").click());
    await settle();
    const put = writes().find((c) => c.method === "PUT")!;
    expect(put.url).toBe("/api/v1/citizen-field-order");
    expect(JSON.parse(put.body!)).toEqual({ codes: ["sonhan01", "hongheo1", "ngaysinh"] });
    expect(H.toast.success).toHaveBeenCalledWith(form.ORDER_SAVED_TOAST);
  });

  it("a refused reorder puts the old order back and says so", async () => {
    H.phien = session(["admin.citizen_field"]);
    answers["PUT /api/v1/citizen-field-order"] = { status: 409, body: { code: "order_mismatch", message: "Danh sách mã không khớp.", trace_id: "" } };
    const el = await mount();
    act(() => byLabel(el, "Đưa Ngày sinh chủ hộ lên").click());
    await settle();
    expect(liveOrder(el)).toEqual(["hongheo1", "sonhan01", "ngaysinh"]);
    expect(H.toast.error).toHaveBeenCalledWith(`${form.ORDER_FAILED_TOAST} Danh sách mã không khớp.`);
  });

  it("each drag handle names its row and position for a screen reader", async () => {
    H.phien = session(["admin.citizen_field"]);
    const el = await mount();
    expect(byLabel(el, "Thuộc tính Số nhân khẩu, vị trí 2 trên 3")).not.toBeNull();
  });
});

function openAdd(el: HTMLElement): HTMLFormElement {
  act(() => button(el, "Thêm thuộc tính").click());
  return document.querySelector<HTMLFormElement>("form[aria-label='Thêm thuộc tính']")!;
}
const codeOf = (dlg: ParentNode) => dlg.querySelector<HTMLInputElement>("#citizen-field-code")!;
const labelOf = (dlg: ParentNode) => dlg.querySelector<HTMLInputElement>("#citizen-field-label")!;
const saveButton = (dlg: ParentNode) => button(dlg, "Lưu");

const CODE_SHAPE = /^[a-z][a-z0-9]{7,11}$/;

describe("add dialog — random code", () => {
  afterEach(() => vi.restoreAllMocks());

  it("Mã comes BEFORE Tên hiển thị, holds a random code at open, read-only, with the ZNS/SMS hint; the label never touches it", async () => {
    H.phien = session(["admin.citizen_field"]);
    const el = await mount();
    const dlg = openAdd(el);
    const ids = Array.from(dlg.querySelectorAll<HTMLElement>("input, select")).map((x) => x.id);
    expect(ids.indexOf("citizen-field-code")).toBeLessThan(ids.indexOf("citizen-field-label"));
    const code = codeOf(dlg).value;
    expect(code).toMatch(CODE_SHAPE);
    expect(codeOf(dlg).readOnly).toBe(true);
    expect(dlg.textContent).toContain(form.CODE_HINT);
    typeInto(labelOf(dlg), "Hộ nghèo");
    expect(codeOf(dlg).value).toBe(code);
  });

  it("a random code the commune already holds (deleted included) is redrawn silently at open", async () => {
    H.phien = session(["admin.citizen_field"]);
    items = [BASE, NUMBER, DATE, RETIRED, { ...RETIRED, code: "aaaaaaaaaa", label: "Cũ" }];
    // First code drawn: every byte 0 → "aaaaaaaaaa" (held, deleted); then every byte 1 → "bbbbbbbbbb".
    let call = 0;
    vi.spyOn(crypto, "getRandomValues").mockImplementation(<T extends ArrayBufferView | null>(buf: T): T => {
      (buf as unknown as Uint8Array).fill(call++ < form.RANDOM_CODE_LENGTH ? 0 : 1);
      return buf;
    });
    const el = await mount();
    const dlg = openAdd(el);
    expect(codeOf(dlg).value).toBe("bbbbbbbbbb");
    expect(dlg.querySelector("[data-field-error]")).toBeNull();
  });

  it("Sinh mã khác draws a new random code", async () => {
    H.phien = session(["admin.citizen_field"]);
    let call = 0;
    vi.spyOn(crypto, "getRandomValues").mockImplementation(<T extends ArrayBufferView | null>(buf: T): T => {
      (buf as unknown as Uint8Array).fill(call++ < form.RANDOM_CODE_LENGTH ? 2 : 3);
      return buf;
    });
    const el = await mount();
    const dlg = openAdd(el);
    expect(codeOf(dlg).value).toBe("cccccccccc");
    act(() => button(dlg, form.CODE_REDRAW).click());
    expect(codeOf(dlg).value).toBe("dddddddddd");
    expect(codeOf(dlg).readOnly).toBe(true);
  });

  it("Sửa → editable, checked live (8–12, a letter first; taken / taken-deleted disable Lưu); Sinh mã khác goes back to random", async () => {
    H.phien = session(["admin.citizen_field"]);
    const el = await mount();
    const dlg = openAdd(el);
    act(() => byLabel(dlg, "Sửa mã thuộc tính").click());
    expect(codeOf(dlg).readOnly).toBe(false);
    expect(document.activeElement).toBe(codeOf(dlg));
    typeInto(codeOf(dlg), "abc1234");
    expect(dlg.textContent).toContain(form.CODE_INVALID);
    typeInto(codeOf(dlg), "1abcdefgh");
    expect(dlg.textContent).toContain(form.CODE_INVALID);
    typeInto(codeOf(dlg), "hongheo1");
    expect(dlg.textContent).toContain(form.CODE_TAKEN);
    expect(saveButton(dlg).disabled).toBe(true);
    typeInto(codeOf(dlg), "cucu0001");
    expect(dlg.textContent).toContain(form.CODE_TAKEN_DELETED);
    expect(saveButton(dlg).disabled).toBe(true);
    typeInto(codeOf(dlg), "plkhac012345");
    expect(dlg.querySelector("[data-field-error]")).toBeNull();
    expect(saveButton(dlg).disabled).toBe(false);
    act(() => button(dlg, form.CODE_REDRAW).click());
    expect(codeOf(dlg).readOnly).toBe(true);
    expect(codeOf(dlg).value).toMatch(CODE_SHAPE);
  });

  it("409 citizen_field_code_taken on a RANDOM code: a new one is drawn and said; the retry POSTs it under the same key", async () => {
    H.phien = session(["admin.citizen_field"]);
    answers["POST /api/v1/citizen-fields"] = { status: 409, body: { code: "citizen_field_code_taken", message: "Mã đã dùng.", trace_id: "" } };
    const el = await mount();
    const dlg = openAdd(el);
    const first = codeOf(dlg).value;
    typeInto(labelOf(dlg), "Hộ nghèo");
    act(() => saveButton(dlg).click());
    await settle();
    expect(JSON.parse(writes()[0]!.body!).code).toBe(first);
    const next = codeOf(dlg).value;
    expect(next).not.toBe(first);
    expect(next).toMatch(CODE_SHAPE);
    expect(dlg.querySelector("[data-field-error]")!.textContent).toBe(form.CODE_REDRAWN_AFTER_CONFLICT);
    answers["POST /api/v1/citizen-fields"] = { status: 201, body: {} };
    act(() => saveButton(dlg).click());
    await settle();
    const second = writes()[1]!;
    expect(JSON.parse(second.body!).code).toBe(next);
    expect(second.headers["Idempotency-Key"]).toBe(writes()[0]!.headers["Idempotency-Key"]);
    expect(H.toast.success).toHaveBeenCalledWith(form.FIELD_ADDED_TOAST);
  });

  it("409 on a TYPED code: the server's sentence under Mã, the code left as typed", async () => {
    H.phien = session(["admin.citizen_field"]);
    answers["POST /api/v1/citizen-fields"] = { status: 409, body: { code: "citizen_field_code_taken", message: "Mã đã dùng.", trace_id: "" } };
    const el = await mount();
    const dlg = openAdd(el);
    typeInto(labelOf(dlg), "X");
    act(() => byLabel(dlg, "Sửa mã thuộc tính").click());
    typeInto(codeOf(dlg), "tuychon12");
    act(() => saveButton(dlg).click());
    await settle();
    expect(codeOf(dlg).value).toBe("tuychon12");
    expect(dlg.querySelector("[data-field-error]")!.textContent).toBe("Mã đã dùng.");
  });
});

describe("add dialog — options of a choice type", () => {
  it.each([
    ["combobox", "Chọn nhiều"],
    ["singleBox", "Chọn một"],
  ])("%s (%s): Lựa chọn appears at once with ONE empty row, and Lưu needs ≥1 filled option", async (type) => {
    H.phien = session(["admin.citizen_field"]);
    answers["POST /api/v1/citizen-fields"] = { status: 201, body: {} };
    const el = await mount();
    const dlg = openAdd(el);
    expect(dlg.querySelector("[data-options-editor]")).toBeNull();
    typeInto(labelOf(dlg), "Phân loại");
    selectValue(dlg.querySelector<HTMLSelectElement>("#citizen-field-type")!, type);
    expect(dlg.querySelector("[data-options-editor]")).not.toBeNull();
    expect(dlg.querySelectorAll("[data-option-row]")).toHaveLength(1);
    expect(dlg.querySelector<HTMLInputElement>("#citizen-field-option-value-0")!.value).toBe("");
    expect(dlg.querySelector<HTMLInputElement>("#citizen-field-option-label-0")!.value).toBe("");
    act(() => saveButton(dlg).click());
    await settle();
    expect(dlg.textContent).toContain(form.OPTION_VALUE_REQUIRED);
    act(() => byLabel(dlg, "Bỏ lựa chọn 1").click());
    act(() => saveButton(dlg).click());
    await settle();
    expect(dlg.querySelector("[data-options-error]")!.textContent).toBe(form.OPTIONS_NONE);
    expect(writes()).toHaveLength(0);
    act(() => button(dlg, "Thêm lựa chọn").click());
    typeInto(dlg.querySelector<HTMLInputElement>("#citizen-field-option-value-0")!, "ho-ngheo");
    typeInto(dlg.querySelector<HTMLInputElement>("#citizen-field-option-label-0")!, "Hộ nghèo");
    act(() => saveButton(dlg).click());
    await settle();
    const body = JSON.parse(writes()[0]!.body!);
    expect(body.field_type).toBe(type);
    expect(body.options).toEqual([{ value: "ho-ngheo", label: "Hộ nghèo", retired: false }]);
  });
});

describe("add dialog", () => {
  it("refuses a bad code and an empty option list BEFORE sending; a clean draft POSTs the body with the key", async () => {
    H.phien = session(["admin.citizen_field"]);
    answers["POST /api/v1/citizen-fields"] = { status: 201, body: {} };
    const el = await mount();
    act(() => button(el, "Thêm thuộc tính").click());
    const dlg = document.querySelector<HTMLFormElement>("form[aria-label='Thêm thuộc tính']")!;
    act(() => byLabel(dlg, "Sửa mã thuộc tính").click());
    typeInto(dlg.querySelector<HTMLInputElement>("#citizen-field-code")!, "1abc");
    typeInto(dlg.querySelector<HTMLInputElement>("#citizen-field-label")!, "Nhu cầu hỗ trợ");
    selectValue(dlg.querySelector<HTMLSelectElement>("#citizen-field-type")!, "combobox");
    act(() => button(dlg, "Lưu").click());
    await settle();
    expect(dlg.textContent).toContain(form.CODE_INVALID);
    expect(dlg.textContent).toContain(form.OPTION_VALUE_REQUIRED);
    expect(writes()).toHaveLength(0);

    typeInto(dlg.querySelector<HTMLInputElement>("#citizen-field-code")!, "nhucau0001");
    typeInto(dlg.querySelector<HTMLInputElement>("#citizen-field-option-value-0")!, "dien");
    typeInto(dlg.querySelector<HTMLInputElement>("#citizen-field-option-label-0")!, "Điện");
    act(() => button(dlg, "Lưu").click());
    await settle();
    const post = writes().find((c) => c.method === "POST")!;
    expect(post.url).toBe("/api/v1/citizen-fields");
    expect(post.headers["Idempotency-Key"]).toBeTruthy();
    expect(JSON.parse(post.body!)).toEqual({
      code: "nhucau0001",
      label: "Nhu cầu hỗ trợ",
      field_type: "combobox",
      placeholder: "",
      options: [{ value: "dien", label: "Điện", retired: false }],
      number_kind: "",
      date_pattern: "",
      required: false,
      show_in_list: false,
      display_mode: "full",
      layout_width: null,
    });
    expect(H.toast.success).toHaveBeenCalledWith(form.FIELD_ADDED_TOAST);
  });

  it("Kiểu hiển thị: only for Chọn nhiều / Chọn một, full by default, dropdown sent when chosen; other types send \"\"", async () => {
    H.phien = session(["admin.citizen_field"]);
    answers["POST /api/v1/citizen-fields"] = { status: 201, body: {} };
    const el = await mount();
    const dlg = openAdd(el);
    typeInto(labelOf(dlg), "Phân loại");
    expect(dlg.querySelector("[data-display-mode]")).toBeNull();
    selectValue(dlg.querySelector<HTMLSelectElement>("#citizen-field-type")!, "singleBox");
    const group = dlg.querySelector<HTMLFieldSetElement>("[data-display-mode]")!;
    expect(group.querySelector("legend")!.textContent).toBe("Kiểu hiển thị");
    const radios = Array.from(group.querySelectorAll<HTMLInputElement>("input[type=radio]"));
    expect(radios.map((r) => [r.value, r.checked, r.parentElement!.textContent])).toEqual([
      ["full", true, "Hiện đủ các lựa chọn"],
      ["dropdown", false, "Danh sách thả xuống"],
    ]);
    act(() => radios[1]!.click());
    typeInto(dlg.querySelector<HTMLInputElement>("#citizen-field-option-value-0")!, "ho-ngheo");
    typeInto(dlg.querySelector<HTMLInputElement>("#citizen-field-option-label-0")!, "Hộ nghèo");
    act(() => saveButton(dlg).click());
    await settle();
    expect(JSON.parse(writes()[0]!.body!).display_mode).toBe("dropdown");
  });

  it("a type without options sends display_mode \"\" and shows no Kiểu hiển thị", async () => {
    H.phien = session(["admin.citizen_field"]);
    answers["POST /api/v1/citizen-fields"] = { status: 201, body: {} };
    const el = await mount();
    const dlg = openAdd(el);
    typeInto(labelOf(dlg), "Ghi chú");
    selectValue(dlg.querySelector<HTMLSelectElement>("#citizen-field-type")!, "combobox");
    selectValue(dlg.querySelector<HTMLSelectElement>("#citizen-field-type")!, "singleText");
    expect(dlg.querySelector("[data-display-mode]")).toBeNull();
    act(() => saveButton(dlg).click());
    await settle();
    expect(JSON.parse(writes()[0]!.body!).display_mode).toBe("");
  });

  it("a code of a DELETED field is refused locally with its own sentence — Lưu disabled, nothing sent", async () => {
    H.phien = session(["admin.citizen_field"]);
    const el = await mount();
    const dlg = openAdd(el);
    typeInto(labelOf(dlg), "X");
    act(() => byLabel(dlg, "Sửa mã thuộc tính").click());
    typeInto(codeOf(dlg), "cucu0001");
    expect(dlg.querySelector("[data-field-error]")!.textContent).toBe("Mã này đã dùng cho một thuộc tính đã xoá, không dùng lại được");
    expect(saveButton(dlg).disabled).toBe(true);
    act(() => saveButton(dlg).click());
    await settle();
    expect(writes()).toHaveLength(0);
  });

  it("at 50 live fields the add button is disabled and the limit is said", async () => {
    H.phien = session(["admin.citizen_field"]);
    items = Array.from({ length: 50 }, (_, i) => ({ ...NUMBER, code: `so${String(i).padStart(4, "0")}`, label: `Số ${i}`, sort_order: i }));
    const el = await mount();
    expect(button(el, "Thêm thuộc tính").disabled).toBe(true);
    expect(el.querySelector("[data-limit-reached]")!.textContent).toContain("Đã đủ 50");
  });
});

describe("edit and delete", () => {
  it("edit: code and type read-only; relabel an option + retire another → PATCH with only what changed (the rest null)", async () => {
    H.phien = session(["admin.citizen_field"]);
    answers["PATCH /api/v1/citizen-fields/hongheo1"] = { status: 200, body: BASE };
    const el = await mount();
    act(() => byLabel(el, "Sửa thuộc tính Phân loại hộ").click());
    const dlg = document.querySelector<HTMLFormElement>("form[aria-label='Sửa thuộc tính']")!;
    expect(dlg.querySelector<HTMLInputElement>("#citizen-field-code")!.readOnly).toBe(true);
    // No Sửa / Sinh mã khác for the code on edit: values on every person are keyed by it (ADR 0101 #1).
    expect(dlg.querySelector("button[aria-label='Sửa mã thuộc tính']")).toBeNull();
    expect(dlg.textContent).not.toContain(form.CODE_REDRAW);
    expect(dlg.textContent).toContain(form.CODE_LOCKED_HINT);
    expect(dlg.querySelector<HTMLInputElement>("#citizen-field-type")!.readOnly).toBe(true);
    expect(dlg.querySelector<HTMLInputElement>("#citizen-field-option-value-0")!.readOnly).toBe(true);
    // No delete for a saved option: only "Ngừng dùng".
    expect(dlg.querySelector("button[aria-label='Bỏ lựa chọn 1']")).toBeNull();
    typeInto(dlg.querySelector<HTMLInputElement>("#citizen-field-option-label-0")!, "Hộ nghèo (2026)");
    act(() => byLabel(dlg, "Ngừng dùng lựa chọn Cận nghèo").click());
    act(() => button(dlg, "Lưu").click());
    await settle();
    const patch = writes().find((c) => c.method === "PATCH")!;
    expect(JSON.parse(patch.body!)).toEqual({
      ...UNCHANGED,
      options: [
        { value: "ho-ngheo", label: "Hộ nghèo (2026)", retired: false },
        { value: "can-ngheo", label: "Cận nghèo", retired: true },
      ],
    });
  });

  it("edit: Kiểu hiển thị is editable; a choice field with display_mode \"\" reads as full; switching PATCHes only display_mode (layout_width untouched)", async () => {
    H.phien = session(["admin.citizen_field"]);
    answers["PATCH /api/v1/citizen-fields/hongheo1"] = { status: 200, body: BASE };
    const el = await mount();
    act(() => byLabel(el, "Sửa thuộc tính Phân loại hộ").click());
    const dlg = document.querySelector<HTMLFormElement>("form[aria-label='Sửa thuộc tính']")!;
    const radios = Array.from(dlg.querySelectorAll<HTMLInputElement>("[data-display-mode] input[type=radio]"));
    expect(radios.map((r) => r.checked)).toEqual([true, false]);
    expect(radios[1]!.disabled).toBe(false);
    act(() => radios[1]!.click());
    act(() => button(dlg, "Lưu").click());
    await settle();
    const patch = writes().find((c) => c.method === "PATCH")!;
    expect(JSON.parse(patch.body!)).toEqual({ ...UNCHANGED, display_mode: "dropdown" });
  });

  it("edit with nothing changed on a dropdown field sends nothing", async () => {
    H.phien = session(["admin.citizen_field"]);
    items = [{ ...BASE, display_mode: "dropdown" }, NUMBER, DATE, RETIRED];
    const el = await mount();
    act(() => byLabel(el, "Sửa thuộc tính Phân loại hộ").click());
    const dlg = document.querySelector<HTMLFormElement>("form[aria-label='Sửa thuộc tính']")!;
    const radios = Array.from(dlg.querySelectorAll<HTMLInputElement>("[data-display-mode] input[type=radio]"));
    expect(radios.map((r) => r.checked)).toEqual([false, true]);
    act(() => button(dlg, "Lưu").click());
    await settle();
    expect(writes()).toHaveLength(0);
  });

  it("Xoá: the confirm names the field and its code, needs a reason, sends DELETE {reason}, then the field is gone", async () => {
    H.phien = session(["admin.citizen_field"]);
    answers["DELETE /api/v1/citizen-fields/sonhan01"] = { status: 204, body: null };
    const el = await mount();
    // No "Ngừng dùng" action on a row any more.
    expect(el.querySelector("button[aria-label='Ngừng dùng thuộc tính Số nhân khẩu']")).toBeNull();
    act(() => byLabel(el, "Xoá thuộc tính Số nhân khẩu").click());
    const box = document.querySelector<HTMLFormElement>("form[aria-label='Xoá thuộc tính']")!;
    expect(box.querySelector("h3")!.textContent).toBe("Xoá thuộc tính “Số nhân khẩu”?");
    expect(box.querySelector("[data-delete-consequence]")!.textContent).toBe(
      "Thuộc tính sẽ không còn hiện ở biểu mẫu, danh sách và bộ lọc danh bạ, cũng không chọn được khi gửi tin ZNS/SMS. Dữ liệu đã nhập cho người dân được giữ nguyên. Mã sonhan01 sẽ không dùng lại được cho thuộc tính mới.",
    );
    expect(box.textContent).toContain("Lý do xoá");
    button(box, "Huỷ");
    // Disabled until a reason is typed; whitespace is not a reason.
    expect(button(box, "Xoá thuộc tính").disabled).toBe(true);
    const reason = box.querySelector<HTMLTextAreaElement>("textarea#citizen-field-sonhan01-delete-reason")!;
    typeIntoArea(reason, "   ");
    expect(button(box, "Xoá thuộc tính").disabled).toBe(true);
    act(() => box.requestSubmit());
    await settle();
    expect(box.textContent).toContain(form.DELETE_REASON_REQUIRED);
    expect(writes()).toHaveLength(0);

    // The server now answers the list without the deleted field among the live ones.
    items = [BASE, DATE, RETIRED, { ...NUMBER, retired_at: "2026-10-17T02:00:00Z" }];
    typeIntoArea(reason, "Không thu thập nữa");
    expect(button(box, "Xoá thuộc tính").disabled).toBe(false);
    act(() => button(box, "Xoá thuộc tính").click());
    await settle();
    const del = writes().find((c) => c.method === "DELETE")!;
    expect(del.url).toBe("/api/v1/citizen-fields/sonhan01");
    expect(JSON.parse(del.body!)).toEqual({ reason: "Không thu thập nữa" });
    expect(H.toast.success).toHaveBeenCalledWith("Đã xoá thuộc tính");
    expect(document.querySelector("form[aria-label='Xoá thuộc tính']")).toBeNull();
    expect(calls.filter((c) => c.method === "GET")).toHaveLength(2);
    expect(liveOrder(el)).toEqual(["hongheo1", "ngaysinh"]);
    expect(el.textContent).not.toContain("Số nhân khẩu");
    expect(el.querySelector("[data-field-count]")!.textContent).toBe("2/50 thuộc tính đang dùng");
  });

  it("Huỷ closes the confirm and sends nothing", async () => {
    H.phien = session(["admin.citizen_field"]);
    const el = await mount();
    act(() => byLabel(el, "Xoá thuộc tính Số nhân khẩu").click());
    const box = document.querySelector<HTMLFormElement>("form[aria-label='Xoá thuộc tính']")!;
    act(() => button(box, "Huỷ").click());
    expect(document.querySelector("form[aria-label='Xoá thuộc tính']")).toBeNull();
    expect(writes()).toHaveLength(0);
    expect(liveOrder(el)).toEqual(["hongheo1", "sonhan01", "ngaysinh"]);
  });

  it("a just-deleted code stays reserved: manual reuse is refused, a random draw skips it", async () => {
    H.phien = session(["admin.citizen_field"]);
    items = [BASE, DATE, RETIRED, { ...NUMBER, code: "bbbbbbbbbb", retired_at: "2026-10-17T02:00:00Z" }];
    // First draw: every byte 1 → "bbbbbbbbbb" (the deleted field's code); then every byte 2 → "cccccccccc".
    let call = 0;
    vi.spyOn(crypto, "getRandomValues").mockImplementation(<T extends ArrayBufferView | null>(buf: T): T => {
      (buf as unknown as Uint8Array).fill(call++ < form.RANDOM_CODE_LENGTH ? 1 : 2);
      return buf;
    });
    const el = await mount();
    const dlg = openAdd(el);
    expect(codeOf(dlg).value).toBe("cccccccccc");
    expect(dlg.querySelector("[data-field-error]")).toBeNull();
    act(() => byLabel(dlg, "Sửa mã thuộc tính").click());
    typeInto(codeOf(dlg), "bbbbbbbbbb");
    expect(dlg.querySelector("[data-field-error]")!.textContent).toBe(form.CODE_TAKEN_DELETED);
    expect(saveButton(dlg).disabled).toBe(true);
    vi.restoreAllMocks();
  });
});

function typeIntoArea(el: HTMLTextAreaElement, value: string) {
  const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!;
  act(() => {
    setter.call(el, value);
    el.dispatchEvent(new Event("input", { bubbles: true }));
  });
}

describe("Biểu mẫu — the designer: the contact form's 2-column grid (ADR 0101 #3c)", () => {
  // A half, B full, C half, D half, E half → rows: [A, empty] [B B] [C, D] [E, empty]
  const A: CitizenField = { ...BASE, code: "aaaaaaa1", label: "A", field_type: "singleText", options: [], layout_width: "half", sort_order: 1 };
  const B: CitizenField = { ...A, code: "bbbbbbb1", label: "B", field_type: "multiText", layout_width: "full", sort_order: 2 };
  const C: CitizenField = { ...A, code: "ccccccc1", label: "C", layout_width: "half", sort_order: 3 };
  const D: CitizenField = { ...BASE, code: "ddddddd1", label: "D", display_mode: "dropdown", layout_width: "half", sort_order: 4 };
  const E: CitizenField = { ...NUMBER, code: "eeeeeee1", label: "E", layout_width: "half", sort_order: 5 };

  async function openLayout(): Promise<HTMLDivElement> {
    H.phien = session(["admin.citizen_field"]);
    items = [E, C, A, D, B, RETIRED];
    const el = await mount();
    // The designer is the default view.
    expect(el.querySelector("[data-field-layout]")).not.toBeNull();
    return el;
  }
  const cards = (el: ParentNode) => Array.from(el.querySelectorAll<HTMLElement>("[data-layout-card]"));
  const card = (el: ParentNode, code: string) => el.querySelector<HTMLElement>(`[data-layout-card="${code}"]`)!;

  it("draws the live fields in sort order as a 1→2 column grid: full = sm:col-span-2, half = one column; the static note above", async () => {
    const el = await openLayout();
    const grid = el.querySelector<HTMLElement>("[data-layout-grid]")!;
    expect(grid.className).toContain("grid-cols-1");
    expect(grid.className).toContain("sm:grid-cols-2");
    expect(cards(el).map((c) => c.dataset.layoutCard)).toEqual(["aaaaaaa1", "bbbbbbb1", "ccccccc1", "ddddddd1", "eeeeeee1"]);
    expect(cards(el).map((c) => c.className.includes("sm:col-span-2"))).toEqual([false, true, false, false, false]);
    expect(el.querySelector("[data-layout-static-note]")!.textContent).toBe(form.LAYOUT_STATIC_NOTE);
    expect(el.querySelector("[data-layout-narrow-note]")!.className).toContain("sm:hidden");
    // The card draws the contact form's REAL control (type × display_mode), inert; the deleted field is not drawn.
    const dropdown = card(el, "ddddddd1").querySelector<HTMLElement>("[data-live-control]")!;
    expect(dropdown.hasAttribute("inert")).toBe(true);
    expect(dropdown.querySelector("select")!.textContent).toContain("Chưa chọn");
    expect(card(el, "eeeeeee1").querySelector("[data-live-control] input")!.getAttribute("inputmode")).toBe("numeric");
    expect(card(el, "bbbbbbb1").querySelector("[data-card-type]")!.textContent).toBe("Chữ nhiều dòng");
    expect(el.textContent).not.toContain("Thuộc tính cũ");
    // Back to the list.
    act(() => button(el, form.LIST_VIEW).click());
    expect(el.querySelector("[data-field-layout]")).toBeNull();
    expect(liveOrder(el)).toEqual(["aaaaaaa1", "bbbbbbb1", "ccccccc1", "ddddddd1", "eeeeeee1"]);
  });

  it("marks the empty right slot after a half field left alone (before a full one, and at the end) — only from sm", async () => {
    const el = await openLayout();
    const slots = Array.from(el.querySelectorAll<HTMLElement>("[data-empty-slot]"));
    expect(slots.map((s) => s.dataset.emptySlot)).toEqual(["aaaaaaa1", "eeeeeee1"]);
    expect(slots[0]!.textContent).toBe(form.EMPTY_SLOT_HINT);
    expect(slots[0]!.className).toContain("hidden");
    expect(slots[0]!.className).toContain("sm:flex");
    // The slot sits right after its card, so the grid's auto-flow puts it in the free column.
    expect(card(el, "aaaaaaa1").nextElementSibling).toBe(slots[0]);
  });

  it("Xuống on a card PUTs the whole live order (the keyboard path of the drag)", async () => {
    answers["PUT /api/v1/citizen-field-order"] = { status: 204, body: null };
    const el = await openLayout();
    act(() => byLabel(card(el, "aaaaaaa1"), "Đưa A xuống").click());
    await settle();
    const put = writes().find((c) => c.method === "PUT")!;
    expect(JSON.parse(put.body!)).toEqual({ codes: ["bbbbbbb1", "aaaaaaa1", "ccccccc1", "ddddddd1", "eeeeeee1"] });
    expect(byLabel(el, "Thuộc tính C, vị trí 3 trên 5")).not.toBeNull();
  });

  it("Cả hàng PATCHes layout_width alone; the card spans at once and stays so on success", async () => {
    answers["PATCH /api/v1/citizen-fields/aaaaaaa1"] = { status: 200, body: { ...A, layout_width: "full" } };
    const el = await openLayout();
    const group = card(el, "aaaaaaa1").querySelector<HTMLElement>("[role=group][aria-label='Độ rộng của A']")!;
    expect(button(group, "Nửa hàng").getAttribute("aria-pressed")).toBe("true");
    act(() => button(group, "Cả hàng").click());
    // Optimistic: drawn full before the server answers.
    expect(card(el, "aaaaaaa1").className).toContain("sm:col-span-2");
    await settle();
    const patch = writes().find((c) => c.method === "PATCH")!;
    expect(patch.url).toBe("/api/v1/citizen-fields/aaaaaaa1");
    expect(JSON.parse(patch.body!)).toEqual({ ...UNCHANGED, layout_width: "full" });
    expect(card(el, "aaaaaaa1").dataset.width).toBe("full");
    expect(H.toast.success).toHaveBeenCalledWith(form.WIDTH_SAVED_TOAST);
    // A is no longer a lone half: only E's slot is left.
    expect(Array.from(el.querySelectorAll<HTMLElement>("[data-empty-slot]")).map((s) => s.dataset.emptySlot)).toEqual(["eeeeeee1"]);
  });

  it("a refused width puts the old one back and says so", async () => {
    answers["PATCH /api/v1/citizen-fields/bbbbbbb1"] = { status: 409, body: { code: "conflict", message: "Thuộc tính đã đổi.", trace_id: "" } };
    const el = await openLayout();
    act(() => button(card(el, "bbbbbbb1"), "Nửa hàng").click());
    expect(card(el, "bbbbbbb1").dataset.width).toBe("half");
    await settle();
    expect(card(el, "bbbbbbb1").dataset.width).toBe("full");
    expect(card(el, "bbbbbbb1").className).toContain("sm:col-span-2");
    expect(H.toast.error).toHaveBeenCalledWith(`${form.WIDTH_FAILED_TOAST} Thuộc tính đã đổi.`);
  });

  it("pressing the width a card already has sends nothing", async () => {
    const el = await openLayout();
    act(() => button(card(el, "ccccccc1"), "Nửa hàng").click());
    await settle();
    expect(writes()).toHaveLength(0);
  });

  it("is the Thêm người dân form: its title, the static fields inert in the dialog's order, then the commune's fields; panel waits", async () => {
    const el = await openLayout();
    const canvas = el.querySelector<HTMLElement>("[data-form-canvas]")!;
    expect(canvas.textContent).toContain("Thêm người dân");
    const statics = canvas.querySelector<HTMLElement>("[data-static-fields]")!;
    expect(statics.hasAttribute("inert")).toBe(true);
    expect(Array.from(statics.querySelectorAll("label")).map((l) => l.textContent?.replace("*", "").trim())).toEqual([
      "Họ và tên",
      "Số điện thoại",
      "Email",
      "Thôn, tổ dân phố",
      "Địa chỉ",
      "Nhãn",
      "Ghi chú",
    ]);
    // Static first, the commune's grid after it — a field can never land among the static ones (#3c).
    expect(statics.compareDocumentPosition(canvas.querySelector("[data-layout-grid]")!) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(el.querySelector("[data-panel-empty]")!.textContent).toBe(form.PANEL_EMPTY);
  });

  /** The code the panel drew for the new field, and that field as the server will list it afterwards. */
  function newFieldFromPanel(el: HTMLElement, label: string, sortOrder: number): CitizenField {
    const code = el.querySelector<HTMLInputElement>("[data-field-panel] #citizen-field-code")!.value;
    return { ...A, code, label, sort_order: sortOrder };
  }

  it("+ before C: the draft card appears THERE, redrawn as it is typed; Lưu POSTs, then PUTs the order with it before C", async () => {
    answers["POST /api/v1/citizen-fields"] = { status: 201, body: {} };
    answers["PUT /api/v1/citizen-field-order"] = { status: 204, body: null };
    const el = await openLayout();
    act(() => byLabel(el, form.insertBeforeLabel("C")).click());
    const panel = el.querySelector<HTMLElement>("[data-field-panel]")!;
    const dlg = panel.querySelector<HTMLFormElement>("form[aria-label='Thêm thuộc tính']")!;
    const grid = el.querySelector<HTMLElement>("[data-layout-grid]")!;
    const draftCard = grid.querySelector<HTMLElement>("[data-new-field-card]")!;
    // In the grid order: A, (slot), B, NEW, C, …
    expect(draftCard.nextElementSibling).toBe(card(el, "ccccccc1"));
    expect(draftCard.textContent).toContain(form.NEW_FIELD_LABEL);
    typeInto(labelOf(dlg), "Nghề nghiệp");
    expect(draftCard.querySelector("[data-live-control]")!.textContent).toContain("Nghề nghiệp");
    act(() => widthRadios(dlg)[1]!.click());
    expect(grid.querySelector<HTMLElement>("[data-new-field-card]")!.className).toContain("sm:col-span-2");

    const created = newFieldFromPanel(el, "Nghề nghiệp", 6);
    act(() => saveButton(dlg).click());
    items = [E, C, A, D, B, created, RETIRED];
    await settle();
    expect(writes().map((c) => c.method)).toEqual(["POST", "PUT"]);
    expect(JSON.parse(writes()[1]!.body!)).toEqual({ codes: ["aaaaaaa1", "bbbbbbb1", created.code, "ccccccc1", "ddddddd1", "eeeeeee1"] });
    expect(H.toast.success).toHaveBeenCalledWith(form.FIELD_ADDED_TOAST);
    expect(el.querySelector("[data-new-field-card]")).toBeNull();
    expect(el.querySelector("[data-panel-empty]")).not.toBeNull();
  });

  it("+ at the end: POST only — identity already appends there", async () => {
    answers["POST /api/v1/citizen-fields"] = { status: 201, body: {} };
    const el = await openLayout();
    act(() => el.querySelector<HTMLButtonElement>("[data-add-at-end]")!.click());
    const dlg = el.querySelector<HTMLFormElement>("[data-field-panel] form")!;
    expect(el.querySelector("[data-layout-grid]")!.querySelector("[data-new-field-card]")!.nextElementSibling!.querySelector("[data-add-at-end]")).not.toBeNull();
    typeInto(labelOf(dlg), "Ghi chú hộ");
    act(() => saveButton(dlg).click());
    await settle();
    expect(writes().map((c) => c.method)).toEqual(["POST"]);
  });

  it("a refused placement says the field is at the END — never a silent success", async () => {
    answers["POST /api/v1/citizen-fields"] = { status: 201, body: {} };
    answers["PUT /api/v1/citizen-field-order"] = { status: 409, body: { code: "conflict", message: "Thứ tự đã đổi.", trace_id: "" } };
    const el = await openLayout();
    act(() => byLabel(el, form.insertBeforeLabel("A")).click());
    const dlg = el.querySelector<HTMLFormElement>("[data-field-panel] form")!;
    typeInto(labelOf(dlg), "Đầu tiên");
    const created = newFieldFromPanel(el, "Đầu tiên", 6);
    act(() => saveButton(dlg).click());
    items = [E, C, A, D, B, created, RETIRED];
    await settle();
    expect(H.toast.error).toHaveBeenCalledWith(`${form.PLACE_FAILED_TOAST} Thứ tự đã đổi.`);
    expect(liveOrder(el).at(-1)).toBe(created.code);
  });

  it("an unsaved new field can be thrown away from its card: gone from the form, panel closed, nothing written", async () => {
    const el = await openLayout();
    act(() => byLabel(el, form.insertBeforeLabel("C")).click());
    typeInto(labelOf(el.querySelector<HTMLFormElement>("[data-field-panel] form")!), "Tạm");
    const draftCard = el.querySelector<HTMLElement>("[data-new-field-card]")!;
    // Draggable before it is saved: it carries its own grip.
    expect(draftCard.querySelector(`button[aria-label^="${form.NEW_FIELD_LABEL}"]`)).not.toBeNull();
    act(() => byLabel(draftCard, form.DISCARD_NEW_FIELD).click());
    expect(el.querySelector("[data-new-field-card]")).toBeNull();
    expect(el.querySelector("[data-field-panel] form")).toBeNull();
    expect(el.querySelector("[data-panel-empty]")).not.toBeNull();
    expect(writes()).toHaveLength(0);
  });

  it("the empty right slot is a +: it adds right after its lone half", async () => {
    const el = await openLayout();
    act(() => el.querySelector<HTMLElement>("[data-empty-slot='aaaaaaa1'] button")!.click());
    expect(card(el, "aaaaaaa1").nextElementSibling!.hasAttribute("data-new-field-card")).toBe(true);
  });

  it("a click on a card opens it in the panel; the label is redrawn on the card as it is typed; Huỷ puts it back", async () => {
    const el = await openLayout();
    act(() => card(el, "ccccccc1").querySelector<HTMLElement>("[data-card-body]")!.click());
    const dlg = el.querySelector<HTMLFormElement>("[data-field-panel] form[aria-label='Sửa thuộc tính']")!;
    expect(card(el, "ccccccc1").dataset.selected).toBe("true");
    typeInto(labelOf(dlg), "C mới");
    expect(card(el, "ccccccc1").querySelector("[data-live-control]")!.textContent).toContain("C mới");
    // The buttons keep the stored name while it is typed.
    expect(byLabel(el, "Sửa thuộc tính C")).not.toBeNull();
    act(() => button(dlg, "Huỷ").click());
    expect(card(el, "ccccccc1").querySelector("[data-live-control]")!.textContent).not.toContain("C mới");
    expect(writes()).toHaveLength(0);
  });
});

describe("Độ rộng trên biểu mẫu — in the field dialog", () => {

  it("add: the type's default is checked (singleText half, multiText full) and null is sent when untouched", async () => {
    H.phien = session(["admin.citizen_field"]);
    answers["POST /api/v1/citizen-fields"] = { status: 201, body: {} };
    const el = await mount();
    const dlg = openAdd(el);
    expect(dlg.querySelector("[data-layout-width] legend")!.textContent).toBe(form.LAYOUT_WIDTH_LABEL);
    expect(widthRadios(dlg).map((r) => [r.value, r.checked, r.parentElement!.textContent])).toEqual([
      ["half", true, "Nửa hàng"],
      ["full", false, "Cả hàng"],
    ]);
    selectValue(dlg.querySelector<HTMLSelectElement>("#citizen-field-type")!, "multiText");
    expect(widthRadios(dlg).map((r) => r.checked)).toEqual([false, true]);
    typeInto(labelOf(dlg), "Ghi chú thêm");
    act(() => saveButton(dlg).click());
    await settle();
    expect(JSON.parse(writes()[0]!.body!).layout_width).toBeNull();
  });

  it("add: a width the person picked is sent", async () => {
    H.phien = session(["admin.citizen_field"]);
    answers["POST /api/v1/citizen-fields"] = { status: 201, body: {} };
    const el = await mount();
    const dlg = openAdd(el);
    typeInto(labelOf(dlg), "Nghề nghiệp");
    act(() => widthRadios(dlg)[1]!.click());
    act(() => saveButton(dlg).click());
    await settle();
    expect(JSON.parse(writes()[0]!.body!).layout_width).toBe("full");
  });

  it("edit: shows the stored width; changing it PATCHes layout_width only", async () => {
    H.phien = session(["admin.citizen_field"]);
    answers["PATCH /api/v1/citizen-fields/sonhan01"] = { status: 200, body: NUMBER };
    items = [BASE, { ...NUMBER, layout_width: "half" }, DATE, RETIRED];
    const el = await mount();
    act(() => byLabel(el, "Sửa thuộc tính Số nhân khẩu").click());
    const dlg = document.querySelector<HTMLFormElement>("form[aria-label='Sửa thuộc tính']")!;
    expect(widthRadios(dlg).map((r) => r.checked)).toEqual([true, false]);
    act(() => widthRadios(dlg)[1]!.click());
    act(() => button(dlg, "Lưu").click());
    await settle();
    const patch = writes().find((c) => c.method === "PATCH")!;
    expect(JSON.parse(patch.body!)).toEqual({ ...UNCHANGED, layout_width: "full" });
  });
});
