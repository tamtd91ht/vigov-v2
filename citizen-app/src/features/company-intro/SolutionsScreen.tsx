import {
  COMPANY,
  ECOSYSTEM,
  GROUP,
  MEMBER_UNITS,
  SOLUTIONS,
  TECH_KEYWORDS,
} from "../../content/company-profile";
import { KiemTraDuongTruyen } from "../tinh-nang/index";

import { BuildingGlyph, KEYWORD_GLYPHS, NetworkBackdrop, SOLUTION_GLYPHS } from "./icons";

/**
 * Screen 2 — what the group's technology lines are, and where VihatSoftware sits among them.
 *
 * WHY THERE IS A PRODUCT LIST NOW, AND WHAT STILL IS NOT HERE:
 *
 *   The lines below (eSMS, OMICall, Contact Center + CRM, digital namecard) are published on
 *   vihatgroup.com and are quoted VERBATIM from company-profile.ts. What is still absent is
 *   everything that was never sourced: customer names, case studies, figures per product, and
 *   the name of the member unit behind the two lines the source leaves unattributed.
 *
 *   They are the PARENT's ecosystem, so `ECOSYSTEM.ownerNote` sits above them for the same
 *   reason the figures on the home screen carry one: without it a reviewer reads a subsidiary
 *   claiming a product line it does not own.
 */
export function SolutionsScreen() {
  return (
    <>
      <h1 className="screen-heading">Giải pháp</h1>
      <p className="screen-lead">{COMPANY.positioning}</p>

      <section className="card card--accent">
        <h2 className="card__title">{COMPANY.name}</h2>
        <p className="card__body">{COMPANY.description}</p>
      </section>

      <h2 className="section-title">
        <span className="section-title__mark" aria-hidden="true">
          <NetworkBackdrop className="section-title__net" />
        </span>
        Hệ sinh thái giải pháp
      </h2>
      <p className="note note--brand">{ECOSYSTEM.ownerNote}</p>

      <ul className="chips">
        {TECH_KEYWORDS.map((keyword) => {
          const Glyph = KEYWORD_GLYPHS[keyword.id];
          return (
            <li className="chip" key={keyword.id}>
              <Glyph className="chip__glyph" />
              {keyword.label}
            </li>
          );
        })}
      </ul>

      <ul className="solutions">
        {SOLUTIONS.map((solution) => {
          const Glyph = SOLUTION_GLYPHS[solution.id];
          return (
            <li className="solution" key={solution.id}>
              <span className="tile" aria-hidden="true">
                <Glyph className="tile__glyph" />
              </span>
              <span className="solution__text">
                {solution.product ? <strong className="solution__product">{solution.product}</strong> : null}
                <span className="solution__headline">{solution.headline}</span>
                {solution.note ? <span className="solution__note">{solution.note}</span> : null}
              </span>
            </li>
          );
        })}
      </ul>

      {/* KIỂM TRA ĐƯỜNG TRUYỀN ĐỨNG NGAY DƯỚI DANH SÁCH GIẢI PHÁP, KHÔNG PHẢI Ở MỘT TAB RIÊNG.
          Người vừa đọc "Tổng đài đa kênh ứng dụng AI" là người đang cân nhắc một tổng đài chạy
          trên đường mạng của chính họ — và câu hỏi kế tiếp của họ là đường mạng ấy đang là gì.
          Một tính năng đặt đúng chỗ người ta cần nó là thứ phân biệt một app có việc để làm với
          một app đi xin quyền. */}
      <KiemTraDuongTruyen />

      <h2 className="section-title">
        <span className="section-title__mark" aria-hidden="true">
          <BuildingGlyph />
        </span>
        Đơn vị thành viên của {GROUP.name}
      </h2>
      <ul className="unit-grid">
        {MEMBER_UNITS.map((unit) => (
          <li className={unit.ownsThisApp ? "unit unit--owner" : "unit"} key={unit.name}>
            <span className="unit__name">{unit.name}</span>
            {unit.ownsThisApp ? <span className="badge">Đơn vị phát hành ứng dụng này</span> : null}
          </li>
        ))}
      </ul>
    </>
  );
}
