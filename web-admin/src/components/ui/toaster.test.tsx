// @vitest-environment jsdom
//
// Owner 18/10/2026: the "Số điện thoại đã có" toast was painted BEHIND the "Thêm người dân" dialog (the
// dialog is in the top layer via showModal). The toast region must sit in the top layer too, raised above
// every dialog that opens, at the bottom-RIGHT.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { toast } from "sonner";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import { Toaster } from "./toaster";

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

let root: Root | null = null;
let host: HTMLDivElement | null = null;
const originalShowModal = HTMLDialogElement.prototype.showModal;

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  HTMLDialogElement.prototype.showModal = originalShowModal;
  vi.restoreAllMocks();
});

/** jsdom has no Popover API: give HTMLElement one that records the calls and the open state. */
function fakePopover() {
  const calls: string[] = [];
  const open = new WeakSet<HTMLElement>();
  const proto = HTMLElement.prototype as unknown as Record<string, unknown>;
  proto.showPopover = function (this: HTMLElement) {
    calls.push("show");
    open.add(this);
  };
  proto.hidePopover = function (this: HTMLElement) {
    calls.push("hide");
    open.delete(this);
  };
  const matches = Element.prototype.matches;
  vi.spyOn(Element.prototype, "matches").mockImplementation(function (this: Element, sel: string) {
    if (sel === ":popover-open") return open.has(this as HTMLElement);
    return matches.call(this, sel);
  });
  return {
    calls,
    cleanup() {
      delete proto.showPopover;
      delete proto.hidePopover;
    },
  };
}

function mount() {
  host = document.createElement("div");
  document.body.appendChild(host);
  root = createRoot(host);
  act(() => root!.render(<Toaster />));
}

describe("Toaster", () => {
  it("is a manual popover layer whose toasts show at the bottom-right", async () => {
    mount();
    const layer = document.querySelector("[data-toast-layer]");
    expect(layer?.getAttribute("popover")).toBe("manual");
    await act(async () => {
      toast.error("Số điện thoại đã có trong danh bạ");
      await new Promise((r) => setTimeout(r, 0));
    });
    const list = layer?.querySelector("[data-sonner-toaster]");
    expect(list?.getAttribute("data-x-position")).toBe("right");
    expect(list?.getAttribute("data-y-position")).toBe("bottom");
  });

  it("enters the top layer on mount and is re-raised above every dialog opened with showModal", () => {
    const p = fakePopover();
    const opened: string[] = [];
    HTMLDialogElement.prototype.showModal = function () {
      opened.push("dialog");
    };
    try {
      mount();
      expect(p.calls).toEqual(["show"]);
      const dialog = document.createElement("dialog");
      document.body.appendChild(dialog);
      dialog.showModal();
      expect(opened).toEqual(["dialog"]);
      // hide + show again = moved to the top of the top layer, above the dialog that just opened.
      expect(p.calls).toEqual(["show", "hide", "show"]);
      dialog.remove();
    } finally {
      p.cleanup();
    }
  });

  it("restores showModal when unmounted", () => {
    const p = fakePopover();
    const mine = function () {};
    HTMLDialogElement.prototype.showModal = mine;
    try {
      mount();
      expect(HTMLDialogElement.prototype.showModal).not.toBe(mine);
      act(() => root!.unmount());
      root = null;
      expect(HTMLDialogElement.prototype.showModal).toBe(mine);
    } finally {
      p.cleanup();
    }
  });
});
