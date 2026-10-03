import { afterEach, describe, expect, it, vi } from "vitest";

import { themNoiDung } from "@/lib/api/noi-dung";

import {
  COVER_ACCEPT,
  COVER_EMPTY,
  COVER_MAX_BYTES,
  COVER_NOT_READY,
  COVER_STORAGE_FAILED,
  COVER_TOO_LARGE,
  COVER_TYPE_REFUSED,
  afterCoverCompletion,
  coverInFlight,
  coverPreviewSrc,
  declaredCoverType,
  retryCoverCompletion,
  runCoverUpload,
  type CoverUploadState,
} from "./cover-image";
import { FORM_TRONG, thanThem } from "./nhan-noi-dung";

/**
 * §7 `Ảnh đại diện` — the cover upload. The pre-check (the user's 01/10/2026 decision: JPG/PNG/WebP,
 * 50 MB, NO HEIC), the completion answers, the preview `src`, and the whole flow a → b → c over a fake
 * `fetch` and a fake `XMLHttpRequest` (no DOM), ending with the id on the create body.
 */

const MB = 1024 * 1024;

describe("pre-check — convenience; the server's policy still decides", () => {
  it("JPG / PNG / WebP by the browser's type, or by extension when the type is empty", () => {
    expect(declaredCoverType({ name: "a.jpg", type: "image/jpeg", size: 1 })).toEqual({ ok: true, contentType: "image/jpeg" });
    expect(declaredCoverType({ name: "a.PNG", type: "", size: 1 })).toEqual({ ok: true, contentType: "image/png" });
    expect(declaredCoverType({ name: "a.webp", type: "image/webp", size: 1 })).toEqual({ ok: true, contentType: "image/webp" });
    expect(COVER_ACCEPT).toContain("image/webp");
  });

  it("HEIC is refused — by type and by extension; so are GIF and SVG", () => {
    for (const f of [
      { name: "IMG_0001.HEIC", type: "image/heic", size: 1 },
      { name: "IMG_0001.heic", type: "", size: 1 },
      { name: "IMG_0001.heif", type: "image/heif", size: 1 },
      { name: "a.gif", type: "image/gif", size: 1 },
      { name: "a.svg", type: "image/svg+xml", size: 1 },
    ]) {
      expect(declaredCoverType(f), f.name).toEqual({ ok: false, message: COVER_TYPE_REFUSED });
    }
    expect(COVER_ACCEPT.toLowerCase()).not.toContain("heic");
  });

  it("more than 50 MB is refused; exactly 50 MB is not; an empty file is refused", () => {
    expect(COVER_MAX_BYTES).toBe(50 * MB);
    expect(declaredCoverType({ name: "a.jpg", type: "image/jpeg", size: 50 * MB }).ok).toBe(true);
    expect(declaredCoverType({ name: "a.jpg", type: "image/jpeg", size: 50 * MB + 1 })).toEqual({
      ok: false,
      message: COVER_TOO_LARGE,
    });
    expect(declaredCoverType({ name: "a.jpg", type: "image/jpeg", size: 0 })).toEqual({ ok: false, message: COVER_EMPTY });
  });
});

describe("completion answers → state", () => {
  const ready = { id: "C", content_item_id: "01JND9", mime_type: "image/jpeg", size_bytes: 9, status: "ready" };
  it("200 ready ⇒ ready; 200 anything else ⇒ refused; 422 ⇒ refused VERBATIM; 503 / 409 / no answer ⇒ retry", () => {
    expect(afterCoverCompletion("C", { ok: true, data: ready })).toEqual({ kind: "ready", id: "C" });
    expect(afterCoverCompletion("C", { ok: true, data: { ...ready, status: "rejected" } })).toEqual({
      kind: "refused",
      message: COVER_NOT_READY,
    });
    expect(afterCoverCompletion("C", { ok: false, status: 422, message: "mã độc" })).toEqual({ kind: "refused", message: "mã độc" });
    for (const status of [503, 409, 0]) {
      expect(afterCoverCompletion("C", { ok: false, status, message: "x" })).toEqual({ kind: "retry", id: "C", message: "x" });
    }
  });

  it("only requesting / uploading / checking hold Lưu", () => {
    expect(coverInFlight({ kind: "requesting" })).toBe(true);
    expect(coverInFlight({ kind: "uploading", id: "C", percent: 1 })).toBe(true);
    expect(coverInFlight({ kind: "checking", id: "C" })).toBe(true);
    for (const s of [{ kind: "idle" }, { kind: "ready", id: "C" }, { kind: "refused", message: "x" }] as const) {
      expect(coverInFlight(s)).toBe(false);
    }
  });
});

