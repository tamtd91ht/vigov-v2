import { proxyBasemap } from "@/lib/may-chu/basemap";

/**
 * `/basemap/*` — the petition maps' self-hosted basemap (archive, glyphs, sprites), streamed from
 * ViGov's object storage with HTTP Range. The why, and what this deliberately does NOT do, live in
 * `lib/may-chu/basemap.ts`; the allow-list of files in `lib/basemap/assets.ts`.
 *
 * `nodejs` because the proxy streams a `node:https` response. `force-dynamic` because every answer
 * depends on the request's Range and on `BASEMAP_URL` read at request time — a build-time render would
 * freeze whatever the build machine had (rule 1 invariant 10, rule 8 invariant 5).
 */
export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export function GET(request: Request): Promise<Response> {
  return proxyBasemap(request);
}

export function HEAD(request: Request): Promise<Response> {
  return proxyBasemap(request);
}
