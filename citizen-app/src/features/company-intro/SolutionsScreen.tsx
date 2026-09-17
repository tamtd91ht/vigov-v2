import { COMPANY, GROUP, MEMBER_UNITS } from "../../content/company-profile";

/**
 * Screen 2 — what ViHAT Software does, and where it sits inside the group.
 *
 * No product list, no customer names, no case studies: none of that was sourced on
 * 2026-09-17, and this app carries a real legal entity's name. It says exactly what the
 * published description says, and stops.
 */
export function SolutionsScreen() {
  return (
    <>
      <h1 className="screen-heading">Giải pháp</h1>
      <p className="screen-lead">{COMPANY.positioning}</p>

      <section className="card">
        <h2 className="card__title">{COMPANY.name}</h2>
        <p className="card__body">{COMPANY.description}</p>
      </section>

      <h2 className="card__title">Đơn vị thành viên của {GROUP.name}</h2>
      <ul className="list">
        {MEMBER_UNITS.map((unit) => (
          <li className="list__item" key={unit.name}>
            {unit.name}
            {unit.ownsThisApp ? <span className="badge">Đơn vị phát hành ứng dụng này</span> : null}
          </li>
        ))}
      </ul>
    </>
  );
}
