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
import {
  AttentionCaption,
  ProjectRecordTabs,
  TrackingTaskPending,
} from "./pending-parts";

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
      <AttentionCaption openIssueCount={2} atRiskCount={1} />
      <TrackingTaskPending />
      <ProjectRecordTabs chart={<p>chart</p>} issues={<p>issues</p>} discussion={<p>discussion</p>}>
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

  it("§9 'Tự sinh mã' is LIVE (9f0a0187): no registry entry, no '?' in the add form, no stale sentence", () => {
    expect(PHAN_CHUA_DUNG_GHI.some((p) => p.ten === "Tự sinh mã")).toBe(false);
    expect(() => pendingPart("Tự sinh mã")).toThrow();
    for (const p of PHAN_CHUA_DUNG_GHI) expect(p.viSao).not.toContain("tự sinh mã");
    const el = mount(screens());
    const box = el.querySelector<HTMLInputElement>("#tu-sinh-ma-du-an")!;
    expect(box.disabled).toBe(false);
    expect(box.closest("[data-pending]")).toBeNull();
  });

  it("§10 'Nhập giải ngân' is LIVE (f181bb76): no registry entry left", () => {
    expect(PHAN_CHUA_DUNG_GHI.some((p) => p.ten === "Nhập giải ngân từ Excel")).toBe(false);
    expect(() => pendingPart("Nhập giải ngân từ Excel")).toThrow();
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
      // The fourth card's at-risk half: LIVE since 81533bb3 (`at_risk_count`, ADR 0081 #2).
      "Nguy cơ không giải ngân hết",
    ]) {
      expect(PHAN_CHUA_DUNG_GHI.some((p) => p.ten === ten)).toBe(false);
      expect(() => pendingPart(ten)).toThrow();
    }
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

  it("§7.2 column, §8.1 and §8.4 tabs and the card's issue count are LIVE (889d4598): no entry left", () => {
    for (const ten of ["Vướng mắc mới nhất", "Vướng mắc", "Trao đổi", "Vướng mắc và nguy cơ không giải ngân hết"]) {
      expect(PHAN_CHUA_DUNG_GHI.some((p) => p.ten === ten)).toBe(false);
      expect(() => pendingPart(ten)).toThrow();
    }
    // No stale sentence claiming issues or discussions cannot be recorded.
    for (const p of PHAN_CHUA_DUNG_GHI) {
      expect(p.viSao).not.toContain("chưa ghi nhận vướng mắc");
      expect(p.viSao).not.toContain("chưa có chức năng");
    }
    // What is still NOT built is said as not built, never as a fact.
    expect(pendingPart("Tự sinh nhiệm vụ theo dõi").viSao).toContain("chưa sinh nhiệm vụ nào");
    // Mention notices are LIVE (dda12fa4): no entry may still claim nobody is notified.
    expect(PHAN_CHUA_DUNG_GHI.some((p) => p.ten === "Thông báo cho người được nhắc tên")).toBe(false);
  });

  it("tabs carry the server's counts once read, and no number before", () => {
    const counted = mount(
      <ProjectRecordTabs voucherCount={3} issueCount={2} chart={null} issues={null} discussion={null}>
        <p>panel</p>
      </ProjectRecordTabs>,
    );
    expect([...counted.querySelectorAll('[role="tab"]')].map((t) => t.textContent)).toEqual([
      "Vướng mắc (2)",
      "Chứng từ (3)",
      "Biểu đồ",
      "Trao đổi",
    ]);
    act(() => root?.unmount());
    host?.remove();

    const loading = mount(
      <ProjectRecordTabs chart={null} issues={null} discussion={null}>
        <p>panel</p>
      </ProjectRecordTabs>,
    );
    expect([...loading.querySelectorAll('[role="tab"]')].map((t) => t.textContent)).toEqual([
      "Vướng mắc",
      "Chứng từ",
      "Biểu đồ",
      "Trao đổi",
    ]);
  });

  it("all four tabs are live; Vướng mắc is the default; the other panels stay mounted, hidden", () => {
    const el = mount(
      <ProjectRecordTabs chart={<p>chart</p>} issues={<p>issues</p>} discussion={<p>discussion</p>}>
        <p>panel</p>
      </ProjectRecordTabs>,
    );
    const tabs = [...el.querySelectorAll<HTMLButtonElement>('[role="tab"]')];
    for (const t of tabs) expect(t.disabled).toBe(false);
    expect(el.querySelector("[data-pending]")).toBeNull();
    const selected = () =>
      [...el.querySelectorAll<HTMLButtonElement>('[role="tab"]')]
        .filter((t) => t.getAttribute("aria-selected") === "true")
        .map((t) => t.textContent);
    expect(selected()).toEqual(["Vướng mắc"]);
    expect(el.querySelector('[role="tabpanel"]:not([hidden])')?.textContent).toBe("issues");

    act(() => tabs[3]!.click());
    expect(selected()).toEqual(["Trao đổi"]);
    // A half-typed issue or voucher survives a look at another tab.
    expect(el.querySelector<HTMLElement>("#panel-vuong-mac-du-an")?.hidden).toBe(true);
    expect(el.querySelector("#panel-vuong-mac-du-an")?.textContent).toBe("issues");
    expect(el.querySelector<HTMLElement>("#panel-chung-tu-du-an")?.hidden).toBe(true);
    expect(el.querySelector("#panel-trao-doi-du-an")?.textContent).toBe("discussion");
  });

  it("Biểu đồ mounts only while selected; ←/→ cycle through the four tabs", () => {
    const el = mount(
      <ProjectRecordTabs chart={<p>chart</p>} issues={<p>issues</p>} discussion={<p>discussion</p>}>
        <p>panel</p>
      </ProjectRecordTabs>,
    );
    expect(el.querySelector("#panel-bieu-do-du-an")).toBeNull();
    const chartTab = el.querySelector<HTMLButtonElement>("#tab-bieu-do-du-an")!;
    act(() => chartTab.click());
    expect(chartTab.getAttribute("aria-selected")).toBe("true");
    expect(el.querySelector("#panel-bieu-do-du-an")?.textContent).toBe("chart");
    expect(el.querySelector<HTMLElement>("#panel-chung-tu-du-an")?.hidden).toBe(true);

    const key = (k: string) =>
      act(() => {
        document.activeElement!.dispatchEvent(new KeyboardEvent("keydown", { key: k, bubbles: true }));
      });
    chartTab.focus();
    key("ArrowRight");
    expect(el.querySelector("#tab-trao-doi-du-an")?.getAttribute("aria-selected")).toBe("true");
    expect(el.querySelector("#panel-bieu-do-du-an")).toBeNull();
    key("ArrowRight");
    expect(el.querySelector("#tab-vuong-mac-du-an")?.getAttribute("aria-selected")).toBe("true");
    key("ArrowLeft");
    expect(el.querySelector("#tab-trao-doi-du-an")?.getAttribute("aria-selected")).toBe("true");
    key("Home");
    expect(el.querySelector("#tab-vuong-mac-du-an")?.getAttribute("aria-selected")).toBe("true");
    key("End");
    expect(el.querySelector("#tab-trao-doi-du-an")?.getAttribute("aria-selected")).toBe("true");
  });
});
