import "server-only";

import type { IncomingMessage, OutgoingHttpHeaders } from "node:http";
import { request as httpsRequest } from "node:https";
import { Readable } from "node:stream";

import { BASEMAP_MISSING_SENTENCE, BASEMAP_ROUTE, basemapAsset } from "@/lib/basemap/assets";

/**
 * `/basemap/*` — the self-hosted Vietnam basemap, streamed SAME-ORIGIN from ViGov's object storage
 * (ADR 0072 §Trả lời 09/10/2026: MinIO + a web-admin route, variable `BASEMAP_URL`).
 *
 * WHY THROUGH web-admin AND NOT STRAIGHT FROM MinIO: the petition maps draw CITIZEN coordinates. Every
 * byte of the basemap must come from the page's own origin, so the browser never names another host
 * (rule 3 stop #2 stays off) and the future CSP stays `connect-src 'self'`.
 *
 * WHAT THIS ROUTE DELIBERATELY DOES NOT DO:
 *   - It does not resolve the commune. It serves the same public OSM-derived bytes under every host and
 *     reads no commune data, so there is nothing to scope; resolving the Host would cost a directory
 *     lookup per byte range (hundreds per map view). It is NOT public either: `proxy.ts` matches it, so
 *     a request without the session cookie is redirected to `/dang-nhap` before it reaches here.
 *   - It forwards NOTHING the browser sent except the range/conditional headers below. No cookie, no
 *     authorization, no Host: the staff session never leaves for the storage endpoint.
 *   - It never buffers. The pod has 512Mi and the archive is gigabytes; the upstream socket is handed
 *     to the response as a stream, so memory is one chunk, not one file.
 *   - It has no fallback host. Unset → 503 + one sentence, the map is not drawn (ADR 0072 H1 "Thiếu
 *     biến", §Sửa đổi 09/10/2026 "Thiếu tệp nền").
 */

export const BASEMAP_URL_VAR = "BASEMAP_URL";

/** Request headers forwarded upstream — an ALLOW-list. Anything else (cookie first of all) is dropped. */
const FORWARDED_REQUEST = ["range", "if-range", "if-match", "if-none-match", "if-modified-since", "if-unmodified-since"];

/** Response headers passed back. Content-Type and Cache-Control are ours (`basemapAsset`), never storage's. */
const PASSED_RESPONSE = ["content-length", "content-range", "accept-ranges", "etag", "last-modified", "content-encoding"];

/** Statuses passed through as they are. 416 carries `Content-Range: bytes *\/<size>`, which PMTiles reads. */
const PASSED_STATUS = new Set([200, 206, 304, 412, 416]);

/** Socket INACTIVITY timeout, same reasoning and value as `goi-noi-bo.ts`: bytes moving = alive. */
export const BASEMAP_IDLE_TIMEOUT_MS = 30_000;

const UNREACHABLE = "Không tải được bản đồ nền. Vui lòng thử lại.";

/**
 * What an operator reads in the log when the variable is missing or wrong — what it is, how to get the
 * value, where to set it (memory "biến mới phải tự giải thích"). Logged ONCE per process: a map view
 * makes hundreds of requests and the same line hundreds of times hides every other line.
 */
const HINT =
  `${BASEMAP_URL_VAR}: địa chỉ thư mục chứa tệp nền PMTiles, glyph và sprite của bản đồ phản ánh, trong MinIO của ViGov ` +
  `(ADR 0072 §Trả lời 09/10/2026). Giá trị: https://<MinIO nội bộ>:<cổng>/<bucket>/basemap/<YYYYMMDD> — https, không ` +
  `thông tin đăng nhập, không truy vấn. Lấy giá trị và tải tệp lên: kb/40-runbooks/basemap-pmtiles.md. ` +
  `Đặt ở: ConfigMap common-config, key BASEMAP-URL (web-admin đọc qua configMapKeyRef). Thiếu thì bản đồ phản ánh không có nền.`;

