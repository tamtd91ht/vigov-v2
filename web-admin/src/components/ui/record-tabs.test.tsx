// @vitest-environment jsdom
//
// jsdom: the strip's keyboard (roving tabindex, arrows, Delete) is focus and events; the state
// rules and the storage wrappers are pure and need no DOM, but `sessionStorage` does.

import { act, useState } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import { RecordTabStrip, recordTabDomId } from "./record-tabs";
import {
  activateRecordTab,
  closeAllRecordTabs,
  closeRecordTab,
  emptyRecordTabs,
  mergeRecordTabs,
  openRecordTab,
  readStoredRecordTabs,
  renameRecordTab,
  writeStoredRecordTabs,
  type RecordTabsState,
} from "./record-tabs-state";

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

function openAll(ids: readonly string[], max?: number): RecordTabsState<string> {
  return ids.reduce((s, id) => openRecordTab(s, id, `t-${id}`, max), emptyRecordTabs<string>());
}

const ids = (s: RecordTabsState<string>) => s.tabs.map((t) => t.id);

describe("record tab state", () => {
  it("opening appends at the right and activates; re-opening activates without duplicating", () => {
    let s = openAll(["A", "B"]);
    expect(ids(s)).toEqual(["A", "B"]);
    expect(s.active).toBe("B");
    s = openRecordTab(s, "A", "t-A2");
    expect(ids(s)).toEqual(["A", "B"]);
    expect(s.active).toBe("A");
    expect(s.tabs[0]!.data).toBe("t-A2");
  });

  it("closing the active tab activates the RIGHT neighbour, else the LEFT; the last leaves none", () => {
    let s = activateRecordTab(openAll(["A", "B", "C"]), "B");
    s = closeRecordTab(s, "B");
    expect(ids(s)).toEqual(["A", "C"]);
    expect(s.active).toBe("C");
    s = closeRecordTab(s, "C");
    expect(s.active).toBe("A");
    s = closeRecordTab(s, "A");
    expect(s.tabs).toEqual([]);
    expect(s.active).toBeNull();
  });

  it("closing an inactive tab keeps the active one", () => {
    const s = closeRecordTab(openAll(["A", "B", "C"]), "A");
    expect(ids(s)).toEqual(["B", "C"]);
    expect(s.active).toBe("C");
  });

  it("the 9th tab evicts the LEAST RECENTLY USED, not the oldest opened", () => {
    let s = openAll(["1", "2", "3", "4", "5", "6", "7", "8"]);
    // `1` is the oldest opened but was just used again; `2` is now the least recently used.
    s = activateRecordTab(s, "1");
    s = openRecordTab(s, "9", "t-9");
    expect(s.tabs).toHaveLength(8);
    expect(ids(s)).not.toContain("2");
    expect(ids(s)).toContain("1");
    expect(s.active).toBe("9");
  });

  it("rename keeps the position and merges a tab already open under the new code", () => {
    const s = renameRecordTab(openAll(["A", "B", "C"]), "B", "C", "t-C2");
    expect(ids(s)).toEqual(["A", "C"]);
    expect(s.active).toBe("C");
    expect(s.tabs[1]!.data).toBe("t-C2");
  });

  it("merge folds stored tabs UNDER the open ones; the open ones win and keep the active tab", () => {
    const open = openAll(["B"]);
    const stored = openAll(["A", "B", "C"]).tabs;
    const s = mergeRecordTabs(open, stored);
    expect(ids(s)).toEqual(["A", "C", "B"]);
    expect(s.active).toBe("B");
    // The open tab is the most recent: a 9th open evicts a stored one first.
    const full = mergeRecordTabs(openAll(["X"]), openAll(["1", "2", "3", "4", "5", "6", "7", "8"]).tabs);
    expect(full.tabs).toHaveLength(8);
    expect(ids(full)).toContain("X");
    expect(ids(full)).not.toContain("1");
  });

  it("close all empties the strip", () => {
    const s = closeAllRecordTabs(openAll(["A", "B"]));
    expect(s.tabs).toEqual([]);
    expect(s.active).toBeNull();
  });
});

describe("record tab storage never throws", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    window.sessionStorage.clear();
  });

  const parse = (d: unknown) => (typeof d === "string" ? d : null);

  it("round-trips, and refuses entries that do not parse", () => {
    writeStoredRecordTabs("k", openAll(["A", "B"]).tabs);
    expect(readStoredRecordTabs("k", parse).map((t) => t.id)).toEqual(["A", "B"]);
    window.sessionStorage.setItem("k", JSON.stringify([{ id: "A", data: 1, used: 1 }, { id: "", data: "x", used: 2 }, "junk"]));
    expect(readStoredRecordTabs("k", parse)).toEqual([]);
    window.sessionStorage.setItem("k", "{not json");
    expect(readStoredRecordTabs("k", parse)).toEqual([]);
  });

  it("a throwing storage reads as empty and writes nothing", () => {
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("SecurityError");
    });
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("QuotaExceededError");
    });
    expect(readStoredRecordTabs("k", parse)).toEqual([]);
    expect(() => writeStoredRecordTabs("k", openAll(["A"]).tabs)).not.toThrow();
  });
});

