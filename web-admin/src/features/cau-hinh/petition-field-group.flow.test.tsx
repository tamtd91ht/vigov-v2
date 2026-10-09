// @vitest-environment jsdom
//
// jsdom: the "Lĩnh vực phản ánh" group of the Danh mục tab (user decision 09/10/2026) — read, edit the
// label in place, switch a code off — is behaviour: clicked, read from the calls made.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import type { PhienDaDoc } from "@/features/phien/phien-hien-tai";
import type { BayDanhMuc } from "@/lib/api/danh-muc-nghiep-vu";
import type { petitions_petitionFieldOut } from "@/lib/api/schema.gen";

const H = vi.hoisted(() => ({
  session: null as PhienDaDoc,
  read: vi.fn(),
  update: vi.fn(),
}));
vi.mock("@/features/phien/phien-hien-tai", () => ({ usePhien: () => H.session }));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock("@/lib/api/danh-muc-nghiep-vu", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/api/danh-muc-nghiep-vu")>()),
  docDanhMucNghiepVu: async () => CATALOGUES,
}));
vi.mock("@/lib/api/trang-thai-nhiem-vu", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/api/trang-thai-nhiem-vu")>()),
  layTrangThaiNhiemVu: async () => ({ ok: true, duLieu: { items: [] } }),
}));
vi.mock("@/lib/api/citizen-report-fields", () => ({
  readCitizenReportFields: H.read,
  updateCitizenReportField: H.update,
}));

const { TabDanhMuc } = await import("./tab-danh-muc");

const empty = { ok: true as const, duLieu: { items: [] } };
const CATALOGUES = {
  loaiTaiNguyenBanDo: empty,
  hangMucKeHoachVon: empty,
  loaiVanBan: empty,
  loaiDonViDanCu: empty,
  khoiNhiemVu: empty,
  loaiNhiemVu: empty,
  mucUuTienNhiemVu: empty,
} as unknown as BayDanhMuc;

const FIELD: petitions_petitionFieldOut = {
  code: "giao-thong",
  label: "Hạ tầng giao thông",
  order: 2,
  default_label: "Hạ tầng giao thông",
  default_order: 2,
  icon: "",
  tone: "",
  active: true,
  enabled: true,
  customised: false,
};
const RETIRED: petitions_petitionFieldOut = { ...FIELD, code: "cu", label: "Lĩnh vực cũ", order: 9, active: false };

function sessionWith(permissions: string[]): PhienDaDoc {
  return { ok: true, duLieu: { staff: { full_name: "A", position: "B" }, role: null, permissions } } as unknown as PhienDaDoc;
}

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

let root: Root | null = null;
let host: HTMLDivElement | null = null;

beforeEach(() => {
  H.read.mockReset();
  H.update.mockReset();
  H.read.mockResolvedValue({ ok: true, duLieu: { items: [FIELD, RETIRED] } });
});

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  H.session = null;
});

async function settle() {
  for (let i = 0; i < 6; i++) await act(async () => {});
}

async function mount(): Promise<HTMLDivElement> {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() => r.render(<TabDanhMuc />));
  await settle();
  return host;
}

const buttons = (el: HTMLElement) => [...el.querySelectorAll("button")];
const rowOf = (el: HTMLElement, code: string) =>
  [...el.querySelectorAll("tbody tr")].find((tr) => tr.textContent?.includes(code)) as HTMLTableRowElement;

function type(input: HTMLInputElement, value: string) {
  const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!;
  act(() => {
    setter.call(input, value);
    input.dispatchEvent(new Event("input", { bubbles: true }));
  });
}

describe("Lĩnh vực phản ánh in Danh mục (user decision 09/10/2026)", () => {
  it("DENIED: without admin.lookup the group is never read and never shown", async () => {
    H.session = sessionWith(["task.read"]);
    const el = await mount();
    expect(H.read).not.toHaveBeenCalled();
    expect(el.textContent).not.toContain("Lĩnh vực phản ánh");
  });

  it("read: a filter button, and rows with code, label, order, status; no add, no bin, no default", async () => {
    H.session = sessionWith(["admin.lookup"]);
    const el = await mount();
    expect(H.read).toHaveBeenCalled();
    expect(buttons(el).some((b) => b.textContent === "Lĩnh vực phản ánh")).toBe(true);
    const row = rowOf(el, "giao-thong");
    expect(row.textContent).toContain("Lĩnh vực phản ánh");
    expect(row.textContent).toContain("Hạ tầng giao thông");
    expect(row.textContent).toContain("2");
    expect(row.textContent).toContain("Đang dùng");
    expect(row.querySelector('button[title="Xoá mục"]')).toBeNull();
    expect(row.textContent).not.toContain("Đặt mặc định");
    // A code the vendor retired: shown, "Ngừng dùng", and no switch.
    const retired = rowOf(el, "Lĩnh vực cũ");
    expect(retired.textContent).toContain("Ngừng dùng");
    expect([...retired.querySelectorAll("button")].some((b) => /^(Tắt|Bật)$/.test(b.textContent ?? ""))).toBe(false);
    // The add row's group select never offers it.
    act(() => buttons(el).find((b) => b.getAttribute("aria-expanded") === "false" && b.textContent?.startsWith("Thêm"))!.click());
    const options = [...el.querySelectorAll<HTMLOptionElement>("#catalogue-add-group option")].map((o) => o.textContent);
    expect(options).not.toContain("Lĩnh vực phản ánh");
  });

  it("edit in place: pencil → label box → Lưu sends PATCH {label, order} for that code", async () => {
    H.session = sessionWith(["admin.lookup"]);
    const el = await mount();
    H.update.mockResolvedValue({ ok: true, duLieu: { ...FIELD, label: "Giao thông" } });
    act(() => rowOf(el, "giao-thong").querySelector<HTMLButtonElement>('button[title="Sửa nhãn"]')!.click());
    const box = [...rowOf(el, "giao-thong").querySelectorAll("input")].find((i) => i.value === "Hạ tầng giao thông")!;
    type(box, "Giao thông");
    act(() => buttons(rowOf(el, "giao-thong")).find((b) => b.textContent === "Lưu")!.click());
    await settle();
    expect(H.update).toHaveBeenCalledWith("giao-thong", { label: "Giao thông", order: 2 });
    expect(H.read.mock.calls.length).toBeGreaterThan(1);
  });

  it("switch: Tắt sends PATCH {enabled: false} — never `active`, the vendor's flag", async () => {
    H.session = sessionWith(["admin.lookup"]);
    const el = await mount();
    H.update.mockResolvedValue({ ok: true, duLieu: { ...FIELD, enabled: false } });
    act(() => buttons(rowOf(el, "giao-thong")).find((b) => b.textContent === "Tắt")!.click());
    await settle();
    expect(H.update).toHaveBeenCalledWith("giao-thong", { enabled: false });
  });

  it("filtered to the group: Nhập từ Excel is disabled with the vendor reason, and no '?'", async () => {
    H.session = sessionWith(["admin.lookup"]);
    const el = await mount();
    act(() => buttons(el).find((b) => b.textContent === "Lĩnh vực phản ánh")!.click());
    const importButton = buttons(el).find((b) => b.textContent?.includes("Nhập từ Excel"))!;
    expect(importButton.disabled).toBe(true);
    expect(el.textContent).toContain("Mã lĩnh vực do nhà cung cấp quản lý; xã chỉ sửa nhãn, thứ tự, bật/tắt.");
    expect(el.querySelector("[data-pending-marker]")).toBeNull();
  });
});
