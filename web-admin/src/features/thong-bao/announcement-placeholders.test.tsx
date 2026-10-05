// @vitest-environment jsdom
//
// jsdom for this file: a placeholder (ADR 0068 §14) is proven by PRESSING its "?" — the description
// opens and nothing reaches the network. A markup string cannot show either.

import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import { pendingMarkerLabel } from "@/components/ui/pending-feature";
import type { comms_thongBaoRa } from "@/lib/api/schema.gen";

import { pendingPart, SAVE_DRAFT_LABEL, SCOPE_ALL, SCOPE_MINE } from "./nhan-thong-bao";
import { ChiTietThongBao, FormSoanThongBao, SoThongBao } from "./so-thong-bao";

// The screen's two reads, replaced FOR THIS FILE by answers that never arrive: the header is drawn
// while the list loads, and the only network the test can then see is what a "?" would cause.
vi.mock("@/lib/api/thong-bao", () => ({
  laySoThongBao: () => new Promise(() => {}),
  phatHanhThongBao: vi.fn(),
}));
vi.mock("@/lib/api/danh-ba-chon-nguoi", () => ({
  layDanhBaChonNguoi: () => new Promise(() => {}),
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

function announcement(): comms_thongBaoRa {
  return {
    id: "01JTB1",
    title: "Thông báo họp giao ban tuần",
    body: "Mời các bộ phận dự họp.",
    status: "da-phat-hanh",
    pinned: false,
    ack_required: true,
    email_requested: true,
    email_status: "chua-gui",
    author_code: "CB-2026-7K3M9Q",
    recipient_count: 8,
    ack_count: 0,
    issued_at: "2026-09-07T06:33:00Z",
    created_at: "2026-09-07T06:33:00Z",
  };
}

function marker(el: ParentNode, id: string): HTMLButtonElement {
  const label = pendingMarkerLabel(pendingPart(id).ten);
  const b = [...el.querySelectorAll<HTMLButtonElement>("button[data-pending-marker]")].find(
    (x) => x.getAttribute("aria-label") === label,
  );
  if (b === undefined) throw new Error(`no '?' for ${id}`);
  return b;
}

/** Press every "?" of `ids` in turn; each opens its own description; the network is never called. */
function pressAll(el: HTMLElement, ids: readonly string[]): void {
  const fetchSpy = vi.fn();
  vi.stubGlobal("fetch", fetchSpy);
  for (const id of ids) {
    const b = marker(el, id);
    expect(b.disabled).toBe(false);
    act(() => b.click());
    const d = document.body.querySelector<HTMLElement>('[role="dialog"]');
    expect(d?.textContent, id).toContain(pendingPart(id).viSao);
    act(() => b.click());
  }
  expect(fetchSpy).not.toHaveBeenCalled();
}

describe("Thông báo — placeholders (ADR 0068 §14)", () => {
  it("panel: Gỡ is a disabled button; the five '?' open their own description; no fetch", () => {
    const el = mount(<ChiTietThongBao thongBao={announcement()} />);
    const withdraw = [...el.querySelectorAll<HTMLButtonElement>("button:not([data-pending-marker])")].find(
      (b) => b.textContent === "Gỡ",
    );
    expect(withdraw?.disabled).toBe(true);
    expect(el.textContent).toContain("Người nhận (8)");
    pressAll(el, ["acknowledge", "emailStatus", "byUnit", "withdraw", "recipients"]);
  });

  it("form: the unit chip and Lưu nháp are disabled; pressing Lưu nháp publishes nothing", () => {
    const publish = vi.fn();
    const el = mount(
      <FormSoanThongBao
        dangGui={false}
        loi={null}
        danhBa={{ ok: true, duLieu: { items: [] } }}
        huy={() => {}}
        phatHanh={publish}
      />,
    );
    const draft = [...el.querySelectorAll<HTMLButtonElement>("button")].find((b) => b.textContent === SAVE_DRAFT_LABEL);
    expect(draft?.disabled).toBe(true);
    expect(draft?.type).toBe("button");
    act(() => draft?.click());
    expect(publish).not.toHaveBeenCalled();
    const chip = [...el.querySelectorAll<HTMLButtonElement>("button")].find((b) => b.textContent === "Chọn bộ phận");
    expect(chip?.disabled).toBe(true);
    pressAll(el, ["byUnit", "saveDraft"]);
    expect(publish).not.toHaveBeenCalled();
  });

  it("header segment: `Gửi cho tôi` is a disabled radio, `Cả sổ thông báo` the checked one; no fetch", () => {
    const el = mount(<SoThongBao />);
    const mine = el.querySelector<HTMLInputElement>(`input[value="${SCOPE_MINE}"]`)!;
    expect(mine.disabled).toBe(true);
    expect(mine.checked).toBe(false);
    expect(el.querySelector<HTMLInputElement>(`input[value="${SCOPE_ALL}"]`)!.checked).toBe(true);
    // The collapsed "N phần … chưa dựng được" block is gone (ADR 0068 §14).
    expect(el.querySelector("details")).toBeNull();
    expect(el.textContent).not.toContain("chưa dựng được");
    pressAll(el, ["scopeMine"]);
  });

  it("`Soạn thông báo` opens the compose form in a DIALOG (prototype), not inline; Huỷ closes it", () => {
    const el = mount(<SoThongBao />);
    expect(el.querySelector("dialog")).toBeNull();
    expect(el.querySelector("#tieu-de-thong-bao")).toBeNull();

    const open = [...el.querySelectorAll<HTMLButtonElement>("button")].find((b) => b.textContent === "Soạn thông báo");
    expect(open?.getAttribute("aria-haspopup")).toBe("dialog");
    act(() => open?.click());

    const dialog = el.querySelector("dialog");
    expect(dialog).not.toBeNull();
    expect(dialog?.getAttribute("aria-labelledby")).toBe("tieu-de-soan-thong-bao");
    expect(dialog?.querySelector("#tieu-de-soan-thong-bao")?.textContent).toBe("Soạn thông báo");
    expect(dialog?.querySelector("#tieu-de-thong-bao")).not.toBeNull();

    const cancel = [...dialog!.querySelectorAll<HTMLButtonElement>("button")].find((b) => b.textContent === "Huỷ");
    act(() => cancel?.click());
    expect(el.querySelector("dialog")).toBeNull();
  });
});
