import { CauHinhXaProvider } from "@/components/cau-hinh-xa";
import { phanHienThi } from "@/lib/cau-hinh-xa-hien-thi";
import { DauTrang } from "@/components/dau-trang";
import { ThanhBen } from "@/components/thanh-ben";
import { SoBienBan } from "@/features/bien-ban/so-bien-ban";
import { PhienProvider } from "@/features/phien/phien-hien-tai";
import { communePageMetadata, layCauHinhXa } from "@/lib/tenant.server";

/**
 * `/nhiem-vu/bien-ban` — Biên bản và kết luận họp (`docs/ui-ux/04-bien-ban-hop.md`).
 *
 * ĐƯỜNG DẪN ĐÚNG NHƯ ĐẶC TẢ GHI Ở ĐẦU CHƯƠNG, và nó nằm TRONG module Nhiệm vụ có lý do: mọi thứ
 * màn này hiện hoặc là nguồn gốc của một nhiệm vụ, hoặc là một con số đếm nhiệm vụ. Máy chủ khai
 * đúng như vậy — mọi tuyến đứng sau `task.read` / `task.create`, riêng ký biên bản sau
 * `task.approve`, không sau một khoá `meeting.*` nào (bảng `quyen` không có khoá ấy; xem
 * `service-petitions/internal/http/bien_ban_hop.go`).
 *
 * TRANG NÀY KHÔNG TẠO `src/app/nhiem-vu/page.tsx`. Màn `/nhiem-vu` là việc của lượt khác đang
 * chạy song song; một đoạn đường dẫn không có `page.tsx` là hợp lệ trong Next.js và không hứa hẹn
 * gì với ai.
 *
 * KHÔNG CÓ CỔNG QUYỀN Ở ĐÂY, đúng khuôn `/van-ban` đang dùng: `src/proxy.ts` chặn người CHƯA ĐĂNG
 * NHẬP ở phía máy chủ, còn `task.*` do dịch vụ `petitions` kiểm trên TỪNG lời gọi API thật (luật 5,
 * cấm #1). Trong thân màn chỉ nút Ký ẩn theo `task.approve` — tiện dụng, xem chú thích đầu
 * `features/bien-ban/so-bien-ban.tsx`.
 *
 * XÃ ĐỌC LÚC CHẠY TỪ `Host`, như mọi trang khác: không có giá trị riêng của xã nào nằm trong
 * bundle, và `Host` không khớp xã nào thì trang này là 404 trước khi dựng gì (luật 1, bất biến 3
 * và 10).
 */
export const dynamic = "force-dynamic";

// Tab title carries the signed-in commune, never the product name (ADR 0068 §13); a Host
// matching no commune 404s here exactly as the page body does.
export function generateMetadata() {
  return communePageMetadata("Biên bản và kết luận họp");
}

export default async function TrangBienBanHop() {
  const xa = await layCauHinhXa();

  return (
    <CauHinhXaProvider giaTri={phanHienThi(xa)}>
      <PhienProvider>
        <div className="khung-trang">
          <ThanhBen />
          <DauTrang />
          <main className="than-trang">
            {/* The page header (`<h1>` = `TIEU_DE_MAN`, subtitle = `MO_TA_MAN`, the `Nhập biên bản`
                button) is drawn by `SoBienBan`: the button sits on the title row (spec §5) and the
                state it toggles — which form is open — lives in that component. */}
            <SoBienBan />
          </main>
        </div>
      </PhienProvider>
    </CauHinhXaProvider>
  );
}
