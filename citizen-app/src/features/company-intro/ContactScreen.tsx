import { COMPANY, CONTACT, OFFICES } from "../../content/company-profile";

/**
 * Screen 4 — contact points.
 *
 * Plain `tel:` / `mailto:` / `https:` anchors, deliberately. Phase 1 collects nothing: there
 * is no form, no `getPhoneNumber`, no OTP, no request to any backend. That is what makes rule
 * 3 (personal data) hold by construction here rather than by argument, and it is what keeps
 * the Zalo review simple — an app that asks for nothing has nothing to justify.
 *
 * The hotline is ViHAT Group's published corporate number, not an individual's.
 */
export function ContactScreen() {
  return (
    <>
      <h1 className="screen-heading">Liên hệ</h1>
      <p className="screen-lead">{CONTACT.ownerNote}</p>

      <a className="action action--primary" href={`tel:${CONTACT.hotlineDialable}`}>
        <span>
          <span className="action__label">Gọi hotline</span>
          <span className="action__value">{CONTACT.hotlineLabel}</span>
        </span>
      </a>

      <a className="action" href={`mailto:${CONTACT.email}`}>
        <span>
          <span className="action__label">Gửi email</span>
          <span className="action__value">{CONTACT.email}</span>
        </span>
      </a>

      <a className="action" href={COMPANY.website} target="_blank" rel="noopener noreferrer">
        <span>
          <span className="action__label">Website</span>
          <span className="action__value">{COMPANY.website}</span>
        </span>
      </a>

      <h2 className="card__title">Địa chỉ văn phòng</h2>
      <ul className="list">
        {OFFICES.map((office) => (
          <li className="list__item" key={office.name}>
            <strong>{office.name}</strong>
            <br />
            {office.address}
          </li>
        ))}
      </ul>
    </>
  );
}
