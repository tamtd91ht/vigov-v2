import { COMPANY, CONTACT } from "../../content/company-profile";
import { DangKyTuVan, TimVanPhong } from "../tinh-nang/index";

import { ChinhSachRiengTu } from "./ChinhSachRiengTu";
import { GlobeGlyph, MailGlyph, NetworkBackdrop, PhoneGlyph } from "./icons";

/**
 * Màn Liên hệ — và là nơi hai trong ba tính năng thật của ứng dụng sống.
 *
 * THỨ TỰ TRÊN MÀN LÀ THỨ TỰ CHẠY ĐƯỢC NGAY → CẦN THÊM MỘT BƯỚC:
 *
 *   1. Hotline · email · website. Ba thứ này chạy được ngay bây giờ, không cần quyền nào, không
 *      cần máy chủ nào. Người muốn nói chuyện với công ty phải gặp chúng trước tiên.
 *   2. Đăng ký nhận tư vấn (`getPhoneNumber`) — nhận được mã, và nói thẳng rằng bản này chưa gửi
 *      yêu cầu đi đâu, kèm lại hai đường liên hệ chạy được ngay.
 *   3. Tìm văn phòng gần bạn (`getLocation`) — ba văn phòng và nút chỉ đường luôn hiện, chia sẻ
 *      vị trí hay không cũng vậy.
 *
 *   Đặt hai tính năng cần quyền lên trước ba đường liên hệ chạy được là bắt một khách hàng đang
 *   cần gọi phải đi qua hai lời xin quyền trước đã. Không ai làm thế với một khách hàng.
 *
 * Ba neo đầu giữ lớp `.action`, có `min-height: var(--tap-min)` được `accessibility.test.ts`
 * ghim: một đích chạm dưới 44px là đích một bàn tay run không bấm trúng.
 *
 * Hotline và email là đầu mối doanh nghiệp đã công bố của ViHAT Group, không phải của một cá
 * nhân nào — nên chúng là dữ liệu doanh nghiệp, không phải dữ liệu cá nhân.
 */
export function ContactScreen() {
  return (
    <>
      <section className="banner banner--contact">
        <NetworkBackdrop className="banner__backdrop" />
        <div className="banner__content">
          <h1 className="banner__title">Liên hệ</h1>
          <p className="banner__lead">{CONTACT.ownerNote}</p>
        </div>
      </section>

      <a className="action action--primary hien-len" href={`tel:${CONTACT.hotlineDialable}`}>
        <span className="tile" aria-hidden="true">
          <PhoneGlyph className="tile__glyph" />
        </span>
        <span>
          <span className="action__label">Gọi hotline</span>
          <span className="action__value">{CONTACT.hotlineLabel}</span>
        </span>
      </a>

      <a className="action hien-len hien-len--2" href={`mailto:${CONTACT.email}`}>
        <span className="tile tile--soft" aria-hidden="true">
          <MailGlyph className="tile__glyph" />
        </span>
        <span>
          <span className="action__label">Gửi email</span>
          <span className="action__value">{CONTACT.email}</span>
        </span>
      </a>

      <a className="action hien-len hien-len--3" href={COMPANY.website} target="_blank" rel="noopener noreferrer">
        <span className="tile tile--soft" aria-hidden="true">
          <GlobeGlyph className="tile__glyph" />
        </span>
        <span>
          <span className="action__label">Website</span>
          <span className="action__value">{COMPANY.website}</span>
        </span>
      </a>

      <DangKyTuVan />
      <TimVanPhong />

      {/* Chính sách quyền riêng tư nằm CUỐI màn Liên hệ, không phải một tab riêng: đây là chỗ
          người tìm thông tin pháp lý đã đứng sẵn, và là chỗ người duyệt Zalo tìm nó. */}
      <ChinhSachRiengTu />
    </>
  );
}
