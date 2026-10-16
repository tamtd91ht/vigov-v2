import { CauHinhXaProvider } from "@/components/cau-hinh-xa";
import { DauTrang } from "@/components/dau-trang";
import { NoticeSendScreen } from "@/features/notice-send/notice-send-screen";
import { PhienProvider } from "@/features/phien/phien-hien-tai";
import { phanHienThi } from "@/lib/cau-hinh-xa-hien-thi";
import { communePageMetadata, layCauHinhXa } from "@/lib/tenant.server";

/**
 * `/gui-thong-bao` — Gửi tin ZNS / SMS (prototype `NoticeWorkspace`, owner 17/10/2026). The menu item left
 * `PENDING_SCREENS` with this page; the parts the server cannot serve yet (preview, send, history, SMS)
 * are drawn inside the screen as "?" (ADR 0068 §14).
 *
 * THE GATE IS IN `NoticeSendScreen` (`notice.send` → `CongQuyen`), as on `/nguoi-dan`. Route protection is
 * `src/proxy.ts`, server-side: no session cookie, no page. Every route the screen calls checks its own
 * key on the server (rule 5, forbidden #1).
 *
 * The commune is read at runtime from `Host`; a `Host` matching no commune is a 404 before anything is
 * built (rule 1, invariants 3 and 10).
 */
export const dynamic = "force-dynamic";

export function generateMetadata() {
  return communePageMetadata("Gửi tin ZNS / SMS");
}

export default async function NoticeSendPage() {
  const xa = await layCauHinhXa(); // vi-name-ok: existing export of tenant.server.ts, the shell every page copies

  return (
    <CauHinhXaProvider giaTri={phanHienThi(xa)}>
      <PhienProvider>
        <div className="khung-trang">
          <DauTrang />
          <main className="than-trang">
            <NoticeSendScreen />
          </main>
        </div>
      </PhienProvider>
    </CauHinhXaProvider>
  );
}