describe("RecordTabStrip", () => {
  let root: Root | null = null;
  let host: HTMLDivElement | null = null;

  afterEach(() => {
    act(() => root?.unmount());
    host?.remove();
    root = null;
    host = null;
  });

  function Harness({ onCloseAll = () => {} }: { onCloseAll?: () => void }) {
    const [s, setS] = useState(() => activateRecordTab(openAll(["NV1", "NV 2", "NV3"]), "NV1"));
    return (
      <RecordTabStrip
        label="Đang mở"
        idPrefix="rt"
        panelId="p"
        tabs={s.tabs.map((t) => ({ id: t.id, title: `[${t.id}] ${t.data}`, secondary: "Mới giao" }))}
        activeId={s.active}
        onSelect={(id) => setS((x) => activateRecordTab(x, id))}
        onClose={(id) => setS((x) => closeRecordTab(x, id))}
        onCloseAll={onCloseAll}
        closeTabLabel={(id) => `Đóng tab ${id}`}
      />
    );
  }

  function mount(onCloseAll?: () => void): void {
    host = document.createElement("div");
    document.body.append(host);
    root = createRoot(host);
    act(() => root!.render(<Harness onCloseAll={onCloseAll} />));
  }

  const tabs = () => Array.from(host!.querySelectorAll<HTMLButtonElement>('[role="tab"]'));
  const key = (el: Element, k: string) =>
    act(() => {
      el.dispatchEvent(new KeyboardEvent("keydown", { key: k, bubbles: true }));
    });

  it("a tablist with ONE tab in the Tab order; ids are DOM-safe for any code", () => {
    mount();
    expect(host!.querySelector('[role="tablist"]')!.getAttribute("aria-label")).toBe("Đang mở");
    expect(tabs().map((t) => t.tabIndex)).toEqual([0, -1, -1]);
    expect(tabs().map((t) => t.getAttribute("aria-selected"))).toEqual(["true", "false", "false"]);
    expect(tabs()[1]!.id).toBe(recordTabDomId("rt", "NV 2"));
    expect(tabs()[1]!.id).not.toContain(" ");
    expect(tabs().every((t) => t.getAttribute("aria-controls") === "p")).toBe(true);
  });

  it("→ / ← / End / Home move AND activate, wrapping; focus follows", () => {
    mount();
    tabs()[0]!.focus();
    key(tabs()[0]!, "ArrowRight");
    expect(tabs()[1]!.getAttribute("aria-selected")).toBe("true");
    expect(document.activeElement).toBe(tabs()[1]);
    key(tabs()[1]!, "End");
    expect(tabs()[2]!.getAttribute("aria-selected")).toBe("true");
    key(tabs()[2]!, "ArrowRight");
    expect(tabs()[0]!.getAttribute("aria-selected")).toBe("true");
    key(tabs()[0]!, "ArrowLeft");
    expect(tabs()[2]!.getAttribute("aria-selected")).toBe("true");
    key(tabs()[2]!, "Home");
    expect(document.activeElement).toBe(tabs()[0]);
  });

  it("each ✕ names its record; Delete on a tab closes it; Đóng tất cả is a button with words", () => {
    const closeAll = vi.fn();
    mount(closeAll);
    const closers = Array.from(host!.querySelectorAll<HTMLButtonElement>('button[aria-label^="Đóng tab"]'));
    expect(closers.map((b) => b.getAttribute("aria-label"))).toEqual(["Đóng tab NV1", "Đóng tab NV 2", "Đóng tab NV3"]);
    // Only the active tab's ✕ is in the Tab order.
    expect(closers.map((b) => b.tabIndex)).toEqual([0, -1, -1]);
    act(() => closers[0]!.click());
    expect(tabs()).toHaveLength(2);
    expect(tabs()[0]!.getAttribute("aria-selected")).toBe("true");
    key(tabs()[0]!, "Delete");
    expect(tabs()).toHaveLength(1);
    const all = Array.from(host!.querySelectorAll("button")).find((b) => b.textContent === "Đóng tất cả");
    expect(all).toBeDefined();
    act(() => all!.click());
    expect(closeAll).toHaveBeenCalledTimes(1);
  });
});
