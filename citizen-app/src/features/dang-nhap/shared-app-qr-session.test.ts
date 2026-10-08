import { afterEach, describe, expect, it, vi } from "vitest";

import { openSharedAppSession, toReopenWithPhoneResult, toSharedAppSessionResult } from "../../App";
import {
  communeAppReopen,
  openCommuneAppSession,
  reopenSessionWithPhone,
  type ReopenWithPhoneResult,
} from "../../cong-dan/api/mo-phien-vigov";
import { datPhienViGov, layPhienViGov } from "../../cong-dan/api/phien-vigov";

import type { KetQuaMoPhienQuaCau, ReopenWithPhoneBridgeResult } from "./cau-vigov";

/**
 * THE SHARED APP ON A COMMUNE QR (owner 06/10/2026) — the session opener `QrCommuneApp` hands `TrangXa`.
 *
 * What must hold:
 *   · every bridge outcome lands on a `CommuneAppSessionResult` branch the citizen can act on;
 *   · the host sent is the QR's host — never the identity host `TrangXa` hands out (that is the OWN app's login);
 *   · the state half's guards still apply on this path: a session for ANOTHER commune than the one on screen is
 *     refused and NOT stored (rule 1 · README §Non-negotiables #2, #5) — the QR steers, it grants nothing.
 */

const QR_HOST = "xa-thu-nghiem.vigov.example";
const SHOWN = "Xã Thử Nghiệm";

const session = (ten_xa: string, da_xac_thuc_so = true): ReopenWithPhoneBridgeResult => ({
  kieu: "xong",
  phien: { token: "tok-test", het_han: "", ten_xa, da_xac_thuc_so, ten_mien_xa: QR_HOST },
});

afterEach(() => {
  datPhienViGov(null);
});

describe("shell table — App.tsx `toSharedAppSessionResult`", () => {
  it.each<[ReopenWithPhoneResult, unknown]>([
    [{ kieu: "tu-choi" }, { kieu: "tu-choi" }],
    [{ kieu: "ngoai-zalo" }, { kieu: "ngoai-zalo" }],
    [{ kieu: "chua-mo" }, { kieu: "tam-ngung" }],
    [{ kieu: "thu-lai" }, { kieu: "thu-lai" }],
    [
      { kieu: "thu-lai", zalo: { capability: "phone", code: -1401, transient: false } },
      { kieu: "thu-lai", zalo: { capability: "phone", code: -1401, transient: false } },
    ],
  ])("%j → %j", (input, expected) => {
    expect(toSharedAppSessionResult(input)).toEqual(expected);
  });

  it("xong carries exactly the bearer, the commune name and phoneVerified", () => {
    expect(toSharedAppSessionResult({ kieu: "xong", token: "t", ten_xa: SHOWN, da_xac_thuc_so: false })).toEqual({
      kieu: "xong",
      token: "t",
      ten_xa: SHOWN,
      da_xac_thuc_so: false,
    });
  });

  it.each<[ReopenWithPhoneBridgeResult, string]>([
    [{ kieu: "cau-tat" }, "tam-ngung"],
    [{ kieu: "chua-san-sang" }, "tam-ngung"],
    [{ kieu: "chua-khai-host" }, "tam-ngung"],
    [{ kieu: "ma-het-han" }, "thu-lai"],
    [{ kieu: "tam-ngung" }, "thu-lai"],
    [{ kieu: "khong-goi-duoc" }, "thu-lai"],
    [{ kieu: "khong-lay-duoc-ma" }, "thu-lai"],
    [{ kieu: "tu-choi" }, "tu-choi"],
    [{ kieu: "ngoai-zalo" }, "ngoai-zalo"],
  ])("bridge %j → %s (through `toReopenWithPhoneResult`)", (bridge, kieu) => {
    expect(toSharedAppSessionResult(toReopenWithPhoneResult(bridge))).toEqual({ kieu });
  });
});

describe("the opener — bound to the QR host, guarded by the state half", () => {
  it("sends the QR host, ignoring the identity host handed in — on first open and on reopen", async () => {
    const reopen = vi.fn(async (_host: string) => session(SHOWN));
    const open = openSharedAppSession(QR_HOST, reopen);
    expect(await openCommuneAppSession(open, SHOWN)).toEqual({ kieu: "da-mo" });
    expect(await reopenSessionWithPhone(communeAppReopen(open))).toEqual({ kieu: "da-xac-thuc" });
    expect(reopen).toHaveBeenCalledTimes(2);
    for (const [host] of reopen.mock.calls) expect(host).toBe(QR_HOST);
    expect(layPhienViGov()).toEqual({ token: "tok-test", ten_xa: SHOWN, phone_verified: true });
  });

  it("a session for ANOTHER commune than the one on screen is refused and not stored — first open and reopen", async () => {
    const open = openSharedAppSession(QR_HOST, async () => session("Xã Khác"));
    expect(await openCommuneAppSession(open, SHOWN)).toEqual({ kieu: "khac-xa" });
    expect(layPhienViGov()).toBeNull();
    datPhienViGov({ token: "tok-old", ten_xa: SHOWN, phone_verified: true });
    expect(await reopenSessionWithPhone(communeAppReopen(open))).toEqual({ kieu: "khac-xa" });
    expect(layPhienViGov()).toEqual({ token: "tok-old", ten_xa: SHOWN, phone_verified: true });
  });

  it("an unverified phone is STORED as phone-less (ADR 0080); a refused dialog sends nothing further", async () => {
    expect(await openCommuneAppSession(openSharedAppSession(QR_HOST, async () => session(SHOWN, false)), SHOWN)).toEqual({
      kieu: "no-phone",
    });
    expect(layPhienViGov()).toEqual({ token: "tok-test", ten_xa: SHOWN, phone_verified: false });
    datPhienViGov(null);
    expect(await openCommuneAppSession(openSharedAppSession(QR_HOST, async () => ({ kieu: "tu-choi" })), SHOWN)).toEqual({
      kieu: "tu-choi",
    });
    expect(layPhienViGov()).toBeNull();
  });

  it("`skip` sends the phone-less bridge body to the SAME QR host and never the phone path (ADR 0080)", async () => {
    const reopen = vi.fn(async (_host: string) => session(SHOWN));
    const withoutPhone = vi.fn(async (_host: string) => session(SHOWN, false) as KetQuaMoPhienQuaCau);
    const open = openSharedAppSession(QR_HOST, reopen, withoutPhone);
    expect(await openCommuneAppSession(open, SHOWN, "skip")).toEqual({ kieu: "no-phone" });
    expect(reopen).not.toHaveBeenCalled();
    expect(withoutPhone).toHaveBeenCalledTimes(1);
    expect(withoutPhone.mock.calls[0]![0]).toBe(QR_HOST);
    // The commune check still applies on this path: another commune is refused and nothing is stored.
    datPhienViGov(null);
    const other = openSharedAppSession(QR_HOST, reopen, async () => session("Xã Khác", false) as KetQuaMoPhienQuaCau);
    expect(await openCommuneAppSession(other, SHOWN, "skip")).toEqual({ kieu: "khac-xa" });
    expect(layPhienViGov()).toBeNull();
  });

  it("a bridge that throws is `thu-lai`, never a frozen screen", async () => {
    const open = openSharedAppSession(QR_HOST, async () => {
      throw new Error("offline");
    });
    expect(await openCommuneAppSession(open, SHOWN)).toEqual({ kieu: "thu-lai" });
    expect(layPhienViGov()).toBeNull();
  });
});