describe("preview `src` — only the server's signed `preview_url`, only http(s)", () => {
  const base = { file_id: "C", status: "ready", public: true };
  it("a signed https link passes through", () => {
    const url = "https://files.example.test/vigov-pub/t_01JXA/c.jpg?X-Amz-Signature=S";
    expect(coverPreviewSrc({ ...base, preview_url: url })).toBe(url);
  });
  it("javascript:, data:, relative, empty, absent, null cover ⇒ no src", () => {
    for (const preview_url of ["javascript:alert(1)", "data:image/png;base64,AAAA", "/anh.jpg", "", undefined]) {
      expect(coverPreviewSrc({ ...base, preview_url }), String(preview_url)).toBeNull();
    }
    expect(coverPreviewSrc(null)).toBeNull();
    expect(coverPreviewSrc(undefined)).toBeNull();
  });
});

/* ── The flow over a fake fetch and a fake XMLHttpRequest ─────────────────────────────────────── */

/** The article id the server reserved for a new article's first file (a ULID-shaped stand-in). */
const RESERVED = "01JRESERVEDITEM00000000000";

const UPLOAD = {
  cover_image: { id: "01JCOVER1", content_item_id: RESERVED, mime_type: "", size_bytes: 0, status: "pending" },
  content_item_id: RESERVED,
  upload: {
    url: "https://files.example.test/vigov-stg-temp",
    fields: { key: "upload/t_01JXA/x", policy: "P", "x-amz-signature": "S", "Content-Type": "image/jpeg" },
    expires_at: "2026-10-01T03:15:00Z",
  },
};

const json = (body: unknown, status: number) =>
  new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });

type Sent = { method: string; url: string; withCredentials: boolean; keys: string[] };

/** Records each upload and answers `status` after one progress tick. */
function fakeXHR(status: number) {
  const sent: Sent[] = [];
  class FakeXHR {
    status = 0;
    withCredentials = false;
    upload: { onprogress: ((e: { lengthComputable: boolean; loaded: number; total: number }) => void) | null } = {
      onprogress: null,
    };
    onload: (() => void) | null = null;
    onerror: (() => void) | null = null;
    private m = "";
    private u = "";
    open(method: string, url: string) {
      this.m = method;
      this.u = url;
    }
    send(form: FormData) {
      sent.push({ method: this.m, url: this.u, withCredentials: this.withCredentials, keys: [...form.keys()] });
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

function fakeFetch(completion: Response) {
  const calls: { url: string; init?: RequestInit }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      calls.push({ url, init });
      if (url === "/api/v1/content-items/cover-images") return json(UPLOAD, 201);
      if (url.endsWith("/completion")) return completion.clone();
      if (url === "/api/v1/content-items") return json({ id: "01JND9" }, 201);
      return json({ code: "not_found", message: "?" }, 404);
    }),
  );
  return calls;
}

