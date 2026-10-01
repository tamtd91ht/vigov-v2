import "server-only";

import {
  request as httpRequest,
  type IncomingMessage,
  type OutgoingHttpHeaders,
} from "node:http";
import { request as httpsRequest } from "node:https";
import { pipeline, Readable } from "node:stream";
import type { ReadableStream as NodeReadableStream } from "node:stream/web";

import { operatorHost } from "./operator-host";
import { platformOrigin } from "./platform-origin";

/**
 * This console's `/api/v1/*` gateway: the browser calls SAME-ORIGIN relative paths, and this
 * server forwards each one to service-platform — the ONLY backend this app has (ADR 0003: no
 * client for any business service exists here, so no path to commune business data exists).
 * Mirrors web-admin's gateway (`web-admin/src/lib/may-chu/chuyen-tiep.ts`), reduced to one owner.
 *
 * WHAT IT DELIBERATELY DOES NOT DO:
 *   - Authenticate or authorise. service-platform checks the `op1.` token, the `sid`, the
 *     `operator` realm and the `ops.*` key on every call (ADR 0048 §01/10 #2). A copy here drifts.
 *   - Read, parse or log the session cookie. It travels in `cookie` unchanged, like every other
 *     end-to-end header; `set-cookie` comes back unchanged.
 *   - Relay the client's `Host`. service-platform's outer mux (`buildOuter`,
 *     `service-platform/cmd/server/operator_edge.go`) does NOT 404 a non-operator host: it sends
 *     Host == OPERATOR_HOST to the operator chain and EVERY other Host to the COMMUNE chain. A
 *     relayed `Host: <commune host>` would reach platform's commune REST surface from this pod,
 *     which may be a trusted proxy there. So the upstream Host is pinned to OPERATOR_HOST, read
 *     server-side per request (`operator-host.ts`); the client's value is dropped. `Origin` is
 *     forwarded unchanged — platform compares it with https://OPERATOR_HOST on writes.
 *   - Pick a route table. TASK-05's operator routes are not published yet, so every `/api/v1/*`
 *     path goes to platform and platform 404s what it does not serve. There is no second owner
 *     to choose between, by design.
 */

/** Idle socket timeout, web-admin's value and reason: no total deadline that cuts slow uploads. */
const IDLE_TIMEOUT_MS = 30_000;

/** Headers meaningful for ONE connection (RFC 9110 §7.6.1) — never forwarded either way. */
const CONNECTION_SCOPED = new Set([
  "connection",
  "keep-alive",
  "proxy-authenticate",
  "proxy-authorization",
  "proxy-connection",
  "te",
  "trailer",
  "transfer-encoding",
  "upgrade",
]);

/**
 * Dropped from the REQUEST on top of the connection-scoped ones: `host` is pinned to OPERATOR_HOST;
 * `forwarded` and `x-forwarded-host` are second claims about the host that some future code might
 * believe; `expect` was already answered by this server.
 */
const REQUEST_DROPPED = new Set(["host", "forwarded", "x-forwarded-host", "expect"]);

const UNREACHABLE = "Hệ thống tạm thời không kết nối được. Vui lòng thử lại.";

class UpstreamError extends Error {
  constructor(readonly kind: "unreachable" | "timeout") {
    super(kind);
  }
}

/** httpx.Error shape (`core/httpx/edge.go`), so a client needs no special branch for the gateway. */
function jsonError(status: number, code: string, message: string): Response {
  return new Response(JSON.stringify({ code, message, trace_id: "" }), {
    status,
    headers: { "Content-Type": "application/json; charset=utf-8" },
  });
}

function connectionTokens(value: string | string[] | null | undefined): Set<string> {
  const joined = Array.isArray(value) ? value.join(",") : (value ?? "");
  return new Set(
    joined
      .split(",")
      .map((t) => t.trim().toLowerCase())
      .filter((t) => t !== ""),
  );
}

function outgoingHeaders(req: Request): OutgoingHttpHeaders {
  const listed = connectionTokens(req.headers.get("connection"));
  const out: OutgoingHttpHeaders = {};
  req.headers.forEach((value, name) => {
    if (CONNECTION_SCOPED.has(name) || listed.has(name) || REQUEST_DROPPED.has(name)) return;
    // The operator realm has no commune; a client naming one is a client granting itself scope
    // (rule 1 forbidden #2). Dropped here as well as at the Go edge.
    if (name.startsWith("x-tenant")) return;
    out[name] = value;
  });
  return out;
}

function returnedHeaders(res: IncomingMessage): Headers {
  const listed = connectionTokens(res.headers.connection);
  const out = new Headers();
  for (const [name, value] of Object.entries(res.headers)) {
    if (value === undefined || CONNECTION_SCOPED.has(name) || listed.has(name)) continue;
    // Each `set-cookie` stays a separate header: joining them corrupts an `Expires` with a comma.
    if (Array.isArray(value)) for (const v of value) out.append(name, v);
    else out.set(name, value);
  }
  return out;
}

