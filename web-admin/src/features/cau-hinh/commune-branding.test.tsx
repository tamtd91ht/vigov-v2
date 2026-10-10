import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import { removeBrandingImage } from "@/lib/api/commune-branding";
import { installFakeUploadXHR, partNames, partValue } from "@/lib/api/upload-test-support";

import {
  BRANDING_ACCEPT,
  BRANDING_EMPTY_FILE,
  BRANDING_MAX_BYTES,
  BRANDING_NOT_READY,
  BRANDING_PICK_BUTTON,
  BRANDING_REPLACE_BUTTON,
  BRANDING_RETRY_BUTTON,
  BRANDING_TEXT,
  BRANDING_TOO_LARGE,
  BRANDING_TYPE_REFUSED,
  afterBrandingUpload,
  brandingInFlight,
  brandingUpdatedText,
  declaredBrandingType,
  runBrandingUpload,
  type BrandingUploadState,
} from "./commune-branding";
import { BrandingCardView, IDLE_CARD, type CardState } from "./commune-branding-tab";

/**
 * Cấu hình › Nhận diện xã (ADR 0069). The pre-check (2 MB, PNG/WebP/JPEG, no HEIC — the server's own
 * `upload_policy` values), the upload answers, the ONE-request flow over the shared fake
 * `XMLHttpRequest`, removal, and the card's markup in each state.
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

const logoFile = () => new File([new Uint8Array([0x89, 0x50, 0x4e, 0x47])], "logo-xa.png", { type: "image/png" });

describe("upload answers → state", () => {
  const ready = { id: "F", mime_type: "image/png", size_bytes: 9, status: "ready", public_url: "https://m.example.test/l.png" };
  const f = logoFile();
  it("201 ready ⇒ done; 201 anything else ⇒ refused; 4xx ⇒ refused VERBATIM; 503 / 408 / no answer ⇒ send again", () => {
    expect(afterBrandingUpload(f, { ok: true, data: ready })).toEqual({ kind: "done" });
    expect(afterBrandingUpload(f, { ok: true, data: { ...ready, status: "rejected" } })).toEqual({
      kind: "refused",
      message: BRANDING_NOT_READY,
    });
    for (const status of [422, 409, 413]) {
      expect(afterBrandingUpload(f, { ok: false, status, code: "c", message: "mã độc" })).toEqual({ kind: "refused", message: "mã độc" });
    }
    for (const status of [503, 408, 0]) {
      expect(afterBrandingUpload(f, { ok: false, status, code: "", message: "x" })).toEqual({ kind: "retry", file: f, message: "x" });
    }
  });

  it("only uploading / checking are in flight", () => {
    expect(brandingInFlight({ kind: "uploading", percent: 1 })).toBe(true);
    expect(brandingInFlight({ kind: "checking" })).toBe(true);
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

/* ── The flow over the shared fake XMLHttpRequest ──────────────────────────────────────────────── */

const READY = { id: "01JLOGO1", mime_type: "image/png", size_bytes: 4, status: "ready", public_url: "https://m.example.test/l.png" };

const json = (body: unknown, status: number) =>
  new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });

const key = () => "11111111-2222-4333-8444-555555555555";

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("runBrandingUpload — ONE request", () => {
  it("happy path: one multipart POST on this origin (Idempotency-Key, size before file) → done", async () => {
    const xhr = installFakeUploadXHR(() => ({ status: 201, body: READY }));
    const fetchSpy = vi.fn();
    vi.stubGlobal("fetch", fetchSpy);
    const states: BrandingUploadState[] = [];

    const last = await runBrandingUpload("logo", logoFile(), key, (s) => states.push(s));

    expect(xhr).toHaveLength(1);
    expect(xhr[0]?.method).toBe("POST");
    expect(xhr[0]?.url).toBe("/api/v1/commune-branding/logo-uploads");
    expect(xhr[0]?.headers["Idempotency-Key"]).toBe(key());
    expect(partNames(xhr[0])).toEqual(["size", "file_name", "file"]);
    expect(partValue(xhr[0], "size")).toBe("4");
    // No declaration, no store host, no completion.
    expect(fetchSpy).not.toHaveBeenCalled();
    expect(states.map((s) => s.kind)).toEqual(["uploading", "uploading", "uploading", "checking", "done"]);
    expect(last).toEqual({ kind: "done" });
  });

  it("the banner goes to the banner route, never the logo's", async () => {
    const xhr = installFakeUploadXHR(() => ({ status: 201, body: READY }));
    await runBrandingUpload("banner", logoFile(), key, () => {});
    expect(xhr.map((x) => x.url)).toEqual(["/api/v1/commune-branding/banner-uploads"]);
  });

  it("a too-large or wrong-type file never reaches the network", async () => {
    const xhr = installFakeUploadXHR(() => ({ status: 201, body: READY }));
    const heic = new File([new Uint8Array([1])], "IMG.HEIC", { type: "image/heic" });
    expect(await runBrandingUpload("logo", heic, key, () => {})).toEqual({ kind: "refused", message: BRANDING_TYPE_REFUSED });
    const big = { name: "a.png", type: "image/png", size: 2 * MB + 1 } as unknown as File;
    expect(await runBrandingUpload("logo", big, key, () => {})).toEqual({ kind: "refused", message: BRANDING_TOO_LARGE });
    expect(xhr).toHaveLength(0);
  });

  it("503 image_processing_busy: the server's sentence; `Gửi lại` sends the SAME file again", async () => {
    const busy = "Hệ thống đang xử lý ảnh khác nên ảnh CHƯA được nhận. Vui lòng thử lại sau ít giây.";
    let n = 0;
    const xhr = installFakeUploadXHR(() =>
      ++n === 1 ? { status: 503, body: { code: "image_processing_busy", message: busy } } : { status: 201, body: READY },
    );
    const f = logoFile();
    const first = await runBrandingUpload("logo", f, key, () => {});
    expect(first).toEqual({ kind: "retry", file: f, message: busy });
    if (first.kind !== "retry") throw new Error("not retry");
    expect(await runBrandingUpload("logo", first.file, key, () => {})).toEqual({ kind: "done" });
    expect(xhr).toHaveLength(2);
  });

  it("422: refused with the server's sentence, verbatim", async () => {
    const cau = "Ảnh bị từ chối vì phát hiện mã độc và không được lưu.";
    installFakeUploadXHR(() => ({ status: 422, body: { code: "image_rejected", message: cau } }));
    expect(await runBrandingUpload("logo", logoFile(), key, () => {})).toEqual({ kind: "refused", message: cau });
  });

  it("DENIED — 403 without admin.org: the server's sentence, final", async () => {
    installFakeUploadXHR(() => ({ status: 403, body: { code: "forbidden", message: "Bạn không có quyền thực hiện thao tác này." } }));
    expect(await runBrandingUpload("logo", logoFile(), key, () => {})).toEqual({
      kind: "refused",
      message: "Bạn không có quyền thực hiện thao tác này.",
    });
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
    const html = card("logo", "", { upload: { kind: "uploading", percent: 0 } });
    expect(html).toContain('role="status"');
    expect(html).toContain("Đang tải lên… 0%");
    expect(html).toMatch(/<input [^>]*disabled=""/);
  });

  it("503 busy: the server's sentence as an alert, and 'Gửi lại'", () => {
    const busy = "Hệ thống đang xử lý ảnh khác nên ảnh CHƯA được nhận. Vui lòng thử lại sau ít giây.";
    const html = card("banner", "", { upload: { kind: "retry", file: logoFile(), message: busy } });
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
