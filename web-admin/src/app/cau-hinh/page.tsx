import { redirect } from "next/navigation";

import { CauHinhXaProvider } from "@/components/cau-hinh-xa";
import { phanHienThi } from "@/lib/cau-hinh-xa-hien-thi";
import { DauTrang } from "@/components/dau-trang";
import { PhienProvider } from "@/features/phien/phien-hien-tai";
import { ConfigPageHeader } from "@/features/cau-hinh/config-page-header";
import { KhungTabCauHinh } from "@/features/cau-hinh/khung-tab-cau-hinh";
import { movedTabRoute } from "@/features/cau-hinh/moved-tabs";
import { communePageMetadata, layCauHinhXa } from "@/lib/tenant.server";

/**
 * `/cau-hinh` — màn hình Cấu hình hệ thống (`docs/ui-ux/14-cau-hinh.md`).
 *
 * ĐƯỜNG DẪN: đúng đường dẫn đặc tả đã ghi (§ tiêu đề), và **không thêm đoạn nào**.
 *
 * NGƯỜI DÙNG VÀ PHÂN QUYỀN ĐÃ RỜI MÀN NÀY (05/10/2026, chủ dự án duyệt theo bản mẫu
 * `vigov-require/apps/admin`): nay là `/nguoi-dung` và `/nguoi-dung/phan-quyen`, mỗi màn một mục
 * menu. Lý do của bản mẫu: thêm cán bộ, gỡ quyền là việc hằng tuần, danh mục thì vài tháng mới đụng
 * — việc thường xuyên không nằm sau một mục menu cộng một tab. Đoạn đường dẫn `nguoi-dung` là của
 * bản mẫu mà chủ dự án đã duyệt, không phải tên tự đặt ở đây. Liên kết cũ `?tab=nguoi-dung` /
 * `?tab=phan-quyen` chuyển sang màn mới (`features/cau-hinh/moved-tabs.ts`).
 *
 * MƯỜI TAB CỦA §0, Ở ĐÂY CÓ SÁU. Bốn tab còn lại — Trường bản đồ, Lời hệ thống, Tự động hoá, Máy
 * chủ thư — chưa có tuyến nào trong hợp đồng REST. Một thanh mười tab mà bốn tab bấm vào không ra
 * gì là bốn lần hứa hẹn suông, nên thanh tab chỉ mọc thêm khi tuyến mọc thêm.
 *
 * BA BẢNG LỊCH NAY Ở TAB "LỊCH LÀM VIỆC" (chủ dự án 08/10/2026, ADR 0079 D1/D2), tab Thời hạn xử lý chỉ
 * còn bảng thời hạn và ẩn khi thiếu `admin.sla`. Đoạn dưới kể lý do của bốn bảng, vẫn đúng:
 *
 * TAB THỜI HẠN XỬ LÝ (§8) TỪNG ĐỦ CẢ BỐN BẢNG, VÀ ĐÓ LÀ TAB GẤP NHẤT TRONG NĂM. Bảng thời hạn —
 * số giờ tiếp nhận và xử lý xong — đã có tuyến (`/api/v1/sla`), cùng ba bảng lịch mà §8 nêu ở
 * cuối (`/working-hours`, `/public-holidays`, `/swap-working-days`). Bốn bảng ấy là nền của cách
 * đếm hạn theo giờ làm việc (ADR 0007), và HAI trong số đó rỗng thì xã KHÔNG vào sổ được văn bản
 * đến và KHÔNG nhận được phản ánh: `identity.ResolveDeadlines` từ chối. `TabThoiHanXuLy` nói đúng
 * câu đó ra trên màn hình, kèm hai nút gieo — đó là thứ thay cho một bước hướng dẫn ban đầu mà hệ
 * thống không có.
 *
 * `TabLichLamViec` (chỉ xem) ĐÃ ĐƯỢC XOÁ cùng lượt này, không chỉ thôi được dựng. Tab mới bao trọn
 * ba bảng lịch của nó và thêm đường ghi, nên giữ lại là hiện hai lần cùng ba bảng — nhưng lý do
 * xoá hẳn nặng hơn thế: hằng `GHI_CHU_CHI_XEM_LICH` của nó nói với cán bộ rằng *"sửa lịch chưa mở
 * vì chưa có quy định ai được sửa"*, và câu ấy nay SAI — `admin.sla` chính là quy định ấy. Bài
 * kiểm của tệp cũ vẫn XANH khi câu ấy đã sai, vì nó chỉ kiểm câu có hiện ra hay không chứ không
 * kiểm câu có còn đúng hay không. Một lời khai sai mà không cổng nào đỏ là thứ chỉ gỡ được bằng
 * cách xoá nguồn của nó.
 *
 * THANH TAB — QUYẾT ĐỊNH CỦA NGƯỜI DÙNG (26/09): tài khoản mở được từ HAI tab trở lên thì hiện
 * thanh tab; chỉ mở được MỘT tab thì KHÔNG hiện thanh, nội dung tab ấy hiện thẳng. Câu này từng
 * để ngỏ (một nút tab đứng trơ, hay không hiện?) và sáu phần đã dựng nối tiếp cho tới khi có lời
 * đáp. Phần dựng ở `features/cau-hinh/khung-tab-cau-hinh.tsx`, phần quyết định ở
 * `features/cau-hinh/thanh-tab-cau-hinh.ts`. Chưa đọc xong phiên thì chưa dựng thanh — tab đầu
 * hiện thẳng — để thanh chỉ có thể xuất hiện, không bao giờ hiện rồi biến mất.
 *
 * CỔNG QUYỀN Ở MỖI TAB ĐI THEO TUYẾN ĐỌC CỦA NÓ Ở MÁY CHỦ, không theo một luật chung ở đây
 * (bảng đầy đủ: `features/cau-hinh/thanh-tab-cau-hinh.ts`):
 *
 *   | Phần            | Khoá           | Cổng bọc                                              |
 *   |-----------------|----------------|-------------------------------------------------------|
 *   | Sơ đồ tổ chức   | `admin.org`    | chỉ nút ghi — tuyến đọc `any-authenticated`           |
 *   | Danh mục        | `admin.lookup` | chỉ nút ghi — cùng lý do                              |
 *   | Thời hạn xử lý  | `admin.sla`    | cả tab — `GET /sla` đòi khoá (ADR 0079 D1)             |
 *   | Lịch làm việc   | `admin.sla`    | chỉ nút ghi — ba tuyến đọc lịch `any-authenticated`   |
 *   | Thôn/Tổ dân phố | —              | không cổng — phần chỉ xem, tuyến `any-authenticated`  |
 *
 * Ẩn cả một phần mà máy chủ vẫn phục vụ là để GIAO DIỆN quyết định điều máy chủ không từ chối —
 * đúng hình dạng luật 5 cấm #1. Lý lẽ đầy đủ ở `features/cau-hinh/tab-danh-muc.tsx`.
 *
 * BẢO VỆ ĐƯỜNG: `src/proxy.ts` chặn ở phía máy chủ trước khi trang này được dựng — chưa có
 * cookie phiên thì chuyển sang `/dang-nhap`. Nó cố ý KHÔNG kiểm quyền: quyền do dịch vụ kiểm
 * trên từng lời gọi API thật (xem `features/cau-hinh/tab-nguoi-dung.tsx`).
 */
