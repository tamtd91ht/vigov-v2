import { CauHinhXaProvider } from "@/components/cau-hinh-xa";
import { phanHienThi } from "@/lib/cau-hinh-xa-hien-thi";
import { DauTrang } from "@/components/dau-trang";
import { ChiTietDuAn } from "@/features/giai-ngan/chi-tiet-du-an";
import { DISBURSEMENT_READ_DENIED } from "@/features/giai-ngan/nhan-du-an";
import { PhienProvider } from "@/features/phien/phien-hien-tai";
import { CongQuyen } from "@/features/quyen/cong-quyen";
import { QUYEN_XEM_GIAI_NGAN } from "@/lib/quyen";
import { communePageMetadata, layCauHinhXa } from "@/lib/tenant.server";

/**
 * `/giai-ngan/du-an/:id` — chi tiết một dự án (`docs/ui-ux/06-giai-ngan.md §8`), đúng đường dẫn
 * con đặc tả ghi ở đầu chương.
 *
 * `id` CHỈ ĐI QUA, KHÔNG ĐƯỢC KIỂM Ở ĐÂY. Trang này không đoán id nào hợp lệ và không so id với
 * gì cả: kho của `finance` bị buộc theo `tenant_id` nên nó không với tới được dự án của xã khác,
 * và cả "không có dự án ấy" lẫn "dự án của xã khác" đều nhận CÙNG một 404 từ máy chủ. Dựng thêm
 * một phép kiểm ở đây chỉ tạo ra một câu trả lời thứ hai, khác câu của máy chủ.
 *
 * SAME FRAME AS `/giai-ngan` (`khung-trang` + the navigation header `DauTrang`, tester report GN-07):
 * without the menu the officer had no way back but the browser button.
 *
 * KHUNG MÀN — ADR 0068 lần 5: prototype `BudgetItemDetail.tsx` — no page header, a back link and one
 * project card, 76rem wide at most. The page `<h1>` stays for assistive tech only; the project name
 * is the card's heading.
 */
export const dynamic = "force-dynamic";

// Tab title carries the signed-in commune, never the product name (ADR 0068 §13); a Host
// matching no commune 404s here exactly as the page body does.
export function generateMetadata() {
  return communePageMetadata("Chi tiết dự án");
}

export default async function TrangChiTietDuAn({ params }: { params: Promise<{ id: string }> }) {
  const [xa, { id }] = await Promise.all([layCauHinhXa(), params]);

  return (
    <CauHinhXaProvider giaTri={phanHienThi(xa)}>
      <PhienProvider>
        <div className="khung-trang">
          <DauTrang />
          <main className="than-trang">
            <div className="mx-auto w-full max-w-[76rem] min-w-0">
              <h1 className="an-thi-giac">Chi tiết dự án</h1>
              <CongQuyen khoa={QUYEN_XEM_GIAI_NGAN} cauThieuQuyen={DISBURSEMENT_READ_DENIED}>
                <ChiTietDuAn id={id} />
              </CongQuyen>
            </div>
          </main>
        </div>
      </PhienProvider>
    </CauHinhXaProvider>
  );
}
