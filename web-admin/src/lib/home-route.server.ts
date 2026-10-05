import "server-only";

import { headers } from "next/headers";

import type { identity_get_sessions_current, identity_phienHienTaiRa } from "./api/schema.gen";
import { chuSoHuu } from "./may-chu/chuyen-tiep";
import { goiNoiBo, LoiGoiNoiBo } from "./may-chu/goi-noi-bo";

/**
 * The current session, read SERVER-SIDE for one purpose: choosing where `/` redirects (`home-route.ts`).
 *
 * THE SAME PATH EVERY BROWSER CALL ALREADY TAKES. `/api/**` is relayed by this server to the Go service
 * with the request's own `Host` and `cookie` header (`chuyen-tiep.ts`); this does exactly that for one
 * GET, through the same `goiNoiBo` (which keeps the commune's Host on the wire). Nothing new holds the
 * cookie: it is forwarded as received, never parsed, stored, logged or set — and only to the service
 * that owns `/api/v1/sessions` per the generated route table.
 *
 * `null` ON ANY FAILURE — no Host, no cookie, 401, a non-200, a dead service, an unparsable body. The
 * caller then sends the officer to `/tong-quan`, whose own reads answer properly (401 → login).
 * No caching of any kind: a session is one person's on one commune's host.
 */
export async function readSessionForHome(): Promise<identity_phienHienTaiRa | null> {
  const h = await headers();
  const host = h.get("host") ?? "";
  const cookie = h.get("cookie") ?? "";
  if (host === "" || cookie === "") return null;

  const path: identity_get_sessions_current["duongDan"] = "/api/v1/sessions/current";
  const service = chuSoHuu(path);
  if (service === null) return null;

  // X-FORWARDED-FOR GOES WITH THE COOKIE, as the relay passes it (chuyen-tiep.ts): this read can make
  // identity revoke an idle session, and that revocation is audited with the caller's IP (rule 6,
  // invariant 2) — without the header the trail would name this pod instead of the officer.
  const xff = h.get("x-forwarded-for");
  const outgoing: Record<string, string> = xff ? { cookie, "x-forwarded-for": xff } : { cookie };

  try {
    const res = await goiNoiBo(service, { method: "GET", duongDan: path, host, headers: outgoing });
    if (res.statusCode !== 200) {
      res.resume();
      return null;
    }
    const chunks: Buffer[] = [];
    for await (const c of res) chunks.push(c as Buffer);
    return JSON.parse(Buffer.concat(chunks).toString("utf8")) as identity_phienHienTaiRa;
  } catch (e) {
    // A CONFIGURATION error (a bad service address) is rethrown, as the relay does (chuyen-tiep.ts),
    // so an operator's typo surfaces as a 500 naming it. Only a failed CALL lands on the dashboard —
    // and is not logged: the failure text of a socket names internal addresses.
    if (!(e instanceof LoiGoiNoiBo) && !(e instanceof SyntaxError)) throw e;
    return null;
  }
}
