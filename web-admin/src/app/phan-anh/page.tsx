import { CauHinhXaProvider } from "@/components/cau-hinh-xa";
import { phanHienThi } from "@/lib/cau-hinh-xa-hien-thi";
import { DauTrang } from "@/components/dau-trang";
import { petitionDeepLinkCode } from "@/features/phan-anh/nhan-phieu";
import { PetitionWorkspace } from "@/features/phan-anh/petition-workspace";
import { PhienProvider } from "@/features/phien/phien-hien-tai";
import { parseDrillDown, type RawSearchParams } from "@/lib/drill-down";
import { communePageMetadata, layCauHinhXa } from "@/lib/tenant.server";

/**
 * `/phan-anh` — Phản ánh của người dân (`docs/ui-ux/09-phan-anh-nguoi-dan.md`), đúng đường dẫn
 * đặc tả ghi ở đầu chương.
 *
 * TRANG NÀY LÀ TRANG CỦA CÁN BỘ, không phải trang của người dân. Kênh công dân là Zalo Mini App
 * và nó không đi qua ứng dụng này. Vì thế trang được bảo vệ bởi `src/proxy.ts` như mọi trang
 * khác, và phần thân đòi `feedback.read` — do dịch vụ `petitions` kiểm trên TỪNG lời gọi, không
 * do cổng ở giao diện.
 *
 * KHUNG MÀN — ADR 0068 lần 5 (06/10/2026): prototype `FeedbackWorkspace.tsx`. Header, cổng
 * `feedback.read` và nút `+ Nhập hộ phản ánh` nằm ở `features/phan-anh/petition-workspace.tsx`.
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
  // không tự đọc thanh địa chỉ. Sổ được khoá theo lọc — đổi lọc là dựng lại sổ từ trang đầu.
  const params = await searchParams;
  const drillDown = parseDrillDown("citizen-reports", params);
  // `?id=<mã tra cứu>` opens that petition's drawer (prototype `app/phan-anh/page.tsx`). A lookup code
  // is a business code, not personal data; the detail route still checks `feedback.read` and the
  // commune, and answers 404 for a code that is not this commune's.
  const openCode = petitionDeepLinkCode(params.id);

  return (
    <CauHinhXaProvider giaTri={phanHienThi(xa)}>
      <PhienProvider>
        <div className="khung-trang">
          <DauTrang />
          <main className="than-trang">
            <PetitionWorkspace drillDown={drillDown} openCode={openCode} />
          </main>
        </div>
      </PhienProvider>
    </CauHinhXaProvider>
  );
}
