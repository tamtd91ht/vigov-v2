// @vitest-environment jsdom
//
// ADR 0068 lần 6 review round 1 (T1-14, T1-15): the prototype's ✕ on EVERY dialog, and no focus ring
// round the title that takes the opening focus.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import { ModalDialog, ModalDialogHeader } from "./modal-dialog";

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

function html(props: Partial<Parameters<typeof ModalDialog>[0]> = {}): string {
  return renderToStaticMarkup(
    <ModalDialog titleId="t" onDismiss={() => {}} {...props}>
      <ModalDialogHeader titleId="t" title="Thêm hạng mục" />
    </ModalDialog>,
  );
}

describe("ModalDialog ✕", () => {
  it("is drawn by default: ghost 28px square, top-right, named “Đóng”, after the content", () => {
    const out = html();
    expect(out).toMatch(/<button[^>]*type="button" aria-label="Đóng" title="Đóng"/);
    expect(out).toMatch(/class="[^"]*absolute top-2 right-2/);
    expect(out).toMatch(/class="[^"]*w-7/);
    // After the header: the title stays the first focusable element of the box.
    expect(out.indexOf("<h2")).toBeLessThan(out.indexOf('aria-label="Đóng"'));
  });

  it("calls onDismiss — the same question Esc asks; the parent decides", () => {
    const onDismiss = vi.fn();
    host = document.createElement("div");
    document.body.append(host);
    const r = createRoot(host);
    root = r;
    act(() =>
      r.render(
        <ModalDialog titleId="t" onDismiss={onDismiss}>
          <ModalDialogHeader titleId="t" title="Thêm hạng mục" />
        </ModalDialog>,
      ),
    );
    act(() => host!.querySelector<HTMLButtonElement>('button[aria-label="Đóng"]')!.click());
    expect(onDismiss).toHaveBeenCalledTimes(1);
  });

  it("opt-out draws none; closeDisabled greys it out; closeLabel renames it", () => {
    expect(html({ showClose: false })).not.toContain("<button");
    expect(html({ closeDisabled: true })).toMatch(/<button[^>]*disabled=""/);
    expect(html({ closeLabel: "Đóng hộp thoại" })).toContain('aria-label="Đóng hộp thoại"');
  });
});

describe("ModalDialog initialFocusId (brief §3.3 / §3.4: the cursor waits in the one field)", () => {
  function open(props: Partial<Parameters<typeof ModalDialog>[0]>, field = <input id="f" />): void {
    host = document.createElement("div");
    document.body.append(host);
    const r = createRoot(host);
    root = r;
    act(() =>
      r.render(
        <ModalDialog titleId="t" onDismiss={() => {}} {...props}>
          <ModalDialogHeader titleId="t" title="Thêm hạng mục" />
          {field}
        </ModalDialog>,
      ),
    );
  }

  it("focuses the named control once the box is open", () => {
    open({ initialFocusId: "f" });
    expect(document.activeElement?.id).toBe("f");
  });

  it("absent → unchanged: no field is focused by the dialog", () => {
    open({});
    expect(document.activeElement?.id).not.toBe("f");
  });

  it("a disabled control is not forced — the default stands", () => {
    open({ initialFocusId: "f" }, <input id="f" disabled />);
    expect(document.activeElement?.id).not.toBe("f");
  });

  it("an id outside the box is never focused (it sits behind the backdrop)", () => {
    const outside = document.createElement("input");
    outside.id = "outside";
    document.body.append(outside);
    try {
      open({ initialFocusId: "outside" });
      expect(document.activeElement).not.toBe(outside);
    } finally {
      outside.remove();
    }
  });

  it("closing still returns focus to the opener", () => {
    const opener = document.createElement("button");
    document.body.append(opener);
    opener.focus();
    try {
      open({ initialFocusId: "f" });
      expect(document.activeElement?.id).toBe("f");
      act(() => root?.unmount());
      root = null;
      expect(document.activeElement).toBe(opener);
    } finally {
      opener.remove();
    }
  });
});

describe("ModalDialogHeader", () => {
  it("the title takes the opening focus but draws no ring (programmatic target, not a control)", () => {
    const out = html();
    expect(out).toMatch(/<h2 id="t" tabindex="-1" class="[^"]*\boutline-none\b/);
  });
});
