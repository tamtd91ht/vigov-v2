/**
 * LỜI MỜI ĐĂNG NHẬP — hiện THAY CHO nội dung, không hiện cạnh nó.
 *
 * ⚠ HAI MÀN CỦA BỀ MẶT YÊU CẦU ĐỀU DÙNG KHỐI NÀY, VÀ CẢ HAI ĐỀU DỪNG HẲN Ở ĐÂY KHI CHƯA CÓ PHIÊN.
 *
 *   Vẽ biểu mẫu ra rồi báo "chưa đăng nhập" lúc bấm gửi là một lần người dùng gõ xong bị mất chữ.
 *   Với người vừa gõ một đoạn câu hỏi, đó là lần cuối họ gõ. Và với màn "Yêu cầu của tôi" thì còn
 *   tệ hơn một bậc: một danh sách rỗng khi chưa đăng nhập đọc ra thành "tôi chưa gửi yêu cầu nào"
 *   — một câu SAI, và người đọc nó sẽ gửi lại lần nữa.
 *
 * ⚠ HOTLINE Ở LẠI NGAY DƯỚI NÚT, VÀ NÓ KHÔNG PHẢI TRANG TRÍ. `tel:` là đường DUY NHẤT của cả bề
 * mặt này chạy được khi chưa đăng nhập. Người đang cần gấp không nên bị buộc đăng nhập trước.
 */
import { CONTACT } from "../../content/company-profile";
import { PhoneGlyph, ShieldGlyph } from "../company-intro/icons";

import { TU_VAN } from "./noi-dung";

export function MoiDangNhap({
  tieu_de,
  li_do,
  nhan_nut,
  onDangNhap,
}: {
  tieu_de: string;
  li_do: string;
  nhan_nut: string;
  onDangNhap: () => void;
}) {
  return (
    <section className="card">
      <span className="tile tile--soft" aria-hidden="true">
        <ShieldGlyph className="tile__glyph" />
      </span>
      <h2 className="card__title">{tieu_de}</h2>
      <p className="card__body">{li_do}</p>

      <button type="button" className="tn__nut" onClick={onDangNhap}>
        {nhan_nut}
      </button>

      <p className="tn__giai-thich">{TU_VAN.van_goi_duoc}</p>
      {/* `tel:` — không phải một lời gọi nền tảng, và không cần khai thêm quyền nào. Xem khối
          chú thích của `MenuNhanh` về vì sao `openPhone` KHÔNG được mượn cho việc này. */}
      <a className="tn-hanh-dong tn-hanh-dong--rong" href={`tel:${CONTACT.hotlineDialable}`}>
        <PhoneGlyph className="tn-hanh-dong__glyph" />
        Gọi {CONTACT.hotlineLabel}
      </a>
    </section>
  );
}
