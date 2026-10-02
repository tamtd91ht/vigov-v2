import { Banknote, FolderKanban } from "lucide-react";

import { CauHinhXaProvider } from "@/components/cau-hinh-xa";
import { PageHeader } from "@/components/ui/page-header";
import { phanHienThi } from "@/lib/cau-hinh-xa-hien-thi";
import { DauTrang } from "@/components/dau-trang";
import { ThanhBen } from "@/components/thanh-ben";
import { BangDuAn } from "@/features/giai-ngan/bang-du-an";
import { PhienProvider } from "@/features/phien/phien-hien-tai";
import { CongQuyen } from "@/features/quyen/cong-quyen";
import { QUYEN_XEM_GIAI_NGAN } from "@/lib/quyen";
import { communePageMetadata, layCauHinhXa } from "@/lib/tenant.server";

/**
 * `/giai-ngan` — Theo dõi giải ngân (`docs/ui-ux/06-giai-ngan.md`).
 *
 * ĐƯỜNG DẪN ĐÚNG NHƯ ĐẶC TẢ GHI Ở ĐẦU CHƯƠNG (`Route: /giai-ngan`), không tự dịch thêm đoạn nào:
 * tên tài nguyên URL cho một khái niệm chưa có trong bảng ánh xạ là câu phải HỎI, không phải câu
 * để đoán (ADR 0011).
 *
 * BẢO VỆ ĐƯỜNG NẰM Ở `src/proxy.ts`, phía máy chủ, trước khi trang này được dựng — chưa có cookie
 * phiên thì chuyển sang `/dang-nhap`. Nó cố ý KHÔNG kiểm quyền: `budget.read` do dịch vụ
 * `finance` kiểm trên TỪNG lời gọi API thật. `CongQuyen` dưới đây chỉ để cán bộ thiếu quyền
 * không phải nhìn một bảng chắc chắn trả 403 (luật 5, cấm #1).
 *
 * XÃ ĐỌC LÚC CHẠY TỪ `Host`, như mọi trang khác: không có giá trị riêng của xã nào nằm trong
 * bundle, và `Host` không khớp xã nào thì trang này là 404 trước khi dựng gì (luật 1, bất biến
 * 3 và 10).
 */
export const dynamic = "force-dynamic";

// Tab title carries the signed-in commune, never the product name (ADR 0068 §13); a Host
// matching no commune 404s here exactly as the page body does.
export function generateMetadata() {
  return communePageMetadata("Theo dõi giải ngân");
}

export default async function TrangGiaiNgan() {
  const xa = await layCauHinhXa();

  return (
    <CauHinhXaProvider giaTri={phanHienThi(xa)}>
      <PhienProvider>
        <div className="khung-trang">
        <ThanhBen />
        <DauTrang />
        <main className="than-trang">
          <PageHeader
            icon={Banknote}
            title="Theo dõi giải ngân"
            subtitle={
              <span className="inline-flex items-center gap-1.5">
                <FolderKanban aria-hidden="true" focusable="false" strokeWidth={1.8} />
                Tiến độ giải ngân theo dự án, chứng từ và vướng mắc cần tháo gỡ.
              </span>
            }
          />
          <CongQuyen
            khoa={QUYEN_XEM_GIAI_NGAN}
            cauThieuQuyen={
              "Tài khoản của bạn không có quyền xem theo dõi giải ngân (budget.read), nên phần " +
              "này không hiển thị. Liên hệ quản trị viên của đơn vị nếu bạn cần quyền này."
            }
          >
            <BangDuAn />
          </CongQuyen>
        </main>
        </div>
      </PhienProvider>
    </CauHinhXaProvider>
  );
}
