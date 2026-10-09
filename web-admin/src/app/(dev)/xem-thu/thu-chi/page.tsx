import { notFound } from "next/navigation";

import { BudgetPreview } from "@/dev-preview/budget-preview";
import { budgetPreviewModal } from "@/dev-preview/budget.fixture";
import { devPreviewEnabled } from "@/dev-preview/preview-gate";
import { previewFullMenu, type PreviewSearchParams } from "@/dev-preview/preview-params";
import { PreviewShell } from "@/dev-preview/preview-shell";
import { BudgetPageHeader } from "@/features/thu-chi/budget-page-header";

/**
 * `/xem-thu/thu-chi` — DEV-ONLY screenshot preview of Thu - Chi ngân sách (ADR 0068 lần 6 #10): the real
 * shell and the real board on fixture data, no backend, no login. `?modal=nap-excel` opens `Nạp từ Excel`
 * with a stand-in file; `?case=ok|closed|entries|errors` picks the preview answer (`budget.fixture.ts`).
 * In a production build this page is a 404 (`preview-gate.ts`).
 */
export const dynamic = "force-dynamic";

export const metadata = { title: "Xem thử · Thu - Chi ngân sách", robots: { index: false, follow: false } };

export default async function BudgetPreviewPage({ searchParams }: { searchParams: PreviewSearchParams }) {
  if (!devPreviewEnabled()) notFound();
  const q = await searchParams;
  return (
    // The real page's `<main className="than-trang p-5">` and header, so the screenshot is the page.
    <PreviewShell fullMenu={previewFullMenu(q.menu)} mainClassName="p-5">
      <BudgetPageHeader />
      <BudgetPreview modal={budgetPreviewModal(q.modal)} />
    </PreviewShell>
  );
}
