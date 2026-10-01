import { createServer, type IncomingHttpHeaders, type Server, type ServerResponse } from "node:http";
import type { AddressInfo } from "node:net";

import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from "vitest";

// `server-only` throws outside a React Server build by design; in Node tests it is inert.
vi.mock("server-only", () => ({}));

import { forwardToPlatform } from "./gateway";
import { OPERATOR_HOST_VAR } from "./operator-host";
import { PLATFORM_ORIGIN_VAR } from "./platform-origin";

/**
 * Every case reads what a REAL socket received — web-admin measured that a mocked `Headers`
 * keeps `host` while the wire carries another one, so a mock goes green over the exact defect.
 */

const OPERATOR_HOST = "admin.example.gov.vn";

type Received = { method: string; url: string; headers: IncomingHttpHeaders; body: Buffer };

let received: Received[] = [];
let respond: (res: ServerResponse) => void = (res) => res.end("{}");
let server: Server;
let upstream = "";

beforeAll(async () => {
  server = createServer((req, res) => {
    const chunks: Buffer[] = [];
    req.on("data", (c: Buffer) => chunks.push(c));
    req.on("end", () => {
      received.push({ method: req.method ?? "", url: req.url ?? "", headers: req.headers, body: Buffer.concat(chunks) });
      respond(res);
    });
  });
  await new Promise<void>((ok) => server.listen(0, "127.0.0.1", ok));
  upstream = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
});

afterAll(async () => {
  await new Promise((ok) => server.close(ok));
});

afterEach(() => {
  received = [];
  respond = (res) => res.end("{}");
  delete process.env[PLATFORM_ORIGIN_VAR];
  process.env[OPERATOR_HOST_VAR] = OPERATOR_HOST;
  vi.restoreAllMocks();
});

beforeAll(() => {
  process.env[OPERATOR_HOST_VAR] = OPERATOR_HOST;
});

afterAll(() => {
  delete process.env[OPERATOR_HOST_VAR];
});

function request(path: string, init: RequestInit & { headers?: Record<string, string> } = {}) {
  return new Request(`http://${OPERATOR_HOST}${path}`, { ...init, headers: { host: OPERATOR_HOST, ...init.headers } });
}

function quiet() {
  vi.spyOn(console, "info").mockImplementation(() => {});
  return vi.spyOn(console, "error").mockImplementation(() => {});
}

describe("forwardToPlatform — fail closed without an origin", () => {
  it("unset PLATFORM_HTTP_ADDR → 503, and NO call is made anywhere", async () => {
    quiet();
    const res = await forwardToPlatform(request("/api/v1/anything"));

    expect(res.status).toBe(503);
    const body = await res.json();
    expect(body.code).toBe("platform_unconfigured");
    expect(received).toHaveLength(0);
  });

  it("blank value is unset, not a default", async () => {
    quiet();
    process.env[PLATFORM_ORIGIN_VAR] = "   ";
    const res = await forwardToPlatform(request("/api/v1/anything"));
    expect(res.status).toBe(503);
    expect(received).toHaveLength(0);
  });

  it.each([
    ["not a URL", "platform:8080"],
    ["a path prefix", "http://127.0.0.1:1/prefix"],
    ["credentials", "http://user:secret-value@127.0.0.1:1"],
    ["another scheme", "ftp://127.0.0.1:1"],
  ])("malformed (%s) → 503, the log names the variable and never the value", async (_why, value) => {
    const error = quiet();
    process.env[PLATFORM_ORIGIN_VAR] = value;
    const res = await forwardToPlatform(request("/api/v1/anything"));

    expect(res.status).toBe(503);
    expect(received).toHaveLength(0);
    const logged = error.mock.calls.map((c) => String(c[0])).join("\n");
    expect(logged).toContain(PLATFORM_ORIGIN_VAR);
    expect(logged).not.toContain(value);
    // The browser gets a sentence, not the configuration.
    expect(JSON.stringify(await res.json())).not.toContain(PLATFORM_ORIGIN_VAR);
  });
});

