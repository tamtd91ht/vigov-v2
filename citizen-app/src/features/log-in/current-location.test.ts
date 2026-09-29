import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { toSceneLocationResult } from "../../App";
import type { KetQuaXin, LocationCodes } from "../tinh-nang/zalo-api";

import { getCurrentLocation } from "./current-location";
import { exchangeLocation, type LocationExchangeResult } from "./server-calls";
import { LOCATION_FIELDS, LOCATION_PATH, locationBody, readLocation } from "./contract";

/**
 * THE LOCATION EXCHANGE — `vihat-miniapp` `POST /api/v1/location` (0dada0f), seen from the Mini App.
 *
 * Only `fetch` is fake. The codes below are sentinels: every case that could leak one (into a URL, a
 * header, a log line) looks for these exact strings.
 */

const ADDRESS = `https://mini.vidu.vn${LOCATION_PATH}`;
const CODES: LocationCodes = { access_token: "ACCESS-SENTINEL-7Q", location_token: "LOC-SENTINEL-4Z" };

type Call = { address: string; init: RequestInit };
let calls: Call[] = [];

function reply(status: number, body: unknown) {
  return { status, ok: status >= 200 && status < 300, json: async () => body };
}

function setFetch(p: ReturnType<typeof reply> | Error) {
  vi.stubGlobal("fetch", (address: string, init: RequestInit) => {
    calls.push({ address, init });
    return p instanceof Error ? Promise.reject(p) : Promise.resolve(p);
  });
}

/** Every console method, spied — rule 3: neither a code nor a coordinate may reach a log. */
function spyConsole() {
  return (["log", "info", "warn", "error", "debug", "trace"] as const).map((m) =>
    vi.spyOn(console, m).mockImplementation(() => {}),
  );
}

