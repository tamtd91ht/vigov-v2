import { createServer, request as httpRequest, type IncomingHttpHeaders, type Server } from "node:http";
import type { AddressInfo } from "node:net";

import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from "vitest";

// `server-only` throws outside a React Server build by design; in Node tests it is inert.
vi.mock("server-only", () => ({}));

import { BASEMAP_MISSING_SENTENCE } from "@/lib/basemap/assets";

import {
  BASEMAP_URL_VAR,
  basemapBaseUrl,
  basemapConfigured,
  BasemapUpstreamError,
  proxyBasemap,
  resetBasemapHintForTest,
  type BasemapUpstream,
} from "./basemap";

/**
 * The upstream below is a REAL socket serving a byte buffer with HTTP Range, like MinIO does — so the
 * cases read what actually went over the wire (no cookie, the Range header) and what actually came
 * back (206 bodies, 416 Content-Range), not what a mock was told.
 *
 * The production transport is https-only; the test transport maps the configured https URL onto the
 * local plain-http server and records the URL it was asked for.
 */

const ARCHIVE = Buffer.from(Array.from({ length: 1000 }, (_, i) => i % 251));
const CONFIGURED = "https://minio.cluster.test:9000/vigov-test-public/basemap/20261009";
const ENV = { [BASEMAP_URL_VAR]: CONFIGURED };
const PAGE = "https://xa-thu.example.test";

type Seen = { method: string; url: string; headers: IncomingHttpHeaders };
let seen: Seen[] = [];
let upstreamStatusOverride: number | null = null;
let server: Server;
let port = 0;
const asked: URL[] = [];

beforeAll(async () => {
  server = createServer((req, res) => {
    seen.push({ method: req.method ?? "", url: req.url ?? "", headers: req.headers });
    if (upstreamStatusOverride !== null) {
      res.writeHead(upstreamStatusOverride, { "Content-Type": "application/xml" });
      res.end("<Error><Code>AccessDenied</Code><BucketName>vigov-test-public</BucketName></Error>");
      return;
    }
    if (!(req.url ?? "").endsWith("/vn-mainland.pmtiles") && !(req.url ?? "").includes("/fonts/")) {
      res.writeHead(404, { "Content-Type": "application/xml" });
      res.end("<Error><Code>NoSuchKey</Code></Error>");
      return;
    }
    const common = { "Accept-Ranges": "bytes", ETag: '"abc123"', "Content-Type": "text/html", "Set-Cookie": "x=1" };
    const range = req.headers.range;
    if (range === undefined) {
      res.writeHead(200, { ...common, "Content-Length": String(ARCHIVE.length) });
      res.end(req.method === "HEAD" ? undefined : ARCHIVE);
      return;
    }
    const m = /^bytes=(\d+)-(\d+)$/.exec(range);
    const start = m ? Number(m[1]) : NaN;
    const end = m ? Math.min(Number(m[2]), ARCHIVE.length - 1) : NaN;
    if (!m || start >= ARCHIVE.length || start > end) {
      res.writeHead(416, { ...common, "Content-Range": `bytes */${ARCHIVE.length}`, "Content-Type": "application/xml" });
      res.end("<Error><Code>InvalidRange</Code><BucketName>vigov-test-public</BucketName></Error>");
      return;
    }
    const part = ARCHIVE.subarray(start, end + 1);
    res.writeHead(206, {
      ...common,
      "Content-Length": String(part.length),
      "Content-Range": `bytes ${start}-${end}/${ARCHIVE.length}`,
    });
    res.end(req.method === "HEAD" ? undefined : part);
  });
  await new Promise<void>((ok) => server.listen(0, "127.0.0.1", ok));
  port = (server.address() as AddressInfo).port;
});

afterAll(async () => {
  await new Promise((ok) => server.close(ok));
});

afterEach(() => {
  seen = [];
  asked.length = 0;
  upstreamStatusOverride = null;
  resetBasemapHintForTest();
  vi.restoreAllMocks();
});

/** Real http to the local server, keeping the path the proxy built from BASEMAP_URL. */
const localUpstream: BasemapUpstream = (url, init) =>
  new Promise((resolve, reject) => {
    asked.push(url);
    const req = httpRequest(
      { host: "127.0.0.1", port, path: url.pathname + url.search, method: init.method, headers: init.headers },
      resolve,
    );
    req.on("error", reject);
    req.end();
  });

