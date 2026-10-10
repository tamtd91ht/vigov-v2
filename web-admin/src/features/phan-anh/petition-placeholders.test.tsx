// @vitest-environment jsdom
//
// jsdom for this file: a placeholder (ADR 0068 §14) is proven by PRESSING its "?" — the description
// opens and nothing reaches the network. A markup string cannot show either.

import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import { pendingMarkerLabel } from "@/components/ui/pending-feature";

import { petitionPendingPart } from "./nhan-phieu";
import { HangLoc } from "./so-phan-anh";

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
  it("scope: `Liên quan đến tôi` is disabled and never changes the filter", () => {
    const datLoc = vi.fn();
    const el = mount(<HangLoc loc={{}} tim="" datTim={() => {}} datLoc={datLoc} thon={[]} />);
    const related = [...el.querySelectorAll<HTMLButtonElement>('[role="group"] button')].find(
      (b) => b.textContent === "Liên quan đến tôi",
    );
    expect(related?.disabled).toBe(true);
    act(() => related?.click());
    expect(datLoc).not.toHaveBeenCalled();
    pressAll(el, ["scopeRelated"]);
    expect(datLoc).not.toHaveBeenCalled();
  });

  it("drawer: the duplicates block is built (ADR 0087) — it has no '?' any more", () => {
    expect(() => petitionPendingPart("duplicates")).toThrow();
  });

  it("the parts built on 09/10/2026 left the list: no '?' for the maps, the report, the composer files, the log", () => {
    for (const gone of ["sceneMap", "heatMapTab", "reportTab", "intakeHamlet", "composerAttachment", "logVisibility", "logFileRemoval"]) {
      expect(() => petitionPendingPart(gone), gone).toThrow();
    }
  });
});
