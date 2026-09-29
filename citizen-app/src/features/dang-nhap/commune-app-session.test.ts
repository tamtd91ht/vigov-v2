import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { toCommuneAppSessionResult } from "../../App";
import { readRuntimeAppId } from "../tinh-nang/zalo-api";

import { type CommuneAppLoginResult, openCommuneAppSessionWithPhone } from "./cau-vigov";
import { type CommuneAppBridgeResult, openCommuneAppSessionCall } from "./goi-may-chu";
import { COMMUNE_APP_SESSION_FIELDS, communeAppSessionBody, type CommuneAppSessionRequest } from "./hop-dong";

/**
 * LOGIN FROM A COMMUNE'S OWN APP — `POST /api/v1/sessions` with `appId` (`vihat-miniapp` 4114f00).
 *
 * What must hold: the body carries `appId` + BOTH Zalo codes and never a commune domain; every status the
 * server documents lands on a branch the citizen can act on; nothing is asked of Zalo when the App ID is
 * unknown (a body without `appId` is the SHARED app to the server, which would spend the phone on a
 * commercial ticket).
 */

const ADDRESS = "https://mini.vidu.vn/api/v1/sessions";
const APP_ID = "1234567890123456789";
const REQUEST: CommuneAppSessionRequest = { ma_truy_cap: "access-test", ma_so_dien_thoai: "phone-test", app_id: APP_ID };

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
    expiresAt: "2026-09-29T10:00:00Z",
    tenantDisplayName: "Xã Thử Nghiệm",
    phoneVerified: true,
  },
};

