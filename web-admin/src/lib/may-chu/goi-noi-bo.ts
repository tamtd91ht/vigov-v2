import "server-only";

import { request as yeuCauHttp, type IncomingMessage, type OutgoingHttpHeaders } from "node:http";
import { request as yeuCauHttps } from "node:https";
import type { Readable } from "node:stream";
import { pipeline } from "node:stream";

import type { DichVuAPI } from "@/lib/api/dinh-tuyen.gen";

import { gocDichVu } from "./goc-dich-vu";

/**
 * One HTTP call from this server to a Go service, with the COMMUNE'S Host on the wire.
 *
 * WHY `node:http` AND NOT `fetch` — MEASURED, NOT PREFERRED (ledger `web-admin/goc-api-noi-bo`,
 * 21/09/2026): Node's `fetch` (undici) overwrites any caller-set `Host` with the host of the URL.
 * `Host`, `host`, a `Headers` object and a `Request` object were all tried; all four put the
 * internal service name on the wire. The Go edge resolves the commune from `r.Host` and nothing
 * else (`core/httpx/edge.go:24`), so a `fetch` here reaches identity as "no commune" → 404 on
 * every page — and the tempting "fix" of registering the internal name as a commune would print
 * that one commune's name under every commune's domain: a leak between two public bodies.
 * `node:http` sends `headers.host` as given; the tests assert it on a real socket, because a
 * `Headers` mock keeps `host` and goes green while the wire carries the wrong one.
 *
 * WHY ONE IDLE TIMEOUT OF 30 s AND NO TOTAL DEADLINE: `timeout` on `http.request` is a socket
 * INACTIVITY timer that also covers the connect phase. An upload of 25 MB over a commune's
 * uplink may take minutes and is fine as long as bytes keep moving; 30 s with nothing moving,
 * in either direction, is a dead peer. A total deadline would cut legitimate slow uploads.
 */
export const THOI_GIAN_CHO_MS = 30_000;

/** Why an internal call produced no response — the two cases map to 502 and 504. */
export class LoiGoiNoiBo extends Error {
  constructor(readonly loai: "khong-ket-noi" | "het-gio") {
    super(loai === "het-gio" ? "dịch vụ nội bộ không phản hồi kịp" : "không kết nối được dịch vụ nội bộ");
  }
}

export type LoiGoi = {
  method: string;
  /** Path + query, sent unchanged. */
  duongDan: string;
  /** The commune's host, exactly as the Go edge must see it. */
  host: string;
  headers?: OutgoingHttpHeaders;
  body?: Readable | null;
  signal?: AbortSignal;
};

/**
 * Resolves once the response HEADERS arrive; the body is the caller's to consume. Rejects
 * with `LoiGoiNoiBo` — never with the raw socket error, whose message carries the upstream
 * address (`connect ECONNREFUSED 10.x.y.z:8080`) and must not reach a client.
 */
export function goiNoiBo(dichVu: DichVuAPI, loi: LoiGoi): Promise<IncomingMessage> {
  const goc = gocDichVu(dichVu);
  const guiDi = goc.protocol === "https:" ? yeuCauHttps : yeuCauHttp;

  return new Promise<IncomingMessage>((xong, hong) => {
    let daXong = false;
    const ket = (loai: LoiGoiNoiBo["loai"]) => {
      if (daXong) return;
      daXong = true;
      hong(new LoiGoiNoiBo(loai));
    };

    const yc = guiDi({
      protocol: goc.protocol,
      hostname: goc.hostname,
      port: goc.port === "" ? undefined : Number(goc.port),
      // SNI and certificate check against the CONFIGURED origin. Without this Node derives the
      // server name from the Host header — the commune's public domain — and an internal TLS
      // endpoint would fail verification, or worse, be verified against the wrong name.
      servername: goc.protocol === "https:" ? goc.hostname : undefined,
      method: loi.method,
      path: loi.duongDan,
      headers: { ...loi.headers, host: loi.host },
      timeout: THOI_GIAN_CHO_MS,
    });

    yc.on("response", (phanHoi) => {
      if (daXong) {
        phanHoi.resume();
        return;
      }
      daXong = true;
      xong(phanHoi);
    });
    yc.on("timeout", () => {
      ket("het-gio");
      yc.destroy(new LoiGoiNoiBo("het-gio"));
    });
    yc.on("error", (loiSocket) => {
      ket(loiSocket instanceof LoiGoiNoiBo ? loiSocket.loai : "khong-ket-noi");
    });

    if (loi.signal) {
      if (loi.signal.aborted) yc.destroy();
      else loi.signal.addEventListener("abort", () => yc.destroy(), { once: true });
    }

    if (loi.body) {
      // `pipeline` streams without buffering and tears both ends down on failure, so a browser
      // that drops an upload mid-way does not leave a half-open socket to the service.
      pipeline(loi.body, yc, () => {});
    } else {
      yc.end();
    }
  });
}
