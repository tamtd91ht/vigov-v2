import { CauHinhXaProvider } from "@/components/cau-hinh-xa";
import { DauTrang } from "@/components/dau-trang";
import { PageHeader } from "@/components/ui/page-header";
import { TabPhanQuyen } from "@/features/cau-hinh/tab-phan-quyen";
import { PhienProvider } from "@/features/phien/phien-hien-tai";
import { phanHienThi } from "@/lib/cau-hinh-xa-hien-thi";
import { communePageMetadata, layCauHinhXa } from "@/lib/tenant.server";

/**
 * `/nguoi-dung/phan-quyen` — the role × permission matrix, until 05/10/2026 the "Phân quyền" tab of
 * `/cau-hinh`. Moved with "Người dùng" to match the prototype the owner approved; see
 * `app/nguoi-dung/page.tsx` for why.
 *
 * THE COMPONENT IS THE TAB'S, UNCHANGED: `TabPhanQuyen` carries the gate (`quyetDinhTabPhanQuyen`,
 * `admin.role`), the edit decision and the denied / unreadable-session states. The server checks
 * `admin.role` on `GET /api/v1/role-permissions` and on every save (rule 5, forbidden #1);
 * `src/proxy.ts` only turns away a request with no session cookie.
 */
export const dynamic = "force-dynamic";

// Tab title carries the signed-in commune, never the product name (ADR 0068 §13).
export function generateMetadata() {
  return communePageMetadata("Phân quyền");
}

export default async function RolePermissionsPage() {
  const xa = await layCauHinhXa();

  return (
    <CauHinhXaProvider giaTri={phanHienThi(xa)}>
      <PhienProvider>
        <div className="khung-trang">
          <DauTrang />
          <main className="than-trang">
            {/* The prototype's header (`AccountWorkspace.tsx` `RoleWorkspace`): no button, 20px to the
                body. Subtitle VERBATIM by owner decision 08/10/2026, knowing the server applies a
                change on the next request rather than the next login. */}
            <PageHeader
              className="mb-5"
              title="Phân quyền"
              subtitle="Mỗi vai trò làm được những gì. Đổi ở đây là đổi cho mọi cán bộ đang giữ vai trò đó, ngay lần đăng nhập sau của họ."
            />
            {/* `min-w-0`: the matrix scrolls inside its own region, never the page. */}
            <div className="min-w-0">
              <TabPhanQuyen />
            </div>
          </main>
        </div>
      </PhienProvider>
    </CauHinhXaProvider>
  );
}
