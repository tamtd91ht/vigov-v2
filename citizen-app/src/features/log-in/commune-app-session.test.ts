import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { toCommuneAppSessionResult } from "../../App";
import { readRuntimeAppId } from "../tinh-nang/zalo-api";

import { type CommuneAppLoginResult, openCommuneAppSessionWithPhone } from "./vigov-bridge";
import { getCommuneAppLocation } from "./current-location";
import { type CommuneAppBridgeResult, exchangeLocation, openCommuneAppSessionCall } from "./server-calls";
import {
  COMMUNE_APP_LOCATION_FIELDS,
  COMMUNE_APP_SESSION_FIELDS,
  communeAppSessionBody,
  type CommuneAppSessionRequest,
  LOCATION_FIELDS,
  locationBody,
} from "./contract";

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
      kind: "xong",
      session: {
        token: "tok-vigov-test",
        expires_at: "2026-09-29T10:00:00Z",
        commune_name: "Xã Thử Nghiệm",
        phone_verified: true,
        commune_domain: null,
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
    expect(await openCommuneAppSessionCall(REQUEST, ADDRESS)).toEqual({ kind: "cau-tat" });
  });

  it.each<[number, CommuneAppBridgeResult["kind"]]>([
    [400, "yeu-cau-hong"],
    [401, "ma-het-han"],
    [422, "app-chua-san-sang"],
    [429, "qua-nhieu-lan"],
    [502, "zalo-khong-tra-loi"],
    [503, "cau-tat"],
    [500, "khong-goi-duoc"],
  ])("%i → %s", async (status, kind) => {
    stubFetch(answer(status, { message: "server sentence — never read" }));
    expect(await openCommuneAppSessionCall(REQUEST, ADDRESS)).toEqual({ kind });
  });

  it("network failure → khong-goi-duoc, never a throw", async () => {
    stubFetch(new Error("offline"));
    expect(await openCommuneAppSessionCall(REQUEST, ADDRESS)).toEqual({ kind: "khong-goi-duoc" });
  });

  it("no server address in the build → nothing is sent", async () => {
    stubFetch(answer(201, SESSION));
    expect(await openCommuneAppSessionCall(REQUEST, "")).toEqual({ kind: "chua-khai-host" });
    expect(calls).toHaveLength(0);
  });
});

describe("composer — App ID first, then the two codes, then one call", () => {
  const codes = async () => ({
    kieu: "xong" as const,
    du_lieu: { ma_truy_cap: "access-test", ma_so_dien_thoai: "phone-test" },
  });
  const neverCalled = () => vi.fn(async (): Promise<CommuneAppBridgeResult> => ({ kind: "khong-goi-duoc" }));

  it("unknown App ID → asks Zalo for NOTHING and sends nothing", async () => {
    const requestCodes = vi.fn(codes);
    const call = neverCalled();
    expect(await openCommuneAppSessionWithPhone(() => null, requestCodes, call)).toEqual({ kind: "khong-ro-app" });
    expect(await openCommuneAppSessionWithPhone(() => "", requestCodes, call)).toEqual({ kind: "khong-ro-app" });
    expect(requestCodes).not.toHaveBeenCalled();
    expect(call).not.toHaveBeenCalled();
  });

  it("refusal of the phone dialog → tu-choi, no call", async () => {
    const call = neverCalled();
    expect(await openCommuneAppSessionWithPhone(() => APP_ID, async () => ({ kieu: "tu-choi" }), call)).toEqual({
      kind: "tu-choi",
    });
    expect(call).not.toHaveBeenCalled();
  });

  it("outside Zalo → ngoai-zalo; an empty code → khong-lay-duoc-ma; neither calls", async () => {
    const call = neverCalled();
    expect(await openCommuneAppSessionWithPhone(() => APP_ID, async () => ({ kieu: "ngoai-zalo" }), call)).toEqual({
      kind: "ngoai-zalo",
    });
    expect(
      await openCommuneAppSessionWithPhone(
        () => APP_ID,
        async () => ({ kieu: "xong", du_lieu: { ma_truy_cap: "access-test", ma_so_dien_thoai: "" } }),
        call,
      ),
    ).toEqual({ kind: "khong-lay-duoc-ma" });
    expect(call).not.toHaveBeenCalled();
  });

  it("all present → exactly one call carrying the App ID and both codes", async () => {
    const call = vi.fn(async (): Promise<CommuneAppBridgeResult> => ({ kind: "app-chua-san-sang" }));
    expect(await openCommuneAppSessionWithPhone(() => APP_ID, codes, call)).toEqual({ kind: "app-chua-san-sang" });
    expect(call).toHaveBeenCalledTimes(1);
    expect(call).toHaveBeenCalledWith(REQUEST);
  });
});

