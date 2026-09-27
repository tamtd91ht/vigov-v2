import { CauHinhXaProvider } from "@/components/cau-hinh-xa";
import { phanHienThi } from "@/lib/cau-hinh-xa-hien-thi";
import { DauTrang } from "@/components/dau-trang";
import { ThanhBen } from "@/components/thanh-ben";
import { SoNhiemVu } from "@/features/nhiem-vu/so-nhiem-vu";
import { PhienProvider } from "@/features/phien/phien-hien-tai";
import { layCauHinhXa } from "@/lib/tenant.server";

/**
 * `/nhiem-vu` — Quản lý nhiệm vụ (`docs/ui-ux/02-nhiem-vu.md`), đúng đường dẫn đặc tả ghi ở đầu
 * chương.
 *
 * KHÔNG CÓ `<CongQuyen>` Ở ĐÂY, VÀ SỰ VẮNG MẶT ẤY LÀ MỘT QUYẾT ĐỊNH CHỨ KHÔNG PHẢI MỘT LẦN BỎ QUA.
 * Hằng `QUYEN_XEM_NHIEM_VU` (`task.read`, `service-identity/migrations/0001_init.sql:311`) CÓ trong
 * `lib/quyen.ts`, và nó được dùng đúng một chỗ: canh MỤC MENU (`components/muc-menu.ts`). Thân màn
 * cố ý không bọc cổng, cùng khuôn `document.read` của hai quyển sổ văn bản (lý do ghi ở chú thích
 * của chính hằng ấy): tài khoản thiếu khoá nhận **403 ngay ở lượt đọc** và màn hình hiện NGUYÊN câu
 * của máy chủ. Các nút GHI trong màn thì có cổng theo từng khoá `task.*` (`quyenNhiemVu`,
 * `features/nhiem-vu/nhan-nhiem-vu.ts`).
 *
 * Lớp chặn thật không đổi: `authz.RequirePermission` ở máy chủ, trên TỪNG lời gọi (luật 5, cấm
 * #1). Cả menu lẫn cổng nút chỉ là tiện dụng.
 */
export const dynamic = "force-dynamic";

export const metadata = {
  title: "Quản lý nhiệm vụ · ViGov",
};

export default async function TrangNhiemVu() {
  const xa = await layCauHinhXa();

  return (
    <CauHinhXaProvider giaTri={phanHienThi(xa)}>
      <PhienProvider>
        <div className="khung-trang">
          <ThanhBen />
          <DauTrang />
          <main className="than-trang">
            <h1>Quản lý nhiệm vụ</h1>
            <p className="mo-ta-trang">
              Giao việc từ kết luận họp, theo dõi tiến độ và đôn đốc tự động.
            </p>
            <SoNhiemVu />
          </main>
        </div>
      </PhienProvider>
    </CauHinhXaProvider>
  );
}