function browser(path: string, headers: Record<string, string> = {}, method = "GET") {
  return new Request(`${PAGE}${path}`, {
    method,
    headers: { host: "xa-thu.example.test", cookie: "vigov_session=secret-token", authorization: "Bearer t", ...headers },
  });
}

const run = (req: Request, env: Record<string, string | undefined> = ENV) =>
  proxyBasemap(req, { env, upstream: localUpstream });

describe("proxyBasemap — Range pass-through", () => {
  it("forwards Range and returns 206 with exactly those bytes", async () => {
    const res = await run(browser("/basemap/vn-mainland.pmtiles", { range: "bytes=0-126" }));
    expect(res.status).toBe(206);
    expect(res.headers.get("content-range")).toBe("bytes 0-126/1000");
    expect(res.headers.get("accept-ranges")).toBe("bytes");
    expect(res.headers.get("etag")).toBe('"abc123"');
    const body = Buffer.from(await res.arrayBuffer());
    expect(body.equals(ARCHIVE.subarray(0, 127))).toBe(true);
    expect(seen[0]!.headers.range).toBe("bytes=0-126");
  });

  it("reads from <BASEMAP_URL>/<file> — the configured prefix, nothing else", async () => {
    await run(browser("/basemap/vn-mainland.pmtiles", { range: "bytes=0-1" }));
    expect(asked[0]!.href).toBe(`${CONFIGURED}/vn-mainland.pmtiles`);
    expect(seen[0]!.url).toBe("/vigov-test-public/basemap/20261009/vn-mainland.pmtiles");
  });

  it("encodes a font stack path for storage", async () => {
    const res = await run(browser("/basemap/fonts/Noto%20Sans%20Regular/0-255.pbf"));
    expect(res.status).toBe(200);
    expect(seen[0]!.url).toBe("/vigov-test-public/basemap/20261009/fonts/Noto%20Sans%20Regular/0-255.pbf");
    expect(res.headers.get("content-type")).toBe("application/x-protobuf");
  });

  it("an unsatisfiable range is 416 with Content-Range, and storage's XML body is NOT relayed", async () => {
    const res = await run(browser("/basemap/vn-mainland.pmtiles", { range: "bytes=5000-6000" }));
    expect(res.status).toBe(416);
    expect(res.headers.get("content-range")).toBe("bytes */1000");
    const body = await res.text();
    expect(body).toBe("");
  });

  it("forwards If-None-Match and passes conditional headers", async () => {
    await run(browser("/basemap/vn-mainland.pmtiles", { "if-none-match": '"abc123"', "if-range": '"abc123"' }));
    expect(seen[0]!.headers["if-none-match"]).toBe('"abc123"');
    expect(seen[0]!.headers["if-range"]).toBe('"abc123"');
  });

  it("HEAD answers headers only", async () => {
    const res = await run(browser("/basemap/vn-mainland.pmtiles", {}, "HEAD"));
    expect(res.status).toBe(200);
    expect(seen[0]!.method).toBe("HEAD");
    expect(res.headers.get("content-length")).toBe("1000");
    expect(res.body).toBeNull();
  });

  it("Content-Type and Cache-Control are OURS; storage's Set-Cookie never reaches the browser", async () => {
    const res = await run(browser("/basemap/vn-mainland.pmtiles", { range: "bytes=0-9" }));
    expect(res.headers.get("content-type")).toBe("application/octet-stream"); // storage said text/html
    expect(res.headers.get("cache-control")).toBe("private, no-cache");
    expect(res.headers.get("x-content-type-options")).toBe("nosniff");
    expect(res.headers.get("set-cookie")).toBeNull();
  });
});

describe("proxyBasemap — nothing of the staff session leaves", () => {
  it("no cookie, no authorization, no commune Host on the wire to storage", async () => {
    await run(browser("/basemap/vn-mainland.pmtiles", { range: "bytes=0-9", "x-tenant-id": "01HX" }));
    const h = seen[0]!.headers;
    expect(h.cookie).toBeUndefined();
    expect(h.authorization).toBeUndefined();
    expect(h["x-tenant-id"]).toBeUndefined();
    expect(h.host).not.toBe("xa-thu.example.test");
    expect(h["accept-encoding"]).toBe("identity");
  });
});

