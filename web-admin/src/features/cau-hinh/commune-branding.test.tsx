import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import { removeBrandingImage } from "@/lib/api/commune-branding";

import {
  BRANDING_ACCEPT,
  BRANDING_EMPTY_FILE,
  BRANDING_MAX_BYTES,
  BRANDING_NOT_READY,
  BRANDING_PICK_BUTTON,
  BRANDING_REPLACE_BUTTON,
  BRANDING_RETRY_BUTTON,
  BRANDING_STORAGE_FAILED,
  BRANDING_TEXT,
  BRANDING_TOO_LARGE,
  BRANDING_TYPE_REFUSED,
  afterBrandingCompletion,
  brandingInFlight,
  brandingUpdatedText,
  declaredBrandingType,
  retryBrandingCompletion,
  runBrandingUpload,
  type BrandingUploadState,
} from "./commune-branding";
import { BrandingCardView, IDLE_CARD, type CardState } from "./commune-branding-tab";

/**
 * Cấu hình › Nhận diện xã (ADR 0069). The pre-check (2 MB, PNG/WebP/JPEG, no HEIC — the server's own
 * `upload_policy` values), the completion answers, the whole a → b → c flow over a fake `fetch` and a
 * fake `XMLHttpRequest`, removal, and the card's markup in each state.
 */

const MB = 1024 * 1024;

describe("pre-check — convenience; the server's policy still decides", () => {
  it("PNG / WebP / JPEG by the browser's type, or by extension when the type is empty", () => {
    expect(declaredBrandingType({ name: "logo.png", type: "image/png", size: 1 })).toEqual({ ok: true, contentType: "image/png" });
    expect(declaredBrandingType({ name: "b.WEBP", type: "", size: 1 })).toEqual({ ok: true, contentType: "image/webp" });
    expect(declaredBrandingType({ name: "b.jpeg", type: "image/jpeg", size: 1 })).toEqual({ ok: true, contentType: "image/jpeg" });
    expect(BRANDING_ACCEPT).toContain("image/png");
  });

  it("HEIC, GIF and SVG are refused (ADR 0069 #4)", () => {
    for (const f of [
      { name: "IMG.HEIC", type: "image/heic", size: 1 },
      { name: "IMG.heic", type: "", size: 1 },
      { name: "a.gif", type: "image/gif", size: 1 },
      { name: "logo.svg", type: "image/svg+xml", size: 1 },
    ]) {
      expect(declaredBrandingType(f), f.name).toEqual({ ok: false, message: BRANDING_TYPE_REFUSED });
    }
    expect(BRANDING_ACCEPT.toLowerCase()).not.toContain("heic");
    expect(BRANDING_ACCEPT.toLowerCase()).not.toContain("svg");
  });

  it("the limit is the server's 2 097 152 bytes: exactly 2 MB passes, one byte more is refused; empty is refused", () => {
    expect(BRANDING_MAX_BYTES).toBe(2097152);
    expect(declaredBrandingType({ name: "a.png", type: "image/png", size: 2 * MB }).ok).toBe(true);
    expect(declaredBrandingType({ name: "a.png", type: "image/png", size: 2 * MB + 1 })).toEqual({
      ok: false,
      message: BRANDING_TOO_LARGE,
    });
    expect(declaredBrandingType({ name: "a.png", type: "image/png", size: 0 })).toEqual({ ok: false, message: BRANDING_EMPTY_FILE });
  });
});

