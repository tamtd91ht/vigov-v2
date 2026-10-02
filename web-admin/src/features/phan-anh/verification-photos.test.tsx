// @vitest-environment jsdom
//
// jsdom for this file: the upload is three calls in order after a file is CHOSEN, and the expired-link
// refetch happens after a thumbnail is CLICKED — neither is visible in static markup.

import { act } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import type { petitions_photoLinkOut } from "@/lib/api/schema.gen";
import type { CallResult } from "@/lib/api/task-attachments";

import {
  AFTER_PHOTO_CLOSED,
  AFTER_PHOTO_UPLOAD_BUTTON,
  AFTER_PHOTO_UPLOAD_DENIED,
  AFTER_PHOTOS_EMPTY,
  AFTER_PHOTOS_UNAVAILABLE,
} from "./nhan-phieu";
import { scenePhotosState } from "./scene-photos";
import {
  afterPhotoCompletion,
  VerificationPhotos,
  VerificationPhotosView,
  type VerificationDeps,
} from "./verification-photos";

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
  vi.useRealTimers();
});

function photo(n: number, change: Partial<petitions_photoLinkOut> = {}): petitions_photoLinkOut {
  return {
    id: `01JAFTER${n}`,
    content_type: "image/jpeg",
    size_bytes: 90_000,
    created_at: "2026-10-02T02:50:00Z",
    url: `https://kho.example.test/t_01JTENANT/after-${n}.jpg?X-Amz-Signature=fake`,
    url_expires_at: "2099-01-01T00:00:00Z",
    ...change,
  };
}

async function flush(times = 6) {
  for (let i = 0; i < times; i++) {
    await act(async () => {
      await Promise.resolve();
    });
  }
}

function deps(change: Partial<VerificationDeps> = {}): VerificationDeps {
  return {
    load: vi.fn(async () => ({ ok: true as const, data: { items: [] } })),
    request: vi.fn(async () => ({
      ok: true as const,
      data: {
        photo: { id: "01JNEW", content_type: "image/jpeg", size_bytes: 3, status: "pending", created_at: "x" },
        upload: { url: "https://kho.example.test/tmp", fields: { key: "t_01J/x" }, expires_at: "x" },
      },
    })),
    upload: vi.fn(async () => ({ ok: true as const, data: null })),
    complete: vi.fn(async () => ({
      ok: true as const,
      data: { id: "01JNEW", content_type: "image/jpeg", size_bytes: 3, status: "stored", created_at: "x" },
    })),
    ...change,
  };
}

function mount(d: VerificationDeps, status = "dang-xu-ly", canUpload = true) {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() => r.render(<VerificationPhotos lookupCode="PA-1" status={status} canUpload={canUpload} deps={d} />));
  return host;
}

function choose(h: HTMLElement, file: File) {
  const input = h.querySelector<HTMLInputElement>('input[type="file"]');
  if (input === null) throw new Error("no file input");
  Object.defineProperty(input, "files", { value: [file], configurable: true });
  act(() => {
    input.dispatchEvent(new Event("change", { bubbles: true }));
  });
}

describe("VerificationPhotosView — what the column says", () => {
  it("empty: the sentence — and NOT the spec's 'Bắt buộc…' (the commune's switch is unknown here)", () => {
    const html = renderToStaticMarkup(<VerificationPhotosView lookupCode="PA-1" state={{ kind: "empty" }} upload="open" />);
    expect(html).toContain(AFTER_PHOTOS_EMPTY);
    expect(html).not.toContain("Bắt buộc phải có trước khi đóng phiếu");
    expect(html).toContain(AFTER_PHOTO_UPLOAD_BUTTON);
    expect(html).toContain('accept="image/jpeg,image/png,image/webp');
  });

  it("list: thumbnails whose alt is a position, no-referrer, never lazy, never an <a> to the link", () => {
    const html = renderToStaticMarkup(
      <VerificationPhotosView lookupCode="PA-1" state={{ kind: "list", items: [photo(1), photo(2)] }} upload="open" />,
    );
    expect(html.match(/<img /g)?.length).toBe(2);
    expect(html).toContain('alt="Ảnh sau xử lý 1/2"');
    expect(html).toContain('referrerPolicy="no-referrer"');
    expect(html).not.toContain('loading="lazy"');
    expect(html).not.toMatch(/<a [^>]*kho\.example\.test/);
  });

  it("503: the storage sentence and a retry", () => {
    const html = renderToStaticMarkup(
      <VerificationPhotosView
        lookupCode="PA-1"
        state={scenePhotosState({ ok: false, status: 503, message: "x" })}
        upload="open"
      />,
    );
    expect(html).toContain(AFTER_PHOTOS_UNAVAILABLE);
    expect(html).toContain("Tải lại ảnh");
  });

  it("ended petition: no upload button, the sentence instead", () => {
    const html = renderToStaticMarkup(<VerificationPhotosView lookupCode="PA-1" state={{ kind: "empty" }} upload="closed" />);
    expect(html).not.toContain('type="file"');
    expect(html).toContain(AFTER_PHOTO_CLOSED);
  });

  it("DENIED — no `feedback.resolve`: no upload button, the sentence names the key; the list still reads", () => {
    const html = renderToStaticMarkup(
      <VerificationPhotosView lookupCode="PA-1" state={{ kind: "list", items: [photo(1)] }} upload="denied" />,
    );
    expect(html).not.toContain('type="file"');
    expect(html).not.toContain(AFTER_PHOTO_UPLOAD_BUTTON);
    expect(html).toContain(AFTER_PHOTO_UPLOAD_DENIED.replace(/&/g, "&amp;"));
    expect(html).toContain("feedback.resolve");
    expect(html).toContain("<img ");
  });
});

