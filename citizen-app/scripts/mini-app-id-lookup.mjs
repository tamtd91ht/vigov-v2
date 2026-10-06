/**
 * The ONE network call of the deploy step: `GET <platform>/api/v1/mini-app-ids?app=vihat | ?host=<domain>`
 * (fixed contract, 06/10/2026). Read-only, unauthenticated, rate-limited on the platform side.
 *
 * Returns the raw outcome and decides nothing — `appIdFromPlatform` (`dich-den.mjs`) maps it, purely, so
 * every branch is tested without a network:
 *
 *   { kind: "http", status, body }   body = parsed JSON, or null when the body is not JSON
 *   { kind: "network", reason }      no answer at all: DNS, refused, TLS, timeout
 *
 * `fetchImpl` is injectable for tests; production uses Node's global `fetch`. The timeout is short on
 * purpose: a person is waiting at the terminal, and "no answer" already has a defined meaning (fail closed).
 */
import { MINI_APP_ID_PATH } from "./dich-den.mjs";

export const LOOKUP_TIMEOUT_MS = 10_000;

export async function lookupMiniAppId(origin, query, { fetchImpl = fetch, timeoutMs = LOOKUP_TIMEOUT_MS } = {}) {
  const url = new URL(MINI_APP_ID_PATH, origin);
  for (const [k, v] of Object.entries(query)) url.searchParams.set(k, v);
  let res;
  try {
    res = await fetchImpl(url, {
      method: "GET",
      headers: { accept: "application/json" },
      redirect: "error",
      signal: AbortSignal.timeout(timeoutMs),
    });
  } catch (err) {
    // The reason names the failure class only — never a body or header (nothing personal travels here
    // anyway: a domain or the word "vihat").
    const reason = err?.name === "TimeoutError" ? `quá ${timeoutMs / 1000} giây` : (err?.cause?.code ?? err?.name ?? "lỗi mạng");
    return { kind: "network", reason: String(reason) };
  }
  let body = null;
  try {
    body = await res.json();
  } catch {
    body = null;
  }
  return { kind: "http", status: res.status, body };
}
