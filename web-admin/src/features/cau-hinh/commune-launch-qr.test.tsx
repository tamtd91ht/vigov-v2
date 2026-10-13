// @vitest-environment jsdom
//
// ADR 0070 §Sửa đổi 13/10/2026: one QR for the commune, no word about the shared app, PNG + print.

import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import { drawQrModules, qrMatrix, qrPixelSize } from "@/components/qr-code";
import { getCommuneLaunchLink, LAUNCH_QR_UNAVAILABLE } from "@/lib/api/commune-launch-link";

import { CommuneLaunchQrView, QR_FILE_NAME, QrPrintSheet, SCAN_INSTRUCTION, type LaunchQrState } from "./commune-launch-qr";

const URL_OWN = "https://zalo.me/s/1234567890123456789/?src=qr";
const NO_ACTIONS = { onDownload: () => {}, onPrint: () => {} };
/** What the owner ruled out on this screen (13/10/2026): any trace of which app the code opens. */
const SHARED_APP_WORDING = /app chung|dùng chung|ViHAT|App ID/i;

/** Static markup parsed into a detached document — read, never mounted. */
function parse(markup: string): HTMLElement {
  return new DOMParser().parseFromString(markup, "text/html").body;
}

function render(state: LaunchQrState): HTMLElement {
  return parse(renderToStaticMarkup(<CommuneLaunchQrView state={state} communeName="Xã Thử Nghiệm" actions={NO_ACTIONS} />));
}

function buttonTexts(el: HTMLElement): string[] {
  return [...el.querySelectorAll("button")].map((b) => b.textContent?.trim() ?? "");
}

describe("CommuneLaunchQrView", () => {
  it("ready: exactly one QR, Tải ảnh PNG and In / Lưu PDF, no shared-app wording, no raw link", () => {
    const el = render({ status: "ready", url: URL_OWN });
    expect(el.querySelectorAll('svg[role="img"]')).toHaveLength(1);
    // Exactly these two: no "Chép liên kết" — the link carries ViHAT's App ID (owner 13/10/2026).
    expect(buttonTexts(el)).toEqual(["Tải ảnh PNG", "In / Lưu PDF"]);
    expect(el.textContent).not.toMatch(SHARED_APP_WORDING);
    // The link carries the App ID: it is encoded in the QR, never printed as text or in an attribute.
    expect(el.innerHTML).not.toContain("1234567890123456789");
  });

  it("refused: the neutral sentence, no QR and no action", () => {
    const el = render({ status: "refused", message: LAUNCH_QR_UNAVAILABLE });
    expect(el.querySelector('[role="alert"]')?.textContent).toBe(LAUNCH_QR_UNAVAILABLE);
    expect(el.querySelector("svg")).toBeNull();
    expect(buttonTexts(el)).toEqual([]);
    expect(el.textContent).not.toMatch(SHARED_APP_WORDING);
  });

  it("loading: no QR yet", () => {
    const el = render({ status: "loading" });
    expect(el.querySelector("svg")).toBeNull();
    expect(el.querySelector('[role="status"]')?.textContent).toContain("Đang tạo mã QR");
  });

  it("the print sheet: commune name, one QR, the scan line — and the PNG name carries no place or person", () => {
    const el = parse(renderToStaticMarkup(<QrPrintSheet communeName="Xã Thử Nghiệm" url={URL_OWN} />));
    expect(el.querySelector("h1")?.textContent).toBe("Xã Thử Nghiệm");
    expect(el.querySelectorAll("svg")).toHaveLength(1);
    expect(el.textContent).toContain(SCAN_INSTRUCTION);
    expect(QR_FILE_NAME).toBe("qr-mini-app.png");
  });
});

describe("QR drawing", () => {
  it("PNG side is a whole number of pixels per module, at least 1024", () => {
    const qr = qrMatrix(URL_OWN);
    const side = qrPixelSize(qr.size, 1024);
    expect(side).toBeGreaterThanOrEqual(1024);
    expect(side % qr.size).toBe(0);
    const rects: number[][] = [];
    drawQrModules({ fillStyle: "", fillRect: (...a) => rects.push(a) }, qr.data, side);
    const dark = qr.data.flat().filter(Boolean).length;
    expect(rects).toHaveLength(dark + 1); // the white ground, then one square per dark module
  });
});

describe("getCommuneLaunchLink", () => {
  afterEach(() => vi.unstubAllGlobals());

  function answer(status: number, body: unknown) {
    const f = vi.fn(async () => new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } }));
    vi.stubGlobal("fetch", f);
    return f;
  }

  it("200: the url only — `source` never leaves the client module", async () => {
    const f = answer(200, { url: URL_OWN, source: "chung" });
    const r = await getCommuneLaunchLink();
    expect(r).toEqual({ ok: true, data: { url: URL_OWN } });
    expect(f).toHaveBeenCalledWith("/api/v1/commune-launch-link", expect.objectContaining({ method: "GET" }));
  });

  it.each(["commune_inactive", "shared_mini_app_not_declared", "shared_mini_app_ambiguous", "own_mini_app_ambiguous", "commune_launch_code_invalid"])(
    "409 %s: the neutral sentence, not the server's",
    async (code) => {
      answer(409, { code, message: "Chưa khai báo Mini App dùng chung" });
      const r = await getCommuneLaunchLink();
      expect(r).toEqual({ ok: false, status: 409, message: LAUNCH_QR_UNAVAILABLE });
    },
  );

  it("500: the server's sentence, as every other web-admin read", async () => {
    answer(500, { code: "internal", message: "Máy chủ bận" });
    const r = await getCommuneLaunchLink();
    expect(r).toEqual({ ok: false, status: 500, message: "Máy chủ bận" });
  });

  it("200 without an https link: refused, nothing to print", async () => {
    answer(200, { url: "", source: "rieng" });
    const r = await getCommuneLaunchLink();
    expect(r).toEqual({ ok: false, status: 200, message: LAUNCH_QR_UNAVAILABLE });
  });
});
