import { CauHinhXaProvider } from "@/components/cau-hinh-xa";
import { phanHienThi } from "@/lib/cau-hinh-xa-hien-thi";
import { layCauHinhXa } from "@/lib/tenant.server";

import { DauTrang } from "@/components/dau-trang";
import { ThanhBen } from "@/components/thanh-ben";
import { PhienProvider } from "@/features/phien/phien-hien-tai";

/**
 * `/` — khung của người đã đăng nhập.
 *
 * Được bảo vệ ở `src/middleware.ts`, phía máy chủ, trước khi trang này được dựng. Không có một
 * lớp kiểm nào ở client, vì một lớp kiểm ở client không kiểm được gì (luật 5, cấm #1).
 *
 * HỌ TÊN VÀ CHỨC VỤ NAY ĐÃ CÓ trên đầu trang: `GET /api/v1/sessions/current` tồn tại, và
 * `PhienProvider` đọc nó một lần cho cả trang.
 *
 * RẼ TRANG CHỦ THEO VAI TRÒ THÌ VẪN CHƯA — nhưng KHÔNG CÒN vì lý do khối chú thích này từng
 * ghi, và đây là lần thứ hai nó được sửa. Lý do cũ đọc là "Hợp đồng hiện trả `staff.position`
 * … Không có vai trò, không có cờ lãnh đạo". Câu ấy NAY SAI:
 * `GET /api/v1/sessions/current` trả `role` kèm `role.is_leader` — đúng cờ cần tìm, đặt đúng
 * chỗ, không phải suy từ chuỗi chức vụ hay từ danh sách quyền.
 *
 *   §1 nói: vai trò **Lãnh đạo** → `/nhiem-vu/so-tay`, vai trò khác → `/tong-quan`.
 *
 * LÝ DO CÒN LẠI LÀ HAI MÀN HÌNH ĐÍCH KHÔNG TỒN TẠI. Ứng dụng này có đúng ba đường: `/`,
 * `/cau-hinh` và `/dang-nhap`. Rẽ một lãnh đạo sang `/nhiem-vu/so-tay` là đưa họ tới 404 ngay
 * sau khi đăng nhập — tệ hơn hẳn trang chủ tạm hiện nay, và tệ theo kiểu người dùng tưởng tài
 * khoản mình hỏng. Nên phép rẽ mở khoá bằng việc DỰNG hai màn hình ấy, không phải bằng một
 * thay đổi hợp đồng nào nữa.
 *
 * VÀ CÒN MỘT CÂU CHƯA CÓ AI TRẢ LỜI, phải chốt cùng lúc với phép rẽ: `role` trong hợp đồng là
 * `identity_vaiTroGon | null`, và `null` — cán bộ chưa được gán vai trò — là một trạng thái có
 * thật (xem cột Vai trò của danh bạ). Đặc tả §1 chỉ chia hai nhánh "Lãnh đạo" và "vai trò
 * khác"; nó không nói người chưa có vai trò nào thì về đâu. Chọn thầm một nhánh cho họ là quyết
 * định thay đơn vị — HỎI trước khi dựng phép rẽ, đừng để nó rơi vào nhánh `else`.
 */
export const dynamic = "force-dynamic";

export default async function TrangChu() {
  const xa = await layCauHinhXa();

  return (
    <CauHinhXaProvider giaTri={phanHienThi(xa)}>
      <PhienProvider>
        <div className="khung-trang">
        <ThanhBen />
        <DauTrang />
        <main className="than-trang">
          <h1>Trang chủ</h1>
          <p>Các phân hệ khác chưa được dựng trong bản này.</p>
        </main>
        </div>
      </PhienProvider>
    </CauHinhXaProvider>
  );
}
