import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { toCommuneAppSessionResult } from "../../App";
import type { KetQuaXin, MaDangNhap } from "../tinh-nang/zalo-api";

import { openCommuneAppSessionWithoutPhone, openCommuneAppSessionWithPhone } from "./cau-vigov";
import { type CommuneAppBridgeResult, openCommuneAppSessionCall } from "./goi-may-chu";
import { COMMUNE_APP_SESSION_FIELDS, communeAppSessionBody, type CommuneAppSessionRequest } from "./hop-dong";

/**
 * THE COMMUNE APP'S PHONE-LESS SESSION (ADR 0080, 08/10/2026) — the citizen-app half of "Zalo gives no number".
 *
 * The incident it answers (user report 08/10): until Zalo approves a commune's app, `getPhoneNumber` hands out a
 * token identity cannot exchange, so `POST /api/v1/citizen-sessions` WITH it answered 401 and nothing could be
 * sent. The server deliberately does not downgrade; the client retries ONCE without the phone code.
 *
 * What must hold:
 *   · 401 after sending `phoneToken` → exactly ONE more call, without `phoneToken`, same access token, no SDK call;
 *   · that retry's answer is final — a second 401 goes up as `ma-het-han`, never a third call (no loop);
 *   · any other answer to the first call is returned untouched (no retry on 422, 429, 502, 503, 201);
 *   · refusal / no phone code still call NOTHING here — the phone-less body goes only through
 *     `openCommuneAppSessionWithoutPhone`, which asks Zalo for the access token alone (no dialog).
 */

const IDENTITY_HOST = "https://identity.vidu.example";
const ADDRESS = `${IDENTITY_HOST}/api/v1/citizen-sessions`;
const APP_ID = "1234567890123456789";
const CODES: KetQuaXin<MaDangNhap> = {
  kieu: "xong",
  du_lieu: { ma_truy_cap: "access-test", ma_so_dien_thoai: "phone-test" },
};

type Call = { address: string; body: Record<string, unknown> };
let calls: Call[] = [];

function answer(status: number, body: unknown) {
  return { status, ok: status >= 200 && status < 300, json: async () => body };
}

/** One answer per call, in order (the last one repeats). */
function stubFetch(...answers: ReturnType<typeof answer>[]) {
  let i = 0;
  vi.stubGlobal("fetch", (address: string, init: RequestInit) => {
    calls.push({ address, body: JSON.parse(init.body as string) as Record<string, unknown> });
    return Promise.resolve(answers[Math.min(i++, answers.length - 1)]!);
  });
}

const session = (phoneVerified: boolean) => ({
  vigovSession: { token: "tok-test", expiresAt: "", tenantDisplayName: "Xã Thử Nghiệm", phoneVerified },
});

