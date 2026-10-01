import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

/**
 * THE `--demo` BUILD'S LOGIN (owner 01/10/2026, ADR 0047 §6) — `POST /api/v1/sessions` with
 * `{ appId, demoIdentity: true }` and NO Zalo token of any kind.
 *
 * What must hold:
 *   · the body is exactly those two keys — no `accessToken`, no `phoneToken`, nothing a server could
 *     exchange with Zalo;
 *   · the opener calls NO Zalo identity API: not `getAccessToken` (an app Zalo has not approved is refused it
 *     too — the 01/10/2026 report), not `getPhoneNumber`, not `getUserInfo`;
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
import { communeAppIdentityHost } from "../../cong-dan/api/mo-phien-vigov";

import {
  communeAppDemoSessionBody,
  type CommuneAppDemoSessionRequest,
  COMMUNE_APP_SESSION_PATH,
  DUONG_DAN_PHIEN,
} from "./hop-dong";

/** A made-up identity host (ADR 0066: the demo login goes to ViGov identity too). */
const IDENTITY_HOST = "https://identity.vidu.example";
const ADDRESS = `${IDENTITY_HOST}${COMMUNE_APP_SESSION_PATH}`;
const APP_ID = "1234567890123456789";
const REQUEST: CommuneAppDemoSessionRequest = { app_id: APP_ID };

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

const EXPECTED_BODY = { appId: APP_ID, demoIdentity: true };

beforeEach(() => {
  calls = [];
  sdk.getAccessToken.mockReset().mockResolvedValue("access-test");
  sdk.getPhoneNumber.mockReset().mockResolvedValue({ token: "phone-test" });
  sdk.getUserInfo.mockReset().mockResolvedValue({ userInfo: { name: "Trần Thị Bình" } }); // invented
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("body — appId + demoIdentity, no Zalo token", () => {
  it("exactly the two keys", () => {
    const body = JSON.parse(communeAppDemoSessionBody(REQUEST)) as Record<string, unknown>;
    expect(body).toEqual(EXPECTED_BODY);
    expect(body).not.toHaveProperty("accessToken");
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

describe("opener — NO Zalo identity call: never `getAccessToken`, `getPhoneNumber`, `getUserInfo`", () => {
  const toWire = (req: CommuneAppDemoSessionRequest) => openCommuneAppDemoSessionCall(req, ADDRESS);

  function expectZaloUntouched() {
    expect(sdk.getAccessToken).not.toHaveBeenCalled();
    expect(sdk.getPhoneNumber).not.toHaveBeenCalled();
    expect(sdk.getUserInfo).not.toHaveBeenCalled();
  }

  it("a session: one call with exactly the demo body, Zalo asked for nothing", async () => {
    stubFetch(answer(201, SESSION));
    const result: CommuneAppLoginResult = await openCommuneAppSessionWithDemoIdentity(IDENTITY_HOST, () => APP_ID, toWire);
    expect(result.kieu).toBe("xong");
    expectZaloUntouched();
    expect(calls).toHaveLength(1);
    expect(JSON.parse(calls[0]!.init.body as string)).toEqual(EXPECTED_BODY);
  });

  it("the default wiring (real `openCommuneAppDemoSessionCall`) also asks Zalo for nothing", async () => {
    // Whatever the test build's server address, the opener's own default path must leave Zalo untouched.
    stubFetch(answer(201, SESSION));
    await openCommuneAppSessionWithDemoIdentity(IDENTITY_HOST, () => APP_ID);
    expectZaloUntouched();
  });

  it("the default wiring posts to ViGov identity `/api/v1/citizen-sessions`, never `vihat-miniapp`", async () => {
    stubFetch(answer(201, SESSION));
    expect((await openCommuneAppSessionWithDemoIdentity(communeAppIdentityHost(), () => APP_ID)).kieu).toBe("xong");
    expect(calls).toHaveLength(1);
    expect(calls[0]!.address).toBe("https://identity.api.vigov.vn/api/v1/citizen-sessions");
    expect(calls[0]!.address).not.toContain(DUONG_DAN_PHIEN);
    // Still no token of any kind on the wire.
    expect(JSON.parse(calls[0]!.init.body as string)).toEqual(EXPECTED_BODY);
    expectZaloUntouched();
  });

  it("no identity host → nothing sent, Zalo asked for nothing", async () => {
    stubFetch(answer(201, SESSION));
    expect(await openCommuneAppSessionWithDemoIdentity("", () => APP_ID, toWire)).toEqual({ kieu: "chua-khai-host" });
    expect(calls).toHaveLength(0);
    expectZaloUntouched();
  });

  it("Zalo would refuse every identity call (an app not yet approved) → the login does not depend on it", async () => {
    const refused = Object.assign(new Error("platform text"), { code: -1402 });
    sdk.getAccessToken.mockRejectedValue(refused);
    sdk.getPhoneNumber.mockRejectedValue(refused);
    sdk.getUserInfo.mockRejectedValue(refused);
    stubFetch(answer(201, SESSION));
    expect((await openCommuneAppSessionWithDemoIdentity(IDENTITY_HOST, () => APP_ID, toWire)).kieu).toBe("xong");
    expectZaloUntouched();
  });

  it("unknown App ID → nothing sent, Zalo asked for nothing", async () => {
    stubFetch(answer(201, SESSION));
    expect(await openCommuneAppSessionWithDemoIdentity(IDENTITY_HOST, () => null, toWire)).toEqual({ kieu: "khong-ro-app" });
    expect(await openCommuneAppSessionWithDemoIdentity(IDENTITY_HOST, () => "", toWire)).toEqual({ kieu: "khong-ro-app" });
    expectZaloUntouched();
    expect(calls).toHaveLength(0);
  });

  it.each<[number, CommuneAppBridgeResult["kieu"]]>([
    [400, "yeu-cau-hong"],
    [422, "app-chua-san-sang"],
    [503, "cau-tat"],
  ])("server refuses the body (%i, as it does until the server side exists) → %s, Zalo untouched", async (status, kieu) => {
    stubFetch(answer(status, { message: "never read" }));
    expect(await openCommuneAppSessionWithDemoIdentity(IDENTITY_HOST, () => APP_ID, toWire)).toEqual({ kieu });
    expectZaloUntouched();
  });

  it("network failure → khong-goi-duoc, never a throw", async () => {
    stubFetch(new Error("offline"));
    expect(await openCommuneAppSessionWithDemoIdentity(IDENTITY_HOST, () => APP_ID, toWire)).toEqual({ kieu: "khong-goi-duoc" });
    expectZaloUntouched();
  });
});
