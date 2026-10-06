import { CauHinhXaProvider } from "@/components/cau-hinh-xa";
import { DauTrang } from "@/components/dau-trang";
import { StaffDirectoryTab } from "@/features/danh-ba/staff-directory-tab";
import { MiniAppWorkspace } from "@/features/mini-app/mini-app-workspace";
import { requestedTab } from "@/features/mini-app/mini-app-tabs";
import { SoNoiDung } from "@/features/noi-dung/so-noi-dung";
import { PhienProvider } from "@/features/phien/phien-hien-tai";
import { phanHienThi } from "@/lib/cau-hinh-xa-hien-thi";
import { communePageMetadata, layCauHinhXa } from "@/lib/tenant.server";

/**
 * `/mini-app` — Quản trị nội dung Mini App: two tabs, `Nội dung` and `Danh bạ cán bộ`, the open one in
 * `?tab=` — the prototype's screen (`vigov-require` `apps/admin/src/app/(workspace)/mini-app/page.tsx`,
 * ADR 0068 lần 5, 06/10/2026). Spec `docs/ui-ux/11-noi-dung-mini-app.md` already named this path.
 *
 * `/noi-dung` and `/danh-ba` are now redirects to here (`app/noi-dung/page.tsx`, `app/danh-ba/page.tsx`).
 *
 * THE PAGE IMPORTS BOTH SCREENS FROM THEIR OWN `features/` FOLDERS, on purpose: `tools/tien_do_san_pham.py`
 * reads a page's `@/features/<x>/` imports to count the screen's "?" parts (`PHAN_CHUA_DUNG`). Routing the
 * two screens through `features/mini-app/` alone would make both lists vanish from the progress table.
 *
 * GATES, ONE PER TAB, EACH THE KEY ITS READ ROUTES DECLARE (`features/mini-app/mini-app-tabs.ts`):
 *
 *   | Tab            | Tab shown with  | Inside the tab                                               |
 *   |----------------|-----------------|--------------------------------------------------------------|
 *   | Nội dung       | `content.read`  | write controls only with `content.update` (`canEditContent`) |
 *   | Danh bạ cán bộ | `admin.user`    | `CongQuyen admin.user`; Mini App buttons `content.update`;   |
 *   |                |                 | 🗑 `admin.user.delete`                                        |
 *
 * The directory stays on `admin.user`, NOT `content.read`/`content.update` as spec §9.4 of chapter 12
 * suggested: `GET /api/v1/staff` and `PATCH /api/v1/staff/{id}` declare `admin.user` on the server
 * (`x-vigov-permission` in `kb/20-contracts/openapi.json`). The contract wins (rule 2, invariant 7).
 *
 * ROUTE PROTECTION: `src/proxy.ts` sends a request without a session cookie to `/dang-nhap` before this
 * renders. It checks no permission — every route behind both tabs does, on every call (rule 5,
 * forbidden #1). Hiding a tab is convenience.
 *
 * THE COMMUNE IS READ AT RUNTIME FROM `Host` (`layCauHinhXa`, 404 when it matches none) — never a value
 * in the bundle (rule 1, invariant 10).
 */
export const dynamic = "force-dynamic";

// Tab title carries the signed-in commune, never the product name (ADR 0068 §13); a Host
// matching no commune 404s here exactly as the page body does.
export function generateMetadata() {
  return communePageMetadata("Nội dung Mini App");
}

export default async function MiniAppPage({
  searchParams,
}: {
  searchParams: Promise<{ [key: string]: string | string[] | undefined }>;
}) {
  const commune = await layCauHinhXa();
  const tab = requestedTab((await searchParams).tab);

  return (
    <CauHinhXaProvider giaTri={phanHienThi(commune)}>
      <PhienProvider>
        <div className="khung-trang">
          <DauTrang />
          <main className="than-trang">
            {/* `key`: switching tab is a fresh screen, as the prototype's `key={tab}` — no filter or open
                dialog of one tab survives into the other. */}
            <MiniAppWorkspace
              key={tab}
              requested={tab}
              content={<SoNoiDung />}
              directory={<StaffDirectoryTab />}
            />
          </main>
        </div>
      </PhienProvider>
    </CauHinhXaProvider>
  );
}
