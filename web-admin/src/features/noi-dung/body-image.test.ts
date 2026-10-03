import { afterEach, describe, expect, it, vi } from "vitest";

import {
  BODY_IMAGE_NOT_READY,
  BODY_IMAGE_URL_INVALID,
  bodyImageErrorText,
  bodyImageInFlight,
  bodyImagePreviewSrc,
  previewsFromItem,
  retryBodyImageCompletion,
  runBodyImageFromUrl,
  runBodyImageUpload,
  type BodyImageState,
} from "./body-image";
import { COVER_STORAGE_FAILED, COVER_TYPE_REFUSED } from "./cover-image";

/**
 * Body images (ADR 0067 §Sửa đổi 03/10/2026): the upload a → b → c and the server fetch over a fake
 * `fetch` and a fake `XMLHttpRequest`, the article id the first image reserves, and one Vietnamese
 * sentence per refusal code — never a raw code.
 */

const FILE_ID = "01JBDYXMG00000000000000001";
const RESERVED = "01JRESERVEDITEM00000000000";
const PREVIEW = "https://files.example.test/vigov-stg-private/thumb-1280.jpg?X-Amz-Signature=s";

const UPLOAD = {
  body_image: { id: FILE_ID, mime_type: "", size_bytes: 0, status: "pending" },
  content_item_id: RESERVED,
  upload: {
    url: "https://files.example.test/vigov-stg-temp",
    fields: { key: "upload/t_01JXA/x", policy: "P", "x-amz-signature": "S", "Content-Type": "image/jpeg" },
    expires_at: "2026-10-03T03:15:00Z",
  },
};

const READY = {
  id: FILE_ID,
  content_item_id: RESERVED,
  mime_type: "image/jpeg",
  size_bytes: 3,
  status: "ready",
  preview_url: PREVIEW,
  preview_expires_at: "2026-10-03T03:15:00Z",
};

const json = (body: unknown, status: number) =>
  new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });

function fakeXHR(status: number) {
  const sent: { url: string; withCredentials: boolean; keys: string[] }[] = [];
  class FakeXHR {
    status = 0;
    withCredentials = false;
    upload: { onprogress: ((e: { lengthComputable: boolean; loaded: number; total: number }) => void) | null } = {
      onprogress: null,
    };
    onload: (() => void) | null = null;
    onerror: (() => void) | null = null;
    private u = "";
    open(_method: string, url: string) {
      this.u = url;
    }
    send(form: FormData) {
      sent.push({ url: this.u, withCredentials: this.withCredentials, keys: [...form.keys()] });
      queueMicrotask(() => {
        this.upload.onprogress?.({ lengthComputable: true, loaded: 50, total: 100 });
        this.status = status;
        this.onload?.();
      });
    }
  }
  vi.stubGlobal("XMLHttpRequest", FakeXHR);
  return sent;
}

function fakeFetch(answers: { request?: Response; completion?: Response; fromUrl?: Response }) {
  const calls: { url: string; init?: RequestInit }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      calls.push({ url, init });
      if (url === "/api/v1/content-items/body-images") return (answers.request ?? json(UPLOAD, 201)).clone();
      if (url === "/api/v1/content-items/body-images/from-url") return (answers.fromUrl ?? json(READY, 201)).clone();
      if (url.endsWith("/completion")) return (answers.completion ?? json(READY, 200)).clone();
      return json({ code: "not_found", message: "?" }, 404);
    }),
  );
  return calls;
}

