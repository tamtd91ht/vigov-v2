import { afterEach, describe, expect, it, vi } from "vitest";

import { themNoiDung } from "@/lib/api/noi-dung";
import { installFakeUploadXHR, partNames, partValue } from "@/lib/api/upload-test-support";

import {
  COVER_ACCEPT,
  COVER_EMPTY,
  COVER_MAX_BYTES,
  COVER_NOT_READY,
  COVER_TOO_LARGE,
  COVER_TYPE_REFUSED,
  afterCoverUpload,
  coverInFlight,
  coverPreviewSrc,
  declaredCoverType,
  runCoverUpload,
  type CoverUploadState,
} from "./cover-image";
import { FORM_TRONG, thanThem } from "./nhan-noi-dung";

/**
 * §7 `Ảnh đại diện` — the cover upload. The pre-check (the user's 01/10/2026 decision: JPG/PNG/WebP,
 * 50 MB, NO HEIC), the upload answers, the preview `src`, and the one-request flow over the shared fake
 * `XMLHttpRequest` (no DOM), ending with the id on the create body.
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

describe("upload answers → state", () => {
  const ready = { id: "C", content_item_id: "01JND9", mime_type: "image/jpeg", size_bytes: 9, status: "ready" };
  const f = new File([new Uint8Array([1])], "a.jpg", { type: "image/jpeg" });
  it("201 ready ⇒ ready; 201 anything else ⇒ refused; 4xx ⇒ refused VERBATIM; 503 / 408 / no answer ⇒ send again", () => {
    expect(afterCoverUpload(f, { ok: true, data: ready })).toEqual({ kind: "ready", id: "C" });
    expect(afterCoverUpload(f, { ok: true, data: { ...ready, status: "rejected" } })).toEqual({
      kind: "refused",
      message: COVER_NOT_READY,
    });
    for (const status of [422, 409, 413]) {
      expect(afterCoverUpload(f, { ok: false, status, code: "c", message: "mã độc" })).toEqual({ kind: "refused", message: "mã độc" });
    }
    for (const status of [503, 408, 0]) {
      expect(afterCoverUpload(f, { ok: false, status, code: "", message: "x" })).toEqual({ kind: "retry", file: f, message: "x" });
    }
  });

  it("only uploading / checking hold Lưu", () => {
    expect(coverInFlight({ kind: "uploading", percent: 1 })).toBe(true);
    expect(coverInFlight({ kind: "checking" })).toBe(true);
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

/* ── The flow over the shared fake XMLHttpRequest ──────────────────────────────────────────────── */

/** The article id the server reserved for a new article's first file (a ULID-shaped stand-in). */
const RESERVED = "01JRESERVEDITEM00000000000";
const READY = { id: "01JCOVER1", content_item_id: RESERVED, mime_type: "image/jpeg", size_bytes: 3, status: "ready" };

const json = (body: unknown, status: number) =>
  new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });

/** Every fetch is recorded: the cover flow must not make any (only the Lưu create does). */
function fakeFetch() {
  const calls: { url: string; init?: RequestInit }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      calls.push({ url, init });
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

describe("runCoverUpload — ONE request, then the id on the create", () => {
  it("happy path: one multipart POST on this origin (size before file) → ready → id sent on Lưu", async () => {
    const xhr = installFakeUploadXHR(() => ({ status: 201, body: READY }));
    const calls = fakeFetch();
    const states: CoverUploadState[] = [];

    const last = await runCoverUpload(photo(), undefined, (s) => states.push(s));

    expect(xhr).toHaveLength(1);
    expect(xhr[0]?.url).toBe("/api/v1/content-items/cover-images");
    expect(xhr[0]?.headers["Idempotency-Key"]).toMatch(/.+/);
    expect(partNames(xhr[0])).toEqual(["size", "file_name", "content_type", "file"]);
    expect(partValue(xhr[0], "size")).toBe("3");
    // No declaration, no store host, no completion: nothing else went out.
    expect(calls).toHaveLength(0);
    expect(states.map((s) => s.kind)).toEqual(["uploading", "uploading", "uploading", "checking", "ready"]);
    expect(states).toContainEqual({ kind: "uploading", percent: 50 });
    expect(last).toEqual({ kind: "ready", id: "01JCOVER1" });

    // Lưu: the form now holds the id, and the create carries it.
    if (last.kind !== "ready") throw new Error("not ready");
    await themNoiDung(thanThem({ ...FORM_TRONG, title: "Hội nghị", cover_image_file_id: last.id }), "k");
    expect(calls[0]?.url).toBe("/api/v1/content-items");
    expect(JSON.parse(String(calls[0]?.init?.body)).cover_image_file_id).toBe("01JCOVER1");
  });

  it("new article: the reserved article id is heard from the reply", async () => {
    installFakeUploadXHR(() => ({ status: 201, body: READY }));
    const heard: string[] = [];
    await runCoverUpload(photo(), undefined, () => {}, (id) => heard.push(id));
    expect(heard).toEqual([RESERVED]);
  });

  it("a refused upload names no article", async () => {
    installFakeUploadXHR(() => ({ status: 413, body: { code: "file_too_large", message: "Ảnh quá lớn." } }));
    const heard: string[] = [];
    await runCoverUpload(photo(), undefined, () => {}, (id) => heard.push(id));
    expect(heard).toEqual([]);
  });

  it("edit form: the upload names the article", async () => {
    const xhr = installFakeUploadXHR(() => ({ status: 201, body: READY }));
    await runCoverUpload(photo(), "01JND1", () => {});
    expect(partValue(xhr[0], "content_item_id")).toBe("01JND1");
  });

  it("422: refused with the server's sentence, verbatim", async () => {
    const cau = "Ảnh bị từ chối vì phát hiện mã độc và không được lưu.";
    installFakeUploadXHR(() => ({ status: 422, body: { code: "cover_rejected", message: cau } }));
    expect(await runCoverUpload(photo(), undefined, () => {})).toEqual({ kind: "refused", message: cau });
  });

  it("503: `retry` carries the SAME file; sending it again is a second request with a new key", async () => {
    let n = 0;
    const xhr = installFakeUploadXHR(() =>
      ++n === 1
        ? { status: 503, body: { code: "malware_scan_unavailable", message: "Chưa quét được mã độc." } }
        : { status: 201, body: READY },
    );
    const f = photo();
    const first = await runCoverUpload(f, undefined, () => {});
    expect(first).toEqual({ kind: "retry", file: f, message: "Chưa quét được mã độc." });
    if (first.kind !== "retry") throw new Error("not retry");
    expect(await runCoverUpload(first.file, undefined, () => {})).toEqual({ kind: "ready", id: "01JCOVER1" });
    expect(xhr).toHaveLength(2);
    expect(xhr[1]?.headers["Idempotency-Key"]).not.toBe(xhr[0]?.headers["Idempotency-Key"]);
  });

  it("a HEIC or a 51 MB file never reaches the network", async () => {
    const xhr = installFakeUploadXHR(() => ({ status: 201, body: READY }));
    const calls = fakeFetch();
    const heic = new File([new Uint8Array([1])], "IMG_0001.HEIC", { type: "image/heic" });
    expect(await runCoverUpload(heic, undefined, () => {})).toEqual({ kind: "refused", message: COVER_TYPE_REFUSED });
    const big = { name: "a.jpg", type: "image/jpeg", size: 51 * MB } as unknown as File;
    expect(await runCoverUpload(big, undefined, () => {})).toEqual({ kind: "refused", message: COVER_TOO_LARGE });
    expect(calls).toHaveLength(0);
    expect(xhr).toHaveLength(0);
  });
});
