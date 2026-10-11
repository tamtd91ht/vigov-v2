// @vitest-environment jsdom
//
// jsdom for this file: a placeholder (ADR 0068 §14) is proven by PRESSING its "?" — the description
// opens and nothing reaches the network. A markup string cannot show either.

import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

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

describe("Phản ánh — placeholders (ADR 0068 §14)", () => {
  it("scope: `Liên quan đến tôi` is LIVE now (row 57) — it sets `related`, and has no '?'", () => {
    const datLoc = vi.fn();
    const el = mount(<HangLoc loc={{}} tim="" datTim={() => {}} datLoc={datLoc} thon={[]} />);
    const related = [...el.querySelectorAll<HTMLButtonElement>('[role="group"] button')].find(
      (b) => b.textContent === "Liên quan đến tôi",
    );
    expect(related?.disabled).toBe(false);
    act(() => related?.click());
    expect(datLoc).toHaveBeenCalledWith({ phamVi: "related" });
    expect(() => petitionPendingPart("scopeRelated")).toThrow();
    expect(el.querySelector("button[data-pending-marker]")).toBeNull();
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
