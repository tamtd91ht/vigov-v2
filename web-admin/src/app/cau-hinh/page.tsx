import { CauHinhXaProvider, phanHienThi } from "@/components/cau-hinh-xa";
import { DauTrang } from "@/components/dau-trang";
import { PhienProvider } from "@/features/phien/phien-hien-tai";
import { TabNguoiDung } from "@/features/cau-hinh/tab-nguoi-dung";
import { TabPhanQuyen } from "@/features/cau-hinh/tab-phan-quyen";
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
 * MƯỜI TAB CỦA §0, Ở ĐÂY CÓ HAI. Tám tab còn lại — Sơ đồ tổ chức, Thôn/Tổ dân phố, Danh mục,
 * Trường bản đồ, Lời hệ thống, Thời hạn xử lý, Tự động hoá, Máy chủ thư — chưa có tuyến nào trong
 * hợp đồng REST. Một thanh mười tab mà tám tab bấm vào không ra gì là tám lần hứa hẹn suông, nên
 * thanh tab chỉ mọc thêm khi tuyến mọc thêm.
 *
 * VÀ Ở ĐÂY CHƯA CÓ THANH TAB NÀO: hai phần dựng nối tiếp trong trang, mỗi phần TỰ ẩn/hiện theo
 * khoá quyền của nó (`admin.user` · `admin.role`). Chưa dựng thanh chuyển tab vì nó đặt ra một
 * câu chưa ai trả lời: tài khoản chỉ mở được MỘT tab thì thanh ấy hiện một nút đứng trơ, hay
 * không hiện? Đó là quyết định về giao diện của khách, và đoán hộ thì phải đoán lại khi tab thứ
 * ba mọc lên. Dựng nối tiếp không mất gì: mỗi phần vẫn đọc dữ liệu của riêng nó, và phần nào
 * thiếu quyền thì không gọi tuyến nào.
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
          <TabNguoiDung />
          <TabPhanQuyen />
        </main>
      </PhienProvider>
    </CauHinhXaProvider>
  );
}
