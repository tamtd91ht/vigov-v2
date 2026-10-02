import { CauHinhXaProvider } from "@/components/cau-hinh-xa";
import { phanHienThi } from "@/lib/cau-hinh-xa-hien-thi";
import { DauTrang } from "@/components/dau-trang";
import { ThanhBen } from "@/components/thanh-ben";
import { SoNoiDung } from "@/features/noi-dung/so-noi-dung";
import { PhienProvider } from "@/features/phien/phien-hien-tai";
import { communePageMetadata, layCauHinhXa } from "@/lib/tenant.server";

/**
 * `/noi-dung` — Quản trị nội dung Mini App (`docs/ui-ux/11-noi-dung-mini-app.md`).
 *
 * ⚠ ĐƯỜNG DẪN LÀ `/noi-dung`, ĐẶC TẢ GHI `/mini-app`. Sự lệch ấy được nói ra chứ không giấu: mục
 * menu `Nội dung Mini App` (`components/muc-menu.ts`) trỏ tới `/noi-dung`, nên đổi đường dẫn là
 * đổi cả hai chỗ cùng lúc.
 *
 * KHÔNG CÓ `<CongQuyen>` Ở ĐÂY, VÀ SỰ VẮNG MẶT ẤY LÀ MỘT QUYẾT ĐỊNH CHỨ KHÔNG PHẢI MỘT LẦN BỎ QUA.
 * Sáu tuyến đứng sau `content.read` / `content.update` thật — hai khoá ĐÃ được gieo sẵn trong bảng
 * `quyen` (`service-identity/migrations/0001_init.sql:292-293`) và tuyến khai chúng trên từng yêu
 * cầu. Hằng `QUYEN_XEM_NOI_DUNG` (`src/lib/quyen.ts`) đã có và mục menu dùng nó để ẩn mục với tài
 * khoản thiếu `content.read`; trang vẫn cố ý không bọc `<CongQuyen>`, cùng khuôn màn Thông báo.
 *
 * Lớp chặn thật không đổi: `authz.RequirePermission` ở máy chủ, trên TỪNG lời gọi (luật 5, cấm
 * #1). Inside the screen the WRITE controls are hidden without `content.update` (`canEditContent`,
 * 02/10/2026) — convenience on top of that check, never instead of it.
 *
 * TIÊU ĐỀ VÀ CÂU MÔ TẢ LẤY TỪ `nhan-noi-dung.ts` (qua `SoNoiDung`), không gõ lại ở đây: chúng là chữ NGUYÊN VĂN của
 * §1 và §3, và một bản thứ hai của một câu là một bản sẽ trôi (luật 9, cấm #2).
 */
export const dynamic = "force-dynamic";

// Tab title carries the signed-in commune, never the product name (ADR 0068 §13); a Host
// matching no commune 404s here exactly as the page body does.
export function generateMetadata() {
  return communePageMetadata("Nội dung Mini App");
}

export default async function TrangNoiDungMiniApp() {
  const xa = await layCauHinhXa();

  return (
    <CauHinhXaProvider giaTri={phanHienThi(xa)}>
      <PhienProvider>
        <div className="khung-trang">
          <ThanhBen />
          <DauTrang />
          <main className="than-trang">
            {/* The page header (`<h1>`, the §1 sentence, `+ Thêm nội dung`) is drawn by `SoNoiDung`: the
                button sits on the title row (spec §5) and its state — the open form, the
                `content.update` gate — lives in that component. Same split as `/nhiem-vu`. */}
            <SoNoiDung />
          </main>
        </div>
      </PhienProvider>
    </CauHinhXaProvider>
  );
}
