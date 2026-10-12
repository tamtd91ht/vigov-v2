// @vitest-environment jsdom
//
// jsdom: the row switch and the delete confirm are behaviour — clicked, read from the call made and from the toasts raised.
// The markup itself is pinned in `map-field-form.test.tsx`.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import type { PhienDaDoc } from "@/features/phien/phien-hien-tai";
import type { KetQua } from "@/lib/api/goi";
import type { comms_loaiTaiNguyenRa, comms_mapFieldSchemaOut } from "@/lib/api/schema.gen";

const H = vi.hoisted(() => ({
  phien: null as PhienDaDoc,
  update: vi.fn(),
  list: vi.fn(),
  remove: vi.fn(),
}));
vi.mock("@/features/phien/phien-hien-tai", () => ({ usePhien: () => H.phien }));
const T = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));
vi.mock("sonner", () => ({ toast: T }));
vi.mock("@/lib/api/danh-muc-nghiep-vu", () => ({
  layLoaiTaiNguyenBanDo: async () => ({
    ok: true,
    duLieu: { items: [{ code: "doanh-nghiep", label: "Doanh nghiệp", active: true } as unknown as comms_loaiTaiNguyenRa] },
  }),
}));
vi.mock("@/lib/api/map-field-schemas", () => ({
  listMapFields: H.list,
  updateMapField: H.update,
  createMapField: vi.fn(),
  deleteMapField: H.remove,
}));

const { MapFieldTab } = await import("./map-field-tab");

const ROW: comms_mapFieldSchemaOut = {
  id: "01JMF1",
  asset_type_code: "doanh-nghiep",
  field_code: "legal_form",
  label: "Loại hình doanh nghiệp",
  value_type: "text",
  options: [],
  is_required: false,
  sort_order: 1,
  is_active: true,
};

function session(permissions: string[]): PhienDaDoc {
  return { ok: true, duLieu: { staff: { full_name: "A", position: "B" }, role: null, permissions } } as unknown as PhienDaDoc;
}

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

let root: Root | null = null;
let host: HTMLDivElement | null = null;

beforeEach(() => {
  T.success.mockReset();
  T.error.mockReset();
  H.update.mockReset();
  H.list.mockReset();
  H.remove.mockReset();
  H.list.mockResolvedValue({ ok: true, duLieu: { items: [ROW] } } satisfies KetQua<{ items: comms_mapFieldSchemaOut[] }>);
});

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  H.phien = null;
});

async function settle() {
  for (let i = 0; i < 6; i++) await act(async () => {});
}

async function mount(): Promise<HTMLDivElement> {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() => r.render(<MapFieldTab />));
  await settle();
  return host;
}

function switchOff(el: HTMLElement): HTMLButtonElement {
  const b = el.querySelector<HTMLButtonElement>('button[aria-label="Tắt trường Loại hình doanh nghiệp"]');
  expect(b, "the row's Tắt").not.toBeNull();
  return b!;
}

describe("Tắt / Bật a field (ADR 0079 lô 6 #3, prototype AssetFieldTable.tsx:236-238)", () => {
  it("success: PATCH { is_active: false }, the list is READ AGAIN, and NO toast", async () => {
    H.phien = session(["asset.read", "admin.lookup"]);
    const el = await mount();
    H.update.mockResolvedValue({ ok: true, duLieu: { ...ROW, is_active: false } });
    const readsBefore = H.list.mock.calls.length;
    act(() => switchOff(el).click());
    await settle();
    expect(H.update).toHaveBeenCalledWith("01JMF1", { is_active: false });
    expect(H.list.mock.calls.length).toBeGreaterThan(readsBefore);
    expect(T.success).not.toHaveBeenCalled();
  });

  it("refusal: the server's sentence shows, no success toast", async () => {
    H.phien = session(["asset.read", "admin.lookup"]);
    const el = await mount();
    H.update.mockResolvedValue({ ok: false, thongBao: "Bạn không có quyền thực hiện thao tác này." });
    act(() => switchOff(el).click());
    await settle();
    expect(el.textContent).toContain("Bạn không có quyền thực hiện thao tác này.");
    expect(T.success).not.toHaveBeenCalled();
  });
});