describe("proxyBasemap — the allow-list", () => {
  it("refuses traversal and unlisted paths with 404, without calling storage", async () => {
    for (const p of [
      "/basemap/%2e%2e/vigov-test-private/petition.jpg",
      "/basemap/..%2Fvigov-test-private%2Fx",
      "/basemap/fonts/..%2F..%2Fx/0-255.pbf",
      "/basemap/fonts/Noto%20Sans%20Regular/0-255.pbf%00",
      "/basemap/other.pmtiles",
      "/basemap/",
      "/basemap/%E0%A4%A",
    ]) {
      const res = await run(browser(p));
      expect(res.status, p).toBe(404);
    }
    expect(seen).toEqual([]);
  });

  it("only GET and HEAD", async () => {
    const res = await run(new Request(`${PAGE}/basemap/vn-mainland.pmtiles`, { method: "POST", body: "x" }));
    expect(res.status).toBe(405);
    expect(seen).toEqual([]);
  });
});

describe("proxyBasemap — no BASEMAP_URL: no map, one sentence, no fallback host", () => {
  it("503 with the sentence, and storage is never called", async () => {
    vi.spyOn(console, "error").mockImplementation(() => {});
    const res = await run(browser("/basemap/vn-mainland.pmtiles"), {});
    expect(res.status).toBe(503);
    expect(await res.text()).toBe(BASEMAP_MISSING_SENTENCE);
    expect(seen).toEqual([]);
  });

  it("the log names the variable, says what it is, how to get it and where to set it — once, never a value", async () => {
    const log = vi.spyOn(console, "error").mockImplementation(() => {});
    const leaked = "https://user:pa55@minio.cluster.test/x";
    await run(browser("/basemap/vn-mainland.pmtiles"), { [BASEMAP_URL_VAR]: leaked });
    await run(browser("/basemap/vn-mainland.pmtiles"), { [BASEMAP_URL_VAR]: leaked });
    expect(log).toHaveBeenCalledTimes(1);
    const line = String(log.mock.calls[0]![0]);
    expect(line).toContain("BASEMAP_URL");
    expect(line).toContain("BASEMAP-URL");
    expect(line).toContain("kb/40-runbooks/basemap-pmtiles.md");
    expect(line).not.toContain("pa55");
  });

  it("validation: https only, no credentials, no query; a path is kept with a trailing slash", () => {
    vi.spyOn(console, "error").mockImplementation(() => {});
    expect(basemapBaseUrl({ [BASEMAP_URL_VAR]: ` ${CONFIGURED} ` })?.href).toBe(`${CONFIGURED}/`);
    for (const bad of [
      "",
      "   ",
      "http://minio.cluster.test:9000/b/basemap",
      "https://u:p@minio.cluster.test/b",
      "https://minio.cluster.test/b?X-Amz-Signature=1",
      "https://minio.cluster.test/b#x",
      "not a url",
    ]) {
      expect(basemapBaseUrl({ [BASEMAP_URL_VAR]: bad }), bad).toBeNull();
    }
    expect(basemapConfigured({})).toBe(false);
    expect(basemapConfigured(ENV)).toBe(true);
  });
});

describe("proxyBasemap — storage failures", () => {
  it("storage 404 (file not uploaded under this prefix) → 404 sentence", async () => {
    vi.spyOn(console, "error").mockImplementation(() => {});
    const res = await run(browser("/basemap/sprites/light.json"));
    expect(res.status).toBe(404);
    expect(await res.text()).toBe("Không tìm thấy tệp bản đồ nền.");
  });

  it("storage 403/500 → 502, and the XML naming the bucket is not relayed", async () => {
    vi.spyOn(console, "error").mockImplementation(() => {});
    for (const s of [403, 500]) {
      upstreamStatusOverride = s;
      const res = await run(browser("/basemap/vn-mainland.pmtiles"));
      expect(res.status, String(s)).toBe(502);
      expect(await res.text()).not.toContain("vigov-test-public");
    }
  });

  it("unreachable → 502, timeout → 504; the socket error text never reaches the client", async () => {
    vi.spyOn(console, "error").mockImplementation(() => {});
    const down: BasemapUpstream = () => Promise.reject(new BasemapUpstreamError("unreachable"));
    const slow: BasemapUpstream = () => Promise.reject(new BasemapUpstreamError("timeout"));
    const a = await proxyBasemap(browser("/basemap/vn-mainland.pmtiles"), { env: ENV, upstream: down });
    const b = await proxyBasemap(browser("/basemap/vn-mainland.pmtiles"), { env: ENV, upstream: slow });
    expect(a.status).toBe(502);
    expect(b.status).toBe(504);
    expect(await a.text()).not.toContain("minio");
  });
});
