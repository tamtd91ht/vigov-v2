import { CauHinhXaProvider, phanHienThi } from "@/components/cau-hinh-xa";
import { DauTrang } from "@/components/dau-trang";
import { PhienProvider } from "@/features/phien/phien-hien-tai";
import { TabDanhMuc } from "@/features/cau-hinh/tab-danh-muc";
import { TabNguoiDung } from "@/features/cau-hinh/tab-nguoi-dung";
import { TabPhanQuyen } from "@/features/cau-hinh/tab-phan-quyen";
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
 * MƯỜI TAB CỦA §0, Ở ĐÂY CÓ BỐN. Sáu tab còn lại — Sơ đồ tổ chức, Trường bản đồ, Lời hệ thống,
 * Thời hạn xử lý, Tự động hoá, Máy chủ thư — chưa có tuyến nào trong hợp đồng REST. Một thanh
 * mười tab mà sáu tab bấm vào không ra gì là sáu lần hứa hẹn suông, nên thanh tab chỉ mọc thêm
 * khi tuyến mọc thêm.
 *
 * VÀ Ở ĐÂY CHƯA CÓ THANH TAB NÀO: bốn phần dựng nối tiếp trong trang, theo đúng thứ tự tab của
 * đặc tả §0. Chưa dựng thanh chuyển tab vì nó đặt ra một câu chưa ai trả lời: tài khoản chỉ mở
 * được MỘT tab thì thanh ấy hiện một nút đứng trơ, hay không hiện? Đó là quyết định về giao diện
 * của khách, và đoán hộ thì phải đoán lại khi tab thứ năm mọc lên. Dựng nối tiếp không mất gì:
 * mỗi phần vẫn đọc dữ liệu của riêng nó, và phần nào thiếu quyền thì không gọi tuyến nào.
 *
 * HAI TRONG BỐN PHẦN TỰ ẨN/HIỆN THEO KHOÁ QUYỀN (`admin.user` · `admin.role`), HAI PHẦN KIA
 * KHÔNG — và sự khác nhau ấy đến từ máy chủ chứ không từ đây. Tuyến sau tab Người dùng và tab
 * Phân quyền khai `RequirePermission`; tám tuyến danh mục và địa bàn khai `any-authenticated`.
 * Dựng một cổng quyền ở giao diện cho hai tab sau sẽ là để GIAO DIỆN quyết định điều máy chủ
 * không từ chối — đúng hình dạng luật 5 cấm #1. Lý lẽ đầy đủ ở `features/cau-hinh/tab-danh-muc.tsx`.
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
        <DauTrang />
        <main className="than-trang">
          <h1>Cấu hình hệ thống</h1>
          <p className="mo-ta-trang">
            Tổ chức, phân quyền, danh mục nghiệp vụ và thời hạn xử lý của đơn vị.
          </p>
          {/* Thứ tự tab của đặc tả §0, giữ nguyên: Thôn/Tổ dân phố · Người dùng · Phân quyền ·
              Danh mục. Sắp lại theo thứ tự dựng xong sẽ làm cán bộ đã quen màn hình cũ phải đi
              tìm lại từng phần. */}
          <TabThonToDanPho />
          <TabNguoiDung />
          <TabPhanQuyen />
          <TabDanhMuc />
        </main>
      </PhienProvider>
    </CauHinhXaProvider>
  );
}