beforeEach(() => {
  calls = [];
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("body — `phoneToken` is left OUT when there is no phone code", () => {
  it("empty phone code → accessToken + appId only; every key still a declared one", () => {
    const req: CommuneAppSessionRequest = { ma_truy_cap: "access-test", ma_so_dien_thoai: "", app_id: APP_ID };
    const body = JSON.parse(communeAppSessionBody(req)) as Record<string, unknown>;
    expect(body).toEqual({ accessToken: "access-test", appId: APP_ID });
    const declared = COMMUNE_APP_SESSION_FIELDS.map((t) => t.khoa);
    for (const k of Object.keys(body)) expect(declared).toContain(k);
  });
});

describe("401 after sending the phone code → ONE retry without it, never a loop", () => {
  it("first 401, retry 201 phone-less → a session with da_xac_thuc_so false; exactly two calls", async () => {
    stubFetch(answer(401, { code: "zalo_token_invalid" }), answer(201, session(false)));
    const requestCodes = vi.fn(async () => CODES);
    const result = await openCommuneAppSessionWithPhone(IDENTITY_HOST, () => APP_ID, requestCodes);
    expect(result).toMatchObject({ kieu: "xong", phien: { token: "tok-test", da_xac_thuc_so: false } });
    expect(calls).toHaveLength(2);
    expect(calls.map((c) => c.address)).toEqual([ADDRESS, ADDRESS]);
    expect(calls[0]!.body).toEqual({ accessToken: "access-test", phoneToken: "phone-test", appId: APP_ID });
    // The retry: SAME access token, NO phone code, same App ID.
    expect(calls[1]!.body).toEqual({ accessToken: "access-test", appId: APP_ID });
    // No second SDK call — so no second consent dialog.
    expect(requestCodes).toHaveBeenCalledTimes(1);
    // The shell keeps the flag: the state half then sees a phone-less session.
    expect(toCommuneAppSessionResult(result)).toEqual({
      kieu: "xong",
      token: "tok-test",
      ten_xa: "Xã Thử Nghiệm",
      da_xac_thuc_so: false,
    });
  });

  it("401 twice → `ma-het-han` (the access token itself was refused), and NO third call", async () => {
    stubFetch(answer(401, { code: "zalo_token_invalid" }));
    const result = await openCommuneAppSessionWithPhone(IDENTITY_HOST, () => APP_ID, async () => CODES);
    expect(result).toEqual({ kieu: "ma-het-han" });
    expect(calls).toHaveLength(2);
    expect(toCommuneAppSessionResult(result)).toEqual({ kieu: "thu-lai" });
  });

  it.each([
    [201, "xong"],
    [422, "app-chua-san-sang"],
    [429, "qua-nhieu-lan"],
    [502, "zalo-khong-tra-loi"],
    [503, "cau-tat"],
  ] as const)("first answer %i → returned as is, no retry", async (status, kieu) => {
    stubFetch(answer(status, status === 201 ? session(true) : { code: "x" }));
    const result = await openCommuneAppSessionWithPhone(IDENTITY_HOST, () => APP_ID, async () => CODES);
    expect(result.kieu).toBe(kieu);
    expect(calls).toHaveLength(1);
  });

  it("refusal and an empty phone code still send NOTHING from the phone path", async () => {
    const call = vi.fn(async (): Promise<CommuneAppBridgeResult> => ({ kieu: "khong-goi-duoc" }));
    expect(await openCommuneAppSessionWithPhone(IDENTITY_HOST, () => APP_ID, async () => ({ kieu: "tu-choi" }), call)).toEqual({
      kieu: "tu-choi",
    });
    const noPhone: KetQuaXin<MaDangNhap> = { kieu: "xong", du_lieu: { ma_truy_cap: "a", ma_so_dien_thoai: "" } };
    expect((await openCommuneAppSessionWithPhone(IDENTITY_HOST, () => APP_ID, async () => noPhone, call)).kieu).toBe(
      "khong-lay-duoc-ma",
    );
    expect(call).not.toHaveBeenCalled();
  });
});

describe("`openCommuneAppSessionWithoutPhone` — the phone-less body, after the citizen's own tap", () => {
  it("asks Zalo for the access token only, and sends accessToken + appId — no phoneToken", async () => {
    stubFetch(answer(201, session(false)));
    const requestAccess = vi.fn(async (): Promise<KetQuaXin<string>> => ({ kieu: "xong", du_lieu: "access-only" }));
    const result = await openCommuneAppSessionWithoutPhone(IDENTITY_HOST, () => APP_ID, requestAccess);
    expect(result).toMatchObject({ kieu: "xong", phien: { da_xac_thuc_so: false } });
    expect(requestAccess).toHaveBeenCalledTimes(1);
    expect(calls).toEqual([{ address: ADDRESS, body: { accessToken: "access-only", appId: APP_ID } }]);
  });

  it("no address / no App ID / outside Zalo / no access token → nothing sent", async () => {
    const call = vi.fn(openCommuneAppSessionCall);
    const access = async (): Promise<KetQuaXin<string>> => ({ kieu: "xong", du_lieu: "a" });
    expect(await openCommuneAppSessionWithoutPhone("", () => APP_ID, access, call)).toEqual({ kieu: "chua-khai-host" });
    expect(await openCommuneAppSessionWithoutPhone(IDENTITY_HOST, () => null, access, call)).toEqual({ kieu: "khong-ro-app" });
    expect(
      await openCommuneAppSessionWithoutPhone(IDENTITY_HOST, () => APP_ID, async () => ({ kieu: "ngoai-zalo" }), call),
    ).toEqual({ kieu: "ngoai-zalo" });
    expect(
      (await openCommuneAppSessionWithoutPhone(IDENTITY_HOST, () => APP_ID, async () => ({ kieu: "xong", du_lieu: "" }), call)).kieu,
    ).toBe("khong-lay-duoc-ma");
    expect(call).not.toHaveBeenCalled();
  });
});