let hintLogged = false;

function logHintOnce(problem: string) {
  if (hintLogged) return;
  hintLogged = true;
  // The variable NAME only, never its value (rule 8 invariant 3): a malformed URL is where credentials get pasted.
  console.error(`${problem} — ${HINT}`);
}

/** Test hook: the once-per-process latch would otherwise leak between cases. */
export function resetBasemapHintForTest() {
  hintLogged = false;
}

type Env = Readonly<Record<string, string | undefined>>;

/**
 * The storage prefix the route reads from, or `null` when unset or unusable.
 *
 * HTTPS ONLY, even though the endpoint is in-cluster: the object-storage convention of this repository
 * is an https MinIO endpoint (`OBJECT_STORAGE_ENDPOINT`, `core/config/hints.go`), and rule 13 invariant 1
 * wants traffic to data stores encrypted and verified. Plain http would be the one unencrypted channel
 * to MinIO in the system. A MinIO certificate from a private CA needs `NODE_EXTRA_CA_CERTS` on the pod —
 * verification is never switched off instead.
 *
 * A PATH IS ALLOWED (unlike `goc-dich-vu.ts`): the value names a bucket and a dated prefix, not a service
 * origin. Query, fragment and credentials are refused — credentials in a ConfigMap are not a Secret.
 */
export function basemapBaseUrl(env: Env = process.env): URL | null {
  const raw = (env[BASEMAP_URL_VAR] ?? "").trim();
  if (raw === "") {
    logHintOnce(`${BASEMAP_URL_VAR} chưa đặt`);
    return null;
  }
  let url: URL;
  try {
    url = new URL(raw);
  } catch {
    logHintOnce(`${BASEMAP_URL_VAR} không phải một địa chỉ hợp lệ`);
    return null;
  }
  if (url.protocol !== "https:" || url.username !== "" || url.password !== "" || url.search !== "" || url.hash !== "") {
    logHintOnce(`${BASEMAP_URL_VAR} phải là https, không thông tin đăng nhập, không truy vấn`);
    return null;
  }
  if (!url.pathname.endsWith("/")) url.pathname += "/";
  return url;
}

/** For the page that will host the map: draw it, or show `BASEMAP_MISSING_SENTENCE`. */
export function basemapConfigured(env: Env = process.env): boolean {
  return basemapBaseUrl(env) !== null;
}

/** Why the upstream produced no response. */
export class BasemapUpstreamError extends Error {
  constructor(readonly kind: "unreachable" | "timeout") {
    super(kind === "timeout" ? "kho bản đồ nền không phản hồi kịp" : "không kết nối được kho bản đồ nền");
  }
}

export type BasemapUpstream = (
  url: URL,
  init: { method: "GET" | "HEAD"; headers: OutgoingHttpHeaders; signal: AbortSignal },
) => Promise<IncomingMessage>;

/**
 * One https request to storage. `node:https` rather than `fetch`: undici's fetch DECODES a
 * `Content-Encoding` body while keeping byte-range semantics of the encoded one — a byte-exact archive
 * read must see the bytes storage holds. Rejects with `BasemapUpstreamError`, never the socket error
 * (its message carries the internal address).
 */
const httpsUpstream: BasemapUpstream = (url, init) =>
  new Promise((resolve, reject) => {
    let settled = false;
    const fail = (kind: BasemapUpstreamError["kind"]) => {
      if (settled) return;
      settled = true;
      reject(new BasemapUpstreamError(kind));
    };
    const req = httpsRequest(url, {
      method: init.method,
      headers: init.headers,
      timeout: BASEMAP_IDLE_TIMEOUT_MS,
      servername: url.hostname,
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
      req.destroy();
    });
    req.on("error", () => fail("unreachable"));
    if (init.signal.aborted) req.destroy();
    else init.signal.addEventListener("abort", () => req.destroy(), { once: true });
    req.end();
  });

