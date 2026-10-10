import { afterEach, describe, expect, it, vi } from "vitest";

import { installFakeUploadXHR, partNames, partValue } from "@/lib/api/upload-test-support";

import {
  BODY_IMAGE_NOT_READY,
  BODY_IMAGE_URL_INVALID,
  bodyImageErrorText,
  bodyImageInFlight,
  bodyImagePreviewSrc,
  previewsFromItem,
  runBodyImageFromUrl,
  runBodyImageUpload,
  type BodyImageState,
} from "./body-image";
import { COVER_TYPE_REFUSED } from "./cover-image";

/**
 * Body images (ADR 0067 §Sửa đổi 03/10/2026): the upload as ONE multipart request (ADR 0052 §Sửa đổi
 * 09/10/2026) and the server fetch, the article id the first image reserves, and one Vietnamese sentence
 * per refusal code — never a raw code.
 */

const FILE_ID = "01JBDYXMG00000000000000001";
const RESERVED = "01JRESERVEDITEM00000000000";
const PREVIEW = "https://files.example.test/vigov-stg-private/thumb-1280.jpg?X-Amz-Signature=s";

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

function fakeFetch(answers: { fromUrl?: Response }) {
  const calls: { url: string; init?: RequestInit }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      calls.push({ url, init });
      if (url === "/api/v1/content-items/body-images/from-url") return (answers.fromUrl ?? json(READY, 201)).clone();
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

describe("Tải ảnh từ máy — ONE multipart request", () => {
  it("first image of a new article: no content_item_id sent, the reserved id heard, ready with its preview", async () => {
    const xhr = installFakeUploadXHR(() => ({ status: 201, body: READY }));
    const calls = fakeFetch({});
    const states: BodyImageState[] = [];
    const articles: string[] = [];

    const last = await runBodyImageUpload(photo(), undefined, (s) => states.push(s), (id) => articles.push(id));

    expect(xhr).toHaveLength(1);
    expect(xhr[0]?.url).toBe("/api/v1/content-items/body-images");
    expect(xhr[0]?.headers["Idempotency-Key"]).toMatch(/.+/);
    expect(partNames(xhr[0])).toEqual(["size", "file_name", "content_type", "file"]);
    // No declaration, no store host, no completion.
    expect(calls).toHaveLength(0);
    expect(articles).toEqual([RESERVED]);
    expect(states.map((s) => s.kind)).toEqual(["uploading", "uploading", "uploading", "checking", "ready"]);
    expect(last).toEqual({ kind: "ready", image: { fileId: FILE_ID, contentItemId: RESERVED, previewUrl: PREVIEW } });
  });

  it("a later image of the same article sends the id back", async () => {
    const xhr = installFakeUploadXHR(() => ({ status: 201, body: READY }));
    await runBodyImageUpload(photo(), RESERVED, () => {}, () => {});
    expect(partValue(xhr[0], "content_item_id")).toBe(RESERVED);
  });

  it("HEIC never reaches the network (the cover's pre-check)", async () => {
    const xhr = installFakeUploadXHR(() => ({ status: 201, body: READY }));
    const calls = fakeFetch({});
    const heic = new File([new Uint8Array([1])], "IMG_0001.HEIC", { type: "image/heic" });
    expect(await runBodyImageUpload(heic, undefined, () => {}, () => {})).toEqual({ kind: "refused", message: COVER_TYPE_REFUSED });
    expect(calls).toHaveLength(0);
    expect(xhr).toHaveLength(0);
  });

  it("409 body_image_limit: our sentence, final", async () => {
    installFakeUploadXHR(() => ({ status: 409, body: { code: "body_image_limit", message: "server words" } }));
    const s = await runBodyImageUpload(photo(), RESERVED, () => {}, () => {});
    expect(s).toEqual({ kind: "refused", message: bodyImageErrorText({ code: "body_image_limit", message: "" }) });
    expect(s.kind === "refused" && s.message).not.toContain("body_image_limit");
  });

  it("422 body_image_rejected: final, worded", async () => {
    installFakeUploadXHR(() => ({ status: 422, body: { code: "body_image_rejected", message: "x" } }));
    const s = await runBodyImageUpload(photo(), undefined, () => {}, () => {});
    expect(s.kind).toBe("refused");
    expect(s.kind === "refused" && s.message).toMatch(/từ chối/);
  });

  it("503: `retry` carries the same file — sending it again is a NEW request", async () => {
    let n = 0;
    const xhr = installFakeUploadXHR(() =>
      ++n === 1 ? { status: 503, body: { code: "malware_scan_unavailable", message: "x" } } : { status: 201, body: READY },
    );
    const f = photo();
    const first = await runBodyImageUpload(f, undefined, () => {}, () => {});
    expect(first).toEqual({ kind: "retry", file: f, message: bodyImageErrorText({ code: "malware_scan_unavailable", message: "x" }) });
    if (first.kind !== "retry") throw new Error("not retry");
    expect((await runBodyImageUpload(first.file, undefined, () => {}, () => {})).kind).toBe("ready");
    expect(xhr).toHaveLength(2);
  });

  it("an upload envelope refusal (408) keeps the server's sentence and may be sent again", async () => {
    const cau = "Tải tệp lên quá thời gian cho phép nên tệp CHƯA được nhận. Vui lòng thử lại.";
    installFakeUploadXHR(() => ({ status: 408, body: { code: "upload_timeout", message: cau } }));
    const s = await runBodyImageUpload(photo(), undefined, () => {}, () => {});
    expect(s.kind === "retry" && s.message).toBe(cau);
  });

  it("a 201 that is not ready is not inserted", async () => {
    installFakeUploadXHR(() => ({ status: 201, body: { ...READY, status: "failed" } }));
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
      expect(msg, code).toBe(bodyImageErrorText({ code, message: "" }));
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
    "body_image_rejected",
    "image_fetch_failed",
    "storage_not_configured",
    "upload_limits_unavailable",
    "malware_scan_unavailable",
  ];

  it("every code of the two routes has a sentence of its own, never the code itself", () => {
    for (const code of CODES) {
      const t = bodyImageErrorText({ code, message: "MÁY CHỦ" });
      expect(t, code).not.toBe("MÁY CHỦ");
      expect(t, code).not.toContain(code);
      expect(t, code).toMatch(/[ảàáạãăâđêôơư]/i);
    }
  });

  it("not_found names the article", () => {
    expect(bodyImageErrorText({ code: "not_found", message: "" })).toMatch(/bài/);
  });

  it("an unknown code (a 403) keeps the server's own sentence", () => {
    expect(bodyImageErrorText({ code: "forbidden", message: "Bạn không có quyền." })).toBe("Bạn không có quyền.");
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

  it("in flight: uploading, checking, fetching — not retry or refused", () => {
    expect(bodyImageInFlight({ kind: "fetching" })).toBe(true);
    expect(bodyImageInFlight({ kind: "checking" })).toBe(true);
    expect(bodyImageInFlight({ kind: "uploading", percent: 1 })).toBe(true);
    expect(bodyImageInFlight({ kind: "retry", file: photo(), message: "" })).toBe(false);
    expect(bodyImageInFlight({ kind: "refused", message: "" })).toBe(false);
  });
});
