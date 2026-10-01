// @vitest-environment jsdom
//
// jsdom for this file: the component mounts a real Tiptap editor, which needs a DOM.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import { LINK_INVALID, LINK_NEEDS_SELECTION, RichTextEditor } from "./rich-text-editor";

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

async function mount(initialHtml: string, onChange = vi.fn()) {
  host = document.createElement("div");
  document.body.append(host);
  root = createRoot(host);
  await act(async () => {
    root!.render(
      <>
        <span id="nhan">Nội dung</span>
        <RichTextEditor
          id="than-bai"
          labelId="nhan"
          initialHtml={initialHtml}
          disabled={false}
          onChange={onChange}
        />
      </>,
    );
  });
  // `immediatelyRender: false` creates the editor after mount, on a later tick — wait for it, bounded,
  // so a component that never mounts its editor fails here instead of hanging.
  for (let i = 0; i < 50 && host.querySelector("#than-bai") === null; i++) {
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });
  }
  expect(host.querySelector("#than-bai"), `the editor never mounted: ${host.innerHTML}`).not.toBeNull();
  return onChange;
}

function typeInto(el: HTMLInputElement, value: string): void {
  Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!.call(el, value);
  act(() => {
    el.dispatchEvent(new Event("input", { bubbles: true }));
  });
}

function buttonByText(text: string): HTMLButtonElement {
  const b = Array.from(host!.querySelectorAll("button")).find((x) => x.textContent === text);
  if (b === undefined) throw new Error(`no button "${text}"`);
  return b;
}

describe("RichTextEditor", () => {
  it("the toolbar holds exactly the allow-list's formats, plus undo/redo", async () => {
    await mount("<p>a</p>");
    const toolbar = host!.querySelector('[role="toolbar"]')!;
    expect(Array.from(toolbar.querySelectorAll("button")).map((b) => b.textContent)).toEqual([
      "Đoạn văn",
      "Tiêu đề lớn",
      "Tiêu đề nhỏ",
      "Đậm",
      "Nghiêng",
      "Danh sách chấm",
      "Danh sách số",
      "Hoàn tác",
      "Làm lại",
    ]);
  });

  it("the editable area is named by the visible label and points at the hint", async () => {
    await mount("<p>a</p>");
    const area = host!.querySelector("#than-bai")!;
    expect(area.getAttribute("contenteditable")).toBe("true");
    expect(area.getAttribute("aria-labelledby")).toBe("nhan");
    expect(area.getAttribute("aria-describedby")).toBe("than-bai-hint");
  });

  it("a hostile stored body is DRAWN from the schema — no script, no image, no handler reaches the DOM", async () => {
    await mount('<p onclick="alert(1)">chữ</p><script>alert(2)</script><img src="x" onerror="alert(3)">');
    const area = host!.querySelector("#than-bai")!;
    expect(area.querySelector("script, img, [onclick], [onerror]")).toBeNull();
    expect(area.textContent).toContain("chữ");
    expect(area.textContent).not.toContain("alert");
  });

  it("opening a stored body is NOT a change — the PATCH must not rewrite a body nobody touched", async () => {
    const onChange = await mount("Văn bản cũ không có thẻ p");
    expect(onChange).not.toHaveBeenCalled();
  });

  it("a non-https link is refused with the sentence; nothing changes", async () => {
    const onChange = await mount("<p>chữ</p>");
    typeInto(host!.querySelector<HTMLInputElement>("#than-bai-link")!, "http://xa.gov.vn");
    await act(async () => buttonByText("Gắn liên kết").click());
    expect(host!.querySelector('[role="alert"]')?.textContent).toBe(LINK_INVALID);
    expect(onChange).not.toHaveBeenCalled();
  });

  it("an https link with no text selected asks for a selection first", async () => {
    await mount("<p>chữ</p>");
    typeInto(host!.querySelector<HTMLInputElement>("#than-bai-link")!, "https://xa.gov.vn");
    await act(async () => buttonByText("Gắn liên kết").click());
    expect(host!.querySelector('[role="alert"]')?.textContent).toBe(LINK_NEEDS_SELECTION);
  });
});
