import { COMPANY, GROUP, GROUP_STATS } from "../../content/company-profile";

/**
 * Screen 1 — who owns this app, in one screen, above the fold.
 *
 * The group figures sit BELOW an explicit owner note. A reviewer reading top to bottom learns
 * whose numbers they are before reading the numbers; putting the note underneath would let the
 * subsidiary appear to claim the parent's scale.
 */
export function HomeScreen() {
  return (
    <>
      <h1 className="screen-heading">{COMPANY.name}</h1>
      <p className="screen-lead">{COMPANY.positioning}</p>

      <section className="card">
        <p className="card__body">{COMPANY.description}</p>
      </section>

      <h2 className="card__title">{GROUP.name}</h2>
      <p className="note">{GROUP.figuresOwnerNote}</p>

      <ul className="stat-grid">
        {GROUP_STATS.map((stat) => (
          <li className="stat" key={stat.label}>
            <span className="stat__value">{stat.value}</span>
            <span className="stat__label">{stat.label}</span>
          </li>
        ))}
      </ul>
    </>
  );
}
