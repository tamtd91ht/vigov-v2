/**
 * Name of the operator session cookie — or `null` while it is not decided.
 *
 * TODO(TASK-06b): set this to the name service-platform's operator sign-in route sets
 * (TASK-05). It is NOT guessed here: a guessed name that later differs from the real one would
 * make the proxy wave through a cookie nobody issues, or lock out the one that is issued.
 *
 * `null` FAILS CLOSED: `src/proxy.ts` treats every request as signed out, so every console route
 * redirects to `/dang-nhap` until the real name lands. The cookie itself is set by the server,
 * HttpOnly and host-only on the operator host (rule 1 forbidden #3, rule 13 inv. 4); this app
 * never reads its value — the proxy asks only whether it is present.
 */
export const OPERATOR_SESSION_COOKIE: string | null = null;
