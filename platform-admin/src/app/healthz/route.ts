/**
 * Liveness/readiness for the k8s probe.
 *
 * The probe connects with the POD IP as Host and carries no cookie. Every other path of this
 * console sits behind `proxy.ts`, which sends a cookie-less request to `/dang-nhap` (307) — a probe
 * on `/` would therefore never see 200, and the pod would never become Ready.
 *
 * NO UPSTREAM CALL and no Host read, on purpose (web-admin's precedent): "is this process serving"
 * must not depend on service-platform being up, nor on PLATFORM_HTTP_ADDR / OPERATOR_HOST being
 * set. A probe that asked platform would restart every console pod whenever platform is slow, and a
 * misconfigured variable must surface as a 503 naming it on the first real request (`gateway.ts`),
 * not as a crash loop that hides the message.
 */
export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export function GET(): Response {
  return new Response("ok", {
    status: 200,
    headers: { "Content-Type": "text/plain; charset=utf-8", "Cache-Control": "no-store" },
  });
}
