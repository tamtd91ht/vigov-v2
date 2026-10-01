import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { toCommuneAppSessionResult } from "../../App";
import { diaChiViGov } from "../../cong-dan/api/dia-chi-vigov";
import {
  communeAppIdentityHost,
  communeAppReopen,
  openCommuneAppSession as openCommuneAppSessionInStateHalf,
} from "../../cong-dan/api/mo-phien-vigov";
import { readRuntimeAppId } from "../tinh-nang/zalo-api";

import { type CommuneAppLoginResult, openCommuneAppSessionWithPhone } from "./cau-vigov";
import { getCommuneAppLocation } from "./current-location";
import { type CommuneAppBridgeResult, exchangeLocation, openCommuneAppSessionCall } from "./goi-may-chu";
import {
  COMMUNE_APP_LOCATION_FIELDS,
  COMMUNE_APP_SESSION_FIELDS,
  COMMUNE_APP_SESSION_PATH,
  communeAppSessionAddress,
  communeAppSessionBody,
  type CommuneAppSessionRequest,
  diaChiPhien,
  DUONG_DAN_PHIEN,
  LOCATION_FIELDS,
  locationBody,
} from "./hop-dong";

/** A made-up identity host: the composer joins the path to whatever host the state half hands it. */
const IDENTITY_HOST = "https://identity.vidu.example";

/**
 * LOGIN FROM A COMMUNE'S OWN APP — `POST /api/v1/citizen-sessions` at ViGov identity with `appId` (ADR 0066;
 * same body and status table as `vihat-miniapp` 4114f00 had).
 *
 * What must hold: the body carries `appId` + BOTH Zalo codes and never a commune domain; every status the
 * server documents lands on a branch the citizen can act on; nothing is asked of Zalo when the App ID is
 * unknown (a body without `appId` is the SHARED app to the server, which would spend the phone on a
 * commercial ticket).
 */

const ADDRESS = `${IDENTITY_HOST}/api/v1/citizen-sessions`;
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
    expect(await openCommuneAppSessionWithPhone(IDENTITY_HOST, () => null, requestCodes, call)).toEqual({ kieu: "khong-ro-app" });
    expect(await openCommuneAppSessionWithPhone(IDENTITY_HOST, () => "", requestCodes, call)).toEqual({ kieu: "khong-ro-app" });
    expect(requestCodes).not.toHaveBeenCalled();
    expect(call).not.toHaveBeenCalled();
  });

  it("refusal of the phone dialog → tu-choi, no call", async () => {
    const call = neverCalled();
    expect(await openCommuneAppSessionWithPhone(IDENTITY_HOST, () => APP_ID, async () => ({ kieu: "tu-choi" }), call)).toEqual({
      kieu: "tu-choi",
    });
    expect(call).not.toHaveBeenCalled();
  });

  it("outside Zalo → ngoai-zalo; an empty code → khong-lay-duoc-ma; neither calls", async () => {
    const call = neverCalled();
    expect(await openCommuneAppSessionWithPhone(IDENTITY_HOST, () => APP_ID, async () => ({ kieu: "ngoai-zalo" }), call)).toEqual({
      kieu: "ngoai-zalo",
    });
    expect(
      await openCommuneAppSessionWithPhone(
        IDENTITY_HOST,
        () => APP_ID,
        async () => ({ kieu: "xong", du_lieu: { ma_truy_cap: "access-test", ma_so_dien_thoai: "" } }),
        call,
      ),
    ).toEqual({ kieu: "khong-lay-duoc-ma" });
    expect(call).not.toHaveBeenCalled();
  });

  it("all present → exactly one call carrying the App ID and both codes, to identity's route", async () => {
    const call = vi.fn(async (): Promise<CommuneAppBridgeResult> => ({ kieu: "app-chua-san-sang" }));
    expect(await openCommuneAppSessionWithPhone(IDENTITY_HOST, () => APP_ID, codes, call)).toEqual({ kieu: "app-chua-san-sang" });
    expect(call).toHaveBeenCalledTimes(1);
    expect(call).toHaveBeenCalledWith(REQUEST, `${IDENTITY_HOST}${COMMUNE_APP_SESSION_PATH}`);
  });

  it("no identity host → asks Zalo for NOTHING, reads no App ID, sends nothing", async () => {
    const readAppId = vi.fn(() => APP_ID);
    const requestCodes = vi.fn(codes);
    const call = neverCalled();
    expect(await openCommuneAppSessionWithPhone("", readAppId, requestCodes, call)).toEqual({ kieu: "chua-khai-host" });
    expect(readAppId).not.toHaveBeenCalled();
    expect(requestCodes).not.toHaveBeenCalled();
    expect(call).not.toHaveBeenCalled();
  });
});

