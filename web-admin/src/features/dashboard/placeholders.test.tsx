// @vitest-environment jsdom
//
// jsdom: every "?" on the overview must open its description on click and reach no server.

import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import { PHASE_2_NOTE, pendingMarkerLabel } from "@/components/ui/pending-feature";

import { PHAN_CHUA_DUNG } from "./labels";
import { periodWindows } from "./period";
import { blockVisibility, DashboardView } from "./view";
import type { DashboardData, SummaryPair } from "./view";

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

const none: SummaryPair<never> = { current: null, previous: null };
const DATA: DashboardData = {
  windows: periodWindows("month", new Date("2026-09-28T09:43:00Z")),
  fetchedAt: Date.parse("2026-09-28T09:43:00Z"),
  tasks: none,
  incomingDocuments: none,
  citizenReports: none,
  fiscal: null,
  fiscalYear: 2026,
  queue: { ok: true, duLieu: { rows: [], failures: [] } },
  taskTypeLabels: null,
};

const ALL_KEYS = blockVisibility(["report.read", "task.read", "document.read", "feedback.read", "budget.read"]);

function page(visible = ALL_KEYS): HTMLDivElement {
  // The view draws its own header (PDF/XLSX/PPTX, Trình chiếu) — the prototype's frame.
  return mount(<DashboardView data={DATA} visible={visible} onPeriodChange={() => {}} />);
}

function marker(el: ParentNode, ten: string): HTMLButtonElement | null {
  return el.querySelector<HTMLButtonElement>(`button[aria-label="${pendingMarkerLabel(ten)}"]`);
}

describe("Tổng quan — unbuilt parts at their spec position (ADR 0068 §14)", () => {
  it("every PHAN_CHUA_DUNG entry has its '?' on the page (all keys)", () => {
    const el = page();
    for (const p of PHAN_CHUA_DUNG) expect(marker(el, p.ten)).not.toBeNull();
  });

  it("without an export source PDF / XLSX / PPTX are DISABLED; Trình chiếu is DISABLED; Tính lại ngay has no spot (ADR 0053)", () => {
    const el = page();
    for (const label of ["PDF", "XLSX", "PPTX", "Trình chiếu"]) {
      const b = [...el.querySelectorAll("button")].find((x) => x.textContent?.trim() === label);
      expect(b?.disabled).toBe(true);
    }
    expect(el.textContent).not.toContain("Tính lại ngay");
    expect(el.textContent).not.toContain("cũ hơn 10 phút");
  });

  it("the unbuilt KPI card shows '—', never a figure and never a link", () => {
    const el = page();
    const card = marker(el, "Đơn thư trong kỳ")!.closest("[data-pending]")!;
    expect(card.textContent).toContain("—");
    expect(card.querySelector("a")).toBeNull();
  });

  it("'Điểm hài lòng' is a real figure now (owner, 02/10/2026): no '?' for it anywhere", () => {
    const el = page();
    expect(marker(el, "Điểm hài lòng")).toBeNull();
    expect(PHAN_CHUA_DUNG.some((p) => p.ten === "Điểm hài lòng")).toBe(false);
  });

  it("DENIED: without budget.read the 'Giải ngân ngân sách' placeholder is not drawn either", () => {
    const el = page(blockVisibility(["report.read"]));
    expect(marker(el, "Giải ngân ngân sách")).toBeNull();
    expect(marker(el, "Đơn thư trong kỳ")).toBeNull();
    // no key of its own: shown under the page gate alone
    expect(marker(el, "Kinh tế & Tài nguyên")).not.toBeNull();
  });

  it("pressing a '?' opens its description (Phase 2 said where it is) and calls no server", () => {
    const fetchSpy = vi.fn();
    vi.stubGlobal("fetch", fetchSpy);
    const el = page();
    act(() => marker(el, "Chế độ trình chiếu phòng họp")!.click());
    const dialog = document.body.querySelector('[role="dialog"]');
    expect(dialog?.textContent).toContain(PHAN_CHUA_DUNG.find((p) => p.ten === "Chế độ trình chiếu phòng họp")!.viSao);
    expect(dialog?.textContent).toContain(PHASE_2_NOTE);
    expect(fetchSpy).not.toHaveBeenCalled();
  });
});
