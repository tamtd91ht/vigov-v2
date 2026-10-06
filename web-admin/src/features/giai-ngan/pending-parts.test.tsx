// @vitest-environment jsdom
//
// jsdom for this file: pressing a "?" must open its description, and NOTHING on these screens'
// placeholders may reach the network or storage. Those are events, which a markup string cannot show.

import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import { pendingMarkerLabel } from "@/components/ui/pending-feature";

import { FormDuAn } from "./ghi-du-an";
import { FORM_DU_AN_TRONG, PHAN_CHUA_DUNG_GHI, pendingPart } from "./nhan-ghi-giai-ngan";
import { AttentionIssuesPending, DisbursementHeaderActions, ProjectRecordTabs } from "./pending-parts";

// Opening ~25 Radix popovers one by one in jsdom takes ~5s on its own and timed out under the full
// parallel suite (06/10/2026); same allowance as `task-record-tabs.test.tsx`. Not a hang guard.
vi.setConfig({ testTimeout: 30_000 });

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

let root: Root | null = null;
let host: HTMLDivElement | null = null;

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

function mount(node: ReactNode): HTMLDivElement {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() => r.render(node));
  return host;
}

function screens(): ReactNode {
  return (
    <>
      <DisbursementHeaderActions />
      <AttentionIssuesPending />
      <ProjectRecordTabs chart={<p>chart</p>}>
        <p>panel</p>
      </ProjectRecordTabs>
      <FormDuAn
        budgetYear={2026}
        tieuDeForm="Thêm dự án"
        giaTriDau={FORM_DU_AN_TRONG}
        danhMuc={[]}
        fundingCatalogue={{ phase: "ready", items: [] }}
        dangGui={false}
        loi={null}
        huy={() => {}}
        luu={() => {}}
      />
    </>
  );
}

