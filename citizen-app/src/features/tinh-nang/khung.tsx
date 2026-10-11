/**
 * KHUNG CHUNG CỦA BA TÍNH NĂNG — một tiêu đề, một lý do, một nút, một chỗ hiện kết quả.
 *
 * VÌ SAO MỖI TÍNH NĂNG PHẢI NÓI VÌ SAO NÓ CẦN QUYỀN ẤY, NGAY CẠNH CÁI NÚT:
 *
 *   Zalo chỉ cấp `getPhoneNumber` · `getLocation` · `scanQRCode` khi bản nộp CÓ chỗ dùng chúng
 *   nhìn thấy được, và chính sách Mini App (điều 3.3.4, trích ngay trên `getPhoneNumber` trong
 *   `zmp-sdk/index.d.ts`) nói thẳng: *"chúng tôi sẽ từ chối xét duyệt cho những Mini App có luồng
 *   xin cấp quyền chưa rõ ràng, không nêu được mục đích xin quyền đến người dùng"*. Một nút trần
 *   không kèm lời giải thích là một vòng duyệt trượt.
 *
 *   Và lời giải thích ấy không chỉ để qua vòng duyệt: người bấm "Đồng ý" mà không biết mình đồng
 *   ý cho việc gì là người sẽ gỡ app ngay lần đầu thấy lạ.
 *
 * ⚠ TỪ CHỐI LÀ ĐƯỜNG ĐI BÌNH THƯỜNG, KHÔNG PHẢI LỖI. Cả ba đều có nhánh riêng cho nó, và câu
 * hiện ra nói rõ vẫn dùng được ứng dụng và bấm lại lúc nào cũng được — không màu đỏ, không mã lỗi.
 *
 * ⚠ BA TÍNH NĂNG NÀY LẤY DỮ LIỆU RỒI HIỆN LÊN MÀN HÌNH, HẾT. Không `fetch`, không `localStorage`,
 * không `console.log` — các dây bẫy trong `phase1-collects-nothing.test.ts` giữ nguyên, và chúng
 * là thứ biến câu "không gửi đi đâu" thành một ràng buộc kiểm được thay vì một lời hứa.
 */
import { type ReactNode, useState } from "react";

import { type MaTinhNang, noiDung } from "./noi-dung";
import type { KetQuaXin } from "./zalo-api";

export type TrangThai<T = string> = { kieu: "chua-goi" } | { kieu: "dang-cho" } | KetQuaXin<T>;

/**
 * `id` của tiêu đề một khối tính năng — và cũng là MỎ NEO để một màn khác cuộn tới nó.
 *
 * MỘT HÀM, KHÔNG PHẢI MỘT CHUỖI GÕ Ở HAI CHỖ. Menu nhanh trên màn chủ có hai mục dẫn thẳng tới
 * một khối tính năng nằm giữa một màn khác ("Văn phòng", "QR"). Nếu mỏ neo ấy được gõ tay ở
 * `features/company-intro/` thì ngày ai đó đổi quy ước `id` ở đây, hai mục menu im lặng không cuộn
 * đi đâu cả — người bấm thấy đúng đầu màn và tự kết luận là nút hỏng. Một hàm thì cả hai bên đọc
 * cùng một chỗ.
 */
export function mocTinhNang(ma: MaTinhNang): string {
  return `tn-${ma}`;
}

/**
 * Khung một tính năng, THUẦN — nhận cả trạng thái qua tham số, không giữ gì.
 *
 * Tách ra vì bộ test ở đây dựng bằng `react-dom/server` và không có DOM để bấm. Bốn nhánh kết
 * quả — nhất là `tu-choi` và `ngoai-zalo`, hai nhánh người dùng gặp nhiều nhất — chỉ kiểm được
 * khi dựng thẳng được chúng. Một câu nói với người vừa từ chối mà không ai kiểm là một câu sẽ
 * trôi thành "Lỗi".
 *
 * `cap_tieu_de` có mặt vì một màn hình chỉ được có MỘT `<h1>`: ở tab Danh thiếp khung này là
 * nội dung chính, còn trên màn Liên hệ nó là một khối trong màn đã có tiêu đề riêng.
 */
