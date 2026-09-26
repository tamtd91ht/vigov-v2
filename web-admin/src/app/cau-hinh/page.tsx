import { CauHinhXaProvider } from "@/components/cau-hinh-xa";
import { phanHienThi } from "@/lib/cau-hinh-xa-hien-thi";
import { DauTrang } from "@/components/dau-trang";
import { ThanhBen } from "@/components/thanh-ben";
import { PhienProvider } from "@/features/phien/phien-hien-tai";
import { KhoiChuaDung } from "@/features/cau-hinh/khoi-chua-dung";
import { TabDanhMuc } from "@/features/cau-hinh/tab-danh-muc";
import { TabNguoiDung } from "@/features/cau-hinh/tab-nguoi-dung";
import { TabPhanQuyen } from "@/features/cau-hinh/tab-phan-quyen";
import { TabSoDoToChuc } from "@/features/cau-hinh/tab-so-do-to-chuc";
import { TabThoiHanXuLy } from "@/features/cau-hinh/tab-thoi-han-xu-ly";
import { TabThonToDanPho } from "@/features/cau-hinh/tab-thon-to-dan-pho";
import { layCauHinhXa } from "@/lib/tenant.server";

/**
 * `/cau-hinh` — màn hình Cấu hình hệ thống (`docs/ui-ux/14-cau-hinh.md`).
 *
 * ĐƯỜNG DẪN: đúng đường dẫn đặc tả đã ghi (§ tiêu đề), và **không thêm đoạn nào**. Đặc tả cũ có
 * `/cau-hinh/nguoi-dung`, nhưng tên tài nguyên URL cho khái niệm "cán bộ" chưa được khách chốt
 * (`kb/00-foundation/ubiquitous-language.md`, dòng Cán bộ: "CHƯA CHỐT — HỎI KHÁCH"), và một
 * đoạn đường dẫn đã chạy thật ở một xã thì không có lần sửa nào rẻ nữa. Nên tab mở ngay trong
 * trang này; đặt tên đoạn đường dẫn là việc của khách, không phải của lượt này.
 *
 * MƯỜI TAB CỦA §0, Ở ĐÂY CÓ SÁU. Bốn tab còn lại — Trường bản đồ, Lời hệ thống, Tự động hoá, Máy
 * chủ thư — chưa có tuyến nào trong hợp đồng REST. Một thanh mười tab mà bốn tab bấm vào không ra
 * gì là bốn lần hứa hẹn suông, nên thanh tab chỉ mọc thêm khi tuyến mọc thêm.
 *
 * TAB THỜI HẠN XỬ LÝ (§8) NAY ĐỦ CẢ BỐN BẢNG, VÀ ĐÓ LÀ TAB GẤP NHẤT TRONG NĂM. Bảng thời hạn —
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
 * VÀ Ở ĐÂY CHƯA CÓ THANH TAB NÀO: sáu phần dựng nối tiếp trong trang, theo đúng thứ tự tab của
 * đặc tả §0. Chưa dựng thanh chuyển tab vì nó đặt ra một câu chưa ai trả lời: tài khoản chỉ mở
 * được MỘT tab thì thanh ấy hiện một nút đứng trơ, hay không hiện? Đó là quyết định về giao diện
 * của khách, và đoán hộ thì phải đoán lại khi tab thứ bảy mọc lên. Dựng nối tiếp không mất gì:
 * mỗi phần vẫn đọc dữ liệu của riêng nó, và phần nào thiếu quyền thì không gọi tuyến nào.
 *
 * CỔNG QUYỀN Ở MỖI PHẦN ĐI THEO TUYẾN ĐỌC CỦA NÓ Ở MÁY CHỦ, không theo một luật chung ở đây:
 *
 *   | Phần            | Khoá           | Cổng bọc                                              |
 *   |-----------------|----------------|-------------------------------------------------------|
 *   | Người dùng      | `admin.user`   | cả phần — tuyến đọc khai `RequirePermission`          |
 *   | Phân quyền      | `admin.role`   | cả phần — cùng lý do                                  |
 *   | Sơ đồ tổ chức   | `admin.org`    | chỉ nút ghi — tuyến đọc `any-authenticated`           |
 *   | Danh mục        | `admin.lookup` | chỉ nút ghi — cùng lý do                              |
 *   | Thời hạn xử lý  | `admin.sla`    | chỉ nút ghi; `GET /sla` đòi khoá, máy chủ tự trả 403  |
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

export const metadata = {
  // Hằng số của sản phẩm, không mang tên xã: tên xã đọc lúc chạy từ `Host` và hiện trên đầu
  // trang (luật 1, bất biến 10).
  title: "Cấu hình hệ thống · ViGov",
};

export default async function TrangCauHinh() {
  const xa = await layCauHinhXa();

  return (
    <CauHinhXaProvider giaTri={phanHienThi(xa)}>
      <PhienProvider>
        <div className="khung-trang">
        <ThanhBen />
        <DauTrang />
        <main className="than-trang">
          <h1>Cấu hình hệ thống</h1>
          <p className="mo-ta-trang">
            Tổ chức, phân quyền, danh mục nghiệp vụ và thời hạn xử lý của đơn vị.
          </p>
          {/* Thứ tự tab của đặc tả §0, giữ nguyên: Sơ đồ tổ chức · Thôn/Tổ dân phố · Người dùng ·
              Phân quyền · Danh mục · Thời hạn xử lý. Sắp lại theo thứ tự dựng xong sẽ làm cán bộ đã
              quen màn hình cũ phải đi tìm lại từng phần. */}
          <TabSoDoToChuc />
          <TabThonToDanPho />
          <TabNguoiDung />
          <TabPhanQuyen />
          <TabDanhMuc />
          <TabThoiHanXuLy />
          {/* Những phần của đặc tả chưa dựng, kèm lý do — ở CUỐI trang để không chen giữa các tab
              đang dùng được (`features/cau-hinh/nhan-cau-hinh.ts`). */}
          <KhoiChuaDung />
        </main>
        </div>
      </PhienProvider>
    </CauHinhXaProvider>
  );
}