describe("Giải ngân placeholders (ADR 0068 §14)", () => {
  it("every placeholder control is disabled; only the '?' buttons are live", () => {
    const el = mount(screens());
    const pending = [...el.querySelectorAll<HTMLElement>("[data-pending]")];
    expect(pending.length).toBeGreaterThan(0);
    for (const spot of pending) {
      for (const c of spot.querySelectorAll<HTMLButtonElement | HTMLInputElement | HTMLSelectElement>(
        "button:not([data-pending-marker]), input, select, textarea",
      )) {
        expect(c.disabled).toBe(true);
      }
    }
  });

  it("pressing each '?' opens its own description and never calls the network or storage", () => {
    const fetchSpy = vi.fn();
    vi.stubGlobal("fetch", fetchSpy);
    const setItem = vi.spyOn(Storage.prototype, "setItem");
    const el = mount(screens());

    const markers = [...el.querySelectorAll<HTMLButtonElement>("button[data-pending-marker]")];
    expect(markers.length).toBeGreaterThan(0);
    for (const m of markers) {
      const part = PHAN_CHUA_DUNG_GHI.find((p) => pendingMarkerLabel(p.ten) === m.getAttribute("aria-label"));
      expect(part).toBeDefined();
      act(() => m.click());
      const dialog = document.body.querySelector<HTMLElement>('[role="dialog"]');
      expect(dialog?.textContent).toContain(part!.viSao);
      act(() => {
        dialog!.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));
      });
    }
    expect(fetchSpy).not.toHaveBeenCalled();
    expect(setItem).not.toHaveBeenCalled();
  });

  it("§6 'Tiến độ theo nguồn vốn' is LIVE: no registry entry", () => {
    expect(PHAN_CHUA_DUNG_GHI.some((p) => p.ten === "Tiến độ theo nguồn vốn")).toBe(false);
    expect(() => pendingPart("Tiến độ theo nguồn vốn")).toThrow();
  });

  it("§3 cards, §4 chart, §5 table, §7.1 filters and §8.3 Biểu đồ are LIVE: no registry entry left", () => {
    for (const ten of [
      "Số liệu tổng hợp của năm",
      "Luỹ kế giải ngân so với kế hoạch",
      "Tiến độ theo hạng mục",
      "Chỉ dự án chậm",
      "Gộp theo hạng mục",
      "Biểu đồ",
    ]) {
      expect(PHAN_CHUA_DUNG_GHI.some((p) => p.ten === ten)).toBe(false);
      expect(() => pendingPart(ten)).toThrow();
    }
    // The fourth card's issue counts stay a "?": no issue data, no at-risk rule.
    expect(pendingPart("Vướng mắc và nguy cơ không giải ngân hết").viSao).toContain("vướng mắc");
  });

  it("project funding (§7.2 chip, §8 block, §9 list) and the §8.2 voucher list + source are LIVE: no registry entry left", () => {
    for (const ten of [
      "Hạng mục",
      "Nguồn vốn của dự án",
      "Giải ngân theo nguồn vốn",
      "Thêm nguồn vốn cho dự án",
      "Nguồn vốn của chứng từ",
      "Danh sách chứng từ của dự án",
    ]) {
      expect(PHAN_CHUA_DUNG_GHI.some((p) => p.ten === ten)).toBe(false);
      expect(() => pendingPart(ten)).toThrow();
    }
    // No stale sentence claiming the voucher list or the source catalogue does not exist.
    for (const p of PHAN_CHUA_DUNG_GHI) {
      expect(p.viSao).not.toContain("danh mục nguồn vốn");
      expect(p.viSao).not.toContain("danh sách chứng từ");
    }
  });

  it("the Chứng từ tab carries the server's count once the list is read, and no number before", () => {
    const counted = mount(
      <ProjectRecordTabs voucherCount={3} chart={null}>
        <p>panel</p>
      </ProjectRecordTabs>,
    );
    expect(counted.querySelector('[role="tab"][aria-selected="true"]')?.textContent).toBe("Chứng từ (3)");
    act(() => root?.unmount());
    host?.remove();

    const loading = mount(
      <ProjectRecordTabs chart={null}>
        <p>panel</p>
      </ProjectRecordTabs>,
    );
    expect(loading.querySelector('[role="tab"][aria-selected="true"]')?.textContent).toBe("Chứng từ");
  });

  it("a disabled tab never becomes selected; Chứng từ is the default tab", () => {
    const el = mount(
      <ProjectRecordTabs chart={<p>chart</p>}>
        <p>panel</p>
      </ProjectRecordTabs>,
    );
    const tabs = [...el.querySelectorAll<HTMLButtonElement>('[role="tab"]')];
    expect(tabs.map((t) => t.textContent)).toEqual(["Vướng mắc", "Chứng từ", "Biểu đồ", "Trao đổi"]);
    const selected = () =>
      [...el.querySelectorAll<HTMLButtonElement>('[role="tab"]')]
        .filter((t) => t.getAttribute("aria-selected") === "true")
        .map((t) => t.textContent);
    expect(selected()).toEqual(["Chứng từ"]);
    act(() => tabs[0]!.click());
    act(() => tabs[3]!.click());
    expect(selected()).toEqual(["Chứng từ"]);
    expect(el.querySelector('[role="tabpanel"]:not([hidden])')?.textContent).toBe("panel");
  });

  it("Biểu đồ is live: selecting it shows the chart panel; the voucher panel stays mounted, hidden", () => {
    const el = mount(
      <ProjectRecordTabs chart={<p>chart</p>}>
        <p>panel</p>
      </ProjectRecordTabs>,
    );
    expect(el.textContent).not.toContain("chart");
    const chartTab = el.querySelector<HTMLButtonElement>("#tab-bieu-do-du-an")!;
    expect(chartTab.disabled).toBe(false);
    act(() => chartTab.click());
    expect(chartTab.getAttribute("aria-selected")).toBe("true");
    expect(el.querySelector("#panel-bieu-do-du-an")?.textContent).toBe("chart");
    // A half-filled voucher form must survive a look at the chart.
    expect(el.querySelector<HTMLElement>("#panel-chung-tu-du-an")?.hidden).toBe(true);
    expect(el.querySelector("#panel-chung-tu-du-an")?.textContent).toBe("panel");

    // ←/→ move between the two live tabs only.
    act(() => {
      chartTab.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowRight", bubbles: true }));
    });
    expect(el.querySelector("#tab-chung-tu-du-an")?.getAttribute("aria-selected")).toBe("true");
    expect(el.querySelector("#panel-bieu-do-du-an")).toBeNull();
  });
});
