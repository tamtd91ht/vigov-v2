// MapLibre's own stylesheet (controls, attribution, markers). A static file from the package, bundled
// by Next — no stylesheet is fetched from a third-party host.
import "maplibre-gl/dist/maplibre-gl.css";

import { CauHinhXaProvider } from "@/components/cau-hinh-xa";
import { DauTrang } from "@/components/dau-trang";
import { ThanhBen } from "@/components/thanh-ben";
import { EconomicMapScreen } from "@/features/map-assets/economic-map-screen";
import { NO_ASSET_READ, PAGE_TITLE } from "@/features/map-assets/labels";
import { PhienProvider } from "@/features/phien/phien-hien-tai";
import { CongQuyen } from "@/features/quyen/cong-quyen";
import { phanHienThi } from "@/lib/cau-hinh-xa-hien-thi";
import { mapStyleUrl } from "@/lib/may-chu/map-style";
import { ASSET_READ_PERMISSION } from "@/lib/quyen";
import { communePageMetadata, layCauHinhXa } from "@/lib/tenant.server";

/**
 * `/ban-do` — Bản đồ phát triển kinh tế số (`docs/ui-ux/10-ban-do-kinh-te-so.md`, ADR 0072).
 *
 * ROUTE PROTECTION is `src/proxy.ts` (server, before render): no session cookie → `/dang-nhap`.
 * `asset.read` is checked by service-comms on EVERY call; `CongQuyen` only spares an officer without it
 * a screen of 403s (rule 5, #1). Same shape as `/giai-ngan`.
 *
 * THE COMMUNE comes from `Host` (`layCauHinhXa`, 404 when it matches none). THE BASEMAP URL is a
 * platform constant read HERE, on the server, at request time (`MAP_STYLE_URL`, `lib/may-chu/map-style.ts`)
 * and passed down as a prop — never `NEXT_PUBLIC_*`, never baked into the bundle (ADR 0072 H1).
 */
export const dynamic = "force-dynamic";

export async function generateMetadata() {
  const base = await communePageMetadata(PAGE_TITLE);
  // Tile, glyph and sprite requests go to the basemap host. Send it the ORIGIN at most, never this
  // page's path (security review 04/10/2026, #3). Scoped to this page; other pages keep the default.
  return { ...base, referrer: "strict-origin-when-cross-origin" as const };
}

export default async function EconomicMapPage() {
  const commune = await layCauHinhXa();
  const styleUrl = mapStyleUrl();

  return (
    <CauHinhXaProvider giaTri={phanHienThi(commune)}>
      <PhienProvider>
        <div className="khung-trang">
          <ThanhBen />
          <DauTrang />
          <main className="than-trang">
            <CongQuyen khoa={ASSET_READ_PERMISSION} cauThieuQuyen={NO_ASSET_READ}>
              <EconomicMapScreen styleUrl={styleUrl} />
            </CongQuyen>
          </main>
        </div>
      </PhienProvider>
    </CauHinhXaProvider>
  );
}
