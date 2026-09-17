import { CauHinhXaProvider, phanHienThi } from "@/components/cau-hinh-xa";
import { layCauHinhXa } from "@/lib/tenant.server";

import { DauTrang } from "@/components/dau-trang";

/**
 * `/` — khung của người đã đăng nhập.
 *
 * Được bảo vệ ở `src/middleware.ts`, phía máy chủ, trước khi trang này được dựng. Không có một
 * lớp kiểm nào ở client, vì một lớp kiểm ở client không kiểm được gì (luật 5, cấm #1).
 *
 * HAI THỨ §1 và §3 CÓ MÀ Ở ĐÂY CHƯA CÓ, và lý do là một:
 *
 *   - `/` lẽ ra rẽ theo vai trò (Lãnh đạo → `/nhiem-vu/so-tay`, còn lại → `/tong-quan`);
 *   - đầu trang lẽ ra hiện họ tên và chức vụ cán bộ đang đăng nhập.
 *
 *   Cả hai đều cần biết "phiên hiện tại là của ai", mà hợp đồng chưa có route nào trả lời. Họ
 *   tên chỉ về đúng một lần trong phản hồi 201 lúc đăng nhập, và tải lại trang là mất. Cách vá
 *   tạm — cất họ tên vào kho của trình duyệt — là dựng thêm một nơi lưu dữ liệu cá nhân (luật
 *   3) để đổi lấy một dòng chữ. Nên ở đây chưa hiện, và lỗ hổng hợp đồng được nêu ra thay vì
 *   được che đi.
 */
export const dynamic = "force-dynamic";

export default async function TrangChu() {
  const xa = await layCauHinhXa();

  return (
    <CauHinhXaProvider giaTri={phanHienThi(xa)}>
      <DauTrang />
      <main className="than-trang">
        <h1>Trang chủ</h1>
        <p>Các phân hệ khác chưa được dựng trong bản này.</p>
      </main>
    </CauHinhXaProvider>
  );
}