describe("afterPhotoCompletion — what a completion answer turns into", () => {
  const r = (x: CallResult<{ id: string; content_type: string; size_bytes: number; status: string; created_at: string }>) =>
    afterPhotoCompletion("01J", x);
  it("stored → stored; 422 → refused for good; 503 / 409 / no answer → retry the completion", () => {
    expect(r({ ok: true, data: { id: "01J", content_type: "image/jpeg", size_bytes: 1, status: "stored", created_at: "" } }).kind).toBe("stored");
    expect(r({ ok: false, status: 422, message: "Ảnh bị từ chối vì phát hiện mã độc." })).toEqual({
      kind: "refused",
      message: "Ảnh bị từ chối vì phát hiện mã độc.",
    });
    for (const status of [503, 409, 0]) expect(r({ ok: false, status, message: "m" }).kind).toBe("retry");
  });
});

describe("VerificationPhotos — the flow", () => {
  it("the list is read ONCE when shown — no timer", async () => {
    vi.useFakeTimers({ toFake: ["setTimeout", "setInterval"] });
    const d = deps();
    mount(d);
    await flush();
    await act(async () => {
      vi.advanceTimersByTime(60 * 60 * 1000);
    });
    expect(d.load).toHaveBeenCalledTimes(1);
  });

  it("choose a photo: declare (type + size, a key) → upload → complete → the list read ONE more time", async () => {
    const d = deps();
    const h = mount(d);
    await flush();
    choose(h, new File([new Uint8Array([1, 2, 3])], "sau.jpg", { type: "image/jpeg" }));
    await flush();

    expect(d.request).toHaveBeenCalledTimes(1);
    const [code, body, key] = (d.request as ReturnType<typeof vi.fn>).mock.calls[0] as [string, unknown, string];
    expect(code).toBe("PA-1");
    expect(body).toEqual({ content_type: "image/jpeg", size: 3 });
    expect(key).toMatch(/^[0-9a-f-]{36}$/);
    expect(d.upload).toHaveBeenCalledTimes(1);
    expect(d.complete).toHaveBeenCalledWith("PA-1", "01JNEW");
    expect(d.load).toHaveBeenCalledTimes(2);
    expect(h.textContent).toContain("Đã lưu vào phiếu.");
  });

  it("409 `photo_limit` at the declaration: the server's sentence, nothing uploaded", async () => {
    const cau = "Phiếu đã có đủ số ảnh sau xử lý tối đa.";
    const d = deps({ request: vi.fn(async () => ({ ok: false as const, status: 409, message: cau })) });
    const h = mount(d);
    await flush();
    choose(h, new File([new Uint8Array([1])], "sau.png", { type: "image/png" }));
    await flush();
    expect(h.querySelector('[role="alert"]')?.textContent).toBe(`Bị từ chối: ${cau}`);
    expect(d.upload).not.toHaveBeenCalled();
    expect(d.load).toHaveBeenCalledTimes(1);
  });

  it("a PDF is refused before any call", async () => {
    const d = deps();
    const h = mount(d);
    await flush();
    choose(h, new File(["%PDF"], "bien-ban.pdf", { type: "application/pdf" }));
    await flush();
    expect(d.request).not.toHaveBeenCalled();
    expect(h.textContent).toContain("Chỉ tải được ảnh JPG, PNG hoặc WebP.");
  });

  it("an EXPIRED link is never used: opening it re-reads the list and shows the NEW link", async () => {
    const stale = photo(1, { url_expires_at: "2000-01-01T00:00:00Z" });
    const renewed = photo(1, { url: "https://kho.example.test/renewed.jpg", url_expires_at: "2099-01-01T00:00:00Z" });
    const load = vi
      .fn<VerificationDeps["load"]>()
      .mockResolvedValueOnce({ ok: true, data: { items: [stale] } })
      .mockResolvedValueOnce({ ok: true, data: { items: [renewed] } });
    const h = mount(deps({ load }));
    await flush();
    const thumb = h.querySelector<HTMLButtonElement>('button[aria-label^="Xem cỡ lớn"]');
    if (thumb === null) throw new Error("no thumbnail");
    act(() => thumb.click());
    await flush();
    expect(load).toHaveBeenCalledTimes(2);
    const srcs = Array.from(h.querySelectorAll("img")).map((i) => i.getAttribute("src"));
    expect(srcs).toContain("https://kho.example.test/renewed.jpg");
    expect(srcs).not.toContain(stale.url);
  });

  it("a usable link opens WITHOUT a new call (no needless signing, no extra audit entry)", async () => {
    const d = deps({ load: vi.fn(async () => ({ ok: true as const, data: { items: [photo(1)] } })) });
    const h = mount(d);
    await flush();
    act(() => h.querySelector<HTMLButtonElement>('button[aria-label^="Xem cỡ lớn"]')?.click());
    await flush();
    expect(d.load).toHaveBeenCalledTimes(1);
    expect(h.querySelectorAll("img").length).toBe(2);
  });
});
