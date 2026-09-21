import { useState } from "react";

import {
  COMPANY,
  MEMBER_UNITS,
  SOLUTIONS,
  type SolutionId,
  TECH_KEYWORDS,
} from "../../content/company-profile";
import { KiemTraDuongTruyen } from "../tinh-nang/index";

import { ChiTietGiaiPhap } from "./ChiTietGiaiPhap";
import { BuildingGlyph, KEYWORD_GLYPHS, NetworkBackdrop, SOLUTION_GLYPHS } from "./icons";

/**
 * Screen 2 — the group's technology lines, and the six member units behind them.
 *
 * WHY THERE IS A PRODUCT LIST NOW, AND WHAT STILL IS NOT HERE:
 *
 *   The lines below (eSMS, OMICall, Contact Center + CRM, digital namecard) are published on
 *   vihatgroup.com and are quoted VERBATIM from company-profile.ts. What is still absent is
 *   everything that was never sourced: customer names, case studies, figures per product, and
 *   the name of the member unit behind the two lines the source leaves unattributed.
 *
 *   They are the publisher's own ecosystem since 2026-09-21, so the note that used to stand
 *   above them ("thuộc hệ sinh thái của công ty mẹ") is gone: with one entity there is nobody
 *   to disclaim against, and a disclaimer nobody needs reads as one somebody is hiding behind.
 *
 *   No unit carries a "publisher of this app" badge any more, for the same reason. VihatSoftware
 *   is still in the list — it is still a member unit, it just no longer publishes this app.
 *
 * ⚠ KHÔNG CÓ Ô TÌM KIẾM TRÊN MÀN NÀY, VÀ ĐÓ LÀ MỘT ĐỘ LỆCH CÓ CHỦ ĐÍCH SO VỚI BẢN MẪU.
 *
 *   Bản mẫu của PM yêu cầu "thẻ bấm được, có tìm kiếm". Vế thứ nhất có ở đây. Vế thứ hai thì
 *   không, vì HAI lý do độc lập, và mỗi lý do một mình đã đủ:
 *
 *   1. MỘT Ô TÌM KIẾM LÀ MỘT `<input>`, VÀ `<input>` BỊ CẤM Ở MỌI TỆP.
 *      `phase1-collects-nothing.test.ts` cấm `<form|input|textarea|select>` trên toàn cây mã:
 *      "một ô nhập là một điểm thu thập". Nới lệnh cấm ấy để có một ô lọc danh sách là nới đúng
 *      dây bẫy đang canh việc app không mọc thêm chỗ thu thập nào — và lệnh cấm ấy được yêu cầu
 *      giữ nguyên. Gỡ nó cho một ô lọc là trả một giá không tương xứng.
 *
 *   2. CÓ BỐN DÒNG GIẢI PHÁP. Một ô tìm kiếm trên bốn mục là một điều khiển luôn hiện đủ bốn mục
 *      — tức một điều khiển không làm gì, đặt ngay đầu màn. Với người lớn tuổi, thứ ấy còn tệ hơn
 *      là không có: nó mời gõ một bàn phím trên một danh sách đọc hết trong ba giây.
 *
 *   Con số bốn là con số CÓ NGUỒN. Bản mẫu nói "15 giải pháp, 3 nhóm nghiệp vụ" và không trang nào
 *   công bố điều đó — xem khối chú thích của `SOLUTIONS` trong `company-profile.ts`.
 */
export function SolutionsScreen() {
  /**
   * Giải pháp đang mở, hoặc `null` cho danh sách.
   *
   * Trạng thái giao diện thuần, sống trong `useState`: không ghi xuống máy, không vào URL, không
   * gửi đi đâu (`ranh-gioi-hai-nua.test.ts` §3b canh vế thứ nhất). Đóng app là mất, và đó là
   * đúng: không có gì ở đây đáng để sống lâu hơn một lần mở app.
   */
  const [dang_mo, datDangMo] = useState<SolutionId | null>(null);
  const giai_phap = SOLUTIONS.find((mot) => mot.id === dang_mo);

  if (giai_phap) {
    return <ChiTietGiaiPhap giai_phap={giai_phap} onQuayLai={() => datDangMo(null)} />;
  }

  return (
    <>
      <h1 className="screen-heading">Giải pháp</h1>
      {COMPANY.positioning ? <p className="screen-lead">{COMPANY.positioning}</p> : null}

      {COMPANY.description ? (
        <section className="card card--accent">
          <h2 className="card__title">{COMPANY.name}</h2>
          <p className="card__body">{COMPANY.description}</p>
        </section>
      ) : null}

      <h2 className="section-title">
        <span className="section-title__mark" aria-hidden="true">
          <NetworkBackdrop className="section-title__net" />
        </span>
        Hệ sinh thái giải pháp
      </h2>

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

      {/* MỖI THẺ LÀ MỘT `<button>` THẬT, không phải một `<div onClick>`.
          Một `div` bấm được thì trình đọc màn hình không gọi nó là nút, bàn phím không tới được,
          và không có gì bảo đảm chiều cao 44px. `.solution--mo` đặt `min-height: var(--tap-min)`
          và `accessibility.test.ts` ghim điều đó. */}
      <ul className="solutions">
        {SOLUTIONS.map((solution) => {
          const Glyph = SOLUTION_GLYPHS[solution.id];
          return (
            <li className="solution" key={solution.id}>
              <button
                type="button"
                className="solution--mo"
                onClick={() => datDangMo(solution.id)}
              >
                <span className="tile" aria-hidden="true">
                  <Glyph className="tile__glyph" />
                </span>
                <span className="solution__text">
                  {solution.product ? (
                    <strong className="solution__product">{solution.product}</strong>
                  ) : null}
                  <span className="solution__headline">{solution.headline}</span>
                  {solution.note ? <span className="solution__note">{solution.note}</span> : null}
                </span>
                <span className="solution__mui" aria-hidden="true">
                  ›
                </span>
              </button>
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
        Đơn vị thành viên của {COMPANY.name}
      </h2>
      <ul className="unit-grid">
        {MEMBER_UNITS.map((unit) => (
          <li className="unit" key={unit.name}>
            <span className="unit__name">{unit.name}</span>
          </li>
        ))}
      </ul>
    </>
  );
}
