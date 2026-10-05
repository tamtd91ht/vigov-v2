import { CauHinhXaProvider } from "@/components/cau-hinh-xa";
import { phanHienThi } from "@/lib/cau-hinh-xa-hien-thi";
import { DauTrang } from "@/components/dau-trang";
import { SoNhiemVu } from "@/features/nhiem-vu/so-nhiem-vu";
import { parseOpenTask } from "@/features/nhiem-vu/task-link";
import { PhienProvider } from "@/features/phien/phien-hien-tai";
import { drillDownKey, parseDrillDown, type RawSearchParams } from "@/lib/drill-down";
import { communePageMetadata, layCauHinhXa } from "@/lib/tenant.server";

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

// Tab title carries the signed-in commune, never the product name (ADR 0068 §13); a Host
// matching no commune 404s here exactly as the page body does.
export function generateMetadata() {
  return communePageMetadata("Quản lý nhiệm vụ");
}

export default async function TrangNhiemVu({
  searchParams,
}: {
  searchParams: Promise<RawSearchParams>;
}) {
  const xa = await layCauHinhXa();
  // Lọc mở từ trang Tổng quan (SRS M7.2.2). Đọc ở MÁY CHỦ và chuyển xuống bằng prop: màn danh sách
  // không tự đọc thanh địa chỉ. `key` theo lọc — đổi lọc là dựng lại màn từ trang đầu.
  const params = await searchParams;
  const drillDown = parseDrillDown("tasks", params);
  // `?task=NV19` — a link from another screen (Sổ tay lãnh đạo) opening one task's detail. Read here,
  // like the drill-down, so the register never reads the address bar for it.
  const openTask = parseOpenTask(params);

  return (
    <CauHinhXaProvider giaTri={phanHienThi(xa)}>
      <PhienProvider>
        <div className="khung-trang">
          <DauTrang />
          <main className="than-trang">
            {/* The page header (`<h1>`, subtitle, `Nhập từ Excel` · `Giao việc mới`) is drawn by
                `SoNhiemVu`: the two buttons sit on the title row (spec §5) and their state — the
                open dialog, the open form, the `task.create` gate — lives in that component. */}
            <SoNhiemVu key={drillDownKey(drillDown)} drillDown={drillDown} openTask={openTask} />
          </main>
        </div>
      </PhienProvider>
    </CauHinhXaProvider>
  );
}
