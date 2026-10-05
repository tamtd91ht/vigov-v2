import { MessageSquareWarning, Smartphone } from "lucide-react";

import { CauHinhXaProvider } from "@/components/cau-hinh-xa";
import { PageHeader } from "@/components/ui/page-header";
import { phanHienThi } from "@/lib/cau-hinh-xa-hien-thi";
import { DauTrang } from "@/components/dau-trang";
import { SoPhanAnh } from "@/features/phan-anh/so-phan-anh";
import { TraCuuPhieu } from "@/features/phan-anh/tra-cuu-phieu";
import { PhienProvider } from "@/features/phien/phien-hien-tai";
import { CongQuyen } from "@/features/quyen/cong-quyen";
import { QUYEN_XEM_PHAN_ANH } from "@/lib/quyen";
import { drillDownKey, parseDrillDown, type RawSearchParams } from "@/lib/drill-down";
import { communePageMetadata, layCauHinhXa } from "@/lib/tenant.server";

/**
 * `/phan-anh` — Phản ánh của người dân (`docs/ui-ux/09-phan-anh-nguoi-dan.md`), đúng đường dẫn
 * đặc tả ghi ở đầu chương.
 *
 * TRANG NÀY LÀ TRANG CỦA CÁN BỘ, không phải trang của người dân. Kênh công dân là Zalo Mini App
 * và nó không đi qua ứng dụng này. Vì thế trang được bảo vệ bởi `src/proxy.ts` như mọi trang
 * khác, và phần thân đòi `feedback.read` — do dịch vụ `petitions` kiểm trên TỪNG lời gọi, không
 * do cổng ở giao diện.
 */
export const dynamic = "force-dynamic";

// Tab title carries the signed-in commune, never the product name (ADR 0068 §13); a Host
// matching no commune 404s here exactly as the page body does.
export function generateMetadata() {
  return communePageMetadata("Phản ánh của người dân");
}

export default async function TrangPhanAnh({
  searchParams,
}: {
  searchParams: Promise<RawSearchParams>;
}) {
  const xa = await layCauHinhXa();
  // Lọc mở từ trang Tổng quan (SRS M7.2.2). Đọc ở MÁY CHỦ và chuyển xuống bằng prop: màn danh sách
  // không tự đọc thanh địa chỉ. `key` theo lọc — đổi lọc là dựng lại màn từ trang đầu.
  const drillDown = parseDrillDown("citizen-reports", await searchParams);

  return (
    <CauHinhXaProvider giaTri={phanHienThi(xa)}>
      <PhienProvider>
        <div className="khung-trang">
        <DauTrang />
        <main className="than-trang">
          <PageHeader
            icon={MessageSquareWarning}
            title="Phản ánh của người dân"
            subtitle={
              <span className="inline-flex items-center gap-1.5">
                <Smartphone aria-hidden="true" focusable="false" strokeWidth={1.8} />
                Tiếp nhận từ Zalo Mini App và các kênh khác, theo dõi thời hạn, đối chiếu ảnh trước và
                sau khi xử lý.
              </span>
            }
          />
          <CongQuyen
            khoa={QUYEN_XEM_PHAN_ANH}
            cauThieuQuyen={
              "Tài khoản của bạn không có quyền xem phản ánh của người dân (feedback.read), nên " +
              "phần này không hiển thị. Liên hệ quản trị viên của đơn vị nếu bạn cần quyền này."
            }
          >
            {/* MỘT CỔNG `feedback.read` CHO CẢ HAI, vì cả sáu tuyến của màn này đứng sau nó (hoặc
                sau một khoá hẹp hơn). Bốn thao tác ghi có cổng RIÊNG bên trong, và một trong bốn
                — `Đóng phiếu` — cố ý đứng sau khoá khác với nút tiến trạng thái; xem
                `QUYEN_DONG_PHAN_ANH` ở `lib/quyen.ts`. */}
            {/* One spacing between the register and the lookup (spec §6.9). */}
            <div className="flex min-w-0 flex-col gap-6">
              <SoPhanAnh key={drillDownKey(drillDown)} drillDown={drillDown} />
              <TraCuuPhieu />
            </div>
          </CongQuyen>
        </main>
        </div>
      </PhienProvider>
    </CauHinhXaProvider>
  );
}
