/**
 * The DEV-ONLY static preview (ADR 0068 §Sửa đổi 07/10/2026 lần 6 #10): the real shell and the real
 * Giải ngân components rendered with fixture data and the built CSS, so a screenshot can be taken
 * before each commit — no backend, no login.
 *
 * IT MUST NOT EXIST IN PRODUCTION, and two independent things make sure of it:
 *   1. every preview page calls `notFound()` when this returns false (`app/(dev)/xem-thu/**`);
 *   2. `proxy.ts` lets the preview path through without a session cookie ONLY when this returns true —
 *      in production the path is guarded like every other one, then 404s.
 *
 * `process.env.NODE_ENV` IS NOT A RUNTIME SWITCH ANYONE CAN TURN ON: Next inlines it at BUILD time, and
 * `next build` always builds with "production". There is no environment variable, flag or query that
 * re-opens the preview in a production image — the opposite of a "mock mode" (rule 8, invariant 7).
 */
export const DEV_PREVIEW_PREFIX = "/xem-thu";

export function devPreviewEnabled(): boolean {
  return process.env.NODE_ENV !== "production";
}

/** `/xem-thu` itself or anything under it — by path SEGMENT, so `/xem-thu-khac` is not the preview. */
export function isDevPreviewPath(pathname: string): boolean {
  return pathname === DEV_PREVIEW_PREFIX || pathname.startsWith(`${DEV_PREVIEW_PREFIX}/`);
}
