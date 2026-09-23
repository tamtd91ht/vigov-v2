import { CauHinhXaProvider, phanHienThi } from "@/components/cau-hinh-xa";
import { DauTrang } from "@/components/dau-trang";
import { ThanhBen } from "@/components/thanh-ben";
import { SoNhiemVu } from "@/features/nhiem-vu/so-nhiem-vu";
import { PhienProvider } from "@/features/phien/phien-hien-tai";
import { layCauHinhXa } from "@/lib/tenant.server";

/**
 * `/nhiem-vu` — Quản lý nhiệm vụ (`docs/ui-ux/02-nhiem-vu.md`), đúng đường dẫn đặc tả ghi ở đầu
 * chương.
 *
 * KHÔNG CÓ `<CongQuyen>` Ở ĐÂY, VÀ SỰ VẮNG MẶT ẤY LÀ MỘT PHÁT HIỆN CHỨ KHÔNG PHẢI MỘT LẦN BỎ QUA.
 * Hai tuyến đọc đứng sau `task.read` thật — khoá có trong bảng `quyen`
 * (`service-identity/migrations/0001_init.sql:304`) và tuyến khai nó trên từng yêu cầu — nhưng
 * `lib/quyen.ts` chưa có hằng nào cho bảy khoá `task.*`, và lượt này không được sửa tệp ấy. Nên
 * tài khoản thiếu khoá nhận **403 ngay ở lượt đọc** và màn hình hiện NGUYÊN câu của máy chủ, cùng
 * khuôn `document.read` đã chọn cho hai quyển sổ văn bản.
 *
 * Lớp chặn thật không đổi: `authz.RequirePermission` ở máy chủ, trên TỪNG lời gọi (luật 5, cấm
 * #1). Thứ thiếu là sự tiện dụng — cán bộ phải bấm vào rồi mới biết mình không có quyền. Đã báo
 * về để bổ sung hằng.
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
