import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

/**
 * THE `--demo` BUILD'S LOGIN (owner 01/10/2026, ADR 0047 §6) — `POST /api/v1/sessions` with
 * `{ appId, accessToken, demoIdentity: true }` and NO `phoneToken`.
 *
 * What must hold:
 *   · the body is exactly those three keys — the shape `vihat-miniapp` accepts for an App ID in its
 *     `DEMO_APP_IDS`, and nothing that could be read as a phone;
 *   · the opener calls `getAccessToken` (no dialog) and NEVER `getPhoneNumber` or `getUserInfo`;
 *   · App ID unknown → nothing asked of Zalo, nothing sent (a body without `appId` is the shared app);
 *   · the status table is the commune app's ordinary one — a refusal by the server is an ordinary branch.
 *
 * `zmp-sdk` is replaced because the real module needs a Zalo runtime; the functions under test are the real
 * ones of `zalo-api.ts`, `cau-vigov.ts` and `goi-may-chu.ts`.
 */
const sdk = vi.hoisted(() => ({
  getAccessToken: vi.fn<() => Promise<string>>(),
  getPhoneNumber: vi.fn<() => Promise<{ token?: string }>>(),
  getUserInfo: vi.fn<() => Promise<{ userInfo: { name?: string } }>>(),
}));
vi.mock("zmp-sdk", () => sdk);

import { type CommuneAppLoginResult, openCommuneAppSessionWithDemoIdentity } from "./cau-vigov";
import { type CommuneAppBridgeResult, openCommuneAppDemoSessionCall } from "./goi-may-chu";
import { communeAppDemoSessionBody, type CommuneAppDemoSessionRequest } from "./hop-dong";

const ADDRESS = "https://mini.vidu.vn/api/v1/sessions";
const APP_ID = "1234567890123456789";
const REQUEST: CommuneAppDemoSessionRequest = { access_token: "access-test", app_id: APP_ID };

type Call = { address: string; init: RequestInit };
let calls: Call[] = [];

function answer(status: number, body: unknown) {
  return { status, ok: status >= 200 && status < 300, json: async () => body };
}

function stubFetch(p: ReturnType<typeof answer> | Error) {
  vi.stubGlobal("fetch", (address: string, init: RequestInit) => {
    calls.push({ address, init });
    return p instanceof Error ? Promise.reject(p) : Promise.resolve(p);
  });
}

const SESSION = {
  vigovSession: {
    token: "tok-vigov-test",
    expiresAt: "2026-10-01T10:00:00Z",
    tenantDisplayName: "Xã Thử Nghiệm",
    phoneVerified: true,
  },
};

const EXPECTED_BODY = { appId: APP_ID, accessToken: "access-test", demoIdentity: true };

