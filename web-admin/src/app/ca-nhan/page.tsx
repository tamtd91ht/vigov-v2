import { UserRound } from "lucide-react";

import { CauHinhXaProvider } from "@/components/cau-hinh-xa";
import { DauTrang } from "@/components/dau-trang";
import { PageHeader } from "@/components/ui/page-header";
import { PhienProvider } from "@/features/phien/phien-hien-tai";
import { PersonalZaloCard } from "@/features/zalo/personal-zalo-card";
import { phanHienThi } from "@/lib/cau-hinh-xa-hien-thi";
import { communePageMetadata, layCauHinhXa } from "@/lib/tenant.server";

/**
 * `/ca-nhan` — the signed-in staff member's own settings. Today one card: "Nhận nhắc việc qua Zalo"
 * (ADR 0074 #6 — `AnyAuthenticated`, filtered by the session's staff code server-side).
 *
 * NO PERMISSION GATE, ON PURPOSE: every route behind it is the person's own (`zalo-links/current`),
 * open to any signed-in account; a gate here would refuse what the server does not (rule 5 forbidden
 * #1). The page is protected by `src/proxy.ts` like every non-public path — no session, no page.
 *
 * `dynamic = "force-dynamic"`: the page depends on `Host`; a static page carrying commune A's name
 * served for commune B is the shape of a leak between two authorities.
 */
export const dynamic = "force-dynamic";

export function generateMetadata() {
  return communePageMetadata("Cá nhân");
}

export default async function PersonalPage() {
  const commune = await layCauHinhXa();

  return (
    <CauHinhXaProvider giaTri={phanHienThi(commune)}>
      <PhienProvider>
        <div className="khung-trang">
          <DauTrang />
          <main className="than-trang">
            <div className="page--form mx-auto w-full">
              <PageHeader icon={UserRound} title="Cá nhân" subtitle="Cài đặt của riêng tài khoản đang đăng nhập." />
              <PersonalZaloCard />
            </div>
          </main>
        </div>
      </PhienProvider>
    </CauHinhXaProvider>
  );
}
