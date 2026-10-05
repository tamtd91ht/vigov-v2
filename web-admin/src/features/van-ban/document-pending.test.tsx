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
  PETITION_COLUMNS,
  PendingPetitionRegister,
  PendingPetitionReport,
  PetitionHeaderActions,
  RaiseTaskButton,
  ScanOcrButton,
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
  it("đúng tám phần, mỗi mục có lý do", () => {
    expect(PHAN_CHUA_DUNG.map((p) => p.ten)).toEqual([
      "Đơn thư công dân",
      "Báo cáo",
      "Quét & OCR",
      "Chuyển trạng thái văn bản đến",
      "Lọc Giao cho tôi / Liên quan đến tôi",
      "Nhập hàng loạt từ Excel",
      "Xuất sổ văn bản đến",
      "Chuyển thành nhiệm vụ",
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
  it("nút Quét & OCR: nút thật vô hiệu, “?” mở lý do chờ cơ quan quyết", () => {
    const el = mount(<ScanOcrButton />);
    const real = realButtons(el)[0]!;
    expect(real.disabled).toBe(true);
    expect(real.textContent).toBe("Quét & OCR");
    act(() => markers(el)[0]!.click());
    expect(dialogText()).toContain("chờ cơ quan quyết định");
  });

  it("đầu trang tab Đơn thư: [Vào sổ đơn thư] hoặc [Nhập từ Excel] — cả hai vô hiệu", () => {
    const el = mount(<PetitionHeaderActions />);
    const buttons = realButtons(el);
    expect(buttons.map((b) => b.textContent)).toEqual(["Vào sổ đơn thư", "Nhập từ Excel"]);
    for (const b of buttons) expect(b.disabled).toBe(true);
    expect(el.textContent).toContain("hoặc");
  });

  it("tab Đơn thư: bộ lọc và bảng của prototype, mọi ô vô hiệu, thân bảng là câu lý do — không một dòng giả", () => {
    const el = mount(<PendingPetitionRegister />);
    const heads = [...el.querySelectorAll("th")].map((th) => th.textContent);
    expect(heads).toEqual([...PETITION_COLUMNS]);
    for (const c of el.querySelectorAll<HTMLInputElement | HTMLSelectElement>("input, select")) {
      expect(c.disabled).toBe(true);
    }
    for (const b of el.querySelectorAll<HTMLButtonElement>('[role="group"] button')) expect(b.disabled).toBe(true);
    const rows = el.querySelectorAll("tbody tr");
    expect(rows).toHaveLength(1);
    expect(rows[0]!.textContent).toBe(PHAN_CHUA_DUNG[0]!.viSao);
    // The duplicate-letter reminder the prototype describes does not exist — not said.
    expect(el.textContent).not.toContain("nhắc khi một người gửi lại đơn");
  });

  it("tab Báo cáo: khung prototype, KHÔNG một con số nào", () => {
    const el = mount(<PendingPetitionReport year={2026} />);
    expect(el.textContent).toContain("Tiến độ tiếp nhận và xử lý đơn thư năm 2026");
    expect(el.textContent).toContain("Tiếp nhận trong năm");
    expect(el.textContent).toContain("Theo loại đơn");
    expect(el.textContent).toContain("Tiến độ xử lý theo đơn vị");
    // A 0 would be a figure the commune reports upward. Only the year is a number on this panel.
    const withoutYear = (el.textContent ?? "").replace("2026", "");
    expect(withoutYear).not.toMatch(/\d/);
    expect(realButtons(el).find((b) => b.textContent === "Xuất Excel")?.disabled).toBe(true);
  });

  it("hàng chuyển trạng thái: bốn nút vô hiệu theo bộ trạng thái riêng của văn bản đến, một “?”", () => {
    const el = mount(<StatusChangeRow />);
    const buttons = realButtons(el);
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
        <ScanOcrButton />
        <PetitionHeaderActions />
        <PendingPetitionRegister />
        <PendingPetitionReport year={2026} />
        <StatusChangeRow />
        <RaiseTaskButton />
      </>,
    );
    for (const b of el.querySelectorAll<HTMLButtonElement>("button")) act(() => b.click());
    for (const m of markers(el)) act(() => m.click());
    expect(fetchSpy).not.toHaveBeenCalled();
    expect(setItem).not.toHaveBeenCalled();
  });
});