beforeEach(() => {
  calls = [];
  sdk.getAccessToken.mockReset().mockResolvedValue("access-test");
  sdk.getPhoneNumber.mockReset().mockResolvedValue({ token: "phone-test" });
  sdk.getUserInfo.mockReset().mockResolvedValue({ userInfo: { name: "Trần Thị Bình" } }); // invented
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("body — appId + accessToken + demoIdentity, no phone token", () => {
  it("exactly the three keys the server side accepts", () => {
    const body = JSON.parse(communeAppDemoSessionBody(REQUEST)) as Record<string, unknown>;
    expect(body).toEqual(EXPECTED_BODY);
    expect(body).not.toHaveProperty("phoneToken");
    expect(body).not.toHaveProperty("communeHostHint");
  });
});

describe("the call — same route, same table as the commune app's ordinary login", () => {
  it("201 → the session; the wire carries exactly the demo body", async () => {
    stubFetch(answer(201, SESSION));
    const result = await openCommuneAppDemoSessionCall(REQUEST, ADDRESS);
    expect(result).toEqual({
      kieu: "xong",
      phien: {
        token: "tok-vigov-test",
        het_han: "2026-10-01T10:00:00Z",
        ten_xa: "Xã Thử Nghiệm",
        da_xac_thuc_so: true,
        ten_mien_xa: null,
      },
    });
    expect(calls).toHaveLength(1);
    expect(calls[0]!.address).toBe(ADDRESS);
    expect(calls[0]!.init.method).toBe("POST");
    expect(JSON.parse(calls[0]!.init.body as string)).toEqual(EXPECTED_BODY);
  });

  it.each<[number, CommuneAppBridgeResult["kieu"]]>([
    [400, "yeu-cau-hong"],
    [401, "ma-het-han"],
    [422, "app-chua-san-sang"],
    [429, "qua-nhieu-lan"],
    [502, "zalo-khong-tra-loi"],
    [503, "cau-tat"],
  ])("%i → %s", async (status, kieu) => {
    stubFetch(answer(status, { message: "never read" }));
    expect(await openCommuneAppDemoSessionCall(REQUEST, ADDRESS)).toEqual({ kieu });
  });

  it("no server address in the build → nothing is sent", async () => {
    stubFetch(answer(201, SESSION));
    expect(await openCommuneAppDemoSessionCall(REQUEST, "")).toEqual({ kieu: "chua-khai-host" });
    expect(calls).toHaveLength(0);
  });
});

describe("opener — `getAccessToken` only; never `getPhoneNumber`, never `getUserInfo`", () => {
  const toWire = (req: CommuneAppDemoSessionRequest) => openCommuneAppDemoSessionCall(req, ADDRESS);

  it("real SDK wrapper: one access token, one call with the demo body, no phone or name asked", async () => {
    stubFetch(answer(201, SESSION));
    const result: CommuneAppLoginResult = await openCommuneAppSessionWithDemoIdentity(() => APP_ID, undefined, toWire);
    expect(result.kieu).toBe("xong");
    expect(sdk.getAccessToken).toHaveBeenCalledTimes(1);
    expect(sdk.getPhoneNumber).not.toHaveBeenCalled();
    expect(sdk.getUserInfo).not.toHaveBeenCalled();
    expect(JSON.parse(calls[0]!.init.body as string)).toEqual(EXPECTED_BODY);
  });

  it("unknown App ID → Zalo asked for NOTHING and nothing sent", async () => {
    stubFetch(answer(201, SESSION));
    expect(await openCommuneAppSessionWithDemoIdentity(() => null, undefined, toWire)).toEqual({ kieu: "khong-ro-app" });
    expect(await openCommuneAppSessionWithDemoIdentity(() => "", undefined, toWire)).toEqual({ kieu: "khong-ro-app" });
    expect(sdk.getAccessToken).not.toHaveBeenCalled();
    expect(calls).toHaveLength(0);
  });

  it("Zalo refuses the access token with a code → that code goes up, nothing sent", async () => {
    stubFetch(answer(201, SESSION));
    sdk.getAccessToken.mockRejectedValue(Object.assign(new Error("platform text"), { code: -1402 }));
    expect(await openCommuneAppSessionWithDemoIdentity(() => APP_ID, undefined, toWire)).toEqual({
      kieu: "khong-lay-duoc-ma",
      failure: { capability: "access-token", code: -1402, transient: false },
    });
    expect(calls).toHaveLength(0);
  });

  it("an empty access token (development platform) is not sent", async () => {
    stubFetch(answer(201, SESSION));
    sdk.getAccessToken.mockResolvedValue("");
    expect(await openCommuneAppSessionWithDemoIdentity(() => APP_ID, undefined, toWire)).toEqual({
      kieu: "khong-lay-duoc-ma",
    });
    expect(calls).toHaveLength(0);
  });

  it("outside Zalo → ngoai-zalo, nothing sent", async () => {
    const call = vi.fn(toWire);
    expect(
      await openCommuneAppSessionWithDemoIdentity(() => APP_ID, async () => ({ kieu: "ngoai-zalo" }), call),
    ).toEqual({ kieu: "ngoai-zalo" });
    expect(call).not.toHaveBeenCalled();
  });
});
