// @vitest-environment jsdom
//
// jsdom for this file: pressing the "?" must open its description and must never reach the network.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import { pendingMarkerLabel } from "@/components/ui/pending-feature";

import { BudgetSheetHeaderActions, IMPORT_EXCEL } from "./header-actions";
import { PHAN_CHUA_DUNG } from "./nhan-thu-chi";

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
  vi.restoreAllMocks();
});

describe("Thu - Chi: `Nạp từ Excel` placeholder (ADR 0068 §14)", () => {
  it("is a disabled button; its '?' opens the PHAN_CHUA_DUNG sentence and calls nothing", () => {
    const fetchSpy = vi.fn();
    vi.stubGlobal("fetch", fetchSpy);
    const setItem = vi.spyOn(Storage.prototype, "setItem");

    host = document.createElement("div");
    document.body.append(host);
    const r = createRoot(host);
    root = r;
    act(() => r.render(<BudgetSheetHeaderActions />));

    const [button, marker] = [...host.querySelectorAll<HTMLButtonElement>("button")];
    expect(button!.disabled).toBe(true);
    expect(button!.textContent).toBe("Nạp từ Excel");
    act(() => button!.click());

    expect(marker!.getAttribute("aria-label")).toBe(pendingMarkerLabel(IMPORT_EXCEL));
    act(() => marker!.click());
    const entry = PHAN_CHUA_DUNG.find((p) => p.ten === IMPORT_EXCEL);
    expect(document.body.querySelector('[role="dialog"]')?.textContent).toContain(entry!.viSao);

    expect(fetchSpy).not.toHaveBeenCalled();
    expect(setItem).not.toHaveBeenCalled();
  });
});
