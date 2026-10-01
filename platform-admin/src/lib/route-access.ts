/** Public paths. An allow-list: a path not declared here is protected. */
const PUBLIC_PATHS = new Set<string>(["/dang-nhap"]);

export const SIGN_IN_PATH = "/dang-nhap";

/**
 * Whether `pathname` must be sent to the sign-in page. Pure, so the DENIED case is tested
 * without a server (`route-access.test.ts`).
 *
 * `cookieName === null` means the session cookie is not decided yet (`lib/session.ts`): every
 * protected path is treated as signed out — fail closed, never "let it through until decided".
 */
export function needsSignIn(
  pathname: string,
  cookieName: string | null,
  hasCookie: (name: string) => boolean,
): boolean {
  if (PUBLIC_PATHS.has(pathname)) return false;
  if (cookieName === null) return true;
  return !hasCookie(cookieName);
}