beforeEach(() => {
  calls = [];
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("body — appId + both codes, no commune domain", () => {
  it("accessToken · phoneToken · appId, locked both ways with the declaration table", () => {
    const body = JSON.parse(communeAppSessionBody(REQUEST)) as Record<string, unknown>;
    expect(body).toEqual({ accessToken: "access-test", phoneToken: "phone-test", appId: APP_ID });
    expect(Object.keys(body).sort()).toEqual(COMMUNE_APP_SESSION_FIELDS.map((t) => t.khoa).sort());
    expect(body).not.toHaveProperty("communeHostHint");
    expect(body).not.toHaveProperty("communeConfirmed");
  });
});

describe("the call — one branch per thing the citizen does next", () => {
  it("201 with a usable vigovSession → the session, read from `vigovSession` only", async () => {
    stubFetch(answer(201, SESSION));
    const result = await openCommuneAppSessionCall(REQUEST, ADDRESS);
    expect(result).toEqual({
      kieu: "xong",
      phien: {
        token: "tok-vigov-test",
        het_han: "2026-09-29T10:00:00Z",
        ten_xa: "Xã Thử Nghiệm",
        da_xac_thuc_so: true,
        ten_mien_xa: null,
      },
    });
    expect(calls).toHaveLength(1);
    expect(calls[0]!.address).toBe(ADDRESS);
    expect(calls[0]!.init.method).toBe("POST");
    expect(JSON.parse(calls[0]!.init.body as string)).toEqual({
      accessToken: "access-test",
      phoneToken: "phone-test",
      appId: APP_ID,
    });
  });

  it("a COMMERCIAL ticket (token at the root, no vigovSession) is never taken as a ViGov session", async () => {
    stubFetch(answer(201, { token: "commercial", expiresAt: "2026-09-29T10:00:00Z" }));
    expect(await openCommuneAppSessionCall(REQUEST, ADDRESS)).toEqual({ kieu: "cau-tat" });
  });

  it.each<[number, CommuneAppBridgeResult["kieu"]]>([
    [400, "yeu-cau-hong"],
    [401, "ma-het-han"],
    [422, "app-chua-san-sang"],
    [429, "qua-nhieu-lan"],
    [502, "zalo-khong-tra-loi"],
    [503, "cau-tat"],
    [500, "khong-goi-duoc"],
  ])("%i → %s", async (status, kieu) => {
    stubFetch(answer(status, { message: "server sentence — never read" }));
    expect(await openCommuneAppSessionCall(REQUEST, ADDRESS)).toEqual({ kieu });
  });

  it("network failure → khong-goi-duoc, never a throw", async () => {
    stubFetch(new Error("offline"));
    expect(await openCommuneAppSessionCall(REQUEST, ADDRESS)).toEqual({ kieu: "khong-goi-duoc" });
  });

  it("no server address in the build → nothing is sent", async () => {
    stubFetch(answer(201, SESSION));
    expect(await openCommuneAppSessionCall(REQUEST, "")).toEqual({ kieu: "chua-khai-host" });
    expect(calls).toHaveLength(0);
  });
});

describe("composer — App ID first, then the two codes, then one call", () => {
  const codes = async () => ({
    kieu: "xong" as const,
    du_lieu: { ma_truy_cap: "access-test", ma_so_dien_thoai: "phone-test" },
  });
  const neverCalled = () => vi.fn(async (): Promise<CommuneAppBridgeResult> => ({ kieu: "khong-goi-duoc" }));

  it("unknown App ID → asks Zalo for NOTHING and sends nothing", async () => {
    const requestCodes = vi.fn(codes);
    const call = neverCalled();
    expect(await openCommuneAppSessionWithPhone(() => null, requestCodes, call)).toEqual({ kieu: "khong-ro-app" });
    expect(await openCommuneAppSessionWithPhone(() => "", requestCodes, call)).toEqual({ kieu: "khong-ro-app" });
    expect(requestCodes).not.toHaveBeenCalled();
    expect(call).not.toHaveBeenCalled();
  });

  it("refusal of the phone dialog → tu-choi, no call", async () => {
    const call = neverCalled();
    expect(await openCommuneAppSessionWithPhone(() => APP_ID, async () => ({ kieu: "tu-choi" }), call)).toEqual({
      kieu: "tu-choi",
    });
    expect(call).not.toHaveBeenCalled();
  });

  it("outside Zalo → ngoai-zalo; an empty code → khong-lay-duoc-ma; neither calls", async () => {
    const call = neverCalled();
    expect(await openCommuneAppSessionWithPhone(() => APP_ID, async () => ({ kieu: "ngoai-zalo" }), call)).toEqual({
      kieu: "ngoai-zalo",
    });
    expect(
      await openCommuneAppSessionWithPhone(
        () => APP_ID,
        async () => ({ kieu: "xong", du_lieu: { ma_truy_cap: "access-test", ma_so_dien_thoai: "" } }),
        call,
      ),
    ).toEqual({ kieu: "khong-lay-duoc-ma" });
    expect(call).not.toHaveBeenCalled();
  });

  it("all present → exactly one call carrying the App ID and both codes", async () => {
    const call = vi.fn(async (): Promise<CommuneAppBridgeResult> => ({ kieu: "app-chua-san-sang" }));
    expect(await openCommuneAppSessionWithPhone(() => APP_ID, codes, call)).toEqual({ kieu: "app-chua-san-sang" });
    expect(call).toHaveBeenCalledTimes(1);
    expect(call).toHaveBeenCalledWith(REQUEST);
  });
});

describe("shell table — App.tsx `toCommuneAppSessionResult`", () => {
  it.each<[CommuneAppLoginResult, string]>([
    [{ kieu: "tu-choi" }, "tu-choi"],
    [{ kieu: "ngoai-zalo" }, "ngoai-zalo"],
    [{ kieu: "khong-ro-app" }, "chua-ket-noi"],
    [{ kieu: "app-chua-san-sang" }, "chua-ket-noi"],
    [{ kieu: "cau-tat" }, "tam-ngung"],
    [{ kieu: "yeu-cau-hong" }, "tam-ngung"],
    [{ kieu: "chua-khai-host" }, "tam-ngung"],
    [{ kieu: "zalo-khong-tra-loi" }, "cho-lat"],
    [{ kieu: "qua-nhieu-lan" }, "cho-lat"],
    [{ kieu: "ma-het-han" }, "thu-lai"],
    [{ kieu: "khong-goi-duoc" }, "thu-lai"],
    [{ kieu: "khong-lay-duoc-ma" }, "thu-lai"],
  ])("%j → %s", (input, kieu) => {
    expect(toCommuneAppSessionResult(input)).toEqual({ kieu });
  });

  it("xong carries the ViGov bearer, commune name and phoneVerified — not the commune domain", () => {
    expect(
      toCommuneAppSessionResult({
        kieu: "xong",
        phien: { token: "t", het_han: "", ten_xa: "Xã Thử Nghiệm", da_xac_thuc_so: true, ten_mien_xa: "xa.vigov.example" },
      }),
    ).toEqual({ kieu: "xong", token: "t", ten_xa: "Xã Thử Nghiệm", da_xac_thuc_so: true });
  });
});

describe("runtime App ID — `window.APP_ID`, digits only", () => {
  it.each<[unknown, string | null]>([
    [APP_ID, APP_ID],
    [undefined, null],
    ["", null],
    ["12ab", null],
    [" 123", null],
    [123, null],
  ])("%j → %j", (value, expected) => {
    vi.stubGlobal("APP_ID", value);
    expect(readRuntimeAppId()).toBe(expected);
  });
});
