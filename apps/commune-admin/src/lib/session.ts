/**
 * Session cookie rules.
 *
 * THE ONE LINE THAT MATTERS MOST IN THIS FILE:
 *
 *   A cookie scoped to the parent domain is sent to EVERY commune subdomain. One line of
 *   configuration, harmless-looking, written once — and it disables the entire isolation
 *   between communes, while every functional test stays green.
 *
 * So the cookie domain is always the commune's own host, and never `.vigov.vn`.
 */

export const SESSION_COOKIE = "vigov_session";

export function cookieOptions(host: string) {
  return {
    name: SESSION_COOKIE,
    httpOnly: true,
    secure: true,
    sameSite: "lax" as const,
    path: "/",
    // Deliberately the exact host. Never a parent domain.
    domain: host,
  };
}