const photo = () => new File([new Uint8Array([0xff, 0xd8, 0xff])], "anh-hoi-nghi.jpg", { type: "image/jpeg" });

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("runCoverUpload — a → b → c, then the id on the create", () => {
  it("happy path: declare → POST form straight to the store (no cookie, file last) → complete → id sent on Lưu", async () => {
    const xhr = fakeXHR(204);
    const calls = fakeFetch(json({ ...UPLOAD.cover_image, mime_type: "image/jpeg", size_bytes: 3, status: "ready" }, 200));
    const states: CoverUploadState[] = [];

    const last = await runCoverUpload(photo(), undefined, (s) => states.push(s));

    // a. the declaration: no content_item_id on the create form, an Idempotency-Key.
    expect(calls[0]?.url).toBe("/api/v1/content-items/cover-images");
    expect(JSON.parse(String(calls[0]?.init?.body))).toEqual({
      file_name: "anh-hoi-nghi.jpg",
      content_type: "image/jpeg",
      size: 3,
    });
    expect(new Headers(calls[0]?.init?.headers).get("Idempotency-Key")).toMatch(/.+/);
    // b. the bytes went to the STORE's url, not through this app, without credentials, file last.
    expect(xhr).toEqual([
      {
        method: "POST",
        url: UPLOAD.upload.url,
        withCredentials: false,
        keys: ["key", "policy", "x-amz-signature", "Content-Type", "file"],
      },
    ]);
    expect(calls.some((c) => c.url === UPLOAD.upload.url)).toBe(false);
    // c. completion of THAT id.
    expect(calls[1]?.url).toBe("/api/v1/content-items/cover-images/01JCOVER1/completion");
    expect(states.map((s) => s.kind)).toEqual(["requesting", "uploading", "uploading", "checking", "ready"]);
    expect(states).toContainEqual({ kind: "uploading", id: "01JCOVER1", percent: 50 });
    expect(last).toEqual({ kind: "ready", id: "01JCOVER1" });

    // Lưu: the form now holds the id, and the create carries it.
    if (last.kind !== "ready") throw new Error("not ready");
    await themNoiDung(thanThem({ ...FORM_TRONG, title: "Hội nghị", cover_image_file_id: last.id }), "k");
    const create = calls[2];
    expect(create?.url).toBe("/api/v1/content-items");
    expect(JSON.parse(String(create?.init?.body)).cover_image_file_id).toBe("01JCOVER1");
  });

  it("new article: the reserved article id is heard right after the declaration, and again at completion", async () => {
    fakeXHR(204);
    fakeFetch(json({ ...UPLOAD.cover_image, status: "ready" }, 200));
    const heard: string[] = [];
    const seenAt: string[] = [];
    await runCoverUpload(
      photo(),
      undefined,
      (s) => seenAt.push(s.kind),
      (id) => {
        heard.push(id);
        seenAt.push(`article:${id}`);
      },
    );
    expect(heard).toEqual([RESERVED, RESERVED]);
    // Before any byte moves: a body image inserted while this cover uploads already names the article.
    expect(seenAt.indexOf(`article:${RESERVED}`)).toBeLessThan(seenAt.indexOf("uploading"));
  });

  it("`Kiểm tra lại` hears the article id from the completion too", async () => {
    fakeFetch(json({ ...UPLOAD.cover_image, status: "ready" }, 200));
    const heard: string[] = [];
    await retryCoverCompletion("01JCOVER1", () => {}, (id) => heard.push(id));
    expect(heard).toEqual([RESERVED]);
  });

  it("a refused declaration names no article", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => json({ code: "invalid_request", message: "Ảnh quá lớn." }, 400)),
    );
    const heard: string[] = [];
    await runCoverUpload(photo(), undefined, () => {}, (id) => heard.push(id));
    expect(heard).toEqual([]);
  });

  it("edit form: the declaration names the article", async () => {
    fakeXHR(204);
    const calls = fakeFetch(json({ ...UPLOAD.cover_image, status: "ready" }, 200));
    await runCoverUpload(photo(), "01JND1", () => {});
    expect(JSON.parse(String(calls[0]?.init?.body)).content_item_id).toBe("01JND1");
  });

  it("422 at completion: refused with the server's sentence, verbatim", async () => {
    fakeXHR(204);
    const cau = "Ảnh bị từ chối vì phát hiện mã độc và không được lưu.";
    fakeFetch(json({ code: "cover_rejected", message: cau }, 422));
    expect(await runCoverUpload(photo(), undefined, () => {})).toEqual({ kind: "refused", message: cau });
  });

  it("503 at completion: retry the COMPLETION only — no second declaration, no second upload", async () => {
    const xhr = fakeXHR(204);
    const calls = fakeFetch(json({ code: "malware_scan_unavailable", message: "Chưa quét được mã độc." }, 503));
    const first = await runCoverUpload(photo(), undefined, () => {});
    expect(first).toEqual({ kind: "retry", id: "01JCOVER1", message: "Chưa quét được mã độc." });

    await retryCoverCompletion("01JCOVER1", () => {});
    expect(calls.filter((c) => c.url === "/api/v1/content-items/cover-images")).toHaveLength(1);
    expect(calls.filter((c) => c.url.endsWith("/completion"))).toHaveLength(2);
    expect(xhr).toHaveLength(1);
  });

  it("the store refuses the form: one sentence of our own, no completion asked", async () => {
    fakeXHR(403);
    const calls = fakeFetch(json({}, 200));
    expect(await runCoverUpload(photo(), undefined, () => {})).toEqual({ kind: "refused", message: COVER_STORAGE_FAILED });
    expect(calls.some((c) => c.url.endsWith("/completion"))).toBe(false);
  });

  it("a HEIC or a 51 MB file never reaches the network", async () => {
    const xhr = fakeXHR(204);
    const calls = fakeFetch(json({}, 200));
    const heic = new File([new Uint8Array([1])], "IMG_0001.HEIC", { type: "image/heic" });
    expect(await runCoverUpload(heic, undefined, () => {})).toEqual({ kind: "refused", message: COVER_TYPE_REFUSED });
    const big = { name: "a.jpg", type: "image/jpeg", size: 51 * MB } as unknown as File;
    expect(await runCoverUpload(big, undefined, () => {})).toEqual({ kind: "refused", message: COVER_TOO_LARGE });
    expect(calls).toHaveLength(0);
    expect(xhr).toHaveLength(0);
  });
});
