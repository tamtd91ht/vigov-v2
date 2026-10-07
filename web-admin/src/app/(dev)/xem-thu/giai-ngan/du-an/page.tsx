import { notFound } from "next/navigation";

import { ProjectDetailPreview } from "@/dev-preview/disbursement-preview";
import { devPreviewEnabled } from "@/dev-preview/preview-gate";
import { previewFullMenu, previewTab, type PreviewSearchParams } from "@/dev-preview/preview-params";
import { PreviewShell } from "@/dev-preview/preview-shell";

/**
 * `/xem-thu/giai-ngan/du-an` — DEV-ONLY screenshot preview of Chi tiết dự án (ADR 0068 lần 6 #10): the
 * real shell and the real detail page on the fixture project. `?tab=vuong-mac|chung-tu|bieu-do|trao-doi`
 * opens one tab; `?menu=day-du` shows the full menu. In a production build this page is a 404.
 */
export const dynamic = "force-dynamic";

export const metadata = { title: "Xem thử · Chi tiết dự án", robots: { index: false, follow: false } };

export default async function ProjectDetailPreviewPage({ searchParams }: { searchParams: PreviewSearchParams }) {
  if (!devPreviewEnabled()) notFound();
  const q = await searchParams;
  return (
    <PreviewShell fullMenu={previewFullMenu(q.menu)}>
      <ProjectDetailPreview tab={previewTab(q.tab)} />
    </PreviewShell>
  );
}
