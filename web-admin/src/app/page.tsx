import { CauHinhXaProvider, phanHienThi } from "@/components/cau-hinh-xa";
import { layCauHinhXa } from "@/lib/tenant.server";

import { DauTrang } from "@/components/dau-trang";
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
 * RẼ TRANG CHỦ THEO VAI TRÒ THÌ CHƯA — và không phải vì chưa làm, mà vì chưa làm được.
 *
 *   §1 nói: vai trò **Lãnh đạo** → `/nhiem-vu/so-tay`, vai trò khác → `/tong-quan`.
 *
 *   "Lãnh đạo" là một NHÃN CỦA VAI TRÒ: `14-cau-hinh.md §4.1` liệt tám vai trò và gắn nhãn ấy
 *   cho đúng hai. Hợp đồng hiện trả `staff.position` — một chuỗi tự do như "Chủ tịch UBND xã" —
 *   và `permissions`. Không có vai trò, không có cờ lãnh đạo.
 *
 *   Suy "Lãnh đạo" từ chuỗi chức vụ là ĐOÁN, và đoán sai ở đây không hỏng to: nó chỉ đưa một
 *   người tới sai màn hình mặc định, mỗi ngày, mà không ai báo. Suy từ `permissions` cũng là
 *   đoán — một quyền không phải một vai trò, và hai vai trò lãnh đạo không được định nghĩa
 *   bằng việc chúng có quyền nào (luật 5, bất biến 3b).
 *
 *   Nên `/` chưa rẽ, và cái thiếu là một trường trong hợp đồng chứ không phải mã ở đây. Hai
 *   màn hình đích (`/tong-quan`, `/nhiem-vu/so-tay`) cũng chưa được dựng.
 */
export const dynamic = "force-dynamic";

export default async function TrangChu() {
  const xa = await layCauHinhXa();

  return (
    <CauHinhXaProvider giaTri={phanHienThi(xa)}>
      <PhienProvider>
        <DauTrang />
        <main className="than-trang">
          <h1>Trang chủ</h1>
          <p>Các phân hệ khác chưa được dựng trong bản này.</p>
        </main>
      </PhienProvider>
    </CauHinhXaProvider>
  );
}
