import { COMPANY, CONTACT, OFFICES } from "../../content/company-profile";
import { ChinhSachRiengTu } from "./ChinhSachRiengTu";
import { GlobeGlyph, MailGlyph, NetworkBackdrop, PhoneGlyph, PinGlyph } from "./icons";

/**
 * Screen 4 — contact points.
 *
 * Plain `tel:` / `mailto:` / `https:` anchors, deliberately. Phase 1 collects nothing: there
 * is no form, no `getPhoneNumber`, no OTP, no request to any backend. That is what makes rule
 * 3 (personal data) hold by construction here rather than by argument, and it is what keeps
 * the Zalo review simple — an app that asks for nothing has nothing to justify.
 *
 * The three anchors keep `.action`, whose `min-height: var(--tap-min)` is pinned by
 * accessibility.test.ts: they are the only tappable things on this screen besides the tab bar,
 * and a target under 44px is one an unsteady hand cannot hit.
 *
 * The hotline is ViHAT Group's published corporate number, not an individual's.
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

      <a className="action" href={COMPANY.website} target="_blank" rel="noopener noreferrer">
        <span className="tile tile--soft" aria-hidden="true">
          <GlobeGlyph className="tile__glyph" />
        </span>
        <span>
          <span className="action__label">Website</span>
          <span className="action__value">{COMPANY.website}</span>
        </span>
      </a>

      <h2 className="section-title">
        <span className="section-title__mark" aria-hidden="true">
          <PinGlyph />
        </span>
        Địa chỉ văn phòng
      </h2>
      <ul className="office-list">
        {OFFICES.map((office) => (
          <li className="office" key={office.name}>
            <PinGlyph className="office__glyph" />
            <span className="office__text">
              <strong className="office__name">{office.name}</strong>
              <span className="office__address">{office.address}</span>
            </span>
          </li>
        ))}
      </ul>

      {/* Chính sách quyền riêng tư nằm CUỐI màn Liên hệ, không phải một tab riêng: đây là chỗ
          người tìm thông tin pháp lý đã đứng sẵn, và là chỗ người duyệt Zalo tìm nó. */}
      <ChinhSachRiengTu />
    </>
  );
}
