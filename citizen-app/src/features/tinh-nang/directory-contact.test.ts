import { beforeEach, describe, expect, it, vi } from "vitest";

/**
 * THE COMMUNE DIRECTORY'S TWO PLATFORM DOORS (customer sheet App Xã 12/10/2026 rows 11, 12, 14) — `dialStaffNumber`
 * (`openPhone`) and `openZaloChat` (`openOutApp` + zalo.me) in `zalo-api.ts`, and how `App.tsx` injects them.
 *
 * What must hold:
 *   · only digits reach the SDK; anything else is refused before the SDK loads
 *   · `openZaloChat` opens zalo.me and nothing else — the URL is built inside the door
 *   · a failure is NOT reported to `vihat-miniapp`: the SDK's message may echo a staff member's number (rule 3)
 *   · both are injected in the two places the commune interface runs, inside Zalo only, never in the ViHAT intro
 *
 * `zmp-sdk` is replaced because the real module needs a Zalo runtime; the functions under test are real.
 */
const sdk = vi.hoisted(() => ({
  openPhone: vi.fn<(args: { phoneNumber: string }) => Promise<void>>(),
  openOutApp: vi.fn<(args: { url: string }) => Promise<void>>(),
}));
vi.mock("zmp-sdk", () => sdk);
const report = vi.hoisted(() => vi.fn(async () => {}));
vi.mock("../dang-nhap/goi-may-chu", () => ({ reportClientError: report }));

import { dialStaffNumber, openZaloChat } from "./zalo-api";

beforeEach(() => {
  sdk.openPhone.mockReset().mockResolvedValue(undefined);
  sdk.openOutApp.mockReset().mockResolvedValue(undefined);
  report.mockClear();
});

describe("dialStaffNumber", () => {
  it("hands the digits to the phone's dialer", async () => {
    expect(await dialStaffNumber("0900000000")).toEqual({ kieu: "xong", du_lieu: undefined });
    expect(await dialStaffNumber("+84900000000")).toMatchObject({ kieu: "xong" });
    expect(sdk.openPhone.mock.calls).toEqual([[{ phoneNumber: "0900000000" }], [{ phoneNumber: "+84900000000" }]]);
  });

  it("anything but digits (one leading +) is refused without a call", async () => {
    for (const bad of ["", "09", "0900 000 000", "tel:0900000000", "javascript:alert(1)"]) {
      expect(await dialStaffNumber(bad), bad).toEqual({ kieu: "khong-lay-duoc" });
    }
    expect(sdk.openPhone).not.toHaveBeenCalled();
  });

  it("a coded failure stays on the phone — no report carries the number away", async () => {
    sdk.openPhone.mockRejectedValue(Object.assign(new Error("cannot open 0900000000"), { code: -1403 }));
    expect(await dialStaffNumber("0900000000")).toMatchObject({ kieu: "khong-lay-duoc" });
    expect(report).not.toHaveBeenCalled();
  });
});

describe("openZaloChat", () => {
  it("opens https://zalo.me/<digits> outside the webview", async () => {
    expect(await openZaloChat("0900000000")).toEqual({ kieu: "xong", du_lieu: undefined });
    expect(sdk.openOutApp.mock.calls).toEqual([[{ url: "https://zalo.me/0900000000" }]]);
  });

  it("too few or too many digits: nothing is opened", async () => {
    for (const bad of ["", "0900", "1234567890123456", "https://example.com/"]) {
      expect(await openZaloChat(bad), bad).toEqual({ kieu: "khong-lay-duoc" });
    }
    expect(sdk.openOutApp).not.toHaveBeenCalled();
  });

  it("a coded failure stays on the phone — no report carries the link away", async () => {
    sdk.openOutApp.mockRejectedValue(Object.assign(new Error("https://zalo.me/0900000000"), { code: -1403 }));
    expect(await openZaloChat("0900000000")).toMatchObject({ kieu: "khong-lay-duoc" });
    expect(report).not.toHaveBeenCalled();
  });
});

describe("App.tsx wiring", () => {
  const nodeFs = "node:fs";
  it("injected in the commune's own app and on the commune QR page, inside Zalo only; never in the intro", async () => {
    const { readFileSync } = (await import(/* @vite-ignore */ nodeFs)) as {
      readFileSync: (path: URL, encoding: "utf8") => string;
    };
    const app = readFileSync(new URL("../../App.tsx", import.meta.url), "utf8");
    const ownStart = app.indexOf("export function AppRieng(");
    const sharedStart = app.indexOf("function AppChung(");
    const qrStart = app.indexOf("function QrCommuneApp(");
    expect(ownStart).toBeGreaterThan(-1);
    expect(qrStart).toBeGreaterThan(sharedStart);
    const own = app.slice(ownStart, app.indexOf("\n}\n", ownStart));
    const qr = app.slice(qrStart, app.indexOf("\n}\n", qrStart));
    expect(own).toContain("dialStaff={appId === null ? undefined : dialCommuneStaff}");
    expect(own).toContain("openZaloChat={appId === null ? undefined : chatWithCommuneStaff}");
    expect(qr).toContain("dialStaff={insideZalo ? dialCommuneStaff : undefined}");
    expect(qr).toContain("openZaloChat={insideZalo ? chatWithCommuneStaff : undefined}");
    expect(app.slice(sharedStart, qrStart)).not.toMatch(/dialStaff=|openZaloChat=/);
  });
});
