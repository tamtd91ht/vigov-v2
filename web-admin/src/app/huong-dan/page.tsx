import { CauHinhXaProvider } from "@/components/cau-hinh-xa";
import { DauTrang } from "@/components/dau-trang";
import { PhienProvider } from "@/features/phien/phien-hien-tai";
import { GUIDE_TITLE } from "@/features/user-guide/guide-content";
import { UserGuide } from "@/features/user-guide/user-guide";
import { phanHienThi } from "@/lib/cau-hinh-xa-hien-thi";
import { communePageMetadata, layCauHinhXa } from "@/lib/tenant.server";

/**
 * `/huong-dan` — "Hướng dẫn sử dụng" (owner 17/10/2026).
 *
 * NO PERMISSION GATE, ON PURPOSE: the page is open to every signed-in staff member (owner decision), and
 * it reads no business data — what it shows is filtered by the session's own permissions in the client
 * (`features/user-guide/guide-visibility.ts`), which is convenience: every screen it links to checks
 * again, and so does the server. Protected by `src/proxy.ts` like every non-public path.
 *
 * `force-dynamic`: the commune's name in the cover comes from `Host` at runtime (rule 1, invariant 10);
 * a static page would carry one commune's name to another.
 */
export const dynamic = "force-dynamic";

// Tab title carries the signed-in commune (ADR 0068 §13); a Host matching no commune 404s.
export function generateMetadata() {
  return communePageMetadata(GUIDE_TITLE);
}

export default async function UserGuidePage() {
  const commune = await layCauHinhXa();

  return (
    <CauHinhXaProvider giaTri={phanHienThi(commune)}>
      <PhienProvider>
        <div className="khung-trang">
          <DauTrang />
          <main className="than-trang">
            <UserGuide />
          </main>
        </div>
      </PhienProvider>
    </CauHinhXaProvider>
  );
}