describe("completion answers → state", () => {
  const ready = { id: "F", mime_type: "image/png", size_bytes: 9, status: "ready", public_url: "https://m.example.test/l.png" };
  it("200 ready ⇒ done; 200 anything else ⇒ refused; 422 ⇒ refused VERBATIM; 503 / 409 / no answer ⇒ retry", () => {
    expect(afterBrandingCompletion("F", { ok: true, data: ready })).toEqual({ kind: "done" });
    expect(afterBrandingCompletion("F", { ok: true, data: { ...ready, status: "rejected" } })).toEqual({
      kind: "refused",
      message: BRANDING_NOT_READY,
    });
    expect(afterBrandingCompletion("F", { ok: false, status: 422, message: "mã độc" })).toEqual({ kind: "refused", message: "mã độc" });
    for (const status of [503, 409, 0]) {
      expect(afterBrandingCompletion("F", { ok: false, status, message: "x" })).toEqual({ kind: "retry", id: "F", message: "x" });
    }
  });

  it("only requesting / uploading / checking are in flight", () => {
    expect(brandingInFlight({ kind: "requesting" })).toBe(true);
    expect(brandingInFlight({ kind: "uploading", id: "F", percent: 1 })).toBe(true);
    expect(brandingInFlight({ kind: "checking", id: "F" })).toBe(true);
    for (const s of [{ kind: "idle" }, { kind: "done" }, { kind: "refused", message: "x" }] as const) {
      expect(brandingInFlight(s)).toBe(false);
    }
  });

  it("the 'last changed' line names the CB- code, and is absent with no profile", () => {
    expect(brandingUpdatedText("2026-10-02T03:04:00Z", "CB-00123")).toMatch(/^Cập nhật lần cuối: .+, bởi CB-00123$/);
    expect(brandingUpdatedText(undefined, undefined)).toBe("");
    expect(brandingUpdatedText(null, "CB-00123")).toBe("");
    expect(brandingUpdatedText("không phải ngày", "CB-00123")).toBe("");
  });
});

/* ── The flow over a fake fetch and a fake XMLHttpRequest ─────────────────────────────────────── */

const UPLOAD = {
  file: { id: "01JLOGO1", mime_type: "", size_bytes: 0, status: "pending", public_url: "" },
  upload: {
    url: "https://files.example.test/vigov-stg-temp",
    fields: { key: "upload/t_01JXA/x", policy: "P", "x-amz-signature": "S", "Content-Type": "image/png" },
    expires_at: "2026-10-02T03:15:00Z",
  },
};

const json = (body: unknown, status: number) =>
  new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });

type Sent = { method: string; url: string; withCredentials: boolean; keys: string[] };

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

function fakeFetch(completion: () => Response) {
  const calls: { url: string; init?: RequestInit }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      calls.push({ url, init });
      if (url === "/api/v1/commune-branding/logo-uploads" || url === "/api/v1/commune-branding/banner-uploads") {
        return json(UPLOAD, 201);
      }
      if (url.endsWith("/completion")) return completion();
      return json({ code: "not_found", message: "?" }, 404);
    }),
  );
  return calls;
}

