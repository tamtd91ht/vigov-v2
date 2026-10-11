// MapLibre's own stylesheet (controls, attribution, markers). A static file from the package, bundled
// by Next — no stylesheet is fetched from a third-party host.
import "maplibre-gl/dist/maplibre-gl.css";

import { CauHinhXaProvider } from "@/components/cau-hinh-xa";
import { DauTrang } from "@/components/dau-trang";
import { EconomicMapScreen } from "@/features/map-assets/economic-map-screen";
import { NO_ASSET_READ, PAGE_TITLE } from "@/features/map-assets/labels";
import { PhienProvider } from "@/features/phien/phien-hien-tai";
import { CongQuyen } from "@/features/quyen/cong-quyen";
import { phanHienThi } from "@/lib/cau-hinh-xa-hien-thi";
import { basemapConfigured } from "@/lib/may-chu/basemap";
import { ASSET_READ_PERMISSION } from "@/lib/quyen";
import { communePageMetadata, layCauHinhXa } from "@/lib/tenant.server";

/**
 * `/ban-do` — Bản đồ phát triển kinh tế số (`docs/ui-ux/10-ban-do-kinh-te-so.md`, ADR 0072).
 *
 * ROUTE PROTECTION is `src/proxy.ts` (server, before render): no session cookie → `/dang-nhap`.
 * `asset.read` is checked by service-comms on EVERY call; `CongQuyen` only spares an officer without it
 * a screen of 403s (rule 5, #1). Same shape as `/giai-ngan`.
 *
 * THE COMMUNE comes from `Host` (`layCauHinhXa`, 404 when it matches none). THE BASEMAP is the
 * self-hosted one `/phan-anh` uses (owner 11/10/2026): whether it is configured (`BASEMAP_URL`,
 * `lib/may-chu/basemap.ts`) is decided HERE, on the server, at request time, and passed down as a flag —
 * never `NEXT_PUBLIC_*`, never baked into the bundle. Unset → one sentence, no map, no fallback provider.
 */
export const dynamic = "force-dynamic";

export async function generateMetadata() {
  return communePageMetadata(PAGE_TITLE);
}

export default async function EconomicMapPage() {
  const commune = await layCauHinhXa();
  const basemapAvailable = basemapConfigured();

  return (
    <CauHinhXaProvider giaTri={phanHienThi(commune)}>
      <PhienProvider>
        <div className="khung-trang">
          <DauTrang />
          {/* `p-0`: this screen is full-bleed (spec 03 — the map runs to the column's edges). On THIS route
              only; `.than-trang`'s padding is a legacy-layer rule, so the utility wins. */}
          <main className="than-trang p-0">
            <CongQuyen khoa={ASSET_READ_PERMISSION} cauThieuQuyen={NO_ASSET_READ}>
              <EconomicMapScreen basemapAvailable={basemapAvailable} />
            </CongQuyen>
          </main>
        </div>
      </PhienProvider>
    </CauHinhXaProvider>
  );
}
