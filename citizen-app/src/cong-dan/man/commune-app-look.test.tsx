import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import {
  BUTTON_CLASS,
  CommuneButton,
  DauManCon,
  DrumPattern,
  KhoiTrangThai,
  RootTabHeader,
  SectionHeader,
} from "./khung-xa";
import { NEWS_TYPE_LABEL, QUAY_LAI, XA_GIAO_DIEN, XA_TN } from "./noi-dung";
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

  // The category chips (card D2) appear only once their route answers; a first render — before any load —
  // has none, so no empty row flashes up. Their markup is pinned in `news-category-chips.test.tsx`.
  it("has no 'Tất cả' tab, and no chip row before the categories have loaded", () => {
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

/**
 * Wave 0 (30/09/2026, `ui-design/prototpye-spec/PROTOTYPE.md` §5–§7): the frame and kit every screen shares.
 * What is pinned is what the owner decided AGAINST the prototype as much as what was taken from it.
 */
describe("header: một khuôn cho mọi màn", () => {
  it("child screen: back button keeps the WORD 'Quay lại' beside the icon, title in the middle slot", () => {
    const html = renderToStaticMarkup(createElement(DauManCon, { tieu_de: "Danh bạ", onQuayLai: () => {} }));
    expect(html).toMatch(/^<header class="xa-header xa-dau-con">/);
    expect(html).toContain(`<span>${QUAY_LAI}</span>`);
    expect(html).toContain('<div class="xa-header__title"><h1 class="xa-dau-con__tieu-de">Danh bạ</h1></div>');
    // Zalo's corner stays empty on a child screen.
    expect(html).toContain('<div class="xa-header__side xa-header__side--end"></div>');
  });

  it("root tab: the commune's logo on the left (not the national emblem), optional subtitle, right slot as given", () => {
    const plain = renderToStaticMarkup(createElement(RootTabHeader, { title: "Tin tức – Sự kiện" }));
    expect(plain).toContain('src="./logo-xa.png"');
    expect(plain).not.toContain("xa-header__subtitle");
    const home = renderToStaticMarkup(
      createElement(RootTabHeader, { title: "Xã Thử", subtitle: "Xin chào", right: createElement("button", { type: "button" }, "x") }),
    );
    expect(home).toContain('<p class="xa-header__subtitle">Xin chào</p>');
    expect(home).toContain('<div class="xa-header__side xa-header__side--end"><button type="button">x</button></div>');
  });

  it("the drum pattern is decoration: aria-hidden, 4 rings + centre, 16 rays, no string-built markup", () => {
    const svg = renderToStaticMarkup(createElement(DrumPattern));
    expect(svg).toMatch(/^<svg class="xa-drum"[^>]*aria-hidden="true"/);
    expect(svg.match(/<circle /g)).toHaveLength(5);
    expect(svg.match(/<line /g)).toHaveLength(16);
  });
});

describe("khối trạng thái: lỗi · trống · đang tải", () => {
  const render = (p: Parameters<typeof KhoiTrangThai>[0]) => renderToStaticMarkup(createElement(KhoiTrangThai, p));

  it("error: the 80px circle, the sentence as an alert, and the retry as a SECONDARY button", () => {
    const html = render({ bieu_tuong: "wifi-off", loi: true, cau: "Chưa kết nối được.", nut: { nhan: "Thử lại", onBam: () => {} } });
    expect(html).toContain('class="xa-trang-thai xa-trang-thai--loi"');
    expect(html).toContain('<span class="xa-status-circle" aria-hidden="true">');
    expect(html).toContain('<p class="xa-status-title" role="alert">Chưa kết nối được.</p>');
    expect(html).toContain(`<button type="button" class="${BUTTON_CLASS.secondary}">Thử lại</button>`);
  });

  it("empty: title plus an optional hint; callers that pass no hint render none", () => {
    expect(render({ bieu_tuong: "chat", cau: "Chưa có phản ánh", hint: "Chạm nút bên dưới." })).toContain(
      '<p class="xa-status-hint">Chạm nút bên dưới.</p>',
    );
    expect(render({ bieu_tuong: "chat", cau: "Chưa có phản ánh" })).not.toContain("xa-status-hint");
  });

  it("loading stays WORDS: a status sentence, no circle, nothing that pulses or spins", () => {
    const html = render({ bieu_tuong: "news", cau: "Đang tải…", dang_tai: true });
    expect(html).toBe('<div class="xa-trang-thai xa-trang-thai--tai"><p role="status">Đang tải…</p></div>');
  });
});

describe("nút và tiêu đề mục", () => {
  it("four shapes on the existing classes — primary stays bare `xa-nut` (the brand red)", () => {
    expect(BUTTON_CLASS).toEqual({
      primary: "xa-nut",
      secondary: "xa-nut xa-nut--phu",
      quiet: "xa-nut xa-nut--quiet",
      danger: "xa-nut xa-nut--danger",
    });
    const html = renderToStaticMarkup(createElement(CommuneButton, { icon: "send", onClick: () => {}, children: "Gửi" }));
    expect(html).toMatch(/^<button type="button" class="xa-nut"><svg[^>]*aria-hidden="true"[\s\S]*<\/svg>Gửi<\/button>$/);
  });

  it("section header: accent modifier, heading, and 'Xem tất cả' as a real button with words", () => {
    const html = renderToStaticMarkup(
      createElement(SectionHeader, { title: "Tin mới", accent: "xanh", more: { label: XA_GIAO_DIEN.xem_tat_ca, onPress: () => {} } }),
    );
    expect(html).toMatch(/^<div class="xa-dau-nhom xa-dau-nhom--xanh"><h2 class="xa-dau-khoi__tieu-de">Tin mới<\/h2>/);
    expect(html).toMatch(new RegExp(`<button type="button" class="xa-dau-khoi__them">${XA_GIAO_DIEN.xem_tat_ca}<svg`));
    expect(renderToStaticMarkup(createElement(SectionHeader, { title: "Chính quyền số" }))).toBe(
      '<div class="xa-dau-nhom xa-dau-nhom--brand"><h2 class="xa-dau-khoi__tieu-de">Chính quyền số</h2></div>',
    );
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
