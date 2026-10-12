import type { ReactElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { KetQuaDoc } from "../../api/goi-may-chu";
import type { MaTrangThai } from "../../api/hop-dong-yeu-cau";
import { CAU_DAU, MUC_CHINH_SACH, TIEU_DE_CHINH_SACH } from "../../content/chinh-sach-rieng-tu";
import { COMPANY } from "../../content/company-profile";
import { PULL_TRIGGER_PX, PullBand, pullBand } from "../yeu-cau/intro-pull-to-refresh";
import { YEU_CAU_CUA_TOI } from "../yeu-cau/noi-dung";
import { NHAN_TRANG_THAI, STATUS_TONE } from "../yeu-cau/trang-thai";
import { afterPullRefresh, RequestStatusChip, ThanDanhSach } from "../yeu-cau/YeuCauCuaToiScreen";

import { ContactScreen } from "./ContactScreen";
import { COLLAPSE_AFTER_CHARS, EXPAND_LABEL, ExpandableText, needsCollapse } from "./ExpandableText";
import { PrivacyPolicyScreen } from "./PrivacyPolicyScreen";
import { QuanLyQuyenScreen } from "./QuanLyQuyenScreen";
import { SolutionsScreen } from "./SolutionsScreen";

/**
 * TASK V1 (owner 12/10/2026, Zalo resubmission): the ViHAT intro's UI refresh — the parts a static render can prove.
 * Every list item below is invented test data; no real request, code or person.
 */

const ve = (el: ReactElement) => renderToStaticMarkup(el);
const textOf = (html: string) => html.replace(/<[^>]+>/g, " ").replace(/&#x27;/g, "'").replace(/&quot;/g, '"');

describe("a long paragraph starts at three lines, with 'Xem thêm'", () => {
  it("collapses only past the threshold — a short text gets no button that does nothing", () => {
    expect(needsCollapse("a".repeat(COLLAPSE_AFTER_CHARS))).toBe(false);
    expect(needsCollapse("a".repeat(COLLAPSE_AFTER_CHARS + 1))).toBe(true);
    expect(ve(<ExpandableText className="card__body" text="Ngắn." />)).toBe('<p class="card__body">Ngắn.</p>');
  });

  it("a long text keeps ALL its words in the page, clamped by CSS, with a labelled toggle", () => {
    const text = `${"Một câu dài. ".repeat(20)}Câu cuối.`;
    const html = ve(<ExpandableText className="card__body" text={text} />);
    expect(html).toContain("vh-clamp");
    expect(html).toContain("Câu cuối.");
    expect(html).toMatch(/<button type="button" class="vh-more" aria-expanded="false" aria-controls="[^"]+">Xem thêm<\/button>/);
  });

  it("the published description on Giải pháp is collapsed — and still printed whole", () => {
    const html = ve(<SolutionsScreen />);
    // Unconditional on purpose: the description is long today, and a test that skips itself when it is not would
    // stay green for the wrong reason the day the threshold or the text changes.
    expect(needsCollapse(COMPANY.description!), "the description is no longer long enough to collapse").toBe(true);
    expect(textOf(html)).toContain(EXPAND_LABEL);
    expect(html).toContain("vh-clamp");
    expect(textOf(html)).toContain(COMPANY.description!);
  });
});

describe("the privacy policy is its own screen, one tap from Liên hệ and from Quản lý quyền", () => {
  it("Liên hệ no longer prints the policy text — it shows a row naming it", () => {
    const text = textOf(ve(<ContactScreen />));
    expect(text).toContain(TIEU_DE_CHINH_SACH);
    expect(text, "the policy's opening sentence is still on Liên hệ").not.toContain(CAU_DAU);
  });

  it("the policy screen has its title as the one <h1>, the opening sentence, every section, and a named way back", () => {
    const html = ve(<PrivacyPolicyScreen backLabel="Quay lại Liên hệ" onBack={() => {}} />);
    expect(html.match(/<h1\b/g) ?? []).toHaveLength(1);
    const text = textOf(html);
    expect(text).toContain(TIEU_DE_CHINH_SACH);
    expect(text).toContain(CAU_DAU);
    for (const m of MUC_CHINH_SACH) expect(text).toContain(m.tieu_de);
    expect(text).toContain("Quay lại Liên hệ");
  });

  it("Quản lý quyền links to the policy when its parent passes the opener", () => {
    expect(textOf(ve(<QuanLyQuyenScreen onQuayLai={() => {}} onOpenPolicy={() => {}} />))).toContain(TIEU_DE_CHINH_SACH);
  });
});

describe("Yêu cầu của tôi: every state says what it is and what to do next", () => {
  const none = () => {};

  it("loading: a visible sentence (`role=status`) and skeleton blocks hidden from screen readers", () => {
    const html = ve(<ThanDanhSach ket_qua={null} onDangNhapLai={none} onGuiMoi={none} />);
    expect(html).toContain('role="status"');
    expect(textOf(html)).toContain(YEU_CAU_CUA_TOI.dang_doc);
    expect(html).toMatch(/class="vh-skeleton" aria-hidden="true"/);
    expect(html).toContain('class="skel');
  });

  it("a failure is an alert with a 'Thử lại' button", () => {
    for (const kieu of ["khong-goi-duoc", "chua-khai-host"] as const) {
      const html = ve(<ThanDanhSach ket_qua={{ kieu }} onDangNhapLai={none} onGuiMoi={none} onThuLai={none} />);
      expect(html, kieu).toContain('role="alert"');
      expect(textOf(html), kieu).toContain(YEU_CAU_CUA_TOI.nut_thu_lai);
    }
  });

  it("empty: the sentence, what sending one gets you, and the button to send one", () => {
    const text = textOf(ve(<ThanDanhSach ket_qua={{ kieu: "xong", danh_sach: [] }} onDangNhapLai={none} onGuiMoi={none} />));
    expect(text).toContain(YEU_CAU_CUA_TOI.rong);
    expect(text).toContain(YEU_CAU_CUA_TOI.rong_goi_y);
    expect(text).toContain(YEU_CAU_CUA_TOI.nut_gui_moi);
  });

  it("the status pill carries an icon AND the words, on a tone named per status", () => {
    for (const status of Object.keys(NHAN_TRANG_THAI) as MaTrangThai[]) {
      const html = ve(<RequestStatusChip status={status} />);
      expect(html).toContain(`vh-status--${STATUS_TONE[status]}`);
      expect(html).toMatch(/<svg[^>]*aria-hidden="true"/);
      expect(textOf(html)).toContain(NHAN_TRANG_THAI[status]);
    }
  });
});

describe("pull to refresh keeps what the user was reading", () => {
  const list: KetQuaDoc = {
    kieu: "xong",
    danh_sach: [{ ma: "YC-TEST-1", loai: "consult", trang_thai: "moi", tao_luc: "2026-10-01T03:00:00Z", photo_count: 0 }],
  };

  it("a fresh list replaces the old one; a failure KEEPS the list and reports failure", () => {
    const fresh: KetQuaDoc = { kieu: "xong", danh_sach: [] };
    expect(afterPullRefresh(list, fresh)).toEqual({ shown: fresh, ok: true });
    expect(afterPullRefresh(list, { kieu: "khong-goi-duoc" })).toEqual({ shown: list, ok: false });
  });

  it("a signed-out answer is always shown — the user must be told to sign in again", () => {
    expect(afterPullRefresh(list, { kieu: "chua-dang-nhap" })).toEqual({ shown: { kieu: "chua-dang-nhap" }, ok: true });
  });

  it("with nothing useful on screen, the new failure is shown (it has its own 'Thử lại')", () => {
    expect(afterPullRefresh({ kieu: "chua-khai-host" }, { kieu: "khong-goi-duoc" })).toEqual({
      shown: { kieu: "khong-goi-duoc" },
      ok: true,
    });
  });

  it("the band: nothing at rest, words while pulling, a status while busy, an alert after a failure", () => {
    expect(pullBand(-10)).toBe(0);
    expect(pullBand(PULL_TRIGGER_PX * 2)).toBe(PULL_TRIGGER_PX);
    expect(pullBand(10_000)).toBe(96);
    expect(ve(<PullBand state={{ kind: "idle" }} />)).toBe("");
    const pulling = ve(<PullBand state={{ kind: "pulling", px: 20 }} />);
    expect(pulling).toContain('aria-hidden="true"');
    expect(textOf(pulling)).toContain(YEU_CAU_CUA_TOI.keo_tai_lai);
    expect(textOf(ve(<PullBand state={{ kind: "pulling", px: PULL_TRIGGER_PX }} />))).toContain(YEU_CAU_CUA_TOI.tha_tai_lai);
    expect(ve(<PullBand state={{ kind: "refreshing" }} />)).toContain('role="status"');
    const failed = ve(<PullBand state={{ kind: "failed" }} />);
    expect(failed).toContain('role="alert"');
    expect(textOf(failed)).toContain(YEU_CAU_CUA_TOI.tai_lai_hong);
    expect(YEU_CAU_CUA_TOI.tai_lai_hong).not.toMatch(/\d{3}/);
  });
});
