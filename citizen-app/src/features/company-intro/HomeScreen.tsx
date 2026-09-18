import { COMPANY, GROUP, GROUP_STATS } from "../../content/company-profile";
import { CloudGlyph, CpaasGlyph, NetworkBackdrop } from "./icons";

/**
 * Screen 1 — who owns this app, in one screen, above the fold.
 *
 * The group figures sit BELOW an explicit owner note. A reviewer reading top to bottom learns
 * whose numbers they are before reading the numbers; putting the note underneath would let the
 * subsidiary appear to claim the parent's scale. The figures moved onto a dark band to make
 * them the thing the eye lands on — the note stays ABOVE that band, in reading order, because
 * a note the eye reaches after the numbers is a note read too late.
 *
 * Everything decorative here is drawn, not photographed: no image file enters the uploaded
 * package (see icons.tsx).
 */
export function HomeScreen() {
  return (
    <>
      <section className="hero">
        <NetworkBackdrop className="hero__backdrop" />
        <div className="hero__content">
          <h1 className="hero__title">{COMPANY.name}</h1>
          <p className="hero__lead">{COMPANY.positioning}</p>
          <p className="hero__body">{COMPANY.description}</p>
          <span className="hero__rule" aria-hidden="true" />
        </div>
      </section>

      <h2 className="section-title">
        <span className="section-title__mark" aria-hidden="true">
          <CpaasGlyph />
        </span>
        {GROUP.name}
      </h2>
      <p className="note note--brand">{GROUP.figuresOwnerNote}</p>

      <section className="band">
        <CloudGlyph className="band__backdrop" />
        <ul className="stat-grid">
          {GROUP_STATS.map((stat, index) => (
            <li className={index === 0 ? "stat stat--wide" : "stat"} key={stat.label}>
              <span className="stat__value">{stat.value}</span>
              <span className="stat__label">{stat.label}</span>
            </li>
          ))}
        </ul>
      </section>
    </>
  );
}
