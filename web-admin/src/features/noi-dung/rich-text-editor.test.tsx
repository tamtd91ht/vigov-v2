// @vitest-environment jsdom
//
// jsdom for this file: the component mounts a real Tiptap editor, which needs a DOM.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import { BODY_IMAGE_AFTER_COVER, BODY_IMAGE_LIMIT_REACHED, BODY_IMAGE_NO_PREVIEW, type BodyImageState } from "./body-image";
import { BODY_IMAGE_URL_BUTTON, type BodyImageSource } from "./body-image-panel";
import { LINK_INVALID, LINK_NEEDS_SELECTION, RichTextEditor } from "./rich-text-editor";

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  // jsdom lays nothing out: ProseMirror's scroll-into-view after a toolbar `focus()` asks a Range for its
  // rectangles, which jsdom does not implement. An empty answer is "nothing to scroll".
  const empty = () => ({ length: 0, item: () => null, [Symbol.iterator]: [][Symbol.iterator] }) as unknown as DOMRectList;
  Range.prototype.getClientRects ??= empty;
  Range.prototype.getBoundingClientRect ??= () => new DOMRect(0, 0, 0, 0);
});

let root: Root | null = null;
let host: HTMLDivElement | null = null;

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
});

async function mount(
  initialHtml: string,
  onChange = vi.fn(),
  extra: { previews?: ReadonlyMap<string, string>; images?: BodyImageSource } = {},
) {
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
          previews={extra.previews}
          images={extra.images}
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

const FILE_A = "01JBDYXMG00000000000000001";
const PREVIEW = "https://files.example.test/vigov-stg-private/thumb.jpg?X-Amz-Signature=s";

/** A source whose calls answer `answer` (a ready image of FILE_A by default) and record what they got. */
function source(answer?: BodyImageState, blocked: string | null = null) {
  const ready: BodyImageState = answer ?? {
    kind: "ready",
    image: { fileId: FILE_A, contentItemId: "01JRESERVEDITEM00000000000", previewUrl: PREVIEW },
  };
  const got: { files: File[]; urls: string[] } = { files: [], urls: [] };
  const src: BodyImageSource = {
    blocked,
    upload: async (file, onState) => {
      got.files.push(file);
      onState(ready);
      return ready;
    },
    fromUrl: async (url, onState) => {
      got.urls.push(url);
      onState(ready);
      return ready;
    },
    retry: async (_id, onState) => {
      onState(ready);
      return ready;
    },
  };
  return { src, got };
}

function lastBody(onChange: ReturnType<typeof vi.fn>): string {
  const calls = onChange.mock.calls;
  return String(calls[calls.length - 1]?.[0] ?? "");
}

function buttonByText(text: string): HTMLButtonElement {
  const b = Array.from(host!.querySelectorAll("button")).find((x) => x.textContent === text);
  if (b === undefined) throw new Error(`no button "${text}"`);
  return b;
}

describe("RichTextEditor", () => {
  it("the toolbar holds exactly the allow-list's formats, the image insert, plus undo/redo", async () => {
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
      "Trích dẫn",
      "Dòng tác giả/nguồn",
      "Chèn ảnh",
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

  it("quote and byline buttons change the body to the stored shapes", async () => {
    const onChange = await mount("<p>Theo Báo X</p>");
    await act(async () => buttonByText("Dòng tác giả/nguồn").click());
    expect(lastBody(onChange)).toBe('<p data-role="byline">Theo Báo X</p>');
    await act(async () => buttonByText("Dòng tác giả/nguồn").click());
    await act(async () => buttonByText("Trích dẫn").click());
    expect(lastBody(onChange)).toBe("<blockquote><p>Theo Báo X</p></blockquote>");
  });
});

describe("body images in the editor", () => {
  const FIGURE = `<figure><img data-file-id="${FILE_A}" alt="Lễ hội"><figcaption>Chú thích</figcaption></figure><p>sau</p>`;

  it("a figure with a preview is DRAWN with the signed link — and the body sent never holds a src", async () => {
    // The cursor opens in the first paragraph, so the toolbar edit below lands there.
    const onChange = await mount("<p>Theo Báo X</p>" + FIGURE, vi.fn(), { previews: new Map([[FILE_A, PREVIEW]]) });
    const area = host!.querySelector("#than-bai")!;
    const img = area.querySelector("img");
    expect(img?.getAttribute("src")).toBe(PREVIEW);
    expect(img?.getAttribute("alt")).toBe("Lễ hội");
    // An edit, so the body is reported: it names the file, never the link.
    await act(async () => buttonByText("Dòng tác giả/nguồn").click());
    const body = lastBody(onChange);
    expect(body).toContain(`<figure><img data-file-id="${FILE_A}" alt="Lễ hội"><figcaption>Chú thích</figcaption></figure>`);
    expect(body).not.toContain("src");
    expect(body).not.toContain("files.example.test");
  });

  it("no preview (missing or expired): the sentence, never a broken image", async () => {
    await mount(FIGURE);
    const area = host!.querySelector("#than-bai")!;
    expect(area.querySelector("img")).toBeNull();
    expect(area.textContent).toContain(BODY_IMAGE_NO_PREVIEW);
    expect(area.textContent).toContain("Chú thích");
  });

  it("a preview link that fails to load falls back to the sentence", async () => {
    await mount(FIGURE, vi.fn(), { previews: new Map([[FILE_A, PREVIEW]]) });
    const img = host!.querySelector("#than-bai img")!;
    await act(async () => {
      img.dispatchEvent(new Event("error"));
    });
    expect(host!.querySelector("#than-bai img")).toBeNull();
    expect(host!.querySelector("#than-bai")!.textContent).toContain(BODY_IMAGE_NO_PREVIEW);
  });

  it("`Chèn ảnh` → link → the server's ready image is inserted at the cursor, with the caption", async () => {
    const { src, got } = source();
    const onChange = await mount("<p>Mở đầu</p>", vi.fn(), { images: src });
    const insert = buttonByText("Chèn ảnh");
    expect(insert.getAttribute("aria-expanded")).toBe("false");
    await act(async () => insert.click());
    expect(insert.getAttribute("aria-expanded")).toBe("true");
    expect(insert.getAttribute("aria-controls")).toBe("than-bai-image-panel");
    typeInto(host!.querySelector<HTMLInputElement>("#than-bai-image-caption")!, "Ảnh lễ hội");
    typeInto(host!.querySelector<HTMLInputElement>("#than-bai-image-url")!, "https://bao.vn/anh.jpg");
    await act(async () => buttonByText(BODY_IMAGE_URL_BUTTON).click());
    expect(got.urls).toEqual(["https://bao.vn/anh.jpg"]);
    const body = lastBody(onChange);
    expect(body).toContain(`<figure><img data-file-id="${FILE_A}"><figcaption>Ảnh lễ hội</figcaption></figure>`);
    expect(body).not.toContain("bao.vn");
    // The panel closes after the insert.
    expect(host!.querySelector("#than-bai-image-panel")).toBeNull();
  });

  it("`Tải ảnh từ máy` hands the chosen file to the form's upload and inserts the ready image", async () => {
    const { src, got } = source();
    const onChange = await mount("<p>a</p>", vi.fn(), { images: src });
    await act(async () => buttonByText("Chèn ảnh").click());
    const input = host!.querySelector<HTMLInputElement>("#than-bai-image-file")!;
    expect(input.getAttribute("accept")).toContain("image/webp");
    const file = new File([new Uint8Array([0xff, 0xd8, 0xff])], "a.jpg", { type: "image/jpeg" });
    Object.defineProperty(input, "files", { value: [file], configurable: true });
    await act(async () => {
      input.dispatchEvent(new Event("change", { bubbles: true }));
    });
    expect(got.files).toEqual([file]);
    expect(lastBody(onChange)).toContain(`data-file-id="${FILE_A}"`);
  });

  it("a refusal is shown in the panel as a sentence (role=alert) and nothing is inserted", async () => {
    const { src } = source({ kind: "refused", message: "Không tải được ảnh từ liên kết này." });
    const onChange = await mount("<p>a</p>", vi.fn(), { images: src });
    await act(async () => buttonByText("Chèn ảnh").click());
    typeInto(host!.querySelector<HTMLInputElement>("#than-bai-image-url")!, "https://bao.vn/x.jpg");
    await act(async () => buttonByText(BODY_IMAGE_URL_BUTTON).click());
    expect(host!.querySelector("#than-bai-image-panel [role=alert]")?.textContent).toBe("Không tải được ảnh từ liên kết này.");
    expect(onChange).not.toHaveBeenCalled();
  });

  it(`20 images: \`Chèn ảnh\` is off and the sentence says why`, async () => {
    const twenty = Array.from(
      { length: 20 },
      (_, i) => `<figure><img data-file-id="01JBDYXMG000000000000000${String(i).padStart(2, "0")}"></figure>`,
    ).join("");
    const { src } = source();
    await mount(twenty + "<p>x</p>", vi.fn(), { images: src });
    const insert = buttonByText("Chèn ảnh");
    expect(insert.disabled).toBe(true);
    expect(insert.getAttribute("aria-describedby")).toBe("than-bai-image-blocked");
    expect(host!.querySelector("#than-bai-image-blocked")?.textContent).toBe(BODY_IMAGE_LIMIT_REACHED);
  });

  it("19 images: `Chèn ảnh` is on", async () => {
    const nineteen = Array.from(
      { length: 19 },
      (_, i) => `<figure><img data-file-id="01JBDYXMG000000000000000${String(i).padStart(2, "0")}"></figure>`,
    ).join("");
    await mount(nineteen + "<p>x</p>", vi.fn(), { images: source().src });
    expect(buttonByText("Chèn ảnh").disabled).toBe(false);
  });

  it("the form's reason to block (cover uploaded first on a new article) is said and holds the button", async () => {
    await mount("<p>a</p>", vi.fn(), { images: source(undefined, BODY_IMAGE_AFTER_COVER).src });
    expect(buttonByText("Chèn ảnh").disabled).toBe(true);
    expect(host!.querySelector("#than-bai-image-blocked")?.textContent).toBe(BODY_IMAGE_AFTER_COVER);
  });

  it("without an image source, `Chèn ảnh` is off", async () => {
    await mount("<p>a</p>");
    expect(buttonByText("Chèn ảnh").disabled).toBe(true);
  });
});
