/**
 * TÍNH NĂNG KIỂM TRA ĐƯỜNG TRUYỀN — `getNetworkType` + `vibrate`.
 *
 * VÌ SAO TÍNH NĂNG NÀY THUỘC VỀ ĐÚNG ỨNG DỤNG NÀY: VihatSoftware bán tổng đài đám mây. Một cuộc
 * gọi của tổng đài đám mây đi qua chính đường mạng của máy người dùng, nên "đang đi bằng Wi-Fi
 * hay bằng mạng di động" là câu hỏi đầu tiên của mọi cuộc gọi nghe không rõ. Nó nằm trên màn
 * Giải pháp, ngay dưới phần nói về tổng đài — không phải trong một tab tên "Quyền".
 *
 * ⚠ KHÔNG BỊA MỘT CON SỐ NÀO. Nền tảng trả về đúng một chuỗi: kiểu kết nối. Không độ trễ, không
 * băng thông, không điểm chất lượng. Màn hình này nói đúng chừng ấy và nói thêm rằng nó KHÔNG
 * đo được những thứ kia (`DUONG_TRUYEN.khong_do_toc_do`). Một con số bịa ra trong ứng dụng của
 * một nhà cung cấp hạ tầng thoại là con số khách hàng của họ sẽ đem đối chiếu với máy đo thật.
 *
 * `vibrate` rung một nhịp khi có kết quả: người vừa bấm nút thường đang nhìn đi chỗ khác, và
 * một nhịp rung là cách rẻ nhất để nói "xong" mà không cần họ nhìn lại màn hình.
 */
import { CloudGlyph } from "../company-intro/icons";

import { TinhNangCoTrangThai } from "./khung";
import { DUONG_TRUYEN, KIEU_KET_NOI, kieuKetNoi } from "./noi-dung";
import { docKieuKetNoi, rungMotNhip } from "./zalo-api";

/**
 * Kết quả một lần đo, THUẦN — nhận chuỗi thô của nền tảng, trả về phần hiện ra.
 *
 * Thuần để kiểm được cả bốn giá trị nền tảng khai HÔM NAY cộng một giá trị lạ, không cần điện
 * thoại. Giá trị lạ là trường hợp đáng kiểm nhất: nó là thứ xảy ra khi nền tảng tốt lên, và một
 * tính năng vỡ vì nền tảng tốt lên là một tính năng vỡ sau khi app đã nằm trên máy người dùng.
 */
export function KetQuaDuongTruyen({ tu_nen_tang }: { tu_nen_tang: string }) {
  const kieu = KIEU_KET_NOI[kieuKetNoi(tu_nen_tang)];

  return (
    <>
      {/* Nhãn nói bằng CHỮ, không bằng riêng màu hay riêng một biểu tượng vạch sóng. */}
      <p className="tn__xong">
        {DUONG_TRUYEN.nhan_ket_qua}: {kieu.nhan}
      </p>
      <p className="tn__giai-thich">{kieu.y_nghia}</p>
      <p className="tn__ranh-gioi">{DUONG_TRUYEN.khong_do_toc_do}</p>
    </>
  );
}

export function KiemTraDuongTruyen() {
  return (
    <TinhNangCoTrangThai
      ma="duong-truyen"
      xin={async () => {
        const ket_qua = await docKieuKetNoi();
        // Rung SAU khi đã có kết quả, và chỉ khi có kết quả: rung lúc vừa bấm là rung cho một
        // việc chưa xong. `rungMotNhip` không bao giờ ném — xem `zalo-api.ts`.
        if (ket_qua.kieu === "xong") await rungMotNhip();
        return ket_qua;
      }}
      cap_tieu_de="h2"
      glyph={<CloudGlyph className="tn__glyph" />}
      dan_nhap={DUONG_TRUYEN.dan_nhap}
      veKetQua={(tu_nen_tang) => <KetQuaDuongTruyen tu_nen_tang={tu_nen_tang} />}
    />
  );
}
