// @vitest-environment jsdom
//
// jsdom: the "?" must open its description on click and reach no server — events, not markup.

import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import { pendingMarkerLabel } from "@/components/ui/pending-feature";
import type { BangTraDanhMuc } from "@/features/cau-hinh/tra-danh-muc";
import type { identity_canBoTomTat } from "@/lib/api/schema.gen";

import { BieuMauGhiCanBo } from "@/components/danh-ba/bieu-mau-ghi-can-bo";
import { banTuCanBo } from "@/components/danh-ba/nhan-ghi-danh-ba";

import { BangLienHe } from "./bang-lien-he";
import { pendingPart, PHAN_CHUA_DUNG } from "./nhan-danh-ba";
import { PendingStaffKpis } from "./pending-staff-kpis";
import { StaffAvatarField } from "./staff-avatar-field";

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  if (!("ResizeObserver" in globalThis)) {
    (globalThis as unknown as { ResizeObserver: unknown }).ResizeObserver = class {
      observe() {}
      unobserve() {}
      disconnect() {}
    };
  }
});

let root: Root | null = null;
let host: HTMLDivElement | null = null;

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  vi.unstubAllGlobals();
});

function mount(node: ReactNode): HTMLDivElement {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() => r.render(node));
  return host;
}

function marker(el: ParentNode, ten: string): HTMLButtonElement {
  const b = el.querySelector<HTMLButtonElement>(`button[aria-label="${pendingMarkerLabel(ten)}"]`);
  if (b === null) throw new Error(`no "?" for ${ten}`);
  return b;
}

const STAFF: identity_canBoTomTat = {
  id: "01J00000000000000000000001",
  code: "CB-00123",
  full_name: "Nguyễn Văn A",
  email: "nva@demo.invalid",
  position: "Chuyên viên",
  department_id: "",
  role_id: "",
  phone: "",
  mobile: "0900000000",
  has_account: false,
  active: true,
  last_login_at: null,
  created_at: "2026-09-01T02:00:00Z",
  has_zalo: false,
  published: false,
  display_order: null,
  consent_recorded_at: null,
};

const TRA: BangTraDanhMuc = { pha: "xong", ten: new Map() };

describe("Danh bạ — unbuilt parts at their spec position (ADR 0068 §14)", () => {
  it("two KPI cards show '—', never a figure, each with its own '?'", () => {
    const el = mount(<PendingStaffKpis />);
    expect(el.textContent).toContain("Tổng số cán bộ");
    expect(el.textContent).toContain("Đang hiện trên Mini App");
    expect([...el.querySelectorAll("p")].filter((p) => p.textContent === "—")).toHaveLength(2);
    expect(el.textContent).not.toMatch(/\d/);
    marker(el, "Tổng số cán bộ");
    marker(el, "Đang hiện trên Mini App");
  });

  it("the table has an 'Ảnh đại diện' column header with '?' and a '—' cell per row", () => {
    const el = mount(<BangLienHe danhSach={[STAFF]} traBoPhan={TRA} onSua={() => undefined} />);
    const th = [...el.querySelectorAll("th")].find((h) => h.textContent?.includes("Ảnh đại diện"));
    expect(th?.getAttribute("scope")).toBe("col");
    marker(th!, "Ảnh đại diện");
    expect(el.querySelector("td[data-pending]")?.textContent).toContain("—");
  });

  it("pressing '?' opens the entry's description and calls no server", () => {
    const fetchSpy = vi.fn();
    vi.stubGlobal("fetch", fetchSpy);
    const el = mount(<PendingStaffKpis />);
    act(() => marker(el, "Tổng số cán bộ").click());
    expect(document.body.querySelector('[role="dialog"]')?.textContent).toContain(
      pendingPart("Tổng số cán bộ").viSao,
    );
    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it("the staff form has the 'Ảnh đại diện' file field after 'Có Zalo': disabled, no name, '?', no fetch", () => {
    const fetchSpy = vi.fn();
    vi.stubGlobal("fetch", fetchSpy);
    const el = mount(
      <BieuMauGhiCanBo
        dangMo={{ kieu: "sua", canBo: STAFF }}
        ban={banTuCanBo(STAFF)}
        datBan={() => undefined}
        vaiTroID=""
        datVaiTroID={() => undefined}
        boPhan={[]}
        vaiTro={[]}
        loiMayChu=""
        dangGui={false}
        onGui={() => undefined}
        onHuy={() => undefined}
        avatarField={<StaffAvatarField />}
      />,
    );
    const file = el.querySelector<HTMLInputElement>('input[type="file"]')!;
    expect(file.id).toBe("o-anh-dai-dien-can-bo");
    expect(file.disabled).toBe(true);
    expect(file.name).toBe("");
    // Spec §5 order: after the Có Zalo checkbox.
    const zalo = el.querySelector("#o-co-zalo-can-bo")!;
    expect(zalo.compareDocumentPosition(file) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    // Nothing of it goes into the submitted form.
    expect([...new FormData(el.querySelector("form")!).keys()]).not.toContain("o-anh-dai-dien-can-bo");

    act(() => marker(el, "Ảnh đại diện").click());
    expect(document.body.querySelector('[role="dialog"]')?.textContent).toContain(pendingPart("Ảnh đại diện").viSao);
    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it("without the slot (Cấu hình's staff dialog) the form has no file field", () => {
    const el = mount(
      <BieuMauGhiCanBo
        dangMo={{ kieu: "sua", canBo: STAFF }}
        ban={banTuCanBo(STAFF)}
        datBan={() => undefined}
        vaiTroID=""
        datVaiTroID={() => undefined}
        boPhan={[]}
        vaiTro={[]}
        loiMayChu=""
        dangGui={false}
        onGui={() => undefined}
        onHuy={() => undefined}
      />,
    );
    expect(el.querySelector('input[type="file"]')).toBeNull();
  });

  it("every entry is drawn somewhere — none is a description with no '?' behind it", () => {
    const html = mount(
      <>
        <PendingStaffKpis />
        <BangLienHe danhSach={[STAFF]} traBoPhan={TRA} onSua={() => undefined} />
      </>,
    ).innerHTML;
    for (const p of PHAN_CHUA_DUNG) expect(html).toContain(pendingMarkerLabel(p.ten));
  });
});
