// @vitest-environment jsdom
//
// Cấu hình → Thuộc tính danh bạ (ADR 0101 #1–#5): the `admin.citizen_field` gate (denied case first), the
// ordered list with Lên / Xuống (the keyboard path of the reorder) and its rollback on refusal, the add
// dialog's validation and body, the edit dialog's read-only code and type, and "Ngừng dùng" with a reason.

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
};
const NUMBER: CitizenField = { ...BASE, code: "sonhan01", label: "Số nhân khẩu", field_type: "number", options: [], number_kind: "integer", sort_order: 2 };
const DATE: CitizenField = { ...BASE, code: "ngaysinh", label: "Ngày sinh chủ hộ", field_type: "dateTime", options: [], date_pattern: "dd/MM/yyyy", sort_order: 3 };
const RETIRED: CitizenField = { ...BASE, code: "cucu0001", label: "Thuộc tính cũ", field_type: "singleText", options: [], sort_order: 0, retired_at: "2026-10-17T01:00:00Z" };

/** The PATCH body with nothing changed: every key the contract names, null. */
const UNCHANGED = { code: null, label: null, field_type: null, placeholder: null, options: null, number_kind: null, date_pattern: null, required: null, show_in_list: null };

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
const liveOrder = (el: HTMLElement) => Array.from(el.querySelectorAll<HTMLElement>("[data-field-row]")).map((r) => r.dataset.fieldRow);
const writes = () => calls.filter((c) => c.method !== "GET");

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

  it("ALLOWED: reads the definitions WITH the retired ones; live in sort order, retired collapsed", async () => {
    H.phien = session(["admin.citizen_field"]);
    const el = await mount();
    expect(calls.map((c) => `${c.method} ${c.url}`)).toEqual(["GET /api/v1/citizen-fields?include_retired=true"]);
    expect(liveOrder(el)).toEqual(["hongheo1", "sonhan01", "ngaysinh"]);
    expect(el.querySelector("[data-field-count]")!.textContent).toBe("3/50 thuộc tính đang dùng");
    expect(el.querySelector("[data-retired-fields]")).toBeNull();
    act(() => button(el, "Đã ngừng dùng (1)").click());
    expect(el.querySelector("[data-retired-fields]")!.textContent).toContain("Thuộc tính cũ");
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

  it("a random code the commune already holds (retired included) is redrawn silently at open", async () => {
    H.phien = session(["admin.citizen_field"]);
    items = [BASE, NUMBER, DATE, RETIRED, { ...RETIRED, code: "aaaaaaaa", label: "Cũ" }];
    // First code drawn: every byte 0 → "aaaaaaaa" (held, retired); then every byte 1 → "bbbbbbbb".
    let call = 0;
    vi.spyOn(crypto, "getRandomValues").mockImplementation(<T extends ArrayBufferView | null>(buf: T): T => {
      (buf as unknown as Uint8Array).fill(call++ < form.RANDOM_CODE_LENGTH ? 0 : 1);
      return buf;
    });
    const el = await mount();
    const dlg = openAdd(el);
    expect(codeOf(dlg).value).toBe("bbbbbbbb");
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
    expect(codeOf(dlg).value).toBe("cccccccc");
    act(() => button(dlg, form.CODE_REDRAW).click());
    expect(codeOf(dlg).value).toBe("dddddddd");
    expect(codeOf(dlg).readOnly).toBe(true);
  });

  it("Sửa → editable, checked live (8–12, a letter first; taken / taken-retired disable Lưu); Sinh mã khác goes back to random", async () => {
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
    expect(dlg.textContent).toContain(form.CODE_TAKEN_RETIRED);
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
    });
    expect(H.toast.success).toHaveBeenCalledWith(form.FIELD_ADDED_TOAST);
  });

  it("a code the commune already holds (a retired one too) is refused locally — Lưu disabled, nothing sent", async () => {
    H.phien = session(["admin.citizen_field"]);
    const el = await mount();
    const dlg = openAdd(el);
    typeInto(labelOf(dlg), "X");
    act(() => byLabel(dlg, "Sửa mã thuộc tính").click());
    typeInto(codeOf(dlg), "cucu0001");
    expect(dlg.textContent).toContain(form.CODE_TAKEN_RETIRED);
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

describe("edit and retire", () => {
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

  it("Ngừng dùng: no reason → the sentence, nothing sent; with a reason → DELETE {reason}", async () => {
    H.phien = session(["admin.citizen_field"]);
    answers["DELETE /api/v1/citizen-fields/sonhan01"] = { status: 204, body: null };
    const el = await mount();
    act(() => byLabel(el, "Ngừng dùng thuộc tính Số nhân khẩu").click());
    const box = document.querySelector<HTMLFormElement>("form[aria-label='Ngừng dùng thuộc tính']")!;
    expect(box.textContent).toContain(form.RETIRE_CONSEQUENCE);
    act(() => button(box, "Ngừng dùng").click());
    await settle();
    expect(box.textContent).toContain(form.RETIRE_REASON_REQUIRED);
    expect(writes()).toHaveLength(0);
    typeInto(box.querySelector<HTMLInputElement>("#citizen-field-sonhan01-retire-reason")!, "Không thu thập nữa");
    act(() => button(box, "Ngừng dùng").click());
    await settle();
    const del = writes().find((c) => c.method === "DELETE")!;
    expect(del.url).toBe("/api/v1/citizen-fields/sonhan01");
    expect(JSON.parse(del.body!)).toEqual({ reason: "Không thu thập nữa" });
  });
});