const logoFile = () => new File([new Uint8Array([0x89, 0x50, 0x4e, 0x47])], "logo-xa.png", { type: "image/png" });
const key = () => "11111111-2222-4333-8444-555555555555";

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("runBrandingUpload — a → b → c", () => {
  it("happy path: declare (Idempotency-Key) → POST form straight to the store (no cookie, file last) → complete", async () => {
    const xhr = fakeXHR(204);
    const calls = fakeFetch(() => json({ ...UPLOAD.file, mime_type: "image/png", size_bytes: 4, status: "ready" }, 200));
    const states: BrandingUploadState[] = [];

    const last = await runBrandingUpload("logo", logoFile(), key, (s) => states.push(s));

    expect(calls[0]?.url).toBe("/api/v1/commune-branding/logo-uploads");
    expect(calls[0]?.init?.method).toBe("POST");
    expect(JSON.parse(String(calls[0]?.init?.body))).toEqual({ file_name: "logo-xa.png", content_type: "image/png", size: 4 });
    expect(new Headers(calls[0]?.init?.headers).get("Idempotency-Key")).toBe(key());
    expect(xhr).toEqual([
      { method: "POST", url: UPLOAD.upload.url, withCredentials: false, keys: ["key", "policy", "x-amz-signature", "Content-Type", "file"] },
    ]);
    expect(calls[1]?.url).toBe("/api/v1/commune-branding/logo-uploads/01JLOGO1/completion");
    expect(states.map((s) => s.kind)).toEqual(["requesting", "uploading", "uploading", "checking", "done"]);
    expect(last).toEqual({ kind: "done" });
  });

  it("the banner goes to the banner routes, never the logo's", async () => {
    fakeXHR(204);
    const calls = fakeFetch(() => json({ ...UPLOAD.file, status: "ready" }, 200));
    await runBrandingUpload("banner", logoFile(), key, () => {});
    expect(calls.map((c) => c.url)).toEqual([
      "/api/v1/commune-branding/banner-uploads",
      "/api/v1/commune-branding/banner-uploads/01JLOGO1/completion",
    ]);
  });

  it("a too-large or wrong-type file never reaches the network", async () => {
    const xhr = fakeXHR(204);
    const calls = fakeFetch(() => json({}, 200));
    const heic = new File([new Uint8Array([1])], "IMG.HEIC", { type: "image/heic" });
    expect(await runBrandingUpload("logo", heic, key, () => {})).toEqual({ kind: "refused", message: BRANDING_TYPE_REFUSED });
    const big = { name: "a.png", type: "image/png", size: 2 * MB + 1 } as unknown as File;
    expect(await runBrandingUpload("logo", big, key, () => {})).toEqual({ kind: "refused", message: BRANDING_TOO_LARGE });
    expect(calls).toHaveLength(0);
    expect(xhr).toHaveLength(0);
  });

  it("503 image_processing_busy: the server's sentence, then 'Hoàn tất lại' retries the COMPLETION only", async () => {
    const xhr = fakeXHR(204);
    const busy = "Hệ thống đang xử lý ảnh khác nên ảnh CHƯA được nhận. Vui lòng bấm hoàn tất lại sau ít giây.";
    let answer = () => json({ code: "image_processing_busy", message: busy }, 503);
    const calls = fakeFetch(() => answer());

    const first = await runBrandingUpload("logo", logoFile(), key, () => {});
    expect(first).toEqual({ kind: "retry", id: "01JLOGO1", message: busy });

    answer = () => json({ ...UPLOAD.file, status: "ready" }, 200);
    expect(await retryBrandingCompletion("logo", "01JLOGO1", () => {})).toEqual({ kind: "done" });
    expect(calls.filter((c) => c.url === "/api/v1/commune-branding/logo-uploads")).toHaveLength(1);
    expect(calls.filter((c) => c.url.endsWith("/completion"))).toHaveLength(2);
    expect(xhr).toHaveLength(1);
  });

  it("422 at completion: refused with the server's sentence, verbatim", async () => {
    fakeXHR(204);
    const cau = "Ảnh bị từ chối vì phát hiện mã độc và không được lưu.";
    fakeFetch(() => json({ code: "image_rejected", message: cau }, 422));
    expect(await runBrandingUpload("logo", logoFile(), key, () => {})).toEqual({ kind: "refused", message: cau });
  });

  it("the store refuses the form: one sentence of our own, no completion asked", async () => {
    fakeXHR(403);
    const calls = fakeFetch(() => json({}, 200));
    expect(await runBrandingUpload("logo", logoFile(), key, () => {})).toEqual({ kind: "refused", message: BRANDING_STORAGE_FAILED });
    expect(calls.some((c) => c.url.endsWith("/completion"))).toBe(false);
  });

  it("the declaration refused (403 without admin.org): the server's sentence, no upload", async () => {
    const xhr = fakeXHR(204);
    vi.stubGlobal("fetch", vi.fn(async () => json({ code: "forbidden", message: "Bạn không có quyền thực hiện thao tác này." }, 403)));
    expect(await runBrandingUpload("logo", logoFile(), key, () => {})).toEqual({
      kind: "refused",
      message: "Bạn không có quyền thực hiện thao tác này.",
    });
    expect(xhr).toHaveLength(0);
  });
});

describe("removeBrandingImage", () => {
  it("DELETE the kind's route; 204 is success", async () => {
    const calls: { url: string; method?: string }[] = [];
    vi.stubGlobal(
      "fetch",
      vi.fn(async (url: string, init?: RequestInit) => {
        calls.push({ url, method: init?.method });
        return new Response(null, { status: 204 });
      }),
    );
    expect(await removeBrandingImage("logo")).toEqual({ ok: true, data: null });
    expect(await removeBrandingImage("banner")).toEqual({ ok: true, data: null });
    expect(calls).toEqual([
      { url: "/api/v1/commune-branding/logo", method: "DELETE" },
      { url: "/api/v1/commune-branding/banner", method: "DELETE" },
    ]);
  });

  it("a refusal carries the server's sentence verbatim", async () => {
    const cau = "Hồ sơ hiển thị của xã đã bị gỡ nên chưa đặt được ảnh nhận diện. Vui lòng liên hệ đơn vị vận hành.";
    vi.stubGlobal("fetch", vi.fn(async () => json({ code: "profile_deleted", message: cau }, 409)));
    expect(await removeBrandingImage("logo")).toEqual({ ok: false, status: 409, message: cau });
  });
});

