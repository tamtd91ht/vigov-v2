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
  number_kind: null,
  date_pattern: null,
  required: false,
  show_in_list: true,
  sort_order: 1,
  retired_at: null,
};
const NUMBER: CitizenField = { ...BASE, code: "sonhan01", label: "Số nhân khẩu", field_type: "number", options: [], number_kind: "integer", sort_order: 2 };
const DATE: CitizenField = { ...BASE, code: "ngaysinh", label: "Ngày sinh chủ hộ", field_type: "dateTime", options: [], date_pattern: "dd/MM/yyyy", sort_order: 3 };
const RETIRED: CitizenField = { ...BASE, code: "cucu0001", label: "Thuộc tính cũ", field_type: "singleText", options: [], sort_order: 0, retired_at: "2026-10-17T01:00:00Z" };

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

describe("add dialog", () => {
  it("refuses a bad code and an empty option list BEFORE sending; a clean draft POSTs the body with the key", async () => {
    H.phien = session(["admin.citizen_field"]);
    answers["POST /api/v1/citizen-fields"] = { status: 201, body: {} };
    const el = await mount();
    act(() => button(el, "Thêm thuộc tính").click());
    const dlg = document.querySelector<HTMLFormElement>("form[aria-label='Thêm thuộc tính']")!;
    typeInto(dlg.querySelector<HTMLInputElement>("#citizen-field-code")!, "1abc");
    typeInto(dlg.querySelector<HTMLInputElement>("#citizen-field-label")!, "Nhu cầu hỗ trợ");
    selectValue(dlg.querySelector<HTMLSelectElement>("#citizen-field-type")!, "combobox");
    act(() => button(dlg, "Lưu").click());
    await settle();
    expect(dlg.textContent).toContain(form.CODE_INVALID);
    expect(dlg.textContent).toContain(form.OPTION_VALUE_REQUIRED);
    expect(writes()).toHaveLength(0);

    typeInto(dlg.querySelector<HTMLInputElement>("#citizen-field-code")!, "nhucau01");
    typeInto(dlg.querySelector<HTMLInputElement>("#citizen-field-option-value-0")!, "dien");
    typeInto(dlg.querySelector<HTMLInputElement>("#citizen-field-option-label-0")!, "Điện");
    act(() => button(dlg, "Lưu").click());
    await settle();
    const post = writes().find((c) => c.method === "POST")!;
    expect(post.url).toBe("/api/v1/citizen-fields");
    expect(post.headers["Idempotency-Key"]).toBeTruthy();
    expect(JSON.parse(post.body!)).toEqual({
      code: "nhucau01",
      label: "Nhu cầu hỗ trợ",
      field_type: "combobox",
      placeholder: "",
      options: [{ value: "dien", label: "Điện", retired: false }],
      number_kind: null,
      date_pattern: null,
      required: false,
      show_in_list: false,
    });
    expect(H.toast.success).toHaveBeenCalledWith(form.FIELD_ADDED_TOAST);
  });

  it("a code the commune already holds (a retired one too) is refused locally", async () => {
    H.phien = session(["admin.citizen_field"]);
    const el = await mount();
    act(() => button(el, "Thêm thuộc tính").click());
    const dlg = document.querySelector<HTMLFormElement>("form[aria-label='Thêm thuộc tính']")!;
    typeInto(dlg.querySelector<HTMLInputElement>("#citizen-field-code")!, "cucu0001");
    typeInto(dlg.querySelector<HTMLInputElement>("#citizen-field-label")!, "X");
    act(() => button(dlg, "Lưu").click());
    await settle();
    expect(dlg.textContent).toContain(form.CODE_TAKEN);
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
  it("edit: code and type read-only; relabel an option + retire another → PATCH with only what changed", async () => {
    H.phien = session(["admin.citizen_field"]);
    answers["PATCH /api/v1/citizen-fields/hongheo1"] = { status: 204, body: null };
    const el = await mount();
    act(() => byLabel(el, "Sửa thuộc tính Phân loại hộ").click());
    const dlg = document.querySelector<HTMLFormElement>("form[aria-label='Sửa thuộc tính']")!;
    expect(dlg.querySelector<HTMLInputElement>("#citizen-field-code")!.readOnly).toBe(true);
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