function text(status: number, body: string, extra: Record<string, string> = {}): Response {
  return new Response(body, {
    status,
    headers: {
      "Content-Type": "text/plain; charset=utf-8",
      "Cache-Control": "no-store",
      "X-Content-Type-Options": "nosniff",
      ...extra,
    },
  });
}

/**
 * The DECODED segments after `/basemap/`, read from the raw request path rather than from Next's params
 * so the check does not depend on how a framework version decodes. `null` on a malformed escape.
 */
function segmentsOf(pathname: string): string[] | null {
  if (!pathname.startsWith(BASEMAP_ROUTE + "/")) return null;
  try {
    return pathname.slice(BASEMAP_ROUTE.length + 1).split("/").map((s) => decodeURIComponent(s));
  } catch {
    return null;
  }
}

export type BasemapDeps = { env?: Env; upstream?: BasemapUpstream };

export async function proxyBasemap(request: Request, deps: BasemapDeps = {}): Promise<Response> {
  const method = request.method === "HEAD" ? "HEAD" : request.method === "GET" ? "GET" : null;
  if (method === null) return text(405, "Phương thức không được hỗ trợ.", { Allow: "GET, HEAD" });

  // Configuration first: an unconfigured basemap is ONE answer for every path, so the page shows the
  // sentence instead of a scatter of 404s.
  const base = basemapBaseUrl(deps.env ?? process.env);
  if (base === null) return text(503, BASEMAP_MISSING_SENTENCE);

  const segments = segmentsOf(new URL(request.url).pathname);
  const asset = segments === null ? null : basemapAsset(segments);
  if (asset === null) return text(404, "Không tìm thấy tệp bản đồ nền.");

  const headers: OutgoingHttpHeaders = { "accept-encoding": "identity" };
  for (const name of FORWARDED_REQUEST) {
    const v = request.headers.get(name);
    if (v !== null) headers[name] = v;
  }

  const target = new URL(asset.key, base);
  let res: IncomingMessage;
  try {
    res = await (deps.upstream ?? httpsUpstream)(target, { method, headers, signal: request.signal });
  } catch (err) {
    if (!(err instanceof BasemapUpstreamError)) throw err;
    console.error(JSON.stringify({ msg: "basemap_proxy", outcome: err.kind }));
    return err.kind === "timeout" ? text(504, UNREACHABLE) : text(502, UNREACHABLE);
  }

  const status = res.statusCode ?? 0;
  if (!PASSED_STATUS.has(status)) {
    res.resume();
    // Status only: no path (it can name a font, harmless, but a range names the area being viewed).
    console.error(JSON.stringify({ msg: "basemap_proxy", upstream_status: status }));
    // 404 from storage = the file was never uploaded under this prefix — the runbook's first check.
    return status === 404 ? text(404, "Không tìm thấy tệp bản đồ nền.") : text(502, UNREACHABLE);
  }

  // 412/416 carry storage's XML error body, which names the bucket and key — internal layout the
  // browser has no use for. PMTiles reads only the status and `Content-Range`, so the body is dropped.
  const noBody = method === "HEAD" || status === 304 || status === 412 || status === 416;

  const out = new Headers({
    "Content-Type": asset.contentType,
    "Cache-Control": asset.cacheControl,
    "X-Content-Type-Options": "nosniff",
  });
  for (const name of PASSED_RESPONSE) {
    if (name === "content-length" && noBody && method !== "HEAD") continue;
    const v = res.headers[name];
    if (typeof v === "string") out.set(name, v);
  }
  if (!out.has("accept-ranges") && asset.key.endsWith(".pmtiles")) out.set("accept-ranges", "bytes");

  if (noBody) res.resume();
  return new Response(noBody ? null : (Readable.toWeb(res) as unknown as ReadableStream<Uint8Array>), {
    status,
    headers: out,
  });
}
