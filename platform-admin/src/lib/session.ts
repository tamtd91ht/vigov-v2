/**
 * Name of the operator session cookie, exactly as service-platform sets it
 * (`service-platform/internal/opauth/opauth.go` `CookieName`).
 *
 * `__Host-` is the browser's own enforcement of host-only: such a cookie is refused unless it is
 * `Secure`, has `Path=/` and NO `Domain` — so it can never be widened to the parent domain and sent
 * to a commune's subdomain (rule 1 forbidden #3, rule 13 inv. 4). The server sets it HttpOnly and
 * SameSite=Strict.
 *
 * This app NEVER reads its value. `src/proxy.ts` asks only whether it is PRESENT, to decide whether
 * a page is worth rendering; service-platform validates it on every API call. If the server's name
 * changes and this one does not, every console page redirects to sign-in (fail closed) — the
 * failure is loud, never a page served without a session.
 */
export const OPERATOR_SESSION_COOKIE: string | null = "__Host-vigov_operator_session";
