import { CauHinhXaProvider } from "@/components/cau-hinh-xa";
import { phanHienThi } from "@/lib/cau-hinh-xa-hien-thi";
import { DauTrang } from "@/components/dau-trang";
import { PhienProvider } from "@/features/phien/phien-hien-tai";
import { DocumentWorkspace } from "@/features/van-ban/document-workspace";
import { drillDownKey, parseDrillDown, type RawSearchParams } from "@/lib/drill-down";
import { communePageMetadata, layCauHinhXa } from "@/lib/tenant.server";

/**
 * `/van-ban` — Văn bản & Đơn thư (`docs/ui-ux/05-van-ban-don-thu.md`).
 *
 * ĐƯỜNG DẪN ĐÚNG NHƯ ĐẶC TẢ GHI Ở ĐẦU CHƯƠNG (`Route: /van-ban`), và **không thêm đoạn nào**. Sổ
 * văn bản đi KHÔNG có đường dẫn riêng `/van-ban/di`, có chủ ý: tên tài nguyên URL cho một khái
 * niệm chưa có trong bảng ánh xạ là câu phải HỎI, không phải câu để đoán (ADR 0011) — và một đoạn
 * đường dẫn đã chạy thật ở một xã thì không có lần sửa nào rẻ nữa. Tab đang chọn vì thế cũng KHÔNG
 * lên đường dẫn.
 *
 * TAB BAR — ADR 0068 lần 5 (06/10/2026): the prototype's frame (`DocumentWorkspace.tsx`). Four tabs:
 * `Văn bản đến` · `Văn bản đi` (the two working registers, one tab each, as the prototype gives each
 * register its tab — this replaces the earlier "two registers one after the other in one tab") ·
 * `Đơn thư công dân` · `Báo cáo` (the prototype's layout as disabled "?" controls, ADR 0068 §14: no
 * service holds a `don_thu` register yet). Both registers read under the same `document.read`, so an
 * account sees both tabs or neither works. The header, the tab state and each tab's header buttons
 * live in `features/van-ban/document-workspace.tsx`; the reasons behind each "?" in `PHAN_CHUA_DUNG`
 * (`features/van-ban/nhan-van-ban.ts`), not copied here.
 *
 * BẢO VỆ ĐƯỜNG NẰM Ở `src/proxy.ts`, phía máy chủ, trước khi trang này được dựng — chưa có cookie
 * phiên thì chuyển sang `/dang-nhap`. Nó cố ý KHÔNG kiểm quyền: `document.read`, `document.create`
 * và `document.route` do dịch vụ `documents` kiểm trên TỪNG lời gọi API thật (luật 5, cấm #1).
 *
 * XÃ ĐỌC LÚC CHẠY TỪ `Host`, như mọi trang khác: không có giá trị riêng của xã nào nằm trong
 * bundle, và `Host` không khớp xã nào thì trang này là 404 trước khi dựng gì (luật 1, bất biến 3
 * và 10).
 */
export const dynamic = "force-dynamic";

// Tab title carries the signed-in commune, never the product name (ADR 0068 §13); a Host
// matching no commune 404s here exactly as the page body does.
export function generateMetadata() {
  return communePageMetadata("Văn bản & đơn thư");
}

export default async function TrangVanBan({
  searchParams,
}: {
  searchParams: Promise<RawSearchParams>;
}) {
  const xa = await layCauHinhXa();
  // Lọc mở từ trang Tổng quan (SRS M7.2.2). Đọc ở MÁY CHỦ và chuyển xuống bằng prop: màn danh sách
  // không tự đọc thanh địa chỉ. `key` theo lọc — đổi lọc là dựng lại màn từ trang đầu (và về tab
  // Văn bản đến, nơi lọc ấy áp vào).
  const drillDown = parseDrillDown("incoming-documents", await searchParams);

  return (
    <CauHinhXaProvider giaTri={phanHienThi(xa)}>
      <PhienProvider>
        <div className="khung-trang">
          <DauTrang />
          <main className="than-trang">
            <DocumentWorkspace key={drillDownKey(drillDown)} drillDown={drillDown} />
          </main>
        </div>
      </PhienProvider>
    </CauHinhXaProvider>
  );
}
