import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { NEWS_TYPE_LABEL, XA_GIAO_DIEN, XA_TN } from "./noi-dung";
import { DanhSachTinXa, NEWS_TABS } from "./TinTucAppXa";
import { CommuneBanner, PetitionSendFooter } from "./TrangXa";

/**
 * Card D1 (owner decisions 30/09/2026, prototype v30): the news type tabs, the Phản ánh footer button and
 * the home banner. Rendered to markup — the same way the other commune-app tests read the screen — so what
 * is pinned is what a screen reader and a finger meet, not a constant.
 */

describe("tab Tin tức – Sự kiện: ba tab loại tin, không có 'Tất cả'", () => {
  const html = renderToStaticMarkup(createElement(DanhSachTinXa, { ten_mien: "xa-thu.vigov.vn", onMo: () => {} }));

  it("is a tablist of exactly the three types, in the prototype's order", () => {
    expect(html).toContain(`role="tablist" aria-label="${XA_TN.loc_loai_tin}"`);
    const tabs = [...html.matchAll(/role="tab"[^>]*>([^<]*)</g)].map((m) => m[1]);
    expect(tabs).toEqual(NEWS_TABS.map((t) => NEWS_TYPE_LABEL[t]));
    expect(tabs).toEqual(["Tin tức", "Sự kiện", "Thông báo"]);
  });

  it("opens on Tin tức, and says so with aria-selected, not colour alone", () => {
    const selected = [...html.matchAll(/role="tab" aria-selected="(true|false)"[^>]*>([^<]*)</g)].map((m) => [m[2], m[1]]);
    expect(selected).toEqual([
      ["Tin tức", "true"],
      ["Sự kiện", "false"],
      ["Thông báo", "false"],
    ]);
  });

  it("has no 'Tất cả' tab or chip, and no chip row at all (categories are card D2)", () => {
    expect(html).not.toContain(`>${XA_TN.loc_tat_ca}<`);
    expect(html).not.toContain("xa-chip");
    expect(html).not.toContain("aria-pressed");
  });

  it("the list under the tabs is the tab's panel", () => {
    expect(html).toMatch(/role="tabpanel" aria-labelledby="xa-tab-tin-tin-tuc"/);
  });

  it("the tab header reads 'Tin tức – Sự kiện', as the prototype", () => {
    expect(XA_TN.news_tab_title).toBe("Tin tức – Sự kiện");
  });
});

describe("tab Phản ánh: nút gửi ở chân trang", () => {
  const html = renderToStaticMarkup(createElement(PetitionSendFooter, { onSend: () => {} }));

  it("is one full-width button in the footer, with its own words", () => {
    expect(html).toMatch(/^<div class="xa-chan-gui"><button type="button" class="xa-nut xa-nut--chan">/);
    expect(html).toContain(XA_TN.send_new_petition);
    expect(XA_TN.send_new_petition).toBe("Gửi phản ánh mới");
  });

  it("the home tile keeps its own shorter words", () => {
    expect(XA_GIAO_DIEN.o_gui).toBe("Gửi phản ánh");
  });
});

describe("banner trang chủ: tạm theo tên miền, như logo", () => {
  it("is one decorative picture from the bundle, no carousel", () => {
    const html = renderToStaticMarkup(createElement(CommuneBanner));
    // React may add a `<link rel="preload">` for the image; the block itself is exactly one <img>.
    expect(html).toContain('<div class="xa-banner"><img class="xa-banner__anh" src="./banner-xa.png" alt=""/></div>');
    expect(html.match(/<img /g)).toHaveLength(1);
  });
});