function trash(el: HTMLElement): HTMLButtonElement {
  const b = el.querySelector<HTMLButtonElement>('table button[aria-label="Xoá trường Loại hình doanh nghiệp"]');
  expect(b, "the row's Trash2").not.toBeNull();
  return b!;
}

function confirmButton(dialog: HTMLDialogElement): HTMLButtonElement {
  const b = Array.from(dialog.querySelectorAll<HTMLButtonElement>('button[type="submit"]')).find(
    (x) => x.textContent === "Xoá",
  );
  expect(b, "the dialog's Xoá").not.toBeUndefined();
  return b!;
}

describe("Xoá a field: a confirm DIALOG, not a row under the table (S33)", () => {
  it("Trash2 opens a dialog with the question; nothing is sent before Xoá", async () => {
    H.phien = session(["asset.read", "admin.lookup"]);
    const el = await mount();
    expect(el.querySelector("dialog")).toBeNull();
    act(() => trash(el).click());
    const dialog = el.querySelector("dialog");
    expect(dialog).not.toBeNull();
    expect(dialog!.textContent).toContain("Xoá trường Loại hình doanh nghiệp?");
    expect(dialog!.querySelector('[name="reason"]')).toBeNull();
    // REGRESSION: no confirm row inserted into the table any more.
    expect(el.querySelector("table form")).toBeNull();
    expect(H.remove).not.toHaveBeenCalled();
  });

  it("Xoá calls deleteMapField(id), closes the dialog, toasts and reads the list again", async () => {
    H.phien = session(["asset.read", "admin.lookup"]);
    const el = await mount();
    H.remove.mockResolvedValue({ ok: true, duLieu: null });
    act(() => trash(el).click());
    const readsBefore = H.list.mock.calls.length;
    act(() => confirmButton(el.querySelector("dialog")!).click());
    await settle();
    expect(H.remove).toHaveBeenCalledTimes(1);
    expect(H.remove).toHaveBeenCalledWith("01JMF1");
    expect(el.querySelector("dialog")).toBeNull();
    expect(T.success).toHaveBeenCalledWith("Đã xoá trường.");
    expect(H.list.mock.calls.length).toBeGreaterThan(readsBefore);
  });

  it("refusal: the server's sentence shows INSIDE the dialog, which stays open; not in the table", async () => {
    H.phien = session(["asset.read", "admin.lookup"]);
    const el = await mount();
    H.remove.mockResolvedValue({ ok: false, thongBao: "Không tìm thấy trường." });
    act(() => trash(el).click());
    act(() => confirmButton(el.querySelector("dialog")!).click());
    await settle();
    const dialog = el.querySelector("dialog");
    expect(dialog).not.toBeNull();
    expect(dialog!.querySelector('[role="alert"]')?.textContent).toBe("Không tìm thấy trường.");
    expect(el.querySelector("table")!.textContent).not.toContain("Không tìm thấy trường.");
    expect(T.success).not.toHaveBeenCalled();
  });

  it("Huỷ closes the dialog without a request", async () => {
    H.phien = session(["asset.read", "admin.lookup"]);
    const el = await mount();
    act(() => trash(el).click());
    const huy = Array.from(el.querySelectorAll<HTMLButtonElement>("dialog button")).find((b) => b.textContent === "Huỷ");
    act(() => huy!.click());
    expect(el.querySelector("dialog")).toBeNull();
    expect(H.remove).not.toHaveBeenCalled();
  });

  it("DENIED: without admin.lookup there is no Trash2 and no dialog", async () => {
    H.phien = session(["asset.read"]);
    const el = await mount();
    expect(el.querySelector('button[aria-label="Xoá trường Loại hình doanh nghiệp"]')).toBeNull();
    expect(el.querySelector("dialog")).toBeNull();
  });
});
