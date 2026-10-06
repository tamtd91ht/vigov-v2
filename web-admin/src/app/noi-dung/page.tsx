import { redirect } from "next/navigation";

import { legacyContentRedirect } from "@/features/mini-app/mini-app-tabs";
import { layCauHinhXa } from "@/lib/tenant.server";

/**
 * `/noi-dung` — moved: the content book is now the `Nội dung` tab of `/mini-app`, the path spec
 * `docs/ui-ux/11-noi-dung-mini-app.md` always named (ADR 0068 lần 5, 06/10/2026). Kept as a redirect: the
 * path sits in bookmarks and messages.
 *
 * COMMUNE FIRST: a `Host` matching no commune 404s here, before any redirect is issued (rule 1, invariant
 * 3) — same order as `app/cau-hinh/page.tsx`. Query parameters other than `tab` are carried over
 * (`legacyContentRedirect`).
 */
export const dynamic = "force-dynamic";

export default async function LegacyContentPage({
  searchParams,
}: {
  searchParams: Promise<{ [key: string]: string | string[] | undefined }>;
}) {
  await layCauHinhXa();
  redirect(legacyContentRedirect(await searchParams));
}
