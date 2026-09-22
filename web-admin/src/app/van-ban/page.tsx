import { CauHinhXaProvider, phanHienThi } from "@/components/cau-hinh-xa";
import { DauTrang } from "@/components/dau-trang";
import { PhienProvider } from "@/features/phien/phien-hien-tai";
import { SoVanBanDen } from "@/features/van-ban/so-van-ban-den";
import { SoVanBanDi } from "@/features/van-ban/so-van-ban-di";
import { layCauHinhXa } from "@/lib/tenant.server";

/**
 * `/van-ban` — Văn bản & Đơn thư (`docs/ui-ux/05-van-ban-don-thu.md`).
 *
 * ĐƯỜNG DẪN ĐÚNG NHƯ ĐẶC TẢ GHI Ở ĐẦU CHƯƠNG (`Route: /van-ban`), và **không thêm đoạn nào**. Sổ
 * văn bản đi KHÔNG có đường dẫn riêng `/van-ban/di`, có chủ ý: tên tài nguyên URL cho một khái
 * niệm chưa có trong bảng ánh xạ là câu phải HỎI, không phải câu để đoán (ADR 0011) — và một đoạn
 * đường dẫn đã chạy thật ở một xã thì không có lần sửa nào rẻ nữa. Hai quyển sổ vì thế dựng nối
 * tiếp trong cùng một trang, đúng khuôn màn Cấu hình đang dùng cho năm phần của nó.
 *
 * CHƯA CÓ THANH TAB: nó đặt ra một câu chưa ai trả lời — tài khoản chỉ mở được một sổ thì thanh
 * ấy hiện một nút đứng trơ, hay không hiện? Dựng nối tiếp không mất gì, vì mỗi sổ đọc dữ liệu của
 * riêng nó và phần nào thiếu quyền thì không gọi tuyến ghi nào.
 *
 * KHÔNG CÓ PHẦN ĐƠN THƯ CÔNG DÂN, dù đặc tả đặt nó chung chương này: hợp đồng REST hôm nay không
 * có tuyến nào cho `don_thu` (`kb/20-contracts/openapi.json`). Vẽ một tab gọi vào tuyến không tồn
 * tại là lời hứa suông — cán bộ bấm, nhận lỗi, và kết luận hệ thống hỏng.
 *
 * BẢO VỆ ĐƯỜNG NẰM Ở `src/proxy.ts`, phía máy chủ, trước khi trang này được dựng — chưa có cookie
 * phiên thì chuyển sang `/dang-nhap`. Nó cố ý KHÔNG kiểm quyền: `document.read`, `document.create`
 * và `document.route` do dịch vụ `documents` kiểm trên TỪNG lời gọi API thật (luật 5, cấm #1).
 *
 * XÃ ĐỌC LÚC CHẠY TỪ `Host`, như mọi trang khác: không có giá trị riêng của xã nào nằm trong
 * bundle, và `Host` không khớp xã nào thì trang này là 404 trước khi dựng gì (luật 1, bất biến 3
 * và 10).
 */
export const dynamic = "force-dynamic";

export const metadata = {
  // Hằng số của sản phẩm, KHÔNG mang tên xã — tên xã hiện trên đầu trang, đọc lúc chạy.
  title: "Văn bản & đơn thư · ViGov",
};

export default async function TrangVanBan() {
  const xa = await layCauHinhXa();

  return (
    <CauHinhXaProvider giaTri={phanHienThi(xa)}>
      <PhienProvider>
        <DauTrang />
        <main className="than-trang">
          <h1>Văn bản &amp; đơn thư</h1>
          <p className="mo-ta-trang">
            Vào sổ văn bản đến, cấp số văn bản đi, phân công xử lý và theo dõi hạn giải quyết.
          </p>
          <SoVanBanDen />
          <SoVanBanDi />
        </main>
      </PhienProvider>
    </CauHinhXaProvider>
  );
}
