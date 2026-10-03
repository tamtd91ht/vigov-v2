// @vitest-environment jsdom
//
// jsdom for this file: the form mounts the real Tiptap editor and runs the real upload calls over a fake
// `fetch` / `XMLHttpRequest`.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import type { comms_noiDungRa } from "@/lib/api/schema.gen";

import { BODY_IMAGE_WAIT_COVER } from "./body-image";
import { BODY_IMAGE_URL_BUTTON } from "./body-image-panel";
// vi-name-ok: existing exports of nhan-noi-dung.ts / so-noi-dung.tsx (rule 12 inv 3)
import { FORM_TRONG, giaTriTuHang, SUMMARY_SAPO_HINT, type GiaTriFormNoiDung } from "./nhan-noi-dung";
import { FormNoiDung } from "./so-noi-dung";

/**
 * The form's half of the body images (ADR 0067 §Sửa đổi 03/10/2026): the ARTICLE ID the first image
 * reserves is sent on every later file — the cover included — and the preview map draws each figure.
 */

const FILE_A = "01JBDYXMG00000000000000001";
const FILE_B = "01JBDYXMG00000000000000002";
const RESERVED = "01JRESERVEDITEM00000000000";
/** The article id a cover uploaded FIRST on a new article reserves. */
const COVER_RESERVED = "01JCOVERRESERVED0000000000";
const COVER_ID = "01JCOVER0000000000000000001";
const PREVIEW_A = "https://files.example.test/p/a.jpg?X-Amz-Signature=a";
const PREVIEW_B = "https://files.example.test/p/b.jpg?X-Amz-Signature=b";

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  const empty = () => ({ length: 0, item: () => null, [Symbol.iterator]: [][Symbol.iterator] }) as unknown as DOMRectList;
  Range.prototype.getClientRects ??= empty;
  Range.prototype.getBoundingClientRect ??= () => new DOMRect(0, 0, 0, 0);
});

const json = (body: unknown, status: number) =>
  new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });

const PRESIGNED = {
  url: "https://files.example.test/vigov-stg-temp",
  fields: { key: "upload/x", policy: "P" },
  expires_at: "2026-10-03T03:15:00Z",
};

/**
 * `coverGate`: when given, the cover's DECLARATION answers only once it resolves — the window in which the
 * reserved id is not known yet. The server ECHOES a named article and reserves one otherwise, as it does.
 */
function stubNetwork(coverGate?: Promise<void>) {
  const calls: { url: string; body: Record<string, unknown> | null }[] = [];
  let coverArticle = COVER_RESERVED;
  let bodyArticle = "01JND1";
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      const body = init?.body ? (JSON.parse(String(init.body)) as Record<string, unknown>) : null;
      calls.push({ url, body });
      switch (url) {
        case "/api/v1/content-items/body-images/from-url":
          return json(
            { id: FILE_A, content_item_id: RESERVED, mime_type: "image/jpeg", size_bytes: 3, status: "ready", preview_url: PREVIEW_A },
            201,
          );
        case "/api/v1/content-items/body-images":
          bodyArticle = typeof body?.content_item_id === "string" ? body.content_item_id : "01JND1";
          return json(
            {
              body_image: { id: FILE_B, content_item_id: bodyArticle, mime_type: "", size_bytes: 0, status: "pending" },
              content_item_id: bodyArticle,
              upload: PRESIGNED,
            },
            201,
          );
        case `/api/v1/content-items/body-images/${FILE_B}/completion`:
          return json(
            { id: FILE_B, content_item_id: bodyArticle, mime_type: "image/png", size_bytes: 3, status: "ready", preview_url: PREVIEW_B },
            200,
          );
        case "/api/v1/content-items/cover-images":
          if (coverGate !== undefined) await coverGate;
          coverArticle = typeof body?.content_item_id === "string" ? body.content_item_id : COVER_RESERVED;
          return json(
            {
              cover_image: { id: COVER_ID, content_item_id: coverArticle, mime_type: "", size_bytes: 0, status: "pending" },
              content_item_id: coverArticle,
              upload: PRESIGNED,
            },
            201,
          );
        default:
          if (url.startsWith("/api/v1/content-items/cover-images/")) {
            return json({ id: COVER_ID, content_item_id: coverArticle, mime_type: "image/jpeg", size_bytes: 3, status: "ready" }, 200);
          }
          return json({ code: "not_found", message: "?" }, 404);
      }
    }),
  );
  class FakeXHR {
    status = 0;
    withCredentials = false;
    upload = { onprogress: null as null | ((e: unknown) => void) };
    onload: (() => void) | null = null;
    onerror: (() => void) | null = null;
    open() {}
    send() {
      queueMicrotask(() => {
        this.status = 204;
        this.onload?.();
      });
    }
  }
  vi.stubGlobal("XMLHttpRequest", FakeXHR);
  return calls;
}

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

