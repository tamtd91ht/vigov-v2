// @vitest-environment jsdom
//
// jsdom: the row switch of `tab-danh-muc.tsx` is behaviour — clicked, read from the call made and from
// the toasts raised. The markup itself is pinned in `tab-danh-muc.test.tsx`.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import type { PhienDaDoc } from "@/features/phien/phien-hien-tai";
import type { MucDanhMucGhi } from "@/lib/api/danh-muc";
import type { BayDanhMuc } from "@/lib/api/danh-muc-nghiep-vu";

import { TANG_DON_VI } from "./tang-danh-muc";

const H = vi.hoisted(() => ({
  session: null as PhienDaDoc,
  update: vi.fn(),
  reads: 0,
}));
vi.mock("@/features/phien/phien-hien-tai", () => ({ usePhien: () => H.session }));
const T = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));
vi.mock("sonner", () => ({ toast: T }));
vi.mock("@/lib/api/danh-muc", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/api/danh-muc")>()),
  suaMuc: H.update,
}));
vi.mock("@/lib/api/danh-muc-nghiep-vu", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/api/danh-muc-nghiep-vu")>()),
  docDanhMucNghiepVu: async () => {
    H.reads += 1;
    return CATALOGUES;
  },
}));
vi.mock("@/lib/api/trang-thai-nhiem-vu", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/api/trang-thai-nhiem-vu")>()),
  layTrangThaiNhiemVu: async () => ({ ok: true, duLieu: { items: [] } }),
}));

// The "Lĩnh vực phản ánh" read is not under test here (`petition-field-group.flow.test.tsx`).
vi.mock("@/lib/api/citizen-report-fields", () => ({
  readCitizenReportFields: async () => ({ ok: true, duLieu: { items: [] } }),
  updateCitizenReportField: vi.fn(),
}));

const { TabDanhMuc } = await import("./tab-danh-muc");

const ENTRY: MucDanhMucGhi = {
  id: "01JH-cong-van",
  code: "cong-van",
  label: "Công văn",
  active: true,
  is_default: false,
  order: 7,
  source: "don-vi",
  tier: TANG_DON_VI,
  color: null,
};

const empty = { ok: true as const, duLieu: { items: [] } };
const CATALOGUES = {
  loaiTaiNguyenBanDo: empty,
  hangMucKeHoachVon: empty,
  loaiVanBan: { ok: true as const, duLieu: { items: [ENTRY] } },
  loaiDonViDanCu: empty,
  khoiNhiemVu: empty,
  loaiNhiemVu: empty,
  mucUuTienNhiemVu: empty,
} as unknown as BayDanhMuc;

function sessionWith(permissions: string[]): PhienDaDoc {
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
  H.reads = 0;
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

function switchOff(el: HTMLElement): HTMLButtonElement {
  const b = [...el.querySelectorAll("button")].find((x) => x.textContent?.trim() === "Tắt");
  expect(b, "the row's Tắt").toBeDefined();
  return b!;
}

describe("Tắt / Bật an entry (ADR 0079 lô 6 #3, prototype LookupTable.tsx:188-194)", () => {
  it("success: PATCH { active: false }, the catalogues are READ AGAIN, and NO toast", async () => {
    H.session = sessionWith(["admin.lookup"]);
    const el = await mount();
    H.update.mockResolvedValue({ ok: true, duLieu: { ...ENTRY, active: false } });
    const readsBefore = H.reads;
    act(() => switchOff(el).click());
    await settle();
    expect(H.update).toHaveBeenCalledWith(expect.anything(), "01JH-cong-van", { active: false });
    expect(H.reads).toBeGreaterThan(readsBefore);
    expect(T.success).not.toHaveBeenCalled();
  });

  it("refusal: the server's sentence shows, no success toast", async () => {
    H.session = sessionWith(["admin.lookup"]);
    const el = await mount();
    H.update.mockResolvedValue({ ok: false, thongBao: "Bạn không có quyền thực hiện thao tác này." });
    act(() => switchOff(el).click());
    await settle();
    expect(el.textContent).toContain("Bạn không có quyền thực hiện thao tác này.");
    expect(T.success).not.toHaveBeenCalled();
  });
});
