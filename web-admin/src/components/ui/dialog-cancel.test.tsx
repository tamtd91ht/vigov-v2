// @vitest-environment jsdom
//
// Sheet web-admin-131026: pressing Cancel in the OS file picker closed the whole "Thêm mới" popup. A
// file input fires a BUBBLING `cancel` event when its picker is dismissed; the dialog's `onCancel` took
// it for Esc. Only a `cancel` fired AT the dialog itself (Esc) may ask to close it.

import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import { LargeDialog } from "./large-dialog";
import { ModalDialog } from "./modal-dialog";

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

function mount(node: ReactNode): HTMLDialogElement {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() => r.render(node));
  return host.querySelector("dialog")!;
}

function cancel(target: EventTarget): void {
  act(() => {
    target.dispatchEvent(new Event("cancel", { bubbles: true, cancelable: true }));
  });
}

const DIALOGS: [string, (onDismiss: () => void) => ReactNode][] = [
  [
    "ModalDialog",
    (onDismiss) => (
      <ModalDialog titleId="t" onDismiss={onDismiss}>
        <h2 id="t">Thêm mới</h2>
        <input type="file" />
      </ModalDialog>
    ),
  ],
  [
    "LargeDialog",
    (onDismiss) => (
      <LargeDialog titleId="t" onDismiss={onDismiss}>
        <h2 id="t">Thêm mới</h2>
        <input type="file" />
      </LargeDialog>
    ),
  ],
];

describe.each(DIALOGS)("%s — `cancel`", (_, render) => {
  it("a file picker's bubbling `cancel` does NOT dismiss the dialog", () => {
    const onDismiss = vi.fn();
    const dialog = mount(render(onDismiss));
    cancel(dialog.querySelector('input[type="file"]')!);
    expect(onDismiss).not.toHaveBeenCalled();
  });

  it("Esc (`cancel` on the dialog itself) still asks to dismiss, and the browser's own close is prevented", () => {
    const onDismiss = vi.fn();
    const dialog = mount(render(onDismiss));
    const e = new Event("cancel", { bubbles: false, cancelable: true });
    act(() => {
      dialog.dispatchEvent(e);
    });
    expect(onDismiss).toHaveBeenCalledTimes(1);
    expect(e.defaultPrevented).toBe(true);
  });
});
