// @vitest-environment jsdom
//
// jsdom: the "?" must open its description on click and reach no server — events, not markup.

import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import { pendingMarkerLabel } from "@/components/ui/pending-feature";

import { BULK_ADD_BUTTON, BULK_DELETE_BUTTON, BULK_WITHDRAW_BUTTON } from "./bulk-publication";
import { SelectionBar } from "./danh-ba-lien-he";
import { pendingPart, PHAN_CHUA_DUNG } from "./nhan-danh-ba";

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

function marker(el: ParentNode, ten: string): HTMLButtonElement {
  const b = el.querySelector<HTMLButtonElement>(`button[aria-label="${pendingMarkerLabel(ten)}"]`);
  if (b === null) throw new Error(`no "?" for ${ten}`);
  return b;
}

function bar(showDelete: boolean, handlers: { onAdd?: () => void; onWithdraw?: () => void } = {}) {
  return (
    <SelectionBar
      count={3}
      busy={false}
      onAdd={handlers.onAdd ?? (() => undefined)}
      onWithdraw={handlers.onWithdraw ?? (() => undefined)}
      showDelete={showDelete}
    />
  );
}

describe("Danh bạ — the selection bar's unbuilt part at its prototype position (ADR 0068 §14)", () => {
  it("only one entry is left, and it is 'Xoá đã chọn' — the KPI and avatar entries are gone", () => {
    expect(PHAN_CHUA_DUNG.map((p) => p.ten)).toEqual([BULK_DELETE_BUTTON]);
  });

  it("the bar reads 'Đã chọn n người', then Thêm · Rút · the disabled 'Xoá đã chọn' with its '?'", () => {
    const el = mount(bar(true));
    expect(el.textContent).toContain("Đã chọn 3 người");
    const labels = [...el.querySelectorAll("button")].map((b) => b.textContent?.trim()).filter((t) => t !== "?");
    expect(labels).toEqual([BULK_ADD_BUTTON, BULK_WITHDRAW_BUTTON, BULK_DELETE_BUTTON]);
    const del = [...el.querySelectorAll("button")].find((b) => b.textContent?.includes(BULK_DELETE_BUTTON))!;
    expect(del.disabled).toBe(true);
    marker(el, BULK_DELETE_BUTTON);
  });

  it("pressing '?' opens the description and calls no server; the disabled button does nothing", () => {
    const fetchSpy = vi.fn();
    vi.stubGlobal("fetch", fetchSpy);
    const el = mount(bar(true));
    const del = [...el.querySelectorAll("button")].find((b) => b.textContent?.includes(BULK_DELETE_BUTTON))!;
    act(() => del.click());
    act(() => marker(el, BULK_DELETE_BUTTON).click());
    expect(document.body.querySelector('[role="dialog"]')?.textContent).toContain(pendingPart(BULK_DELETE_BUTTON).viSao);
    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it("without `admin.user.delete` the pending delete is not drawn at all (denied case)", () => {
    const el = mount(bar(false));
    expect(el.textContent).not.toContain(BULK_DELETE_BUTTON);
    expect(el.querySelector(`button[aria-label="${pendingMarkerLabel(BULK_DELETE_BUTTON)}"]`)).toBeNull();
  });

  it("'Thêm vào danh bạ Mini App' and 'Rút khỏi danh bạ' call the screen's handlers — no route from the bar", () => {
    const fetchSpy = vi.fn();
    vi.stubGlobal("fetch", fetchSpy);
    const calls: string[] = [];
    const el = mount(bar(true, { onAdd: () => calls.push("add"), onWithdraw: () => calls.push("withdraw") }));
    const byText = (t: string) => [...el.querySelectorAll("button")].find((b) => b.textContent?.trim() === t)!;
    act(() => byText(BULK_ADD_BUTTON).click());
    act(() => byText(BULK_WITHDRAW_BUTTON).click());
    expect(calls).toEqual(["add", "withdraw"]);
    expect(fetchSpy).not.toHaveBeenCalled();
  });
});
