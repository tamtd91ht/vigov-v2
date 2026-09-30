import { chuyenTiep } from "@/lib/may-chu/chuyen-tiep";

/**
 * Every `/api/v1/*` call on a commune's host is forwarded to the Go service that owns it.
 * The why, and what this deliberately does NOT do, live in `lib/may-chu/chuyen-tiep.ts`.
 *
 * `nodejs` because the forwarder needs `node:http` — `fetch` cannot carry the commune's Host
 * (`lib/may-chu/goi-noi-bo.ts`). `force-dynamic` because an API answer is per commune and per
 * session: a cached response is one commune's data served under another's Host.
 */
export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export const GET = chuyenTiep;
export const POST = chuyenTiep;
export const PUT = chuyenTiep;
export const PATCH = chuyenTiep;
export const DELETE = chuyenTiep;
export const HEAD = chuyenTiep;
export const OPTIONS = chuyenTiep;
