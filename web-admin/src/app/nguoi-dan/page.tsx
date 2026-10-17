import { CauHinhXaProvider } from "@/components/cau-hinh-xa";
import { phanHienThi } from "@/lib/cau-hinh-xa-hien-thi";
import { DauTrang } from "@/components/dau-trang";
import { CitizenDirectoryScreen } from "@/features/citizen-directory/citizen-directory-screen";
import { PhienProvider } from "@/features/phien/phien-hien-tai";
import { parseOpenCreate } from "@/lib/create-link";
import type { RawSearchParams } from "@/lib/drill-down";
import { communePageMetadata, layCauHinhXa } from "@/lib/tenant.server";

/**
 * `/nguoi-dan` — Danh bạ người dân (`docs/ui-ux/16-danh-ba-nguoi-dan.md`, ADR 0097), the route the
 * chapter names at its head. The menu item left `PENDING_SCREENS` with this page (16/10/2026).
 *
 * THE GATE IS IN `CitizenDirectoryScreen` (`citizen.read` → `CongQuyen`), not here: the screen draws
 * its own title in the refused state. Route protection lives in `src/proxy.ts`, server-side, before this
 * page is built — no session cookie, no page. Neither checks a permission: `service-identity` checks
 * `citizen.read` / `citizen.update` on every one of the sixteen directory routes (rule 5, forbidden #1).
 *
 * XÃ ĐỌC LÚC CHẠY TỪ `Host`, như mọi trang khác: không có giá trị riêng của xã nào nằm trong bundle, và
 * `Host` không khớp xã nào thì trang này là 404 trước khi dựng gì (luật 1, bất biến 3 và 10).
 */
export const dynamic = "force-dynamic";

// Tab title carries the signed-in commune, never the product name (ADR 0068 §13); a Host
// matching no commune 404s here exactly as the page body does.
export function generateMetadata() {
  return communePageMetadata("Danh bạ người dân");
}

export default async function CitizenDirectoryPage({
  searchParams,
}: {
  searchParams: Promise<RawSearchParams>;
}) {
  const xa = await layCauHinhXa(); // vi-name-ok: existing export of tenant.server.ts, the shell every page copies
  // `?create=1` — the Hướng dẫn sử dụng link to `Thêm người dân`, read here and passed down.
  const openCreateOnLoad = parseOpenCreate(await searchParams);

  return (
    <CauHinhXaProvider giaTri={phanHienThi(xa)}>
      <PhienProvider>
        <div className="khung-trang">
          <DauTrang />
          <main className="than-trang">
            <CitizenDirectoryScreen openCreateOnLoad={openCreateOnLoad} />
          </main>
        </div>
      </PhienProvider>
    </CauHinhXaProvider>
  );
}
