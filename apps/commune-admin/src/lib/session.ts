/**
 * Session cookie rules.
 *
 * THE ONE LINE THAT MATTERS MOST IN THIS FILE:
 *
 *   A cookie scoped to the parent domain is sent to EVERY commune subdomain. One line of
 *   configuration, harmless-looking, written once — and it disables the entire isolation
 *   between communes, while every functional test stays green.
 *
 * So the cookie carries NO `domain` attribute at all. Not the parent domain, and not the
 * commune's own host either — see the comment on the absent attribute below for why naming
 * the host is worse than saying nothing.
 */

export const SESSION_COOKIE = "vigov_session";

export function cookieOptions() {
  return {
    name: SESSION_COOKIE,
    httpOnly: true,
    secure: true,
    sameSite: "lax" as const,
    path: "/",
    // THERE IS DELIBERATELY NO `domain` HERE, and the absence is the point.
    //
    // Without the attribute a browser makes the cookie HOST-ONLY: returned only to the exact
    // host that set it, which is exactly one commune's domain.
    //
    // `domain: host` would ALSO be host-only, and it used to be written here — but it is worse,
    // for the reason services/identity/internal/http/cookie.go gives: a present attribute
    // invites the next person to "generalise" it to `.vigov.vn`, and an absent one cannot be
    // widened by accident. One line, harmless-looking, and the isolation between two public
    // authorities is gone while every functional test stays green (rule 1, forbidden #3).
    //
    // The parameter this function used to take was the host, and it existed only to be written
    // into that attribute. Removing the attribute removed the reason for the parameter: a host
    // in scope here is an invitation to use it.
    //
    // The Go service is what actually sets this cookie. These options exist so the two sides
    // cannot drift; they drifted anyway until 2026-09-17, and this is the side that was wrong.
  };
}
