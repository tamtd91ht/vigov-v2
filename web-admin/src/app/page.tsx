import { redirect } from "next/navigation";

import { homePathFor } from "@/lib/home-route";
import { readSessionForHome } from "@/lib/home-route.server";
import { layCauHinhXa } from "@/lib/tenant.server";

/**
 * `/` — no page of its own: a server-side redirect by role (`docs/ui-ux/15-phu-luc-giao-dien-chung.md:20`,
 * user decision 05/10/2026, tester report TQ-01). Leader role → `/nhiem-vu/so-tay`; any other role, no
 * role, or a session that could not be read → `/tong-quan`. The rule and why the fallback is safe:
 * `lib/home-route.ts`.
 *
 * THE COMMUNE IS RESOLVED FIRST: a `Host` matching no commune answers 404 here, exactly as on every
 * screen, before any redirect could tell a prober anything (rule 1, invariant 3).
 *
 * Protected by `src/proxy.ts` before this runs; the role read here only chooses a landing page. The two
 * targets check their own permissions on every call (rule 5, forbidden #1).
 */
export const dynamic = "force-dynamic";

export default async function TrangChu() {
  await layCauHinhXa();
  redirect(homePathFor(await readSessionForHome()));
}
