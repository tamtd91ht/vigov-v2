import { CauHinhXaProvider } from "@/components/cau-hinh-xa";
import { phanHienThi } from "@/lib/cau-hinh-xa-hien-thi";
import { DauTrang } from "@/components/dau-trang";
import { ThanhBen } from "@/components/thanh-ben";
import { PhienProvider } from "@/features/phien/phien-hien-tai";
import { CongQuyen } from "@/features/quyen/cong-quyen";
import { BangThuChi } from "@/features/thu-chi/bang-thu-chi";
import { CAU_THIEU_QUYEN_XEM } from "@/features/thu-chi/nhan-thu-chi";
import { QUYEN_XEM_GIAI_NGAN } from "@/lib/quyen";
import { layCauHinhXa } from "@/lib/tenant.server";

/**
 * `/giai-ngan/thu-chi` — Thu - Chi ngân sách xã (`docs/ui-ux/07-thu-chi-ngan-sach.md`).
 *
 * ĐƯỜNG DẪN ĐÚNG NHƯ ĐẶC TẢ GHI Ở ĐẦU CHƯƠNG (`Route: /giai-ngan/thu-chi`), không tự dịch thêm
 * đoạn nào: tên tài nguyên URL cho một khái niệm chưa có trong bảng ánh xạ là câu phải HỎI, không
 * phải câu để đoán (ADR 0011).
 *
 * BẢO VỆ ĐƯỜNG NẰM Ở `src/proxy.ts`, phía máy chủ, trước khi trang này được dựng — chưa có cookie
 * phiên thì chuyển sang `/dang-nhap`. Nó cố ý KHÔNG kiểm quyền: ba khoá `budget.*` do dịch vụ
 * `finance` kiểm trên TỪNG lời gọi API thật. `CongQuyen` dưới đây chỉ để cán bộ thiếu quyền không
 * phải nhìn một bảng chắc chắn trả 403 (luật 5, cấm #1).
 *
 * MỘT CỔNG DUY NHẤT Ở ĐÂY, VÀ NÓ LÀ `budget.read`: hai khoá còn lại — `budget.update` (lập bảng,
 * thêm, sửa) và `budget.confirm` (**gỡ**, và **đánh dấu dòng tổng**) — ẩn/hiện từng NÚT bên trong
 * `BangThuChi`, vì một tài khoản chỉ đọc vẫn phải xem được cả bảng.
 *
 * XÃ ĐỌC LÚC CHẠY TỪ `Host`, như mọi trang khác: không có giá trị riêng của xã nào nằm trong
 * bundle, và `Host` không khớp xã nào thì trang này là 404 trước khi dựng gì (luật 1, bất biến 3
 * và 10).
 */
export const dynamic = "force-dynamic";

export const metadata = {
  // Hằng số của sản phẩm, KHÔNG mang tên xã — tên xã hiện trên đầu trang, đọc lúc chạy.
  title: "Thu - Chi ngân sách · ViGov",
};

export default async function TrangThuChiNganSach() {
  const xa = await layCauHinhXa();

  return (
    <CauHinhXaProvider giaTri={phanHienThi(xa)}>
      <PhienProvider>
        <div className="khung-trang">
          <ThanhBen />
          <DauTrang />
          <main className="than-trang">
            <h1>Thu - Chi ngân sách xã</h1>
            {/* MÔ TẢ TRANG KHÔNG CHÉP NGUYÊN VĂN §1, và đó là một lựa chọn có lý do. Câu của đặc
                tả mở đầu bằng "Nạp thẳng tệp Excel của Phòng Tài chính" — một chức năng hợp đồng
                hôm nay KHÔNG có tuyến nào phía sau. In nguyên câu ấy lên đầu trang là hứa với cán
                bộ một việc họ sẽ đi tìm nút để làm và không thấy. Khối "chưa dựng được" trong màn
                nói đủ vì sao. */}
            <p className="mo-ta-trang">
              Bảng thu và bảng chi ngân sách của đơn vị theo từng năm: khoản mục dựng thành cây, số
              liệu nhập trên lưới, và ba chỉ số của năm.
            </p>
            {/* Câu thiếu quyền nằm ở `nhan-thu-chi.ts`, không viết thẳng ở đây: nhánh ấy là nhánh
                người viết mã không bao giờ nhìn thấy, nên nó phải kiểm được bằng một bài test. */}
            <CongQuyen khoa={QUYEN_XEM_GIAI_NGAN} cauThieuQuyen={CAU_THIEU_QUYEN_XEM}>
              <BangThuChi />
            </CongQuyen>
          </main>
        </div>
      </PhienProvider>
    </CauHinhXaProvider>
  );
}
