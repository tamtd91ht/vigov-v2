import { BRAND_STATEMENTS, CERTIFICATES, GROUP } from "../../content/company-profile";

/**
 * Screen 3 — philosophy, vision, mission and certificates.
 *
 * All four belong to ViHAT GROUP, so the screen says so once at the top rather than repeating
 * a disclaimer on every card. A certificate attributed to the wrong entity is a claim about
 * an audit that entity never passed.
 */
export function AboutScreen() {
  return (
    <>
      <h1 className="screen-heading">Về {GROUP.name}</h1>
      <p className="screen-lead">
        Những nội dung dưới đây là của Tập đoàn {GROUP.name}, công ty mẹ của ViHAT Software.
      </p>

      {BRAND_STATEMENTS.map((statement) => (
        <section className="card" key={statement.title}>
          <h2 className="card__title">{statement.title}</h2>
          <p className="card__body">{statement.body}</p>
        </section>
      ))}

      <h2 className="card__title">Chứng chỉ và chứng nhận</h2>
      <ul className="list">
        {CERTIFICATES.map((certificate) => (
          <li className="list__item" key={certificate.name}>
            <strong>{certificate.name}</strong>
            {certificate.scope ? (
              <>
                <br />
                {certificate.scope}
              </>
            ) : null}
          </li>
        ))}
      </ul>
    </>
  );
}
