import { CauHinhXaProvider, phanHienThi } from "@/components/cau-hinh-xa";
import { DauTrang } from "@/components/dau-trang";
import { ChiTietDuAn } from "@/features/giai-ngan/chi-tiet-du-an";
import { PhienProvider } from "@/features/phien/phien-hien-tai";
import { CongQuyen } from "@/features/quyen/cong-quyen";
import { QUYEN_XEM_GIAI_NGAN } from "@/lib/quyen";
import { layCauHinhXa } from "@/lib/tenant.server";

/**
 * `/giai-ngan/du-an/:id` — chi tiết một dự án (`docs/ui-ux/06-giai-ngan.md §8`), đúng đường dẫn
 * con đặc tả ghi ở đầu chương.
 *
 * `id` CHỈ ĐI QUA, KHÔNG ĐƯỢC KIỂM Ở ĐÂY. Trang này không đoán id nào hợp lệ và không so id với
 * gì cả: kho của `finance` bị buộc theo `tenant_id` nên nó không với tới được dự án của xã khác,
 * và cả "không có dự án ấy" lẫn "dự án của xã khác" đều nhận CÙNG một 404 từ máy chủ. Dựng thêm
 * một phép kiểm ở đây chỉ tạo ra một câu trả lời thứ hai, khác câu của máy chủ.
 */
export const dynamic = "force-dynamic";

export const metadata = {
  title: "Chi tiết dự án · ViGov",
};

export default async function TrangChiTietDuAn({ params }: { params: Promise<{ id: string }> }) {
  const [xa, { id }] = await Promise.all([layCauHinhXa(), params]);

  return (
    <CauHinhXaProvider giaTri={phanHienThi(xa)}>
      <PhienProvider>
        <DauTrang />
        <main className="than-trang">
          <h1>Chi tiết dự án</h1>
          <CongQuyen
            khoa={QUYEN_XEM_GIAI_NGAN}
            cauThieuQuyen={
              "Tài khoản của bạn không có quyền xem theo dõi giải ngân (budget.read), nên phần " +
              "này không hiển thị. Liên hệ quản trị viên của đơn vị nếu bạn cần quyền này."
            }
          >
            <ChiTietDuAn id={id} />
          </CongQuyen>
        </main>
      </PhienProvider>
    </CauHinhXaProvider>
  );
}
