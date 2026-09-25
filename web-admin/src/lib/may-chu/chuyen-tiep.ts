import "server-only";

import type { IncomingMessage, OutgoingHttpHeaders } from "node:http";
import { Readable } from "node:stream";
import type { ReadableStream as ReadableStreamNode } from "node:stream/web";

import { DINH_TUYEN_API, type DichVuAPI } from "@/lib/api/dinh-tuyen.gen";

import { goiNoiBo, LoiGoiNoiBo } from "./goi-noi-bo";

/**
 * web-admin AS THE GATEWAY for `/api/v1/*`: every call the browser makes lands here and is
 * forwarded to the ONE Go service that owns the path.
 *
 * WHY THIS EXISTS (decided by the user, 25/09/2026): the cluster sends every path on a commune's
 * host, `/api` included, to web-admin, and k8s supplies only Secrets and a ConfigMap — routing
 * lives in the application. It supersedes options A/B/C recorded in the ledger
 * (`web-admin/goc-api-noi-bo`).
 *
 * WHAT THIS GATEWAY DELIBERATELY DOES NOT DO:
 *   - It does not authenticate or authorise. Every Go service checks the session and the
 *     permission itself on every call (rule 5); a check here would be a second copy that drifts.
 *   - It does not decide the commune. It forwards `Host` untouched and the Go edge resolves it
 *     (rule 1, invariant 3). What it DOES do is make sure nothing else can: every `x-tenant*`
 *     header is dropped here as well as at the edge (rule 1, forbidden #2).
 *   - It has no default service. A path the generated table does not cover is 404 — sending it
 *     "somewhere" is how a route nobody declared ends up answered by the wrong owner.
 */

