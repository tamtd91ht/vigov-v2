// @vitest-environment jsdom
//
// jsdom for this file: pressing a "?" must open its description and must never reach the network.
// Those are events, which a markup string cannot show.

import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import { pendingMarkerLabel } from "@/components/ui/pending-feature";

import { ScanAttachmentField, SuggestedDeadlineHint } from "./meeting-pending";
import { PHAN_CHUA_DUNG } from "./nhan-bien-ban";

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
});

function mount(node: ReactNode): HTMLDivElement {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() => r.render(node));
  return host;
}

function entry(prefix: string) {
  const e = PHAN_CHUA_DUNG.find((p) => p.ten.startsWith(prefix));
  if (e === undefined) throw new Error(prefix);
  return e;
}

describe("chỗ giữ “?” của màn Biên bản (ADR 0068 §14)", () => {
  it("ô Tệp đính kèm: ô tệp vô hiệu; “?” mang tên đầy đủ, bấm mở đúng lý do, không gọi mạng", () => {
    const fetchSpy = vi.fn();
    vi.stubGlobal("fetch", fetchSpy);
    const el = mount(<ScanAttachmentField />);
    const input = el.querySelector<HTMLInputElement>('input[type="file"]')!;
    expect(input.disabled).toBe(true);
    expect(input.name).toBe("");

    const muc = entry("Tệp đính kèm");
    const marker = el.querySelector<HTMLButtonElement>("button[data-pending-marker]")!;
    expect(marker.getAttribute("aria-label")).toBe(pendingMarkerLabel(muc.ten));
    act(() => marker.click());
    expect(document.body.querySelector('[role="dialog"]')?.textContent).toContain(muc.viSao);
    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it("dòng Hạn gợi ý: không gợi ý ngày nào; “?” mở lý do chờ khách, không gọi mạng", () => {
    const fetchSpy = vi.fn();
    vi.stubGlobal("fetch", fetchSpy);
    const el = mount(<SuggestedDeadlineHint />);
    expect(el.querySelector('[aria-disabled="true"]')).not.toBeNull();
    expect(el.textContent).not.toMatch(/\d{1,2}\/\d{1,2}/);

    const muc = entry("Hạn gợi ý");
    const marker = el.querySelector<HTMLButtonElement>("button[data-pending-marker]")!;
    expect(marker.getAttribute("aria-label")).toBe(pendingMarkerLabel(muc.ten));
    act(() => marker.click());
    expect(document.body.querySelector('[role="dialog"]')?.textContent).toContain(muc.viSao);
    expect(muc.viSao).toContain("chờ đơn vị quyết định");
    expect(fetchSpy).not.toHaveBeenCalled();
  });
});
