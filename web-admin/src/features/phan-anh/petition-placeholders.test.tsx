// @vitest-environment jsdom
//
// jsdom for this file: a placeholder (ADR 0068 §14) is proven by PRESSING its "?" — the description
// opens and nothing reaches the network. A markup string cannot show either.

import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import { pendingMarkerLabel } from "@/components/ui/pending-feature";
import type { petitions_phieuPhanAnhRa } from "@/lib/api/schema.gen";

import { congThaoTac, petitionPendingPart } from "./nhan-phieu";
import { ChiTietPhieu, HangLoc, SoPhanAnh } from "./so-phan-anh";

// No permission at all: no KPI row, no intake button — the placeholders are drawn for every account
// (they grant nothing), and nothing else on the screen competes with them.
vi.mock("@/features/phien/phien-hien-tai", () => ({
  usePhien: () => ({ ok: true, duLieu: { permissions: [] } }),
}));

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  // Radix measures the trigger to place the popover; jsdom has no ResizeObserver.
  if (!("ResizeObserver" in globalThis)) {
    (globalThis as unknown as { ResizeObserver: unknown }).ResizeObserver = class {
      observe() {}
      unobserve() {}
      disconnect() {}
    };
  }
});

/** Every read the screen starts on mount answers never: what is counted afterwards is the "?" alone. */
const fetchSpy = vi.fn(() => new Promise<Response>(() => {}));

beforeEach(() => {
  fetchSpy.mockClear();
  vi.stubGlobal("fetch", fetchSpy);
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

function marker(el: ParentNode, id: string): HTMLButtonElement {
  const label = pendingMarkerLabel(petitionPendingPart(id).ten);
  const b = [...el.querySelectorAll<HTMLButtonElement>("button[data-pending-marker]")].find(
    (x) => x.getAttribute("aria-label") === label,
  );
  if (b === undefined) throw new Error(`no '?' for ${id}`);
  return b;
}

/** Press every "?" of `ids` in turn: each opens its own description, and the network is never called. */
function pressAll(el: HTMLElement, ids: readonly string[]): void {
  fetchSpy.mockClear();
  for (const id of ids) {
    const b = marker(el, id);
    expect(b.disabled).toBe(false);
    act(() => b.click());
    expect(document.body.querySelector('[role="dialog"]')?.textContent, id).toContain(petitionPendingPart(id).viSao);
    act(() => b.click());
  }
  expect(fetchSpy).not.toHaveBeenCalled();
}

describe("Phản ánh — placeholders (ADR 0068 §14)", () => {
  it("main tabs: Danh sách selected; Bản đồ nhiệt and Báo cáo disabled, out of the tab order; no block", () => {
    const el = mount(<SoPhanAnh />);
    const tabs = [...el.querySelectorAll<HTMLButtonElement>('[role="tablist"] [role="tab"]')];
    expect(tabs.map((t) => t.textContent)).toEqual(["Danh sách", "Bản đồ nhiệt", "Báo cáo"]);
    expect(tabs[0]!.getAttribute("aria-selected")).toBe("true");
    expect(document.getElementById(tabs[0]!.getAttribute("aria-controls")!)?.getAttribute("role")).toBe("tabpanel");
    for (const t of tabs.slice(1)) {
      expect(t.disabled).toBe(true);
      expect(t.tabIndex).toBe(-1);
    }
    // The collapsed "N phần … chưa dựng được" block is gone.
    expect(el.textContent).not.toContain("chưa dựng được");
    pressAll(el, ["heatMapTab", "reportTab"]);
  });

  it("scope: `Liên quan đến tôi` is disabled and never changes the filter", () => {
    const datLoc = vi.fn();
    const el = mount(<HangLoc loc={{}} tim="" datTim={() => {}} datLoc={datLoc} boPhan={[]} thon={[]} />);
    const related = [...el.querySelectorAll<HTMLButtonElement>('[role="group"] button')].find(
      (b) => b.textContent === "Liên quan đến tôi",
    );
    expect(related?.disabled).toBe(true);
    act(() => related?.click());
    expect(datLoc).not.toHaveBeenCalled();
    pressAll(el, ["scopeRelated"]);
    expect(datLoc).not.toHaveBeenCalled();
  });

  it("drawer §8.4: the scene map is a placeholder section that loads no tile", () => {
    const petition: petitions_phieuPhanAnhRa = {
      code: "PA-2026-0021",
      channel: "zalo-mini-app",
      status: "dang-xu-ly",
      field: "rac-thai",
      field_label: "Rác thải – Vệ sinh môi trường",
      content: "Rác tồn đọng ở đầu ngõ.",
      address: "Đầu ngõ thôn Hà Lam",
      lat: 15.5,
      lng: 108.2,
      reporter_name: "",
      reporter_phone: "",
      anonymous: true,
      clock_from: "2026-09-09T07:20:00Z",
      booked_at: "2026-09-09T07:21:00Z",
      acknowledge_due: "2026-09-09T09:20:00Z",
      resolve_due: "2026-09-10T09:20:00Z",
      classify_due: "2026-09-09T11:20:00Z",
      unit: "",
      assignee: "",
      result: "",
      public: false,
    };
    const el = mount(
      <ChiTietPhieu
        phieu={petition}
        bayGio={new Date("2026-09-09T08:00:00Z")}
        cong={congThaoTac(false, false, false)}
        tenBoPhan={new Map()}
        boPhan={[]}
        danhBa={null}
        dangGui={false}
        loiGhi={null}
        dong={() => {}}
        phanLoai={() => {}}
        chuyenXuLy={() => {}}
        tienTrangThai={() => {}}
        dongPhieuLai={() => {}}
        khongTiepNhan={() => {}}
        chuyenCapTren={() => {}}
      />,
    );
    const section = marker(el, "sceneMap").closest("section");
    expect(section?.querySelector("h3")?.textContent).toBe(petitionPendingPart("sceneMap").ten);
    // No map is drawn: no image, no frame, nothing that could fetch a tile.
    expect(section?.querySelector("img, iframe")).toBeNull();
    pressAll(el, ["sceneMap"]);
  });
});