export const dynamic = "force-dynamic";

// Tab title carries the signed-in commune, never the product name (ADR 0068 §13); a Host
// matching no commune 404s here exactly as the page body does.
export function generateMetadata() {
  return communePageMetadata("Cấu hình hệ thống");
}

export default async function TrangCauHinh({
  searchParams,
}: {
  searchParams: Promise<{ [key: string]: string | string[] | undefined }>;
}) {
  // Commune first: a Host matching no commune 404s before any redirect is issued.
  const xa = await layCauHinhXa();

  // Người dùng and Phân quyền moved to `/nguoi-dung` (05/10/2026). A saved `?tab=` link to either
  // lands on its new screen instead of on the first tab here (`moved-tabs.ts`).
  const moved = movedTabRoute((await searchParams).tab);
  if (moved !== null) redirect(moved);

  return (
    <CauHinhXaProvider giaTri={phanHienThi(xa)}>
      <PhienProvider>
        <div className="khung-trang">
        <DauTrang />
        <main className="than-trang">
          {/* Spec 02 frame: `than-trang` is the page's `p-7` (28px from 768px, 16px below — every
              page's shell), then the `mb-6` header with no icon and no button. */}
          <ConfigPageHeader />
          {/* Thứ tự tab nằm ở `TAB_CAU_HINH` (`thanh-tab-cau-hinh.ts`). `min-w-0`: a wide tab scrolls
              inside its own region, never the page. No gap here: the frame owns the 28px between the
              strip and the content (`khung-tab-cau-hinh.tsx`). */}
          <div className="min-w-0">
            <KhungTabCauHinh />
          </div>
        </main>
        </div>
      </PhienProvider>
    </CauHinhXaProvider>
  );
}