/** One log line: method, status, duration. No path, no header, no cookie (rule 3, rule 8). */
function logLine(method: string, status: number, startedAt: number) {
  console.info(JSON.stringify({ msg: "api_gateway", method, status, ms: Date.now() - startedAt }));
}

type UpstreamCall = {
  method: string;
  path: string;
  host: string;
  headers: OutgoingHttpHeaders;
  body: Readable | null;
  signal: AbortSignal;
};

/**
 * WHY `node:http` AND NOT `fetch`, measured in web-admin (ledger `web-admin/goc-api-noi-bo`):
 * undici overwrites a caller-set `Host` with the URL's host, so platform would see its internal
 * service name instead of OPERATOR_HOST — and route every operator call to the commune chain, which
 * 404s a host it does not know.
 */
function callPlatform(origin: URL, call: UpstreamCall): Promise<IncomingMessage> {
  const send = origin.protocol === "https:" ? httpsRequest : httpRequest;
  return new Promise<IncomingMessage>((resolve, reject) => {
    let settled = false;
    const fail = (kind: UpstreamError["kind"]) => {
      if (settled) return;
      settled = true;
      // Never the raw socket error: its message carries the upstream address.
      reject(new UpstreamError(kind));
    };

    const req = send({
      protocol: origin.protocol,
      hostname: origin.hostname,
      port: origin.port === "" ? undefined : Number(origin.port),
      // Certificate checked against the CONFIGURED origin, not against the public Host header.
      servername: origin.protocol === "https:" ? origin.hostname : undefined,
      method: call.method,
      path: call.path,
      headers: { ...call.headers, host: call.host },
      timeout: IDLE_TIMEOUT_MS,
    });

    req.on("response", (res) => {
      if (settled) {
        res.resume();
        return;
      }
      settled = true;
      resolve(res);
    });
    req.on("timeout", () => {
      fail("timeout");
      req.destroy(new UpstreamError("timeout"));
    });
    req.on("error", (err) => fail(err instanceof UpstreamError ? err.kind : "unreachable"));

    if (call.signal.aborted) req.destroy();
    else call.signal.addEventListener("abort", () => req.destroy(), { once: true });

    if (call.body) pipeline(call.body, req, () => {});
    else req.end();
  });
}

export async function forwardToPlatform(req: Request): Promise<Response> {
  const startedAt = Date.now();

  // FAIL CLOSED: no configured origin, no call, and no guessed one (`platform-origin.ts`).
  const resolved = platformOrigin();
  if (!resolved.ok) {
    // The operator reads the log; the browser reads a sentence. Neither gets the value.
    console.error(
      JSON.stringify({ msg: "api_gateway_unconfigured", reason: resolved.reason, detail: resolved.detail }),
    );
    logLine(req.method, 503, startedAt);
    return jsonError(
      503,
      "platform_unconfigured",
      "Khu vận hành chưa được cấu hình kết nối tới dịch vụ nền tảng. Vui lòng báo bộ phận kỹ thuật.",
    );
  }

  // FAIL CLOSED the same way: no pinned host, no call. Never the client's Host as a fallback —
  // that is the exact relay into platform's commune chain this variable exists to prevent.
  const pinned = operatorHost();
  if (!pinned.ok) {
    console.error(
      JSON.stringify({ msg: "api_gateway_unconfigured", reason: pinned.reason, detail: pinned.detail }),
    );
    logLine(req.method, 503, startedAt);
    return jsonError(
      503,
      "platform_unconfigured",
      "Khu vận hành chưa được cấu hình kết nối tới dịch vụ nền tảng. Vui lòng báo bộ phận kỹ thuật.",
    );
  }
  const host = pinned.host;

  const url = new URL(req.url);
  const hasBody = req.method !== "GET" && req.method !== "HEAD" && req.body !== null;

  let res: IncomingMessage;
  try {
    res = await callPlatform(resolved.origin, {
      method: req.method,
      path: url.pathname + url.search,
      host,
      headers: outgoingHeaders(req),
      body: hasBody ? Readable.fromWeb(req.body as unknown as NodeReadableStream<Uint8Array>) : null,
      signal: req.signal,
    });
  } catch (err) {
    if (!(err instanceof UpstreamError)) throw err;
    const timeout = err.kind === "timeout";
    logLine(req.method, timeout ? 504 : 502, startedAt);
    return timeout
      ? jsonError(504, "gateway_timeout", "Hệ thống phản hồi quá lâu. Vui lòng thử lại.")
      : jsonError(502, "bad_gateway", UNREACHABLE);
  }

  const status = res.statusCode ?? 0;
  logLine(req.method, status, startedAt);
  if (status < 200 || status > 599) {
    res.resume();
    return jsonError(502, "bad_gateway", UNREACHABLE);
  }

  const noBody = req.method === "HEAD" || status === 204 || status === 205 || status === 304;
  if (noBody) res.resume();

  return new Response(noBody ? null : (Readable.toWeb(res) as unknown as ReadableStream<Uint8Array>), {
    status,
    statusText: res.statusMessage,
    headers: returnedHeaders(res),
  });
}
