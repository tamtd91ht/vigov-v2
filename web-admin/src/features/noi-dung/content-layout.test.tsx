// @vitest-environment jsdom
//
// jsdom for this file: what is checked here only happens on a key press or on mount — the arrow keys
// moving the selected tab, and the overlay handing focus back to the control that opened it.

import { act, useState, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import { OverlayDialog } from "./overlay-dialog";
import { ThanhTabLoai } from "./so-noi-dung";

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
});

function render(node: ReactNode): void {
  host = document.createElement("div");
  document.body.append(host);
  root = createRoot(host);
  act(() => root!.render(node));
}

function Tabs({ onChange }: { onChange: (t: string) => void }) {
  const [type, setType] = useState("");
  return (
    <ThanhTabLoai
      loai={type}
      datLoai={(t) => {
        setType(t);
        onChange(t);
      }}
    />
  );
}

const selected = () => host!.querySelector<HTMLButtonElement>('[role="tab"][aria-selected="true"]')!;

function press(el: HTMLElement, key: string): void {
  act(() => {
    el.dispatchEvent(new KeyboardEvent("keydown", { key, bubbles: true }));
  });
}

describe("content-type tabs — WAI-ARIA keyboard", () => {
  it("→ selects AND focuses the next tab; ← wraps from the first to the last; Home / End", () => {
    const onChange = vi.fn();
    render(<Tabs onChange={onChange} />);
    expect(selected().textContent).toBe("Tất cả");

    press(selected(), "ArrowRight");
    expect(selected().textContent).toBe("Tin tức");
    expect(document.activeElement).toBe(selected());
    expect(onChange).toHaveBeenLastCalledWith("tin-tuc");

    press(selected(), "Home");
    expect(selected().textContent).toBe("Tất cả");

    press(selected(), "ArrowLeft");
    expect(selected().textContent).toBe("Banner");
    expect(document.activeElement).toBe(selected());

    press(selected(), "End");
    expect(selected().textContent).toBe("Banner");
  });

  it("any other key is left to the browser — Tab still leaves the tablist", () => {
    const onChange = vi.fn();
    render(<Tabs onChange={onChange} />);
    press(selected(), "Tab");
    press(selected(), "a");
    expect(onChange).not.toHaveBeenCalled();
  });

  it("exactly one tab is in the Tab order after a move (roving tabIndex)", () => {
    render(<Tabs onChange={() => {}} />);
    press(selected(), "ArrowRight");
    const inOrder = Array.from(host!.querySelectorAll<HTMLButtonElement>('[role="tab"]')).filter((b) => b.tabIndex === 0);
    expect(inOrder.map((b) => b.textContent)).toEqual(["Tin tức"]);
  });
});

function Opener() {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button type="button" id="opener" onClick={() => setOpen(true)}>
        Mở
      </button>
      {open && (
        <OverlayDialog titleId="t" onDismiss={() => setOpen(false)}>
          <h3 id="t">Tiêu đề</h3>
          <button type="button" id="inside" onClick={() => setOpen(false)}>
            Huỷ
          </button>
        </OverlayDialog>
      )}
    </>
  );
}

describe("OverlayDialog", () => {
  it("opens as a dialog named by its heading, and gives focus back to the opener when it closes", () => {
    render(<Opener />);
    const opener = host!.querySelector<HTMLButtonElement>("#opener")!;
    opener.focus();
    act(() => opener.click());

    const dialog = host!.querySelector("dialog")!;
    expect(dialog.hasAttribute("open")).toBe(true);
    expect(dialog.getAttribute("aria-labelledby")).toBe("t");

    const inside = host!.querySelector<HTMLButtonElement>("#inside")!;
    inside.focus();
    act(() => inside.click());
    expect(host!.querySelector("dialog")).toBeNull();
    expect(document.activeElement).toBe(opener);
  });

  it("uses `showModal()` where the browser has it (top layer, page behind made inert)", () => {
    const showModal = vi.fn(function (this: HTMLDialogElement) {
      this.setAttribute("open", "");
    });
    const proto = HTMLDialogElement.prototype as unknown as { showModal?: () => void };
    const before = proto.showModal;
    proto.showModal = showModal;
    try {
      render(<Opener />);
      act(() => host!.querySelector<HTMLButtonElement>("#opener")!.click());
      expect(showModal).toHaveBeenCalledTimes(1);
    } finally {
      if (before === undefined) delete proto.showModal;
      else proto.showModal = before;
    }
  });

  it("Esc (`cancel`) is prevented and handed to the parent, which decides", () => {
    render(<Opener />);
    act(() => host!.querySelector<HTMLButtonElement>("#opener")!.click());
    const ev = new Event("cancel", { cancelable: true });
    act(() => {
      host!.querySelector("dialog")!.dispatchEvent(ev);
    });
    expect(ev.defaultPrevented).toBe(true);
    expect(host!.querySelector("dialog")).toBeNull();
  });
});