async function settle(): Promise<void> {
  for (let i = 0; i < 10; i++) {
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });
  }
}

async function mountForm(initial: GiaTriFormNoiDung, saved?: comms_noiDungRa, onSave = vi.fn()) {
  host = document.createElement("div");
  document.body.append(host);
  root = createRoot(host);
  await act(async () => {
    root!.render(
      <FormNoiDung
        tieuDeForm="Thêm nội dung cho Mini App"
        moTa="mô tả"
        giaTriDau={initial}
        hang={saved}
        danhMuc={[]}
        dangGui={false}
        loi={null}
        huy={() => {}}
        luu={onSave}
      />,
    );
  });
  await settle();
  expect(host.querySelector("#than-bai-noi-dung"), "the editor never mounted").not.toBeNull();
  return onSave;
}

function button(text: string): HTMLButtonElement {
  const b = Array.from(host!.querySelectorAll("button")).find((x) => x.textContent?.trim() === text);
  if (b === undefined) throw new Error(`no button "${text}"`);
  return b;
}

function typeInto(el: HTMLInputElement, value: string): void {
  Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!.call(el, value);
  act(() => {
    el.dispatchEvent(new Event("input", { bubbles: true }));
  });
}

async function pickFile(input: HTMLInputElement, file: File): Promise<void> {
  Object.defineProperty(input, "files", { value: [file], configurable: true });
  await act(async () => {
    input.dispatchEvent(new Event("change", { bubbles: true }));
  });
  await settle();
}

const jpg = (name: string) => new File([new Uint8Array([0xff, 0xd8, 0xff])], name, { type: "image/jpeg" });

function savedArticle(over: Partial<comms_noiDungRa> = {}): comms_noiDungRa {
  return {
    id: "01JND1",
    type: "tin-tuc",
    category_id: "",
    title: "Lễ hội",
    summary: "",
    body: `<figure><img data-file-id="${FILE_A}"><figcaption>Cũ</figcaption></figure><p>x</p>`,
    image_url: "",
    has_image: false,
    published_on: "2026-10-03",
    view_count: 0,
    status: "an",
    source: "thu-cong",
    source_url: "",
    source_ref: "",
    hand_edited: false,
    author_code: "CB-2026-7K3M9Q",
    created_at: "2026-10-03T02:00:00Z",
    updated_at: "2026-10-03T02:00:00Z",
    body_images: [{ file_id: FILE_A, status: "ready", public: false, preview_url: PREVIEW_A }],
    ...over,
  };
}