beforeEach(() => {
  calls = [];
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("request shape", () => {
  it("POST to the vihat-miniapp host, JSON, exactly {accessToken, locationToken} — no session, nothing in the URL", async () => {
    setFetch(reply(200, { latitude: 15.5, longitude: 108.25 }));
    await exchangeLocation(CODES, ADDRESS);

    expect(calls).toHaveLength(1);
    const call = calls[0]!;
    expect(call.address).toBe(ADDRESS);
    expect(call.init.method).toBe("POST");
    const headers = call.init.headers as Record<string, string>;
    expect(headers).toEqual({ "Content-Type": "application/json" });
    // Public route: neither the commercial ticket nor a ViGov bearer goes with it.
    expect(Object.keys(headers).map((h) => h.toLowerCase())).not.toContain("authorization");
    expect(JSON.parse(call.init.body as string)).toEqual({
      accessToken: CODES.access_token,
      locationToken: CODES.location_token,
    });
    // The codes are in the body only — never on the path or the query (rule 3, forbidden #4).
    expect(call.address).not.toContain(CODES.access_token);
    expect(call.address).not.toContain(CODES.location_token);
  });

  it("the body's keys ARE the declared fields, both ways", () => {
    const keys = Object.keys(JSON.parse(locationBody(CODES)) as Record<string, unknown>).sort();
    expect(keys).toEqual(LOCATION_FIELDS.map((f) => f.khoa).sort());
  });

  it("no host in the build: fails closed, no call", async () => {
    const spy = vi.fn();
    vi.stubGlobal("fetch", spy);
    expect(await exchangeLocation(CODES, "")).toEqual({ kind: "chua-khai-host" });
    expect(spy).not.toHaveBeenCalled();
  });
});

describe("answers → branches (by status code; the server's `message` is never read)", () => {
  it("200 in range → the coordinates", async () => {
    setFetch(reply(200, { latitude: 15.57123456, longitude: 108.47654321 }));
    expect(await exchangeLocation(CODES, ADDRESS)).toEqual({
      kind: "xong",
      location: { latitude: 15.57123456, longitude: 108.47654321 },
    });
  });

  it("200 out of shape → never a half location", async () => {
    for (const body of [
      {},
      { latitude: 15.5 },
      { longitude: 108.2 },
      { latitude: "15.5", longitude: "108.2" },
      { latitude: 91, longitude: 108 },
      { latitude: 15, longitude: -181 },
      { latitude: Number.NaN, longitude: 108 },
      null,
      "15.5,108.2",
    ]) {
      setFetch(reply(200, body));
      expect(await exchangeLocation(CODES, ADDRESS), JSON.stringify(body)).toEqual({ kind: "khong-goi-duoc" });
    }
    // The world's edges are places.
    expect(readLocation({ latitude: -90, longitude: 180 })).toEqual({ latitude: -90, longitude: 180 });
    expect(readLocation({ latitude: 0, longitude: 0 })).toEqual({ latitude: 0, longitude: 0 });
  });

  it("each contract status has its branch", async () => {
    const table: Array<[number, LocationExchangeResult["kind"]]> = [
      [400, "yeu-cau-hong"],
      [405, "yeu-cau-hong"],
      [429, "qua-nhieu-lan"],
      [502, "zalo-khong-tra-loi"],
      [503, "tam-ngung"],
      [500, "khong-goi-duoc"],
      [401, "khong-goi-duoc"],
    ];
    for (const [status, kind] of table) {
      setFetch(reply(status, { message: "Câu của máy chủ", code: "x" }));
      expect((await exchangeLocation(CODES, ADDRESS)).kind, String(status)).toBe(kind);
    }
  });

  it("network failure → khong-goi-duoc, never a throw", async () => {
    setFetch(new Error("mất mạng"));
    expect(await exchangeLocation(CODES, ADDRESS)).toEqual({ kind: "khong-goi-duoc" });
  });
});

describe("codes then exchange (`getCurrentLocation`)", () => {
  const exchangeSpy = () => vi.fn(async (_c: LocationCodes): Promise<LocationExchangeResult> => ({
    kind: "xong",
    location: { latitude: 1, longitude: 2 },
  }));

  it("passes the two codes, untouched, to exactly one exchange", async () => {
    const exchange = exchangeSpy();
    const result = await getCurrentLocation(async () => ({ kieu: "xong", du_lieu: CODES }), exchange);
    expect(exchange).toHaveBeenCalledTimes(1);
    expect(exchange).toHaveBeenCalledWith(CODES);
    expect(result).toEqual({ kind: "xong", location: { latitude: 1, longitude: 2 } });
  });

  it("refused, outside Zalo, failed, or an empty code → NO exchange", async () => {
    const cases: Array<[KetQuaXin<LocationCodes>, string]> = [
      [{ kieu: "tu-choi" }, "tu-choi"],
      [{ kieu: "ngoai-zalo" }, "ngoai-zalo"],
      [{ kieu: "khong-lay-duoc" }, "khong-lay-duoc-ma"],
      [{ kieu: "xong", du_lieu: { access_token: "", location_token: "v" } }, "khong-lay-duoc-ma"],
      [{ kieu: "xong", du_lieu: { access_token: "a", location_token: "" } }, "khong-lay-duoc-ma"],
    ];
    for (const [codes, kind] of cases) {
      const exchange = exchangeSpy();
      expect((await getCurrentLocation(async () => codes, exchange)).kind).toBe(kind);
      expect(exchange).not.toHaveBeenCalled();
    }
  });
});

describe("shell mapping → the state half's branches (`App.tsx` `toSceneLocationResult`)", () => {
  it("coordinates are renamed, never rounded or changed", () => {
    expect(toSceneLocationResult({ kind: "xong", location: { latitude: 15.571234, longitude: 108.476543 } })).toEqual({
      kind: "xong",
      location: { lat: 15.571234, lng: 108.476543 },
    });
  });

  it("each failure goes to the branch that says what to do next", () => {
    const table: Array<[Parameters<typeof toSceneLocationResult>[0]["kind"], string]> = [
      ["tu-choi", "tu-choi"],
      ["ngoai-zalo", "ngoai-zalo"],
      ["qua-nhieu-lan", "qua-nhieu-lan"],
      ["zalo-khong-tra-loi", "thu-lai"],
      ["khong-goi-duoc", "thu-lai"],
      ["yeu-cau-hong", "thu-lai"],
      ["khong-lay-duoc-ma", "thu-lai"],
      ["tam-ngung", "tam-ngung"],
      ["chua-khai-host", "tam-ngung"],
    ];
    for (const [from, to] of table) {
      expect(toSceneLocationResult({ kind: from } as Parameters<typeof toSceneLocationResult>[0]).kind, from).toBe(to);
    }
  });
});

describe("rule 3 — neither a code nor a coordinate is logged", () => {
  it("no console output on any path of the exchange and the composer", async () => {
    const spies = spyConsole();
    for (const p of [
      reply(200, { latitude: 15.5, longitude: 108.25 }),
      reply(200, { latitude: "x" }),
      reply(429, {}),
      reply(502, {}),
      reply(503, {}),
      new Error("mất mạng"),
    ]) {
      setFetch(p);
      await exchangeLocation(CODES, ADDRESS);
      await getCurrentLocation(async () => ({ kieu: "xong", du_lieu: CODES }), (c) => exchangeLocation(c, ADDRESS));
    }
    await getCurrentLocation(async () => ({ kieu: "tu-choi" }));
    for (const spy of spies) expect(spy).not.toHaveBeenCalled();
  });

  it("the source of the three location files writes no console call and no storage", () => {
    const files = import.meta.glob(["./current-location.ts", "./server-calls.ts", "./contract.ts"], {
      query: "?raw",
      import: "default",
      eager: true,
    }) as Record<string, string>;
    expect(Object.keys(files)).toHaveLength(3);
    for (const [path, source] of Object.entries(files)) {
      const code = source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/(?<!:)\/\/[^\n]*/g, "");
      expect(code, path).not.toMatch(/\bconsole\s*\.|localStorage|sessionStorage|indexedDB/);
    }
  });
});