const photo = () => new File([new Uint8Array([0xff, 0xd8, 0xff])], "anh-le-hoi.jpg", { type: "image/jpeg" });
const body = (c: { init?: RequestInit } | undefined) => JSON.parse(String(c?.init?.body)) as Record<string, unknown>;

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("Tải ảnh từ máy — declare → store → completion", () => {
  it("first image of a new article: no content_item_id sent, the reserved id heard, ready with its preview", async () => {
    const xhr = fakeXHR(204);
    const calls = fakeFetch({});
    const states: BodyImageState[] = [];
    const articles: string[] = [];

    const last = await runBodyImageUpload(photo(), undefined, (s) => states.push(s), (id) => articles.push(id));

    expect(calls[0]?.url).toBe("/api/v1/content-items/body-images");
    expect(body(calls[0])).toEqual({ file_name: "anh-le-hoi.jpg", content_type: "image/jpeg", size: 3 });
    expect(new Headers(calls[0]?.init?.headers).get("Idempotency-Key")).toMatch(/.+/);
    // The bytes go to the STORE, without credentials, the file last.
    expect(xhr).toEqual([
      { url: UPLOAD.upload.url, withCredentials: false, keys: ["key", "policy", "x-amz-signature", "Content-Type", "file"] },
    ]);
    expect(calls[1]?.url).toBe(`/api/v1/content-items/body-images/${FILE_ID}/completion`);
    expect(articles).toEqual([RESERVED]);
    expect(states.map((s) => s.kind)).toEqual(["requesting", "uploading", "uploading", "checking", "ready"]);
    expect(last).toEqual({ kind: "ready", image: { fileId: FILE_ID, contentItemId: RESERVED, previewUrl: PREVIEW } });
  });

  it("a later image of the same article sends the id back", async () => {
    fakeXHR(204);
    const calls = fakeFetch({});
    await runBodyImageUpload(photo(), RESERVED, () => {}, () => {});
    expect(body(calls[0]).content_item_id).toBe(RESERVED);
  });

  it("HEIC never reaches the network (the cover's pre-check)", async () => {
    const xhr = fakeXHR(204);
    const calls = fakeFetch({});
    const heic = new File([new Uint8Array([1])], "IMG_0001.HEIC", { type: "image/heic" });
    expect(await runBodyImageUpload(heic, undefined, () => {}, () => {})).toEqual({ kind: "refused", message: COVER_TYPE_REFUSED });
    expect(calls).toHaveLength(0);
    expect(xhr).toHaveLength(0);
  });

  it("409 body_image_limit at the declaration: our sentence, no upload", async () => {
    const xhr = fakeXHR(204);
    fakeFetch({ request: json({ code: "body_image_limit", message: "server words" }, 409) });
    const s = await runBodyImageUpload(photo(), RESERVED, () => {}, () => {});
    expect(s).toEqual({ kind: "refused", message: bodyImageErrorText("request", { code: "body_image_limit", message: "" }) });
    expect(s.kind === "refused" && s.message).not.toContain("body_image_limit");
    expect(xhr).toHaveLength(0);
  });

  it("the store refuses the form: one sentence of our own, no completion", async () => {
    fakeXHR(403);
    const calls = fakeFetch({});
    expect(await runBodyImageUpload(photo(), undefined, () => {}, () => {})).toEqual({
      kind: "refused",
      message: COVER_STORAGE_FAILED,
    });
    expect(calls.some((c) => c.url.endsWith("/completion"))).toBe(false);
  });

  it("422 body_image_rejected at completion: final, worded", async () => {
    fakeXHR(204);
    fakeFetch({ completion: json({ code: "body_image_rejected", message: "x" }, 422) });
    const s = await runBodyImageUpload(photo(), undefined, () => {}, () => {});
    expect(s.kind).toBe("refused");
    expect(s.kind === "refused" && s.message).toMatch(/từ chối/);
  });

  it("503 at completion: retry the COMPLETION only", async () => {
    const xhr = fakeXHR(204);
    const calls = fakeFetch({ completion: json({ code: "malware_scan_unavailable", message: "x" }, 503) });
    const first = await runBodyImageUpload(photo(), undefined, () => {}, () => {});
    expect(first.kind).toBe("retry");
    await retryBodyImageCompletion(FILE_ID, () => {});
    expect(calls.filter((c) => c.url === "/api/v1/content-items/body-images")).toHaveLength(1);
    expect(calls.filter((c) => c.url.endsWith("/completion"))).toHaveLength(2);
    expect(xhr).toHaveLength(1);
  });

  it("a 200 that is not ready is not inserted", async () => {
    fakeXHR(204);
    fakeFetch({ completion: json({ ...READY, status: "failed" }, 200) });
    expect(await runBodyImageUpload(photo(), undefined, () => {}, () => {})).toEqual({
      kind: "refused",
      message: BODY_IMAGE_NOT_READY,
    });
  });
});