describe("forwardToPlatform — the upstream Host is OPERATOR_HOST, never the client's", () => {
  // buildOuter sends every Host other than OPERATOR_HOST to platform's COMMUNE chain. A relayed
  // client Host is a request into a commune's REST surface from a trusted-proxy pod.
  it("ignores a commune Host from the client and sends OPERATOR_HOST", async () => {
    quiet();
    process.env[PLATFORM_ORIGIN_VAR] = upstream;
    const res = await forwardToPlatform(
      new Request("http://xa-a.vigov.vn/api/v1/x", { headers: { host: "xa-a.vigov.vn" } }),
    );

    expect(res.status).toBe(200);
    expect(received).toHaveLength(1);
    expect(received[0]?.headers.host).toBe(OPERATOR_HOST);
  });

  it("a request with no Host at all still goes out with OPERATOR_HOST", async () => {
    quiet();
    process.env[PLATFORM_ORIGIN_VAR] = upstream;
    await forwardToPlatform(new Request("http://127.0.0.1/api/v1/x"));
    expect(received[0]?.headers.host).toBe(OPERATOR_HOST);
  });

  it("forwards Origin unchanged — platform checks it against https://OPERATOR_HOST on writes", async () => {
    quiet();
    process.env[PLATFORM_ORIGIN_VAR] = upstream;
    await forwardToPlatform(
      request("/api/v1/x", { method: "POST", body: "{}", headers: { origin: "https://evil.example" } }),
    );
    expect(received[0]?.headers.origin).toBe("https://evil.example");
  });

  it.each([
    ["unset", undefined],
    ["blank", "   "],
  ])("%s OPERATOR_HOST → 503, and NO call is made", async (_why, value) => {
    const error = quiet();
    process.env[PLATFORM_ORIGIN_VAR] = upstream;
    if (value === undefined) delete process.env[OPERATOR_HOST_VAR];
    else process.env[OPERATOR_HOST_VAR] = value;

    const res = await forwardToPlatform(request("/api/v1/x"));

    expect(res.status).toBe(503);
    expect((await res.json()).code).toBe("platform_unconfigured");
    expect(received).toHaveLength(0);
    expect(error.mock.calls.map((c) => String(c[0])).join("\n")).toContain(OPERATOR_HOST_VAR);
  });

  it.each([
    ["a scheme", "https://admin.example.gov.vn"],
    ["a path", "admin.example.gov.vn/x"],
    ["a port", "admin.example.gov.vn:443"],
    ["upper case", "Admin.example.gov.vn"],
    ["a trailing dot", "admin.example.gov.vn."],
    ["an IP", "10.0.0.7"],
    ["a single label", "localhost"],
    ["a bad label", "adm_in.example.gov.vn"],
    ["the platform root", "vigov.vn"],
    ["a commune host under vigov.vn", "xa-a.vigov.vn"],
    ["a commune host under stg.vigov.vn", "xa-a.stg.vigov.vn"],
  ])("malformed OPERATOR_HOST (%s) → 503, no call, log names the variable and never the value", async (_why, value) => {
    const error = quiet();
    process.env[PLATFORM_ORIGIN_VAR] = upstream;
    process.env[OPERATOR_HOST_VAR] = value;

    const res = await forwardToPlatform(request("/api/v1/x"));

    expect(res.status).toBe(503);
    expect(received).toHaveLength(0);
    const logged = error.mock.calls.map((c) => String(c[0])).join("\n");
    expect(logged).toContain(OPERATOR_HOST_VAR);
    expect(logged).not.toContain(value);
    expect(JSON.stringify(await res.json())).not.toContain(OPERATOR_HOST_VAR);
  });

  it.each(["admin.vigov.vn", "admin-stg.vigov.vn", "admin.stg.vigov.vn"])(
    "accepts the reserved console host %s",
    async (value) => {
      quiet();
      process.env[PLATFORM_ORIGIN_VAR] = upstream;
      process.env[OPERATOR_HOST_VAR] = value;
      await forwardToPlatform(request("/api/v1/x"));
      expect(received[0]?.headers.host).toBe(value);
    },
  );
});

describe("forwardToPlatform — with an origin", () => {
  it("forwards path, query, the OPERATOR Host and the session cookie unchanged", async () => {
    quiet();
    process.env[PLATFORM_ORIGIN_VAR] = upstream;
    const res = await forwardToPlatform(
      request("/api/v1/x?page=2&q=a%20b", { headers: { cookie: "s=op1.opaque; other=1" } }),
    );

    expect(res.status).toBe(200);
    expect(received).toHaveLength(1);
    expect(received[0]?.headers.host).toBe(OPERATOR_HOST);
    expect(received[0]?.url).toBe("/api/v1/x?page=2&q=a%20b");
    expect(received[0]?.headers.cookie).toBe("s=op1.opaque; other=1");
  });

  it("drops any client-supplied tenant header and second host claims", async () => {
    quiet();
    process.env[PLATFORM_ORIGIN_VAR] = upstream;
    await forwardToPlatform(
      request("/api/v1/x", {
        headers: { "x-tenant-id": "01HX", "x-forwarded-host": "evil.example", forwarded: "host=evil.example" },
      }),
    );

    const h = received[0]?.headers ?? {};
    expect(h["x-tenant-id"]).toBeUndefined();
    expect(h["x-forwarded-host"]).toBeUndefined();
    expect(h.forwarded).toBeUndefined();
  });

  it("streams a POST body and returns every set-cookie separately", async () => {
    quiet();
    process.env[PLATFORM_ORIGIN_VAR] = upstream;
    respond = (res) => {
      res.setHeader("set-cookie", ["a=1; Path=/; HttpOnly", "b=2; Expires=Wed, 01 Oct 2026 00:00:00 GMT"]);
      res.statusCode = 201;
      res.end("{}");
    };
    const res = await forwardToPlatform(
      request("/api/v1/x", { method: "POST", body: '{"k":"v"}', headers: { "content-type": "application/json" } }),
    );

    expect(res.status).toBe(201);
    expect(received[0]?.body.toString()).toBe('{"k":"v"}');
    expect(res.headers.getSetCookie()).toEqual([
      "a=1; Path=/; HttpOnly",
      "b=2; Expires=Wed, 01 Oct 2026 00:00:00 GMT",
    ]);
  });

  it("an unreachable platform → 502 without the upstream address in the body", async () => {
    quiet();
    // Port 1 on loopback: nothing listens there.
    process.env[PLATFORM_ORIGIN_VAR] = "http://127.0.0.1:1";
    const res = await forwardToPlatform(request("/api/v1/x"));

    expect(res.status).toBe(502);
    expect(await res.text()).not.toContain("127.0.0.1");
  });
});
