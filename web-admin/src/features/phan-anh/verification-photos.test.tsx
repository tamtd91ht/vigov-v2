// @vitest-environment jsdom
//
// jsdom for this file: the upload is ONE request after a file is CHOSEN, and the expired-link
// refetch happens after a thumbnail is CLICKED — neither is visible in static markup.

import { act } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import type { petitions_photoLinkOut } from "@/lib/api/schema.gen";
import type { UploadResult } from "@/lib/api/upload";
import { installFakeUploadXHR, partNames } from "@/lib/api/upload-test-support";

import {
  AFTER_PHOTO_CLOSED,
  AFTER_PHOTO_UPLOAD_BUTTON,
  AFTER_PHOTO_UPLOAD_DENIED,
  AFTER_PHOTOS_EMPTY,
  AFTER_PHOTOS_UNAVAILABLE,
} from "./nhan-phieu";
import { scenePhotosState } from "./scene-photos";
import {
  AFTER_PHOTO_RETRY_BUTTON,
  afterPhotoUpload,
  VerificationPhotos,
  VerificationPhotosView,
  type VerificationDeps,
} from "./verification-photos";

const revoked: string[] = [];
let previews = 0;

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  // jsdom's `File` is not the Blob vitest's own `createObjectURL` expects: a local stand-in.
  URL.createObjectURL = () => `blob:preview-${++previews}`;
  URL.revokeObjectURL = (u: string) => {
    revoked.push(u);
  };
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
    upload: vi.fn<VerificationDeps["upload"]>(async () => ({
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

describe("VerificationPhotosView — the staged photos (bug sheet row 58)", () => {
  const staged = (kind: "uploading" | "checking" | "stored" | "retry" | "refused") => {
    const state =
      kind === "uploading"
        ? { kind, percent: 40 }
        : kind === "retry" || kind === "refused"
          ? { kind, message: "Lý do máy chủ." }
          : { kind };
    return renderToStaticMarkup(
      <VerificationPhotosView
        lookupCode="PA-1"
        state={{ kind: "empty" }}
        upload="open"
        uploads={[{ key: "k1", preview: "blob:local-1", state }]}
      />,
    );
  };

  it("a thumbnail, no file name, no size, no 'Bỏ' text button", () => {
    for (const k of ["uploading", "checking", "stored", "retry", "refused"] as const) {
      const html = staged(k);
      expect(html, k).toContain('src="blob:local-1"');
      expect(html, k).not.toMatch(/>Bỏ</);
      expect(html, k).not.toMatch(/\d+(,\d+)? ?(B|KB|MB)(?![a-zA-Z])/);
    }
  });

  it("× ONLY where nothing was stored and nothing is in flight; never on a saved photo", () => {
    for (const k of ["retry", "refused"] as const) expect(staged(k), k).toContain('aria-label="Bỏ ảnh đang chọn 1/1 khỏi danh sách"');
    for (const k of ["uploading", "checking", "stored"] as const) expect(staged(k), k).not.toContain("Bỏ ảnh đang chọn");
    const saved = renderToStaticMarkup(
      <VerificationPhotosView lookupCode="PA-1" state={{ kind: "list", items: [photo(1)] }} upload="open" />,
    );
    expect(saved).not.toContain("Bỏ ảnh");
  });
});

describe("afterPhotoUpload — what an upload's answer turns into", () => {
  const r = (x: UploadResult<{ id: string; content_type: string; size_bytes: number; status: string; created_at: string }>) =>
    afterPhotoUpload(x);
  it("stored → stored; 422 / 409 → refused for good; 503 / 408 / no answer → send again", () => {
    expect(r({ ok: true, data: { id: "01J", content_type: "image/jpeg", size_bytes: 1, status: "stored", created_at: "" } }).kind).toBe("stored");
    expect(r({ ok: false, status: 422, code: "photo_rejected", message: "Ảnh bị từ chối vì phát hiện mã độc." })).toEqual({
      kind: "refused",
      message: "Ảnh bị từ chối vì phát hiện mã độc.",
    });
    expect(r({ ok: false, status: 409, code: "photo_limit", message: "m" }).kind).toBe("refused");
    for (const status of [503, 408, 0]) expect(r({ ok: false, status, code: "", message: "m" }).kind).toBe("retry");
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

  it("choose a photo: ONE upload (type, a key) → the list read ONE more time", async () => {
    const d = deps();
    const h = mount(d);
    await flush();
    choose(h, new File([new Uint8Array([1, 2, 3])], "sau.jpg", { type: "image/jpeg" }));
    await flush();

    const upload = d.upload as ReturnType<typeof vi.fn>;
    expect(upload).toHaveBeenCalledTimes(1);
    const [code, file, type, key] = upload.mock.calls[0] as [string, File, string, string];
    expect(code).toBe("PA-1");
    expect(file.size).toBe(3);
    expect(type).toBe("image/jpeg");
    expect(key).toMatch(/^[0-9a-f-]{36}$/);
    expect(d.load).toHaveBeenCalledTimes(2);
    // Stored, and the re-read holds it: it leaves the staging row (it is in the saved grid now).
    expect(h.querySelector('[aria-label="Ảnh sau xử lý đang tải lên"]')).toBeNull();
  });

  it("the real route: one multipart POST on this origin, no completion call", async () => {
    const sent = installFakeUploadXHR(() => ({
      status: 201,
      body: { id: "01JNEW", content_type: "image/jpeg", size_bytes: 3, status: "stored", created_at: "x" },
    }));
    const d = deps();
    const h = mount({ load: d.load, upload: (await import("@/lib/api/phieu-phan-anh")).uploadVerificationPhoto });
    await flush();
    choose(h, new File([new Uint8Array([1, 2, 3])], "sau.jpg", { type: "image/jpeg" }));
    await flush(10);
    expect(sent.map((x) => x.url)).toEqual(["/api/v1/citizen-reports/PA-1/verification-photos"]);
    expect(partNames(sent[0])).toEqual(["size", "content_type", "file"]);
    // Stored, and the re-read holds it: it leaves the staging row (it is in the saved grid now).
    expect(h.querySelector('[aria-label="Ảnh sau xử lý đang tải lên"]')).toBeNull();
    vi.unstubAllGlobals();
  });

  it("409 `photo_limit`: the server's sentence, final — no retry button", async () => {
    const cau = "Phiếu đã có đủ số ảnh sau xử lý tối đa.";
    const d = deps({ upload: vi.fn(async () => ({ ok: false as const, status: 409, code: "photo_limit", message: cau })) });
    const h = mount(d);
    await flush();
    choose(h, new File([new Uint8Array([1])], "sau.png", { type: "image/png" }));
    await flush();
    expect(h.querySelector('[role="alert"]')?.textContent).toBe(`Bị từ chối: ${cau}`);
    expect(h.textContent).not.toContain(AFTER_PHOTO_RETRY_BUTTON);
    expect(d.load).toHaveBeenCalledTimes(1);
  });

  it("503 upload_busy: `Gửi lại` sends the SAME photo again", async () => {
    const busy = "Hệ thống đang nhận nhiều tệp cùng lúc. Vui lòng thử lại sau ít giây.";
    const upload = vi
      .fn<VerificationDeps["upload"]>()
      .mockResolvedValueOnce({ ok: false, status: 503, code: "upload_busy", message: busy })
      .mockResolvedValueOnce({
        ok: true,
        data: { id: "01JNEW", content_type: "image/jpeg", size_bytes: 1, status: "stored", created_at: "x" },
      });
    const h = mount(deps({ upload }));
    await flush();
    const f = new File([new Uint8Array([1])], "sau.png", { type: "image/png" });
    choose(h, f);
    await flush();
    expect(h.textContent).toContain(busy);
    const again = [...h.querySelectorAll("button")].find((b) => b.textContent === AFTER_PHOTO_RETRY_BUTTON);
    act(() => again?.click());
    await flush();
    expect(upload).toHaveBeenCalledTimes(2);
    expect(upload.mock.calls[1]?.[1]).toBe(f);
    // Stored, and the re-read holds it: it leaves the staging row (it is in the saved grid now).
    expect(h.querySelector('[aria-label="Ảnh sau xử lý đang tải lên"]')).toBeNull();
  });

  it("a refused photo: × in its corner removes it from the selection, with no call", async () => {
    const d = deps({ upload: vi.fn(async () => ({ ok: false as const, status: 409, code: "photo_limit", message: "m" })) });
    const h = mount(d);
    await flush();
    choose(h, new File([new Uint8Array([1])], "sau.png", { type: "image/png" }));
    await flush();
    const preview = h.querySelector('[aria-label="Ảnh sau xử lý đang tải lên"] img')?.getAttribute("src") ?? "";
    expect(preview).toMatch(/^blob:preview-/);
    const x = h.querySelector<HTMLButtonElement>('button[aria-label^="Bỏ ảnh đang chọn"]');
    expect(x).not.toBeNull();
    act(() => x?.click());
    // The local preview is released with the photo.
    expect(revoked).toContain(preview);
    expect(h.querySelector('[aria-label="Ảnh sau xử lý đang tải lên"]')).toBeNull();
    expect(d.upload).toHaveBeenCalledTimes(1);
    expect(d.load).toHaveBeenCalledTimes(1);
  });

  it("a PDF is refused before any call", async () => {
    const d = deps();
    const h = mount(d);
    await flush();
    choose(h, new File(["%PDF"], "bien-ban.pdf", { type: "application/pdf" }));
    await flush();
    expect(d.upload).not.toHaveBeenCalled();
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
