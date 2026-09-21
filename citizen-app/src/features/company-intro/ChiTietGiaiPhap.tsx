import { COMPANY, CONTACT, type Solution } from "../../content/company-profile";

import { MailGlyph, PhoneGlyph, SOLUTION_GLYPHS } from "./icons";
import { NutWebsite } from "./NutWebsite";

/**
 * Trang riêng của MỘT giải pháp — mở ra từ thẻ trên màn Giải pháp.
 *
 * ⚠ MÀN NÀY KHÔNG BIẾT GÌ HƠN MÀN DANH SÁCH, VÀ ĐÓ LÀ CHỦ ĐÍCH.
 *
 *   Một trang chi tiết mời gọi đúng một việc: viết thêm cho đầy. Bản mẫu có chỗ cho tính năng,
 *   khách hàng tiêu biểu, con số theo sản phẩm — và **không một dòng nào trong số đó được công bố**
 *   trên vihatgroup.com hay vihatsoftware.com. Viết ra là bịa một tuyên bố về sản phẩm của một
 *   pháp nhân có thật, trong một hồ sơ Zalo đọc.
 *
 *   Nên màn này chỉ có ba thứ, và cả ba đều có nguồn: hai câu nguyên văn đã công bố, và ba đường
 *   liên hệ đã công bố để người đọc hỏi tiếp. Nó ngắn. Một trang ngắn và đúng đọc tốt hơn hẳn một
 *   trang dài mà người bán hàng phải đính chính.
 *
 * VÌ SAO BA ĐƯỜNG LIÊN HỆ LẶP LẠI Ở ĐÂY thay vì để người đọc tự sang tab Liên hệ: người vừa đọc
 * xong một giải pháp là người đang có câu hỏi về đúng giải pháp ấy. Bắt họ nhớ một tab rồi đi tìm
 * lại là chỗ người ta bỏ cuộc — và đây là ba neo chạy được ngay, không xin quyền gì.
 */
export function ChiTietGiaiPhap(props: { giai_phap: Solution; onQuayLai: () => void }) {
  const { giai_phap } = props;
  const Glyph = SOLUTION_GLYPHS[giai_phap.id];

  return (
    <>
      {/* NÚT QUAY LẠI LÀ MỘT NÚT THẬT, CÓ CHỮ, CAO ĐỦ 44px. Nút back của Zalo đóng cả Mini App
          chứ không quay về danh sách, nên một màn con không có đường về của riêng nó là một ngõ
          cụt — và người gặp ngõ cụt đầu tiên là người ít kiên nhẫn nhất với nó. */}
      <button type="button" className="quay-lai" onClick={props.onQuayLai}>
        ‹ Tất cả giải pháp
      </button>

      <section className="banner banner--giai-phap">
        <div className="banner__content">
          <span className="tile tile--lon" aria-hidden="true">
            <Glyph className="tile__glyph" />
          </span>
          <h1 className="banner__title">{giai_phap.product ?? giai_phap.headline}</h1>
        </div>
      </section>

      <section className="card">
        {/* Khi thẻ mang tên sản phẩm thì `<h1>` ở trên là tên ấy, và câu headline là nội dung.
            Khi không có tên sản phẩm thì headline ĐÃ là `<h1>`, và in lại nó ngay dưới là đọc
            cùng một câu hai lần liền nhau. */}
        {giai_phap.product ? <p className="card__body card__body--lon">{giai_phap.headline}</p> : null}
        {giai_phap.note ? <p className="card__body">{giai_phap.note}</p> : null}
        {giai_phap.product === undefined && giai_phap.note === undefined ? (
          <p className="card__body">
            Đây là một dòng giải pháp trong hệ sinh thái của {COMPANY.name}.
          </p>
        ) : null}
      </section>

      <h2 className="section-title">Hỏi thêm về giải pháp này</h2>

      <a className="action action--primary" href={`tel:${CONTACT.hotlineDialable}`}>
        <span className="tile" aria-hidden="true">
          <PhoneGlyph className="tile__glyph" />
        </span>
        <span>
          <span className="action__label">Gọi hotline</span>
          <span className="action__value">{CONTACT.hotlineLabel}</span>
        </span>
      </a>

      <a className="action" href={`mailto:${CONTACT.email}`}>
        <span className="tile tile--soft" aria-hidden="true">
          <MailGlyph className="tile__glyph" />
        </span>
        <span>
          <span className="action__label">Gửi email</span>
          <span className="action__value">{CONTACT.email}</span>
        </span>
      </a>

      {/* CÙNG MỘT NÚT với màn Liên hệ, một chỗ viết — và CÙNG MỘT ĐÍCH ĐẾN, nên chính sách vẫn
          đếm nó là MỘT dòng. Xem `content/dich-ra-ngoai.ts` về việc đếm theo đích chứ không theo
          số nút. */}
      <NutWebsite />
    </>
  );
}