describe("shell table — App.tsx `toCommuneAppSessionResult`", () => {
  it.each<[CommuneAppLoginResult, string]>([
    [{ kind: "tu-choi" }, "tu-choi"],
    [{ kind: "ngoai-zalo" }, "ngoai-zalo"],
    [{ kind: "khong-ro-app" }, "chua-ket-noi"],
    [{ kind: "app-chua-san-sang" }, "chua-ket-noi"],
    [{ kind: "cau-tat" }, "tam-ngung"],
    [{ kind: "yeu-cau-hong" }, "tam-ngung"],
    [{ kind: "chua-khai-host" }, "tam-ngung"],
    [{ kind: "zalo-khong-tra-loi" }, "cho-lat"],
    [{ kind: "qua-nhieu-lan" }, "cho-lat"],
    [{ kind: "ma-het-han" }, "thu-lai"],
    [{ kind: "khong-goi-duoc" }, "thu-lai"],
    [{ kind: "khong-lay-duoc-ma" }, "thu-lai"],
  ])("%j → %s", (input, kind) => {
    expect(toCommuneAppSessionResult(input)).toEqual({ kind });
  });

  it("xong carries the ViGov bearer, commune name and phoneVerified — not the commune domain", () => {
    expect(
      toCommuneAppSessionResult({
        kind: "xong",
        session: { token: "t", expires_at: "", commune_name: "Xã Thử Nghiệm", phone_verified: true, commune_domain: "xa.vigov.example" },
      }),
    ).toEqual({ kind: "xong", token: "t", commune_name: "Xã Thử Nghiệm", phone_verified: true });
  });
});

describe("location in the commune app — `appId` in the exchange, so the server uses THIS app's secret", () => {
  const CODES = { access_token: "access-test", location_token: "location-test" };

  it("shared app body unchanged (two keys); commune body adds `appId`; both locked to their declarations", () => {
    const shared = JSON.parse(locationBody(CODES)) as Record<string, unknown>;
    expect(Object.keys(shared).sort()).toEqual(LOCATION_FIELDS.map((t) => t.khoa).sort());
    const commune = JSON.parse(locationBody(CODES, APP_ID)) as Record<string, unknown>;
    expect(commune).toEqual({ accessToken: "access-test", locationToken: "location-test", appId: APP_ID });
    expect(Object.keys(commune).sort()).toEqual(COMMUNE_APP_LOCATION_FIELDS.map((t) => t.khoa).sort());
  });

  it("422 app_not_configured → tam-ngung (pressing again changes nothing)", async () => {
    stubFetch(answer(422, { code: "app_not_configured" }));
    expect(await exchangeLocation(CODES, "https://mini.vidu.vn/api/v1/location", APP_ID)).toEqual({ kind: "tam-ngung" });
    expect(JSON.parse(calls[0]!.init.body as string)).toHaveProperty("appId", APP_ID);
  });

  it("unknown App ID → tam-ngung BEFORE Zalo is asked for the location", async () => {
    const getCodes = vi.fn(async () => ({ kieu: "xong" as const, du_lieu: CODES }));
    const exchange = vi.fn();
    expect(await getCommuneAppLocation(() => null, getCodes, exchange)).toEqual({ kind: "tam-ngung" });
    expect(getCodes).not.toHaveBeenCalled();
    expect(exchange).not.toHaveBeenCalled();
  });

  it("known App ID → codes, then one exchange carrying it", async () => {
    const exchange = vi.fn(async () => ({ kind: "xong" as const, location: { latitude: 15.5, longitude: 108.4 } }));
    const result = await getCommuneAppLocation(() => APP_ID, async () => ({ kieu: "xong", du_lieu: CODES }), exchange);
    expect(result).toEqual({ kind: "xong", location: { latitude: 15.5, longitude: 108.4 } });
    expect(exchange).toHaveBeenCalledWith(CODES, APP_ID);
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