export function KhungTinhNang<T>({
  ma,
  trang_thai,
  onBam,
  cap_tieu_de = "h1",
  glyph,
  truoc_nut,
  veKetQua,
  duoi_cung,
}: {
  ma: MaTinhNang;
  trang_thai: TrangThai<T>;
  onBam: () => void;
  cap_tieu_de?: "h1" | "h2";
  /** Hình trang trí cạnh tiêu đề. `aria-hidden` như mọi glyph khác — chữ mới là nội dung. */
  glyph?: ReactNode;
  /**
   * Phần đứng GIỮA lời giải thích và cái nút.
   *
   * Có mặt vì một tính năng có thể đã cho người dùng thứ họ cần TRƯỚC khi bấm gì cả: mã QR danh
   * thiếp của chúng tôi hiện ngay, không cần quyền nào, và cái nút bên dưới chỉ là đường tải nó
   * về. Đẩy mã xuống dưới nút thì thứ chính của màn hình nằm sau một lời xin quyền — đúng thứ
   * tự ngược với mọi màn còn lại của ứng dụng này.
   */
  truoc_nut?: ReactNode;
  veKetQua: (du_lieu: T) => ReactNode;
  /** Phần luôn hiện, bất kể người dùng đã bấm hay chưa (danh sách văn phòng, đường liên hệ). */
  duoi_cung?: ReactNode;
}) {
  const nd = noiDung(ma);
  const dang_cho = trang_thai.kieu === "dang-cho";
  const TieuDe = cap_tieu_de;

  return (
    <section className="tn hien-len" aria-labelledby={mocTinhNang(ma)}>
      <div className="tn__dau">
        {glyph !== undefined && (
          <span className="tn__huy-hieu" aria-hidden="true">
            {glyph}
          </span>
        )}
        <TieuDe className="tn__tieu-de" id={mocTinhNang(ma)}>
          {nd.tieu_de}
        </TieuDe>
      </div>
      <p className="tn__vi-sao">{nd.vi_sao}</p>

      {truoc_nut}

      <button
        type="button"
        className="tn__nut"
        onClick={onBam}
        disabled={dang_cho}
        aria-busy={dang_cho}
      >
        {/* Trạng thái "đang chờ" nói bằng CHỮ trên chính cái nút, không bằng riêng màu nền. */}
        {dang_cho ? nd.dang_cho : nd.nut}
      </button>

      {/* `role="status"` để trình đọc màn hình đọc kết quả ra khi nó hiện. Không có nó thì người
          khiếm thị bấm nút xong không biết có gì xảy ra hay không. */}
      <div className="tn__ket-qua" role="status">
        {trang_thai.kieu === "xong" && veKetQua(trang_thai.du_lieu)}
        {trang_thai.kieu === "tu-choi" && <p className="tn__loi">{nd.tu_choi}</p>}
        {trang_thai.kieu === "ngoai-zalo" && <p className="tn__loi">{nd.ngoai_zalo}</p>}
        {trang_thai.kieu === "khong-lay-duoc" && <p className="tn__loi">{nd.khong_lay_duoc}</p>}
      </div>

      {duoi_cung}
    </section>
  );
}

/** Tính năng có trạng thái: bấm → chờ → một trong bốn nhánh. Không giữ gì sau khi đóng app. */
export function TinhNangCoTrangThai<T>({
  ma,
  xin,
  cap_tieu_de,
  glyph,
  truoc_nut,
  veKetQua,
  duoi_cung,
}: {
  ma: MaTinhNang;
  xin: () => Promise<KetQuaXin<T>>;
  cap_tieu_de?: "h1" | "h2";
  glyph?: ReactNode;
  truoc_nut?: ReactNode;
  veKetQua: (du_lieu: T) => ReactNode;
  duoi_cung?: ReactNode;
}) {
  const [trang_thai, datTrangThai] = useState<TrangThai<T>>({ kieu: "chua-goi" });

  async function bam() {
    datTrangThai({ kieu: "dang-cho" });
    // `xin` không bao giờ ném — mọi đường đã quy về bốn nhánh trong `zalo-api.ts`. Không có
    // `catch` ở đây vì một `catch` thừa sẽ che mất lỗi lập trình thật của chính màn này.
    datTrangThai(await xin());
  }

  return (
    <KhungTinhNang
      ma={ma}
      trang_thai={trang_thai}
      onBam={() => void bam()}
      cap_tieu_de={cap_tieu_de}
      glyph={glyph}
      truoc_nut={truoc_nut}
      veKetQua={veKetQua}
      duoi_cung={duoi_cung}
    />
  );
}
