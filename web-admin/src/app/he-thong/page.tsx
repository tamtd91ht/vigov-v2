import { CauHinhXaProvider } from "@/components/cau-hinh-xa";
import { DauTrang } from "@/components/dau-trang";
import { PageHeader } from "@/components/ui/page-header";
import { SystemTabFrame } from "@/features/cau-hinh/khung-tab-cau-hinh";
import { systemTabFromQuery } from "@/features/cau-hinh/thanh-tab-cau-hinh";
import { PhienProvider } from "@/features/phien/phien-hien-tai";
import { phanHienThi } from "@/lib/cau-hinh-xa-hien-thi";
import { communePageMetadata, layCauHinhXa } from "@/lib/tenant.server";

/**
 * `/he-thong` — the "Hệ thống" menu item (owner decision 12/10/2026, customer bug sheet S27).
 *
 * WHY THIS SCREEN EXISTS: `/cau-hinh` must show exactly the prototype's nine tabs. Three tabs it carried
 * beyond them could not simply go: "Lịch làm việc" is the only screen for working hours, holidays and
 * swapped working days — the input of every deadline (ADR 0007); "Nhật ký hệ thống" is the only reader of
 * the audit trail (ADR 0054); "Nhận diện xã" is the only place a commune sets its logo and banner
 * (ADR 0069). They live here now, in that order, with the components and gates they had
 * (`SYSTEM_TABS` in `features/cau-hinh/thanh-tab-cau-hinh.ts`). The route segment is Vietnamese like
 * every other route of this app (rule 12, invariant 2).
 *
 * `?tab=<id>` picks the starting tab — old `/cau-hinh?tab=` links are redirected here with it
 * (`features/cau-hinh/moved-tabs.ts`). An unknown value is ignored, and a tab the account may not see
 * falls back to the first visible one: the parameter steers the screen and grants nothing.
 *
 * ROUTE PROTECTION: `src/proxy.ts` sends a request with no session cookie to `/dang-nhap` before this
 * renders, and deliberately checks no permission — each route behind these tabs declares its own key and
 * the owning service checks it on every request (rule 5, forbidden #1). The menu item hiding is UX.
 */
export const dynamic = "force-dynamic";

// Tab title carries the signed-in commune, never the product name (ADR 0068 §13); a Host matching no
// commune 404s here exactly as the page body does.
export function generateMetadata() {
  return communePageMetadata("Hệ thống");
}

export default async function SystemPage({
  searchParams,
}: {
  searchParams: Promise<{ [key: string]: string | string[] | undefined }>;
}) {
  // Commune first: a Host matching no commune 404s before anything else is read.
  const commune = await layCauHinhXa();
  const initialTab = systemTabFromQuery((await searchParams).tab);

  return (
    <CauHinhXaProvider giaTri={phanHienThi(commune)}>
      <PhienProvider>
        <div className="khung-trang">
          <DauTrang />
          <main className="than-trang">
            {/* `mb-6`: the same 24px from header to tab strip as `/cau-hinh` (spec 02). */}
            <PageHeader
              className="mb-6"
              title="Hệ thống"
              subtitle="Lịch làm việc và ngày nghỉ, nhật ký hệ thống, nhận diện của xã."
            />
            {/* `min-w-0`: a wide tab scrolls inside its own region, never the page. */}
            <div className="min-w-0">
              <SystemTabFrame initialTab={initialTab} />
            </div>
          </main>
        </div>
      </PhienProvider>
    </CauHinhXaProvider>
  );
}
