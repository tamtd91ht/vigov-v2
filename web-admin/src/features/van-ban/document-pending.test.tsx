// @vitest-environment jsdom
//
// jsdom for this file: pressing a "?" must open its description and must never reach the network.
// Those are events, which a markup string cannot show.

import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import { pendingMarkerLabel } from "@/components/ui/pending-feature";

import { DOCUMENT_PANEL_ID, DocumentTabs, ScanOcrButton, StatusChangeRow } from "./document-pending";
import { PHAN_CHUA_DUNG } from "./nhan-van-ban";

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

function markers(el: ParentNode): HTMLButtonElement[] {
  return [...el.querySelectorAll<HTMLButtonElement>("button[data-pending-marker]")];
}

function dialogText(): string {
  return document.body.querySelector('[role="dialog"]')?.textContent ?? "";
}

describe("PHAN_CHUA_DUNG của màn Văn bản", () => {
  it("đúng bốn phần đã duyệt, mỗi mục có lý do", () => {
    expect(PHAN_CHUA_DUNG.map((p) => p.ten)).toEqual([
      "Đơn thư công dân",
      "Báo cáo",
      "Quét & OCR",
      "Chuyển trạng thái văn bản đến",
    ]);
    for (const p of PHAN_CHUA_DUNG) expect(p.viSao.trim()).not.toBe("");
  });
});

describe("chỗ giữ “?” của màn Văn bản & Đơn thư (ADR 0068 §14)", () => {
  it("thanh tab: “Văn bản” là tab sống đang chọn; hai tab kia vô hiệu, ngoài thứ tự Tab, có “?”", () => {
    const el = mount(
      <DocumentTabs>
        <p>nội dung sổ</p>
      </DocumentTabs>,
    );
    const tabs = [...el.querySelectorAll<HTMLButtonElement>('[role="tab"]')];
    expect(tabs.map((t) => t.textContent)).toEqual(["Văn bản", "Đơn thư công dân", "Báo cáo"]);
    expect(tabs[0]!.getAttribute("aria-selected")).toBe("true");
    expect(tabs[0]!.disabled).toBe(false);
    for (const t of tabs.slice(1)) {
      expect(t.disabled).toBe(true);
      expect(t.getAttribute("aria-selected")).toBe("false");
      expect(t.tabIndex).toBe(-1);
    }
    // The registers keep rendering inside the one live panel — behaviour unchanged.
    const panel = el.querySelector(`#${DOCUMENT_PANEL_ID}`)!;
    expect(panel.getAttribute("role")).toBe("tabpanel");
    expect(panel.textContent).toBe("nội dung sổ");
    expect(markers(el).map((m) => m.getAttribute("aria-label"))).toEqual([
      pendingMarkerLabel("Đơn thư công dân"),
      pendingMarkerLabel("Báo cáo"),
    ]);
  });

  it("nút Quét & OCR: nút thật vô hiệu, “?” mở lý do chờ cơ quan quyết", () => {
    const el = mount(<ScanOcrButton />);
    const real = el.querySelector<HTMLButtonElement>("button:not([data-pending-marker])")!;
    expect(real.disabled).toBe(true);
    expect(real.textContent).toBe("Quét & OCR");
    act(() => markers(el)[0]!.click());
    expect(dialogText()).toContain("chờ cơ quan quyết định");
  });

  it("hàng chuyển trạng thái: bốn nút vô hiệu theo bộ trạng thái riêng của văn bản đến, một “?”", () => {
    const el = mount(<StatusChangeRow />);
    const buttons = [...el.querySelectorAll<HTMLButtonElement>("button:not([data-pending-marker])")];
    expect(buttons.map((b) => b.getAttribute("aria-label"))).toEqual([
      "Chuyển sang Chờ trình/phân luồng",
      "Chuyển sang Đã chuyển xử lý",
      "Chuyển sang Đang xử lý",
      "Chuyển sang Hoàn thành",
    ]);
    for (const b of buttons) expect(b.disabled).toBe(true);
    // Not the petition-letter labels the spec drew: the customer gave incoming documents their own.
    expect(el.textContent).not.toContain("Chuyển cấp trên");
    expect(markers(el)).toHaveLength(1);
    expect(markers(el)[0]!.getAttribute("aria-label")).toBe(pendingMarkerLabel("Chuyển trạng thái văn bản đến"));
  });

  it("bấm mọi “?” và mọi nút vô hiệu không gọi mạng, không lưu gì", () => {
    const fetchSpy = vi.fn();
    vi.stubGlobal("fetch", fetchSpy);
    const setItem = vi.spyOn(Storage.prototype, "setItem");
    const el = mount(
      <>
        <ScanOcrButton />
        <DocumentTabs>
          <StatusChangeRow />
        </DocumentTabs>
      </>,
    );
    for (const b of el.querySelectorAll<HTMLButtonElement>("button")) act(() => b.click());
    for (const m of markers(el)) act(() => m.click());
    expect(fetchSpy).not.toHaveBeenCalled();
    expect(setItem).not.toHaveBeenCalled();
  });
});
