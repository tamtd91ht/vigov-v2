import { BRAND_STATEMENTS, CERTIFICATES, GROUP } from "../../content/company-profile";
import { CompassGlyph, HandshakeGlyph, NetworkBackdrop, ShieldGlyph, TargetGlyph } from "./icons";

/**
 * Screen 3 — philosophy, vision, mission and certificates.
 *
 * All four belong to ViHAT GROUP, so the screen says so once at the top rather than repeating
 * a disclaimer on every card. A certificate attributed to the wrong entity is a claim about
 * an audit that entity never passed.
 *
 * WHY THE GLYPHS ARE POSITIONAL AND NOT PART OF THE CONTENT FILE:
 *
 *   The three statements are published in a fixed order (company-profile.test.ts pins it), so
 *   the glyph is chosen by position. It is decoration: nothing on this screen is readable only
 *   through a picture, and the statement titles say in words what each card is.
 *
 * The certificates stay LAST on this screen. The Zalo accreditation is published without a
 * scope, and this app prints none — screens.test.tsx checks that its list item carries the name
 * and nothing else, which only holds while nothing further is appended after it.
 */
const STATEMENT_GLYPHS = [HandshakeGlyph, CompassGlyph, TargetGlyph];

export function AboutScreen() {
  return (
    <>
      <section className="banner">
        <NetworkBackdrop className="banner__backdrop" />
        <div className="banner__content">
          <h1 className="banner__title">Về {GROUP.name}</h1>
          <p className="banner__lead">
            Những nội dung dưới đây là của Tập đoàn {GROUP.name}, công ty mẹ của VihatSoftware.
          </p>
        </div>
      </section>

      {BRAND_STATEMENTS.map((statement, index) => {
        const Glyph = STATEMENT_GLYPHS[index] ?? HandshakeGlyph;
        return (
          <section className="card card--statement" key={statement.title}>
            <span className="tile tile--soft" aria-hidden="true">
              <Glyph className="tile__glyph" />
            </span>
            <h2 className="card__title">{statement.title}</h2>
            <p className="card__body">{statement.body}</p>
          </section>
        );
      })}

      <h2 className="section-title">
        <span className="section-title__mark" aria-hidden="true">
          <ShieldGlyph />
        </span>
        Chứng chỉ và chứng nhận
      </h2>
      <ul className="cert-grid">
        {CERTIFICATES.map((certificate) => (
          <li className="cert" key={certificate.name}>
            <ShieldGlyph className="cert__glyph" />
            <strong className="cert__name">{certificate.name}</strong>
            {certificate.scope ? <span className="cert__scope">{certificate.scope}</span> : null}
          </li>
        ))}
      </ul>
    </>
  );
}
