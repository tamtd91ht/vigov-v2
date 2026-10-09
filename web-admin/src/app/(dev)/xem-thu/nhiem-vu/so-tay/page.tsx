import { notFound } from "next/navigation";

import { NotebookPreview } from "@/dev-preview/notebook-preview";
import { devPreviewEnabled } from "@/dev-preview/preview-gate";
import { previewFullMenu, type PreviewSearchParams } from "@/dev-preview/preview-params";
import { PreviewShell } from "@/dev-preview/preview-shell";
import { PREVIEW_TASK_PERMISSIONS } from "@/dev-preview/shell.fixture";

/**
 * `/xem-thu/nhiem-vu/so-tay` — DEV-ONLY screenshot preview of the Sổ tay lãnh đạo (ADR 0068 lần 6 #10):
 * the real shell and the real `LeaderNotebook` on fixture data, no backend, no login. The path mirrors
 * the real `/nhiem-vu/so-tay`, so the menu lights the item the real page lights.
 *   `?mo=NV105`   that task's row pressed — its detail drawer open in place
 *   `?menu=day-du`, `?sidebar=thu-gon`, `?toast=1`, `?menu-tai-khoan=1`, `?chuong=1`  as every preview
 * In a production build this page is a 404 (`preview-gate.ts`).
 */
export const dynamic = "force-dynamic";

export const metadata = { title: "Xem thử · Sổ tay lãnh đạo", robots: { index: false, follow: false } };

/** Longest code accepted from `?mo=`; longer is a broken link, not a code. */
const MAX_CODE_LENGTH = 100;

export default async function NotebookPreviewPage({ searchParams }: { searchParams: PreviewSearchParams }) {
  if (!devPreviewEnabled()) notFound();
  const q = await searchParams;
  const open = typeof q.mo === "string" && q.mo.trim() !== "" && q.mo.length <= MAX_CODE_LENGTH ? q.mo.trim() : null;
  return (
    <PreviewShell fullMenu={previewFullMenu(q.menu)} permissions={PREVIEW_TASK_PERMISSIONS} person="lanh-dao">
      <NotebookPreview open={open} />
    </PreviewShell>
  );
}
