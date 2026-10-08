// @vitest-environment jsdom
//
// jsdom: the row switch is behaviour — clicked, read from the call made and from the toasts raised.
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
  deleteMapField: vi.fn(),
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