/* ── The card's markup ─────────────────────────────────────────────────────────────────────────── */

const LOGO_URL = "https://media.example.test/vigov-public/t_01JXA/logo-512.png";
const noop = () => {};

function card(kind: "logo" | "banner", currentUrl: string, state: Partial<CardState> = {}) {
  return renderToStaticMarkup(
    <BrandingCardView
      kind={kind}
      currentUrl={currentUrl}
      updatedText="Cập nhật lần cuối: 10:04 02/10/2026, bởi CB-00123"
      card={{ ...IDLE_CARD, ...state }}
      onPick={noop}
      onRetry={noop}
      onAskRemove={noop}
      onCancelRemove={noop}
      onConfirmRemove={noop}
    />,
  );
}

/** React escapes some characters in text; compare against the escaped form. */
const esc = (s: string) => s.replace(/&/g, "&amp;");

describe("BrandingCardView", () => {
  it("no logo: the building-icon empty state, 'Tải ảnh lên', no remove button, no image", () => {
    const html = card("logo", "");
    expect(html).toContain(esc(BRANDING_TEXT.logo.empty));
    expect(html).toContain("lucide-landmark");
    expect(html).toContain(BRANDING_PICK_BUTTON);
    expect(html).not.toContain(BRANDING_TEXT.logo.removeButton);
    expect(html).not.toContain("<img");
    expect(html).toContain('type="file"');
    expect(html).toContain(`accept="${BRANDING_ACCEPT}"`);
    expect(html).toContain(esc(BRANDING_TEXT.logo.hint));
  });

  it("no banner: 'Chưa có banner' sentence", () => {
    expect(card("banner", "")).toContain("Chưa có banner");
  });

  it("a logo: the preview (named, contained), 'Tải ảnh khác', 'Gỡ logo', who changed it last (CB- code)", () => {
    const html = card("logo", LOGO_URL);
    expect(html.match(/<img /g)).toHaveLength(1);
    expect(html).toContain(`src="${LOGO_URL}"`);
    expect(html).toContain(`alt="${BRANDING_TEXT.logo.previewLabel}"`);
    expect(html).toContain("object-contain");
    expect(html).toContain(BRANDING_REPLACE_BUTTON);
    expect(html).toContain(">Gỡ logo</button>");
    expect(html).toContain("bởi CB-00123");
  });

  it("remove asks first: the specific question, the consequence, an action-named button and Huỷ", () => {
    const html = card("logo", LOGO_URL, { confirming: true });
    expect(html).toContain(BRANDING_TEXT.logo.confirmTitle);
    expect(html).toContain(esc(BRANDING_TEXT.logo.confirmConsequence));
    expect(html.match(/>Gỡ logo<\/button>/g)).toHaveLength(1);
    expect(html).toContain(">Huỷ</button>");
    expect(html).not.toContain(">OK<");
  });

  it("in flight: the busy sentence as a status, the picker disabled", () => {
    const html = card("logo", "", { upload: { kind: "requesting" } });
    expect(html).toContain('role="status"');
    expect(html).toContain("Đang tải lên…");
    expect(html).toMatch(/<input [^>]*disabled=""/);
  });

  it("503 busy: the server's sentence as an alert, and 'Hoàn tất lại'", () => {
    const busy = "Hệ thống đang xử lý ảnh khác nên ảnh CHƯA được nhận. Vui lòng bấm hoàn tất lại sau ít giây.";
    const html = card("banner", "", { upload: { kind: "retry", id: "01JLOGO1", message: busy } });
    expect(html).toContain('role="alert"');
    expect(html).toContain(busy);
    expect(html).toContain(BRANDING_RETRY_BUTTON);
  });

  it("refused by the pre-check: the refusal as an alert, no retry", () => {
    const html = card("logo", "", { upload: { kind: "refused", message: BRANDING_TOO_LARGE } });
    expect(html).toContain('role="alert"');
    expect(html).toContain(BRANDING_TOO_LARGE);
    expect(html).not.toContain(BRANDING_RETRY_BUTTON);
  });
});
