import { CauHinhXaProvider } from "@/components/cau-hinh-xa";
import { phanHienThi } from "@/lib/cau-hinh-xa-hien-thi";
import { DauTrang } from "@/components/dau-trang";
import { DisbursementWorkspace } from "@/features/giai-ngan/disbursement-workspace";
import { PhienProvider } from "@/features/phien/phien-hien-tai";
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
 * `finance` kiểm trên TỪNG lời gọi API thật. Cổng `CongQuyen` trong `DisbursementWorkspace` chỉ để
 * cán bộ thiếu quyền không phải nhìn một bảng chắc chắn trả 403 (luật 5, cấm #1).
 *
 * XÃ ĐỌC LÚC CHẠY TỪ `Host`, như mọi trang khác: không có giá trị riêng của xã nào nằm trong
 * bundle, và `Host` không khớp xã nào thì trang này là 404 trước khi dựng gì (luật 1, bất biến
 * 3 và 10).
 *
 * KHUNG MÀN — ADR 0068 lần 5 (06/10/2026): prototype `BudgetWorkspace.tsx`. Header, năm ngân sách và
 * nút `+ Thêm dự án` nằm ở `features/giai-ngan/disbursement-workspace.tsx`.
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
          <DauTrang />
          <main className="than-trang">
            <DisbursementWorkspace />
          </main>
        </div>
      </PhienProvider>
    </CauHinhXaProvider>
  );
}