describe("Dán liên kết ảnh — the server downloads it", () => {
  it("sends the link (and the article id when known) and comes back ready with a preview", async () => {
    const calls = fakeFetch({});
    const states: BodyImageState[] = [];
    const s = await runBodyImageFromUrl(" https://bao.vn/anh.jpg ", RESERVED, (x) => states.push(x));
    expect(calls[0]?.url).toBe("/api/v1/content-items/body-images/from-url");
    expect(body(calls[0])).toEqual({ url: "https://bao.vn/anh.jpg", content_item_id: RESERVED });
    expect(new Headers(calls[0]?.init?.headers).get("Idempotency-Key")).toMatch(/.+/);
    expect(states.map((x) => x.kind)).toEqual(["fetching", "ready"]);
    expect(s).toEqual({ kind: "ready", image: { fileId: FILE_ID, contentItemId: RESERVED, previewUrl: PREVIEW } });
  });

  it("first image of a new article: no content_item_id in the request", async () => {
    const calls = fakeFetch({});
    await runBodyImageFromUrl("https://bao.vn/anh.jpg", undefined, () => {});
    expect(body(calls[0])).toEqual({ url: "https://bao.vn/anh.jpg" });
  });

  it("http:, javascript: or an empty link never reach the network", async () => {
    const calls = fakeFetch({});
    for (const bad of ["http://bao.vn/a.jpg", "javascript:alert(1)", "", "https://"]) {
      expect(await runBodyImageFromUrl(bad, undefined, () => {})).toEqual({ kind: "refused", message: BODY_IMAGE_URL_INVALID });
    }
    expect(calls).toHaveLength(0);
  });

  it("502 image_fetch_failed, 400 invalid_image_url, 422, 409: each its own Vietnamese sentence, final", async () => {
    for (const [status, code] of [
      [502, "image_fetch_failed"],
      [400, "invalid_image_url"],
      [422, "body_image_rejected"],
      [409, "body_image_limit"],
      [503, "malware_scan_unavailable"],
    ] as const) {
      fakeFetch({ fromUrl: json({ code, message: "server words" }, status) });
      const s = await runBodyImageFromUrl("https://bao.vn/anh.jpg", undefined, () => {});
      expect(s.kind, code).toBe("refused");
      const msg = s.kind === "refused" ? s.message : "";
      expect(msg, code).toBe(bodyImageErrorText("from-url", { code, message: "" }));
      expect(msg, code).not.toBe("");
      expect(msg, code).not.toContain(code);
    }
  });
});

describe("words for every code", () => {
  const CODES = [
    "invalid_request",
    "invalid_image_url",
    "not_found",
    "body_image_limit",
    "body_image_state",
    "upload_not_received",
    "upload_expired",
    "upload_changed",
    "body_image_rejected",
    "image_fetch_failed",
    "storage_not_configured",
    "upload_limits_unavailable",
    "malware_scan_unavailable",
  ];

  it("every code of the three routes has a sentence of its own, never the code itself", () => {
    for (const step of ["request", "complete", "from-url"] as const) {
      for (const code of CODES) {
        const t = bodyImageErrorText(step, { code, message: "MÁY CHỦ" });
        expect(t, `${step} ${code}`).not.toBe("MÁY CHỦ");
        expect(t, `${step} ${code}`).not.toContain(code);
        expect(t, `${step} ${code}`).toMatch(/[ảàáạãăâđêôơư]/i);
      }
    }
  });

  it("not_found names the article on a declaration, the upload at completion", () => {
    expect(bodyImageErrorText("request", { code: "not_found", message: "" })).toMatch(/bài/);
    expect(bodyImageErrorText("complete", { code: "not_found", message: "" })).toMatch(/lượt tải/);
  });

  it("an unknown code (a 403) keeps the server's own sentence", () => {
    expect(bodyImageErrorText("request", { code: "forbidden", message: "Bạn không có quyền." })).toBe("Bạn không có quyền.");
  });
});

describe("preview", () => {
  it("`bodyImagePreviewSrc`: only an absolute http(s) link", () => {
    expect(bodyImagePreviewSrc(PREVIEW)).toBe(PREVIEW);
    for (const bad of ["javascript:alert(1)", "blob:https://x/1", "data:image/png;base64,AA", "/a.png", "", undefined]) {
      expect(bodyImagePreviewSrc(bad), String(bad)).toBeNull();
    }
  });

  it("`previewsFromItem` maps the detail's body_images that carry a preview", () => {
    const m = previewsFromItem([
      { file_id: FILE_ID, status: "ready", public: true, preview_url: PREVIEW },
      { file_id: "01JBDYXMG00000000000000002", status: "", public: false },
    ]);
    expect([...m.entries()]).toEqual([[FILE_ID, PREVIEW]]);
    expect(previewsFromItem(undefined).size).toBe(0);
  });

  it("in flight: requesting, uploading, checking, fetching — not retry or refused", () => {
    expect(bodyImageInFlight({ kind: "fetching" })).toBe(true);
    expect(bodyImageInFlight({ kind: "checking", id: FILE_ID })).toBe(true);
    expect(bodyImageInFlight({ kind: "retry", id: FILE_ID, message: "" })).toBe(false);
    expect(bodyImageInFlight({ kind: "refused", message: "" })).toBe(false);
  });
});
