import { CauHinhXaProvider } from "@/components/cau-hinh-xa";
import { DauTrang } from "@/components/dau-trang";
import { DashboardPage } from "@/features/dashboard/overview";
import { PhienProvider } from "@/features/phien/phien-hien-tai";
import { phanHienThi } from "@/lib/cau-hinh-xa-hien-thi";
import { communePageMetadata, layCauHinhXa } from "@/lib/tenant.server";

/**
 * `/tong-quan` — Tổng quan điều hành (`docs/ui-ux/01-tong-quan-dieu-hanh.md`).
 *
 * THE COMMUNE COMES FROM `Host`, server-side, like every other screen: `layCauHinhXa` answers 404
 * for a host that names no commune, and nothing commune-specific is in the bundle (rule 1 inv. 10).
 *
 * FIGURES ARE READ IN THE BROWSER, live, from the services that own them — the user decided against
 * a snapshot. No department filter, on purpose: the page is always the whole commune (spec §9).
 *
 * THE GATE IS `report.read` (`<CongQuyen>` inside `DashboardPage`), and each block additionally
 * needs its module's read key. Both are convenience: every route checks both keys server-side.
 */
export const dynamic = "force-dynamic";

// Tab title carries the signed-in commune, never the product name (ADR 0068 §13); a Host
// matching no commune 404s here exactly as the page body does.
export function generateMetadata() {
  return communePageMetadata("Tổng quan điều hành");
}

export default async function OverviewPage() {
  const commune = await layCauHinhXa();

  return (
    <CauHinhXaProvider giaTri={phanHienThi(commune)}>
      <PhienProvider>
        <div className="khung-trang">
          <DauTrang />
          <main className="than-trang">
            {/* Draws its own header: the period line and buttons need the figures' state, and the
                title stays visible outside the `report.read` gate (`DashboardPage`). */}
            <DashboardPage />
          </main>
        </div>
      </PhienProvider>
    </CauHinhXaProvider>
  );
}
