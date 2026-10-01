import { forwardToPlatform } from "@/lib/server/gateway";

/**
 * Every `/api/v1/*` call is forwarded to service-platform. The why, and what this deliberately
 * does NOT do, live in `lib/server/gateway.ts`.
 *
 * `nodejs` because the forwarder needs `node:http` (`fetch` cannot carry the operator Host).
 * `force-dynamic` because an API answer is per session: a cached one is one operator's answer
 * served to another.
 */
export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export const GET = forwardToPlatform;
export const POST = forwardToPlatform;
export const PUT = forwardToPlatform;
export const PATCH = forwardToPlatform;
export const DELETE = forwardToPlatform;
export const HEAD = forwardToPlatform;
export const OPTIONS = forwardToPlatform;
