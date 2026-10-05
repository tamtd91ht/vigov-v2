import { ChartColumn } from "lucide-react";

import { CauHinhXaProvider } from "@/components/cau-hinh-xa";
import { PageHeader } from "@/components/ui/page-header";
import { DauTrang } from "@/components/dau-trang";
import { PhienProvider } from "@/features/phien/phien-hien-tai";
import { ReportHeaderActions } from "@/features/report/header-actions";
import { ReportPage } from "@/features/report/report-overview";
import { phanHienThi } from "@/lib/cau-hinh-xa-hien-thi";
import { communePageMetadata, layCauHinhXa } from "@/lib/tenant.server";

/**
 * `/bao-cao` — Báo cáo điều hành (`docs/ui-ux/13-bao-cao.md`, as amended by ADR 0053 04/10/2026).
 *
 * THE COMMUNE COMES FROM `Host`, server-side, like every other screen: `layCauHinhXa` answers 404
 * for a host that names no commune, and nothing commune-specific is in the bundle (rule 1 inv. 10).
 *
 * FIGURES ARE COUNTED LIVE AT THE OWNING SERVICES, with `/tong-quan`'s own loader and blocks — never
 * through `service-reporting` (B1): two sources for one figure would drift, and the spec forbids the
 * two pages disagreeing (§10).
 *
 * THE GATE IS `report.read` (`<CongQuyen>` inside `ReportPage`); each block, the unit table and the
 * export row add their own key. All of it is convenience: every route checks its keys server-side.
 */
export const dynamic = "force-dynamic";

// Tab title carries the signed-in commune, never the product name (ADR 0068 §13); a Host
// matching no commune 404s here exactly as the page body does.
export function generateMetadata() {
  return communePageMetadata("Báo cáo điều hành");
}

export default async function ReportRoutePage() {
  const commune = await layCauHinhXa();

  return (
    <CauHinhXaProvider giaTri={phanHienThi(commune)}>
      <PhienProvider>
        <div className="khung-trang">
          <DauTrang />
          <main className="than-trang">
            {/* Outside the `report.read` gate: an account without the key still reads the title. */}
            <PageHeader icon={ChartColumn} title="Báo cáo điều hành" actions={<ReportHeaderActions />} />
            <ReportPage />
          </main>
        </div>
      </PhienProvider>
    </CauHinhXaProvider>
  );
}
