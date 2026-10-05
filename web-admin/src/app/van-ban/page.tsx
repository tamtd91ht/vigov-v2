import { ListTodo, Mail } from "lucide-react";

import { CauHinhXaProvider } from "@/components/cau-hinh-xa";
import { PageHeader } from "@/components/ui/page-header";
import { phanHienThi } from "@/lib/cau-hinh-xa-hien-thi";
import { DauTrang } from "@/components/dau-trang";
import { PhienProvider } from "@/features/phien/phien-hien-tai";
import { DocumentTabs, ScanOcrButton } from "@/features/van-ban/document-pending";
import { SoVanBanDen } from "@/features/van-ban/so-van-ban-den";
import { SoVanBanDi } from "@/features/van-ban/so-van-ban-di";
import { drillDownKey, parseDrillDown, type RawSearchParams } from "@/lib/drill-down";
import { communePageMetadata, layCauHinhXa } from "@/lib/tenant.server";

/**
 * `/van-ban` — Văn bản & Đơn thư (`docs/ui-ux/05-van-ban-don-thu.md`).
 *
 * ĐƯỜNG DẪN ĐÚNG NHƯ ĐẶC TẢ GHI Ở ĐẦU CHƯƠNG (`Route: /van-ban`), và **không thêm đoạn nào**. Sổ
 * văn bản đi KHÔNG có đường dẫn riêng `/van-ban/di`, có chủ ý: tên tài nguyên URL cho một khái
 * niệm chưa có trong bảng ánh xạ là câu phải HỎI, không phải câu để đoán (ADR 0011) — và một đoạn
 * đường dẫn đã chạy thật ở một xã thì không có lần sửa nào rẻ nữa. Hai quyển sổ vì thế dựng nối
 * tiếp trong cùng một trang, đúng khuôn màn Cấu hình đang dùng cho năm phần của nó.
 *
 * THANH TAB CỦA ĐẶC TẢ (§2) CÓ MỘT TAB SỐNG: "Văn bản" — hai quyển sổ đang có, vẫn dựng nối tiếp
 * trong cùng một khung, như trước. "Đơn thư công dân" và "Báo cáo" là tab vô hiệu mang dấu "?"
 * (ADR 0068 §14): hợp đồng REST hôm nay không có tuyến nào cho `don_thu`, nên một tab gọi vào đó là
 * lời hứa suông — cán bộ bấm, nhận lỗi, và kết luận hệ thống hỏng. Lý do hiện ra khi bấm "?" nằm ở
 * `PHAN_CHUA_DUNG` (`features/van-ban/nhan-van-ban.ts`), không chép ở đây.
 *
 * HAI SỔ KHÔNG TÁCH THÀNH HAI TAB, có chủ ý: tài khoản chỉ mở được một sổ thì một thanh như thế
 * hiện một nút đứng trơ hay không hiện — câu ấy chưa ai trả lời. Dựng nối tiếp không mất gì, vì mỗi
 * sổ đọc dữ liệu của riêng nó và phần nào thiếu quyền thì không gọi tuyến ghi nào.
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
  // không tự đọc thanh địa chỉ. `key` theo lọc — đổi lọc là dựng lại màn từ trang đầu.
  const drillDown = parseDrillDown("incoming-documents", await searchParams);

  return (
    <CauHinhXaProvider giaTri={phanHienThi(xa)}>
      <PhienProvider>
        <div className="khung-trang">
        <DauTrang />
        <main className="than-trang">
          <PageHeader
            icon={Mail}
            title="Văn bản & đơn thư"
            subtitle={
              <span className="inline-flex items-center gap-1.5">
                <ListTodo aria-hidden="true" focusable="false" strokeWidth={1.8} />
                Vào sổ văn bản đến, cấp số văn bản đi, phân công xử lý và theo dõi hạn giải quyết.
              </span>
            }
            // Second header button of spec §2; the first ("+ Nhập tay") belongs to the petition-letter
            // register, which does not exist yet — so it has nothing to stand beside today.
            actions={<ScanOcrButton />}
          />
          <DocumentTabs>
            {/* The two registers stay one after the other inside the one live tab — see "HAI SỔ" above. */}
            <div className="flex min-w-0 flex-col gap-6">
              <SoVanBanDen key={drillDownKey(drillDown)} drillDown={drillDown} />
              <SoVanBanDi />
            </div>
          </DocumentTabs>
        </main>
        </div>
      </PhienProvider>
    </CauHinhXaProvider>
  );
}
