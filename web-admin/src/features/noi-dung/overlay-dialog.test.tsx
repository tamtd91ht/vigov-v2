// @vitest-environment jsdom
//
// Sheet web-admin-131026: pressing Cancel in the OS file picker closed the whole "Thêm mới" popup of
// Nội dung mini app. A file input fires a BUBBLING `cancel` event when its picker is dismissed; the
// dialog's `onCancel` took it for Esc. Only a `cancel` fired AT the dialog itself may ask to close it.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import { OverlayDialog } from "./overlay-dialog";

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

function mount(onDismiss: () => void): HTMLDialogElement {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() =>
    r.render(
      <OverlayDialog titleId="t" onDismiss={onDismiss}>
        <h2 id="t">Thêm mới</h2>
        <input type="file" />
      </OverlayDialog>,
    ),
  );
  return host.querySelector("dialog")!;
}

describe("OverlayDialog — `cancel`", () => {
  it("a file picker's bubbling `cancel` does NOT dismiss the dialog", () => {
    const onDismiss = vi.fn();
    const dialog = mount(onDismiss);
    act(() => {
      dialog.querySelector('input[type="file"]')!.dispatchEvent(new Event("cancel", { bubbles: true, cancelable: true }));
    });
    expect(onDismiss).not.toHaveBeenCalled();
  });

  it("Esc (`cancel` on the dialog itself) still asks to dismiss, and the browser's own close is prevented", () => {
    const onDismiss = vi.fn();
    const dialog = mount(onDismiss);
    const e = new Event("cancel", { bubbles: false, cancelable: true });
    act(() => {
      dialog.dispatchEvent(e);
    });
    expect(onDismiss).toHaveBeenCalledTimes(1);
    expect(e.defaultPrevented).toBe(true);
  });
});
