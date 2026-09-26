import { CauHinhXaProvider } from "@/components/cau-hinh-xa";
import { phanHienThi } from "@/lib/cau-hinh-xa-hien-thi";
import { DauTrang } from "@/components/dau-trang";
import { ThanhBen } from "@/components/thanh-ben";
import { SoPhanAnh } from "@/features/phan-anh/so-phan-anh";
import { TraCuuPhieu } from "@/features/phan-anh/tra-cuu-phieu";
import { PhienProvider } from "@/features/phien/phien-hien-tai";
import { CongQuyen } from "@/features/quyen/cong-quyen";
import { QUYEN_XEM_PHAN_ANH } from "@/lib/quyen";
import { layCauHinhXa } from "@/lib/tenant.server";

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

export const metadata = {
  title: "Phản ánh của người dân · ViGov",
};

export default async function TrangPhanAnh() {
  const xa = await layCauHinhXa();

  return (
    <CauHinhXaProvider giaTri={phanHienThi(xa)}>
      <PhienProvider>
        <div className="khung-trang">
        <ThanhBen />
        <DauTrang />
        <main className="than-trang">
          <h1>Phản ánh của người dân</h1>
          <p className="mo-ta-trang">
            Tiếp nhận từ Zalo Mini App và các kênh khác, theo dõi thời hạn, đối chiếu ảnh trước và
            sau khi xử lý.
          </p>
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
            <SoPhanAnh />
            <TraCuuPhieu />
          </CongQuyen>
        </main>
        </div>
      </PhienProvider>
    </CauHinhXaProvider>
  );
}
