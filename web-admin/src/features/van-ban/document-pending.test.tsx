// @vitest-environment jsdom
//
// jsdom for this file: pressing a "?" must open its description, pressing a tab must switch the
// panel, and neither may reach the network. Those are events, which a markup string cannot show.

import { act, useState, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import { pendingMarkerLabel } from "@/components/ui/pending-feature";

import {
  DOCUMENT_TABS,
  DocumentTabBar,
  IncomingRowLinks,
  IncomingScopeFilter,
  LetterImportButton,
  RaiseTaskButton,
  StatusChangeRow,
  documentTabDomId,
  type DocumentTabId,
} from "./document-pending";
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

function realButtons(el: ParentNode): HTMLButtonElement[] {
  return [...el.querySelectorAll<HTMLButtonElement>("button:not([data-pending-marker])")];
}

function dialogText(): string {
  return document.body.querySelector('[role="dialog"]')?.textContent ?? "";
}

describe("PHAN_CHUA_DUNG của màn Văn bản", () => {
  it("đúng mười một phần, mỗi mục có lý do; hai tab Đơn thư/Báo cáo đã dựng, chỉ còn Excel của chúng", () => {
    expect(PHAN_CHUA_DUNG.map((p) => p.ten)).toEqual([
      "Nhập đơn thư từ Excel",
      "Xuất báo cáo đơn thư",
      "Chuyển trạng thái văn bản đến",
      "Lọc Giao cho tôi / Liên quan đến tôi",
      "Nhập hàng loạt từ Excel",
      "Xuất sổ văn bản đến",
      "Chuyển thành nhiệm vụ",
      "Nguồn nhập văn bản đến",
      "Gợi ý cơ quan ban hành",
      "Chuyển ngay khi vào sổ",
      "Ghi chú văn bản đến",
    ]);
    for (const p of PHAN_CHUA_DUNG) expect(p.viSao.trim()).not.toBe("");
  });
});

function TabHarness() {
  const [tab, setTab] = useState<DocumentTabId>("incoming");
  return (
    <>
      <DocumentTabBar selected={tab} onSelect={setTab} />
      <p data-testid="dang-chon">{tab}</p>
    </>
  );
}

describe("thanh tab theo prototype", () => {
  it("bốn tab đúng thứ tự, đều bấm được; tab đầu (Văn bản đến) được chọn khi vào", () => {
    const el = mount(<TabHarness />);
    const tabs = [...el.querySelectorAll<HTMLButtonElement>('[role="tab"]')];
    expect(tabs.map((t) => t.textContent)).toEqual(["Văn bản đến", "Văn bản đi", "Đơn thư công dân", "Báo cáo"]);
    expect(tabs.map((t) => t.getAttribute("aria-selected"))).toEqual(["true", "false", "false", "false"]);
    for (const t of tabs) expect(t.disabled).toBe(false);
    // The prototype's counts are not drawn — a page count is not the register's count.
    for (const t of tabs) expect(t.textContent).not.toMatch(/\(\d+\)/);
  });

  it("bấm tab Đơn thư thì chọn tab ấy; mũi tên trái/phải đi vòng", () => {
    const el = mount(<TabHarness />);
    act(() => el.querySelector<HTMLButtonElement>(`#${documentTabDomId("petitions")}`)!.click());
    expect(el.querySelector('[data-testid="dang-chon"]')!.textContent).toBe("petitions");

    const list = el.querySelector<HTMLElement>('[role="tablist"]')!;
    act(() => list.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowRight", bubbles: true })));
    expect(el.querySelector('[data-testid="dang-chon"]')!.textContent).toBe("report");
    act(() => list.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowRight", bubbles: true })));
    expect(el.querySelector('[data-testid="dang-chon"]')!.textContent).toBe(DOCUMENT_TABS[0].id);
  });
});

describe("chỗ giữ “?” của màn Văn bản & Đơn thư (ADR 0068 §14)", () => {
  it("phạm vi của prototype: “Toàn xã” đang bấm, hai lựa chọn kia vô hiệu, MỘT “?”", () => {
    const el = mount(<IncomingScopeFilter />);
    const buttons = realButtons(el);
    expect(buttons.map((b) => b.textContent)).toEqual(["Toàn xã", "Giao cho tôi", "Liên quan đến tôi"]);
    expect(buttons.map((b) => b.getAttribute("aria-pressed"))).toEqual(["true", "false", "false"]);
    expect(buttons.map((b) => b.disabled)).toEqual([false, true, true]);
    expect(markers(el)).toHaveLength(1);
    act(() => markers(el)[0]!.click());
    expect(dialogText()).toContain("Sổ đang hiện toàn bộ văn bản của xã");
  });

  it("hai liên kết cuối hàng lọc: Nhập hàng loạt từ Excel · Xuất sổ {năm} — vô hiệu, mỗi cái một “?”", () => {
    const el = mount(<IncomingRowLinks year={2026} />);
    const buttons = realButtons(el);
    expect(buttons.map((b) => b.textContent)).toEqual(["Nhập hàng loạt từ Excel", "Xuất sổ 2026"]);
    for (const b of buttons) expect(b.disabled).toBe(true);
    expect(markers(el)).toHaveLength(2);
  });

  it("đầu trang tab Đơn thư: [Nhập từ Excel] vô hiệu, “?” nói chưa có đường nhập", () => {
    const el = mount(<LetterImportButton />);
    const buttons = realButtons(el);
    expect(buttons.map((b) => b.textContent)).toEqual(["Nhập từ Excel"]);
    expect(buttons[0]!.disabled).toBe(true);
    act(() => markers(el)[0]!.click());
    expect(dialogText()).toContain("chưa nhận sổ đơn thư từ tệp Excel");
  });

  it("dải trạng thái: năm bước C2 của văn bản đến, bước đang đứng sáng, mọi nút vô hiệu, một “?”", () => {
    const el = mount(<StatusChangeRow current={0} />);
    const buttons = realButtons(el);
    expect(buttons.map((b) => b.getAttribute("aria-label"))).toEqual([
      "Đã vào sổ, đang ở đây",
      "Chuyển sang Chờ trình/phân luồng",
      "Chuyển sang Đã chuyển xử lý",
      "Chuyển sang Đang xử lý",
      "Chuyển sang Hoàn thành",
    ]);
    expect(buttons[0]!.getAttribute("aria-current")).toBe("step");
    expect(buttons[0]!.textContent).toContain("đang ở đây");
    for (const b of buttons) expect(b.disabled).toBe(true);
    // Not the petition-letter labels the spec drew: the customer gave incoming documents their own.
    expect(el.textContent).not.toContain("Chuyển cấp trên");
    expect(markers(el)).toHaveLength(1);
    expect(markers(el)[0]!.getAttribute("aria-label")).toBe(pendingMarkerLabel("Chuyển trạng thái văn bản đến"));
  });

  it("Chuyển thành nhiệm vụ: nút vô hiệu, “?” nói vì sao", () => {
    const el = mount(<RaiseTaskButton />);
    expect(realButtons(el)[0]!.disabled).toBe(true);
    act(() => markers(el)[0]!.click());
    expect(dialogText()).toContain("màn Nhiệm vụ");
  });

  it("bấm mọi “?” và mọi nút vô hiệu không gọi mạng, không lưu gì", () => {
    const fetchSpy = vi.fn();
    vi.stubGlobal("fetch", fetchSpy);
    const setItem = vi.spyOn(Storage.prototype, "setItem");
    const el = mount(
      <>
        <IncomingScopeFilter />
        <IncomingRowLinks year={2026} />
        <LetterImportButton />
        <StatusChangeRow current={null} />
        <RaiseTaskButton />
      </>,
    );
    for (const b of el.querySelectorAll<HTMLButtonElement>("button")) act(() => b.click());
    for (const m of markers(el)) act(() => m.click());
    expect(fetchSpy).not.toHaveBeenCalled();
    expect(setItem).not.toHaveBeenCalled();
  });
});