/** Headers meaningful only for ONE connection (RFC 9110 §7.6.1) — never forwarded. */
const HOP_BY_HOP = new Set([
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
 * Dropped from the REQUEST on top of hop-by-hop:
 *   - `host` — set explicitly from the incoming Host, never copied twice.
 *   - `forwarded`, `x-forwarded-host` — the Go edge reads only `r.Host`; a second host header
 *     on the wire is a second claim about the commune that some future code might believe.
 *   - `expect` — web-admin's own server already answered `100-continue` on its socket.
 */
const BO_KHOI_YEU_CAU = new Set(["host", "forwarded", "x-forwarded-host", "expect"]);

const LOI_KET_NOI = "Hệ thống tạm thời không kết nối được. Vui lòng thử lại.";

/** Owner of `duong` by PATH SEGMENT — `/api/v1/roles` covers `/api/v1/roles/7`, not `/api/v1/role-permissions`. */
export function chuSoHuu(duong: string): DichVuAPI | null {
  for (const { tienTo, dichVu } of DINH_TUYEN_API) {
    if (duong === tienTo || duong.startsWith(tienTo + "/")) return dichVu;
  }
  return null;
}

function tenTrongConnection(giaTri: string | string[] | null | undefined): Set<string> {
  const chuoi = Array.isArray(giaTri) ? giaTri.join(",") : (giaTri ?? "");
  return new Set(
    chuoi
      .split(",")
      .map((t) => t.trim().toLowerCase())
      .filter((t) => t !== ""),
  );
}

/** httpx.Error shape (`core/httpx/edge.go:88`), so a client needs no special branch for the gateway. */
function loiJson(status: number, code: string, message: string): Response {
  return new Response(JSON.stringify({ code, message, trace_id: "" }), {
    status,
    headers: { "Content-Type": "application/json; charset=utf-8" },
  });
}

function headerGuiDi(yeuCau: Request): OutgoingHttpHeaders {
  const theoConnection = tenTrongConnection(yeuCau.headers.get("connection"));
  const ra: OutgoingHttpHeaders = {};
  yeuCau.headers.forEach((giaTri, ten) => {
    // `Headers` hands names back lowercased, so one comparison form is enough.
    if (HOP_BY_HOP.has(ten) || theoConnection.has(ten) || BO_KHOI_YEU_CAU.has(ten)) return;
    if (ten.startsWith("x-tenant")) return;
    ra[ten] = giaTri;
  });
  // X-FORWARDED-FOR GOES THROUGH UNCHANGED, AND THAT IS A MEASURED LIMIT, NOT A CHOICE.
  // Next's server fills it with `??=` from the socket peer (node_modules/next/dist/server/
  // base-server.js:612), i.e. ONLY when the incoming request carried none, and a route handler
  // is never given the socket — so when a proxy in front already sent the header, the peer this
  // process saw is unobtainable here and cannot be appended. Behind the cluster ingress the last
  // entry is therefore what the INGRESS observed, not the ingress's own address.
  return ra;
}

function headerTraVe(phanHoi: IncomingMessage): Headers {
  const theoConnection = tenTrongConnection(phanHoi.headers.connection);
  const ra = new Headers();
  for (const [ten, giaTri] of Object.entries(phanHoi.headers)) {
    if (giaTri === undefined || HOP_BY_HOP.has(ten) || theoConnection.has(ten)) continue;
    // Node keeps every `set-cookie` as a separate array element; `append` keeps them separate
    // on the way out. Joining them would corrupt any cookie whose `Expires` contains a comma.
    if (Array.isArray(giaTri)) for (const g of giaTri) ra.append(ten, g);
    else ra.set(ten, giaTri);
  }
  return ra;
}

/** The single log line: method, owning service, status, duration. Nothing a citizen typed (rule 3). */
function ghiNhat(method: string, dichVu: DichVuAPI, status: number, batDau: number) {
  console.info(
    JSON.stringify({ msg: "api_gateway", method, service: dichVu, status, ms: Date.now() - batDau }),
  );
}

export async function chuyenTiep(yeuCau: Request): Promise<Response> {
  const batDau = Date.now();
  const url = new URL(yeuCau.url);

  const dichVu = chuSoHuu(url.pathname);
  if (dichVu === null) return loiJson(404, "not_found", "Không tìm thấy đường dẫn này.");

  // No Host, no commune — the same answer the Go edge gives, and no call is made to find out.
  const host = yeuCau.headers.get("host") ?? "";
  if (host === "") {
    return loiJson(404, "tenant_not_found", "Không tìm thấy trang cho tên miền này.");
  }

  const coThan = yeuCau.method !== "GET" && yeuCau.method !== "HEAD" && yeuCau.body !== null;

  let phanHoi: IncomingMessage;
  try {
    phanHoi = await goiNoiBo(dichVu, {
      method: yeuCau.method,
      duongDan: url.pathname + url.search,
      host,
      headers: headerGuiDi(yeuCau),
      body: coThan ? Readable.fromWeb(yeuCau.body as unknown as ReadableStreamNode<Uint8Array>) : null,
      signal: yeuCau.signal,
    });
  } catch (loi) {
    // Only a failed CALL becomes 502/504. A misconfigured origin (`gocDichVu` throwing, naming
    // the variable) is rethrown so Next logs it and answers 500 — swallowing it into a 502 would
    // present an operator's typo as "the service is down".
    if (!(loi instanceof LoiGoiNoiBo)) throw loi;
    const hetGio = loi.loai === "het-gio";
    ghiNhat(yeuCau.method, dichVu, hetGio ? 504 : 502, batDau);
    // No upstream address, no socket error text, no stack: the body is read by a browser.
    return hetGio
      ? loiJson(504, "gateway_timeout", "Hệ thống phản hồi quá lâu. Vui lòng thử lại.")
      : loiJson(502, "bad_gateway", LOI_KET_NOI);
  }

  const status = phanHoi.statusCode ?? 0;
  ghiNhat(yeuCau.method, dichVu, status, batDau);
  if (status < 200 || status > 599) {
    phanHoi.resume();
    return loiJson(502, "bad_gateway", LOI_KET_NOI);
  }

  const khongThan = yeuCau.method === "HEAD" || status === 204 || status === 205 || status === 304;
  if (khongThan) phanHoi.resume();

  return new Response(
    khongThan ? null : (Readable.toWeb(phanHoi) as unknown as ReadableStream<Uint8Array>),
    { status, statusText: phanHoi.statusMessage, headers: headerTraVe(phanHoi) },
  );
}
