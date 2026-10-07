import { notFound } from "next/navigation";

import { DisbursementPreview } from "@/dev-preview/disbursement-preview";
import { devPreviewEnabled } from "@/dev-preview/preview-gate";
import { previewFullMenu, previewModal, type PreviewSearchParams } from "@/dev-preview/preview-params";
import { PreviewShell } from "@/dev-preview/preview-shell";

/**
 * `/xem-thu/giai-ngan` — DEV-ONLY screenshot preview of Theo dõi giải ngân (ADR 0068 lần 6 #10):
 * the real shell and the real list screen on fixture data, no backend, no login.
 * `?modal=hang-muc|them-du-an|nhap-excel|them-nguon|chi-tiet-nguon|xoa-nhieu` opens one dialog;
 * `?menu=day-du` shows the full menu. In a production build this page is a 404 (`preview-gate.ts`).
 */
export const dynamic = "force-dynamic";

export const metadata = { title: "Xem thử · Theo dõi giải ngân", robots: { index: false, follow: false } };

export default async function DisbursementPreviewPage({ searchParams }: { searchParams: PreviewSearchParams }) {
  if (!devPreviewEnabled()) notFound();
  const q = await searchParams;
  return (
    <PreviewShell fullMenu={previewFullMenu(q.menu)}>
      <DisbursementPreview modal={previewModal(q.modal)} />
    </PreviewShell>
  );
}
