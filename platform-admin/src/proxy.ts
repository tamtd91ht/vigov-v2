import { NextResponse, type NextRequest } from "next/server";

import { buildDocumentCsp, createNonce } from "@/lib/csp";
import { needsSignIn, SIGN_IN_PATH } from "@/lib/route-access";
import { OPERATOR_SESSION_COOKIE } from "@/lib/session";

/**
 * Server-side gate in front of every page (Next.js 16 renamed `middleware.ts` to `proxy.ts`).
 *
 * TWO JOBS, BOTH OF WHICH MUST NOT LIVE IN THE BROWSER:
 *   1. Route protection (`lib/route-access.ts`). Hiding a link is UX, not security (rule 5
 *      forbidden #1); this runs before any page renders and has no copy in the bundle.
 *   2. The per-request CSP nonce (`lib/csp.ts`).
 *
 * WHAT IT DELIBERATELY DOES NOT DO: validate the session. It sees only whether the cookie is
 * present. Signature, `sid` liveness, idle expiry, revocation and the `operator` realm are checked
 * by service-platform on every real request (ADR 0048 §01/10 #2). A stale cookie passing here buys
 * a 401 on the next API call, never data — and the shell holds no data of its own.
 */
export default function proxy(request: NextRequest) {
  const nonce = createNonce();
  const csp = buildDocumentCsp(nonce, process.env.NODE_ENV === "development");

  if (needsSignIn(request.nextUrl.pathname, OPERATOR_SESSION_COOKIE, (n) => request.cookies.has(n))) {
    const target = request.nextUrl.clone();
    target.pathname = SIGN_IN_PATH;
    target.search = "";
    const redirect = NextResponse.redirect(target);
    redirect.headers.set("Content-Security-Policy", csp);
    return redirect;
  }

  // Next reads the nonce from the REQUEST's CSP header and stamps it on the scripts it renders.
  const requestHeaders = new Headers(request.headers);
  requestHeaders.set("Content-Security-Policy", csp);
  const response = NextResponse.next({ request: { headers: requestHeaders } });
  response.headers.set("Content-Security-Policy", csp);
  return response;
}

export const config = {
  // Every path except Next's static assets, the favicon and `/api/`.
  //
  // `/api/` goes through this app (`app/api/v1/[...path]/route.ts` forwards it to service-platform)
  // but deliberately NOT through the proxy, for web-admin's measured reason: a path matched by the
  // proxy has its body copied and CUT at `proxyClientMaxBodySize` (10 MB by default), and an API
  // client expects a 401 JSON, not a 307 to the sign-in page. service-platform authenticates and
  // authorises every API call itself.
  matcher: ["/((?!api/|_next/static|_next/image|favicon.ico).*)"],
};
