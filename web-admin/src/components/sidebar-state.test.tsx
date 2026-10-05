// @vitest-environment jsdom
//
// jsdom: the collapse choice is read from and written to `localStorage`, and must survive a reload.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import { SIDEBAR_STORAGE_KEY, useSidebarCollapsed } from "./sidebar-state";

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

let root: Root | null = null;
let host: HTMLDivElement | null = null;

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  vi.restoreAllMocks();
  window.localStorage.clear();
});

function Probe() {
  const { collapsed, toggle } = useSidebarCollapsed();
  return (
    <button type="button" data-collapsed={collapsed ? "1" : "0"} onClick={toggle}>
      toggle
    </button>
  );
}

/** Mounting afresh = a page load: the store re-reads storage when its first listener subscribes. */
function mount(): HTMLButtonElement {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() => r.render(<Probe />));
  return host.querySelector("button")!;
}

function unmount() {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
}

describe("sidebar collapse — remembered per browser on this commune's host", () => {
  it("expanded by default when nothing was stored", () => {
    expect(mount().dataset.collapsed).toBe("0");
  });

  it("toggling collapses, writes the choice, and a reload reads it back", () => {
    const button = mount();
    act(() => button.click());
    expect(button.dataset.collapsed).toBe("1");
    expect(window.localStorage.getItem(SIDEBAR_STORAGE_KEY)).toBe("1");
    unmount();
    expect(mount().dataset.collapsed).toBe("1");
  });

  it("toggling back expands and stores that too", () => {
    window.localStorage.setItem(SIDEBAR_STORAGE_KEY, "1");
    const button = mount();
    expect(button.dataset.collapsed).toBe("1");
    act(() => button.click());
    expect(button.dataset.collapsed).toBe("0");
    expect(window.localStorage.getItem(SIDEBAR_STORAGE_KEY)).toBe("0");
  });

  it("storage blocked: no throw — expanded on load, and the toggle still works for this page", () => {
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("blocked");
    });
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("blocked");
    });
    const button = mount();
    expect(button.dataset.collapsed).toBe("0");
    act(() => button.click());
    expect(button.dataset.collapsed).toBe("1");
  });
});