describe("new article: one reserved id for every file", () => {
  it("a pasted link inserts the figure with its preview; the cover then carries the id the image reserved", async () => {
    const calls = stubNetwork();
    const onSave = await mountForm({ ...FORM_TRONG, title: "Lễ hội" });

    await act(async () => button("Chèn ảnh").click());
    typeInto(host!.querySelector<HTMLInputElement>("#than-bai-noi-dung-image-url")!, "https://bao.vn/anh.jpg");
    await act(async () => button(BODY_IMAGE_URL_BUTTON).click());
    await settle();

    // First image of an unsaved article: no id sent.
    expect(calls.find((c) => c.url.endsWith("/from-url"))?.body).toEqual({ url: "https://bao.vn/anh.jpg" });
    // Drawn from the server's signed preview.
    expect(host!.querySelector("#than-bai-noi-dung img")?.getAttribute("src")).toBe(PREVIEW_A);

    // The cover, chosen after: it names the article the body image reserved.
    await pickFile(host!.querySelector<HTMLInputElement>("#anh-noi-dung")!, jpg("bia.jpg"));
    expect(calls.find((c) => c.url === "/api/v1/content-items/cover-images")?.body?.content_item_id).toBe(RESERVED);

    // Lưu: the body names the file, never a link.
    await act(async () => button("Lưu").click());
    const sent = onSave.mock.calls[0]?.[0] as GiaTriFormNoiDung;
    expect(sent.body).toContain(`<figure><img data-file-id="${FILE_A}"></figure>`);
    expect(sent.body).not.toContain("src");
  });

  it("a cover uploaded FIRST reserves the id; a body image after it names THAT article, and Lưu carries both", async () => {
    const calls = stubNetwork();
    const onSave = await mountForm({ ...FORM_TRONG, title: "Lễ hội" });
    await pickFile(host!.querySelector<HTMLInputElement>("#anh-noi-dung")!, jpg("bia.jpg"));
    // First file of an unsaved article: no id sent.
    expect(calls.find((c) => c.url === "/api/v1/content-items/cover-images")?.body?.content_item_id).toBeUndefined();

    const insert = button("Chèn ảnh");
    expect(insert.disabled).toBe(false);
    expect(host!.textContent).not.toContain(BODY_IMAGE_WAIT_COVER);
    await act(async () => insert.click());
    await pickFile(host!.querySelector<HTMLInputElement>("#than-bai-noi-dung-image-file")!, jpg("than-bai.jpg"));
    expect(calls.find((c) => c.url === "/api/v1/content-items/body-images")?.body?.content_item_id).toBe(COVER_RESERVED);

    await act(async () => button("Lưu").click());
    const sent = onSave.mock.calls[0]?.[0] as GiaTriFormNoiDung;
    expect(sent.cover_image_file_id).toBe(COVER_ID);
    expect(sent.body).toContain(`data-file-id="${FILE_B}"`);
  });

  it("only while the cover's FIRST declaration is unanswered is `Chèn ảnh` held, with the sentence", async () => {
    let open!: () => void;
    const calls = stubNetwork(new Promise<void>((r) => (open = r)));
    await mountForm({ ...FORM_TRONG, title: "Lễ hội" });
    await pickFile(host!.querySelector<HTMLInputElement>("#anh-noi-dung")!, jpg("bia.jpg"));
    expect(button("Chèn ảnh").disabled).toBe(true);
    expect(host!.textContent).toContain(BODY_IMAGE_WAIT_COVER);

    await act(async () => open());
    await settle();
    expect(button("Chèn ảnh").disabled).toBe(false);
    expect(host!.textContent).not.toContain(BODY_IMAGE_WAIT_COVER);
    expect(calls.filter((c) => c.url === "/api/v1/content-items/cover-images")).toHaveLength(1);
  });
});

describe("saved article", () => {
  it("opens with the stored figure drawn from `body_images`; a new upload names the article and is inserted", async () => {
    const calls = stubNetwork();
    const saved = savedArticle();
    const onSave = await mountForm(giaTriTuHang(saved), saved);
    expect(host!.querySelector("#than-bai-noi-dung img")?.getAttribute("src")).toBe(PREVIEW_A);

    await act(async () => button("Chèn ảnh").click());
    await pickFile(host!.querySelector<HTMLInputElement>("#than-bai-noi-dung-image-file")!, jpg("moi.jpg"));

    expect(calls.find((c) => c.url === "/api/v1/content-items/body-images")?.body?.content_item_id).toBe("01JND1");
    const srcs = Array.from(host!.querySelectorAll("#than-bai-noi-dung img")).map((i) => i.getAttribute("src"));
    expect(srcs).toContain(PREVIEW_B);

    await act(async () => button("Lưu").click());
    const sent = onSave.mock.calls[0]?.[0] as GiaTriFormNoiDung;
    expect(sent.body).toContain(`data-file-id="${FILE_A}"`);
    expect(sent.body).toContain(`data-file-id="${FILE_B}"`);
    expect(sent.body).not.toMatch(/\ssrc=/);
  });

  it("the cover on a saved article still names that article", async () => {
    const calls = stubNetwork();
    const saved = savedArticle();
    await mountForm(giaTriTuHang(saved), saved);
    await pickFile(host!.querySelector<HTMLInputElement>("#anh-noi-dung")!, jpg("bia.jpg"));
    expect(calls.find((c) => c.url === "/api/v1/content-items/cover-images")?.body?.content_item_id).toBe("01JND1");
  });
});

it("`Tóm tắt` says it is the sapo shown bold under the title", async () => {
  stubNetwork();
  await mountForm(FORM_TRONG);
  expect(host!.textContent).toContain(SUMMARY_SAPO_HINT);
});
