import { NotebookTabs } from "lucide-react";

import { CauHinhXaProvider } from "@/components/cau-hinh-xa"; // vi-name-ok: existing commune-config provider
import { DauTrang } from "@/components/dau-trang"; // vi-name-ok: existing page header bar
import { PageHeader } from "@/components/ui/page-header";
import { LeaderNotebook } from "@/features/leader-notebook/leader-notebook";
import { PAGE_SUBTITLE, PAGE_TITLE } from "@/features/leader-notebook/notebook-queries";
import { PhienProvider } from "@/features/phien/phien-hien-tai"; // vi-name-ok: existing session provider
import { phanHienThi } from "@/lib/cau-hinh-xa-hien-thi"; // vi-name-ok: existing commune-config projection
import { communePageMetadata, layCauHinhXa } from "@/lib/tenant.server"; // vi-name-ok: existing tenant resolver

/**
 * `/nhiem-vu/so-tay` — Sổ tay lãnh đạo (`docs/ui-ux/03-so-tay-lanh-dao.md`, columns per ADR 0071).
 *
 * THE COMMUNE COMES FROM `Host`, server-side, like every screen: `layCauHinhXa` answers 404 for a host
 * naming no commune. Nothing commune-specific is in the bundle (rule 1, invariant 10).
 *
 * No filter, no search, no action button — on purpose (spec 03 §5). The `task.read` gate lives in
 * `LeaderNotebook`, below the title, so an account without the key still reads where it is.
 */
export const dynamic = "force-dynamic";

// Tab title carries the signed-in commune, never the product name (ADR 0068 §13); a Host matching
// no commune 404s here exactly as the page body does.
export function generateMetadata() {
  return communePageMetadata(PAGE_TITLE);
}

export default async function LeaderNotebookPage() {
  const commune = await layCauHinhXa();

  return (
    <CauHinhXaProvider giaTri={phanHienThi(commune)}>
      <PhienProvider>
        <div className="khung-trang">
          <DauTrang />
          <main className="than-trang">
            {/* The prototype's header: title + one subtitle line, no action, 24px above the columns. */}
            <PageHeader icon={NotebookTabs} title={PAGE_TITLE} subtitle={PAGE_SUBTITLE} className="mb-6" />
            <LeaderNotebook />
          </main>
        </div>
      </PhienProvider>
    </CauHinhXaProvider>
  );
}
