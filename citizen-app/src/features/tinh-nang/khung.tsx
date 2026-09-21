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

import { CHI_HIEN_LEN_MAN_HINH, type MaTinhNang, MA_RONG, noiDung, TOKEN_KHONG_CHUA_GI } from "./noi-dung";
import type { KetQuaXin } from "./zalo-api";

/**
 * Che token, chỉ để vài ký tự đầu.
 *
 * Token KHÔNG chứa số điện thoại hay toạ độ (xem `zalo-api.ts`), nên đây không phải chuyện dữ
 * liệu cá nhân — nó là một CHỨNG TỪ đổi được dữ liệu ở máy chủ. Hiện trọn vẹn lên màn hình là
 * mời người đứng cạnh chụp lại trong hai phút nó còn sống. Vài ký tự đầu đủ để người duyệt thấy
 * "đã nhận được một chuỗi thật", và không đủ để dùng lại.
 */
export function cheToken(token: string): string {
  return token.length <= 6 ? "…" : `${token.slice(0, 6)}…`;
}

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
  dan_nhap,
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
  /** Câu dẫn riêng của tính năng, đứng trên nút. */
  dan_nhap?: string;
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
      {dan_nhap !== undefined && <p className="tn__dan">{dan_nhap}</p>}
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

      <p className="tn__loi-hua">{CHI_HIEN_LEN_MAN_HINH}</p>
    </section>
  );
}

/** Tính năng có trạng thái: bấm → chờ → một trong bốn nhánh. Không giữ gì sau khi đóng app. */
export function TinhNangCoTrangThai<T>({
  ma,
  xin,
  cap_tieu_de,
  glyph,
  dan_nhap,
  truoc_nut,
  veKetQua,
  duoi_cung,
}: {
  ma: MaTinhNang;
  xin: () => Promise<KetQuaXin<T>>;
  cap_tieu_de?: "h1" | "h2";
  glyph?: ReactNode;
  dan_nhap?: string;
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
      dan_nhap={dan_nhap}
      truoc_nut={truoc_nut}
      veKetQua={veKetQua}
      duoi_cung={duoi_cung}
    />
  );
}

/**
 * Kết quả của hai tính năng dùng token: độ dài, vài ký tự đầu, và vì sao màn hình này không có
 * gì để che.
 *
 * `noi_them` là câu nói ra thứ bản dựng này CHƯA làm được với cái token ấy. NÓ TUỲ CHỌN, và
 * khác biệt ấy có lý do: với `van-phong` thì bản dựng thật sự dừng ở chỗ nhận được mã — không
 * xếp được văn phòng theo khoảng cách — nên có một câu ranh giới để nói. Với `dang-nhap` thì
 * KHÔNG CÓ RANH GIỚI NÀO ĐỂ NÓI: mã đi thẳng tới máy chủ và một phiên được mở, nên phần đuôi là
 * TRẠNG THÁI THẬT của lời gọi ấy (`features/dang-nhap/PhatHanhPhien.tsx`), không phải một câu
 * ghim sẵn. Ghim một câu "bản này chưa làm gì" ở đây là nói dối bằng giao diện.
 */
export function KetQuaToken({
  ma,
  token,
  da_nhan,
  noi_them,
}: {
  ma: "dang-nhap" | "van-phong";
  token: string;
  da_nhan: string;
  noi_them?: string;
}) {
  if (token === "") return <p className="tn__loi">{MA_RONG}</p>;
  return (
    <>
      <p className="tn__xong">{da_nhan}</p>
      <ul className="tn__do">
        <li>Độ dài mã: {token.length} ký tự</li>
        <li>Vài ký tự đầu: {cheToken(token)}</li>
      </ul>
      <p className="tn__giai-thich">{TOKEN_KHONG_CHUA_GI[ma]}</p>
      {noi_them !== undefined && <p className="tn__ranh-gioi">{noi_them}</p>}
    </>
  );
}
