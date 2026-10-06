import { redirect } from "next/navigation";

import { LEGACY_DIRECTORY_REDIRECT } from "@/features/mini-app/mini-app-tabs";
import { layCauHinhXa } from "@/lib/tenant.server";

/**
 * `/danh-ba` — moved: the staff directory is now the `Danh bạ cán bộ` tab of `/mini-app` (ADR 0068 lần 5,
 * 06/10/2026, as the prototype's `(workspace)/danh-ba/page.tsx` does). Kept, not deleted: the path sits in
 * officers' bookmarks and in messages they sent each other.
 *
 * COMMUNE FIRST: a `Host` matching no commune 404s here, before any redirect is issued — a redirect would
 * otherwise confirm the route exists on a domain that is no commune (rule 1, invariant 3). Same order as
 * `app/cau-hinh/page.tsx`.
 *
 * NO QUERY IS CARRIED OVER (`LEGACY_DIRECTORY_REDIRECT`): the directory's search text never lives in a URL.
 */
export const dynamic = "force-dynamic";

export default async function LegacyDirectoryPage() {
  await layCauHinhXa();
  redirect(LEGACY_DIRECTORY_REDIRECT);
}