/**
 * WHERE THE COMMUNE APP'S LOGIN GOES — ADR 0066 (01/10/2026). The commune app (`--vao-thang`) posts to ViGov
 * identity `POST /api/v1/citizen-sessions`, with the host taken from the state half's address map; the
 * SHARED app keeps `vihat-miniapp` `/api/v1/sessions`, from `VIGOV_API_HOST`. Measured end to end on the
 * real composition (state half's host → commercial opener → the one `fetch`).
 */
describe("route target per app", () => {
  const IDENTITY_ROUTE = "https://identity.api.vigov.vn/api/v1/citizen-sessions";

  it("commune app: the state half's identity host + `/api/v1/citizen-sessions` — the exact URL", async () => {
    expect(communeAppIdentityHost()).toBe(new URL(diaChiViGov("identity", "/")).origin);
    expect(communeAppSessionAddress(communeAppIdentityHost())).toBe(IDENTITY_ROUTE);

    stubFetch(answer(201, SESSION));
    const codes = async () => ({
      kieu: "xong" as const,
      du_lieu: { ma_truy_cap: "access-test", ma_so_dien_thoai: "phone-test" },
    });
    const result = await openCommuneAppSessionWithPhone(communeAppIdentityHost(), () => APP_ID, codes);
    expect(result.kieu).toBe("xong");
    expect(calls).toHaveLength(1);
    expect(calls[0]!.address).toBe(IDENTITY_ROUTE);
    expect(JSON.parse(calls[0]!.init.body as string)).toEqual({
      accessToken: "access-test",
      phoneToken: "phone-test",
      appId: APP_ID,
    });
  });

  it("commune app: never the `vihat-miniapp` route, whatever `VIGOV_API_HOST` holds", () => {
    expect(IDENTITY_ROUTE).not.toContain(DUONG_DAN_PHIEN);
    expect(COMMUNE_APP_SESSION_PATH).not.toBe(DUONG_DAN_PHIEN);
    const shared = diaChiPhien();
    if (shared !== "") expect(new URL(shared).host).not.toBe(new URL(IDENTITY_ROUTE).host);
  });

  it("shared app: still `vihat-miniapp` `/api/v1/sessions`, untouched by the move", () => {
    // `diaChiPhien` is the shared app's only login address (`moPhienViGovQuaCau`, `phatHanhPhien`,
    // `reopenViGovSessionWithPhone` default to it). It may be empty under Vitest (no `VIGOV_API_HOST`); when
    // it is not, it ends in the commercial route and is not identity's host.
    const shared = diaChiPhien();
    expect(shared === "" || shared.endsWith(DUONG_DAN_PHIEN)).toBe(true);
    expect(shared).not.toContain(COMMUNE_APP_SESSION_PATH);
    expect(shared).not.toContain("identity.api.vigov.vn");
  });

  it("the state half hands the identity host to the injected opener — on first open and on reopen", async () => {
    const open = vi.fn(async (_host: string) => ({ kieu: "tam-ngung" as const }));
    await openCommuneAppSessionInStateHalf(open, "Xã Thử Nghiệm");
    await communeAppReopen(open)();
    expect(open).toHaveBeenCalledTimes(2);
    for (const [host] of open.mock.calls) expect(host).toBe(communeAppIdentityHost());
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
    expect(await exchangeLocation(CODES, "https://mini.vidu.vn/api/v1/location", APP_ID)).toEqual({ kieu: "tam-ngung" });
    expect(JSON.parse(calls[0]!.init.body as string)).toHaveProperty("appId", APP_ID);
  });

  it("unknown App ID → tam-ngung BEFORE Zalo is asked for the location", async () => {
    const getCodes = vi.fn(async () => ({ kieu: "xong" as const, du_lieu: CODES }));
    const exchange = vi.fn();
    expect(await getCommuneAppLocation(() => null, getCodes, exchange)).toEqual({ kieu: "tam-ngung" });
    expect(getCodes).not.toHaveBeenCalled();
    expect(exchange).not.toHaveBeenCalled();
  });

  it("known App ID → codes, then one exchange carrying it", async () => {
    const exchange = vi.fn(async () => ({ kieu: "xong" as const, location: { latitude: 15.5, longitude: 108.4 } }));
    const result = await getCommuneAppLocation(() => APP_ID, async () => ({ kieu: "xong", du_lieu: CODES }), exchange);
    expect(result).toEqual({ kieu: "xong", location: { latitude: 15.5, longitude: 108.4 } });
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
