import { UsersRound } from "lucide-react";

import { CauHinhXaProvider } from "@/components/cau-hinh-xa";
import { DauTrang } from "@/components/dau-trang";
import { PageHeader } from "@/components/ui/page-header";
import { TabNguoiDung } from "@/features/cau-hinh/tab-nguoi-dung";
import { PhienProvider } from "@/features/phien/phien-hien-tai";
import { phanHienThi } from "@/lib/cau-hinh-xa-hien-thi";
import { communePageMetadata, layCauHinhXa } from "@/lib/tenant.server";

/**
 * `/nguoi-dung` — staff login accounts (role, lock, temporary password), until 05/10/2026 the
 * "Người dùng" tab of `/cau-hinh`. Moved to its own screen and menu item to match the prototype the
 * owner approved (`vigov-require/apps/admin`, `src/lib/navigation.ts`): adding an account is weekly
 * work and should not sit behind a tab. Route segment and title are the prototype's.
 *
 * THE COMPONENT IS THE TAB'S, UNCHANGED: `TabNguoiDung` carries the gate (`quyetDinhTabNguoiDung`,
 * `admin.user`) and its denied / unreadable-session states, so this screen treats an account without
 * the key exactly as the tab did. It mounts with `active` defaulting to `true`, so units and roles are
 * read on every arrival here (ND-01/ND-02).
 *
 * ROUTE PROTECTION: `src/proxy.ts` sends a request with no session cookie to `/dang-nhap` before this
 * renders, and deliberately checks no permission — `GET /api/v1/staff` declares `admin.user` and the
 * identity service checks it on every request (rule 5, forbidden #1).
 *
 * Not to be confused with `/danh-ba`: same table, but that screen manages CONTACT details.
 */
export const dynamic = "force-dynamic";

// Tab title carries the signed-in commune, never the product name (ADR 0068 §13); a Host matching
// no commune 404s here exactly as the page body does.
export function generateMetadata() {
  return communePageMetadata("Người dùng");
}

export default async function UsersPage() {
  const xa = await layCauHinhXa();

  return (
    <CauHinhXaProvider giaTri={phanHienThi(xa)}>
      <PhienProvider>
        <div className="khung-trang">
          <DauTrang />
          <main className="than-trang">
            <PageHeader
              icon={UsersRound}
              title="Người dùng"
              subtitle="Tài khoản cán bộ của đơn vị: bộ phận công tác, vai trò được gán và trạng thái hoạt động."
            />
            {/* `[&>section]:mt-0`: the section kept the 2.5rem top margin it had as one of several
                stacked parts of `/cau-hinh`; under a page header it is a gap with no reason. */}
            <div className="flex min-w-0 flex-col gap-4 [&>section]:mt-0">
              <TabNguoiDung />
            </div>
          </main>
        </div>
      </PhienProvider>
    </CauHinhXaProvider>
  );
}
