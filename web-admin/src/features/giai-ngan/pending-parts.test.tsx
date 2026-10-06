// @vitest-environment jsdom
//
// jsdom for this file: pressing a "?" must open its description, and NOTHING on these screens'
// placeholders may reach the network or storage. Those are events, which a markup string cannot show.

import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import { pendingMarkerLabel } from "@/components/ui/pending-feature";

import { FormDuAn } from "./ghi-du-an";
import { FORM_DU_AN_TRONG, PHAN_CHUA_DUNG_GHI } from "./nhan-ghi-giai-ngan";
import {
  DisbursementHeaderActions,
  DisbursementOverviewPending,
  ProjectFilterPending,
  ProjectFundingPending,
  ProjectRecordTabs,
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
      <DisbursementHeaderActions />
      <DisbursementOverviewPending />
      <ProjectFilterPending />
      <ProjectFundingPending />
      <ProjectRecordTabs>
        <p>panel</p>
      </ProjectRecordTabs>
      <FormDuAn
        budgetYear={2026}
        tieuDeForm="Thêm dự án"
        giaTriDau={FORM_DU_AN_TRONG}
        danhMuc={[]}
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

  it("a disabled tab never becomes selected; Chứng từ stays the selected tab", () => {
    const el = mount(
      <ProjectRecordTabs>
        <p>panel</p>
      </ProjectRecordTabs>,
    );
    const tabs = [...el.querySelectorAll<HTMLButtonElement>('[role="tab"]')];
    expect(tabs.map((t) => t.textContent)).toEqual(["Vướng mắc", "Chứng từ", "Biểu đồ", "Trao đổi"]);
    for (const t of tabs) act(() => t.click());
    expect(tabs.filter((t) => t.getAttribute("aria-selected") === "true").map((t) => t.textContent)).toEqual([
      "Chứng từ",
    ]);
    expect(el.querySelector('[role="tabpanel"]')?.textContent).toBe("panel");
  });
});
