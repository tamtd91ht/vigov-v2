import { CauHinhXaProvider } from "@/components/cau-hinh-xa";
import { phanHienThi } from "@/lib/cau-hinh-xa-hien-thi";
import { DauTrang } from "@/components/dau-trang";
import { PhienProvider } from "@/features/phien/phien-hien-tai";
import { SoThongBao } from "@/features/thong-bao/so-thong-bao";
import { communePageMetadata, layCauHinhXa } from "@/lib/tenant.server";

/**
 * `/thong-bao` — Thông báo nội bộ (`docs/ui-ux/08-thong-bao.md`), đúng đường dẫn đặc tả ghi ở đầu
 * chương.
 *
 * KHÔNG CÓ `<CongQuyen>` Ở ĐÂY, VÀ SỰ VẮNG MẶT ẤY LÀ MỘT QUYẾT ĐỊNH CHỨ KHÔNG PHẢI MỘT LẦN BỎ QUA.
 * Cả hai tuyến đứng sau `announcement.create` thật — khoá có trong bảng `quyen`
 * (`service-identity/migrations/0001_init.sql:286`, nhóm `THÔNG BÁO`, "Soạn và gửi thông báo" —
 * ĐỌC LẠI NGÀY 23/09/2026: chú thích ở `service-comms/internal/http/thong_bao_noi_bo.go` ghi dòng
 * 279, số ấy đã cũ) và tuyến khai nó trên từng yêu cầu.
 *
 * Lớp chặn thật không đổi: `authz.RequirePermission` ở máy chủ, trên TỪNG lời gọi (luật 5, cấm
 * #1). Thứ thiếu là sự tiện dụng — cán bộ phải bấm vào rồi mới biết mình không có quyền.
 *
 * ⚠ CÂU TRÊN TỪNG NÓI THÊM "`lib/quyen.ts` chưa có hằng nào cho khoá ấy, và lượt này không được
 * sửa tệp đó". Hết đúng ở commit 3c7ec26, HAI COMMIT SAU commit dựng màn này: `QUYEN_SOAN_THONG_BAO`
 * đã có và mục menu đã mở. Xoá câu ấy vì nó mô tả một lệnh cấm của MỘT LƯỢT LÀM như thể đó là
 * hiện trạng của mã — loại chú thích hết hạn nhanh nhất, và phiên này đã gặp năm lần trong một
 * ngày. Điều CÒN đúng là dòng trên: màn vẫn cố ý không bọc `<CongQuyen>`.
 *
 * TIÊU ĐỀ VÀ CÂU MÔ TẢ LẤY TỪ `nhan-thong-bao.ts`, không gõ lại: chúng là chữ NGUYÊN VĂN của §1 và
 * §3, và một bản thứ hai của một câu là một bản sẽ trôi (luật 9, cấm #2). They are drawn by
 * `SoThongBao`'s `PageHeader` (ADR 0068): the `Soạn thông báo` button sits on the title row and the
 * state it toggles lives in that component.
 */
export const dynamic = "force-dynamic";

// Tab title carries the signed-in commune, never the product name (ADR 0068 §13); a Host
// matching no commune 404s here exactly as the page body does.
export function generateMetadata() {
  return communePageMetadata("Thông báo");
}

export default async function TrangThongBao() {
  const xa = await layCauHinhXa();

  return (
    <CauHinhXaProvider giaTri={phanHienThi(xa)}>
      <PhienProvider>
        <div className="khung-trang">
          <DauTrang />
          <main className="than-trang">
            <SoThongBao />
          </main>
        </div>
      </PhienProvider>
    </CauHinhXaProvider>
  );
}
