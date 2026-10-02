import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import {
  BUTTON_CLASS,
  CommuneButton,
  DauManCon,
  DrumPattern,
  illustrationFor,
  KhoiTrangThai,
  RootTabHeader,
  SectionHeader,
} from "./khung-xa";
import { EmptyIllustration, LoadingSkeleton } from "./state-visuals";
import { TIN_DAU, ThanTinXa } from "./TinTucXaScreen"; // vi-name-ok: existing shared-app names
import { NEWS_TYPE_LABEL, QUAY_LAI, TIN_XA, XA_GIAO_DIEN, XA_TN } from "./noi-dung";
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

  /**
   * UI-1 (owner, 02/10/2026) replaced "loading stays WORDS, nothing that pulses": the sentence stays — visible,
   * `role="status"` on it and on nothing else — and decorative blocks in the shape of what is coming follow it.
   */
  it("loading: the visible status sentence FIRST, then aria-hidden blocks of the asked shape; no circle, no spinner", () => {
    const html = render({ bieu_tuong: "news", cau: "Đang tải…", dang_tai: true });
    expect(html).toMatch(
      /^<div class="xa-trang-thai xa-trang-thai--tai"><p role="status">Đang tải…<\/p><span class="xa-skeleton xa-skeleton--list" aria-hidden="true">/,
    );
    expect(html.match(/role="status"/g)).toHaveLength(1);
    expect(html.match(/class="skel-row"/g)).toHaveLength(3);
    expect(html).not.toContain("xa-status-circle");
    expect(html).not.toContain("<svg");
    expect(render({ bieu_tuong: "news", cau: "x", dang_tai: true, rows: 2 }).match(/class="skel-row"/g)).toHaveLength(2);
    expect(render({ bieu_tuong: "news", cau: "x", dang_tai: true, shape: "article" })).toContain("xa-skeleton--article");
    // An article skeleton promises no cover: a grey 16:9 band would stand for a picture the item may not have.
    expect(render({ bieu_tuong: "news", cau: "x", dang_tai: true, shape: "article" })).not.toMatch(/skel--thumb|cover/);
    // `none`: a wait that is not for content (opening a session) — the sentence alone, exactly the old markup.
    expect(render({ bieu_tuong: "user", cau: "Đang mở…", dang_tai: true, shape: "none" })).toBe(
      '<div class="xa-trang-thai xa-trang-thai--tai"><p role="status">Đang mở…</p></div>',
    );
  });

  it("empty: a small drawing instead of the circle — aria-hidden, unfocusable, chosen by the icon", () => {
    const html = render({ bieu_tuong: "news", cau: "Chưa có tin" });
    expect(html).toMatch(/^<div class="xa-trang-thai"><svg class="xa-illus" viewBox="0 0 120 90"[^>]*aria-hidden="true" focusable="false">/);
    expect(html).not.toContain("xa-status-circle");
    expect(illustrationFor("news")).toBe("news");
    expect(illustrationFor("message-square-plus")).toBe("petition");
    expect(illustrationFor("chat")).toBe("petition");
    expect(illustrationFor("users")).toBe("directory");
    expect(illustrationFor("map")).toBe("generic");
    // An error keeps its red circle: a drawing there would soften a sentence that asks the citizen to act.
    expect(render({ bieu_tuong: "alert", cau: "Lỗi", loi: true })).not.toContain("xa-illus");
  });
});

describe("loading blocks and drawings, both apps (UI-1)", () => {
  it("every skeleton shape is aria-hidden; heights follow the text size (em), no gradient; the pulse is opacity only", async () => {
    for (const shape of ["list", "rows", "card", "article"] as const) {
      for (const prefix of ["xa", "cd"] as const) {
        const html = renderToStaticMarkup(createElement(LoadingSkeleton, { prefix, shape }));
        expect(html).toMatch(new RegExp(`^<span class="${prefix}-skeleton ${prefix}-skeleton--${shape}" aria-hidden="true">`));
      }
    }
    expect(renderToStaticMarkup(createElement(LoadingSkeleton, { prefix: "xa", shape: "none" }))).toBe("");
    const nodeFs = "node:fs";
    const { readFileSync } = (await import(/* @vite-ignore */ nodeFs)) as {
      readFileSync: (path: URL, encoding: "utf8") => string;
    };
    const css = readFileSync(new URL("../../styles.css", import.meta.url), "utf8");
    const rules = [...css.matchAll(/(?:\.(?:xa|cd)-skeleton )?\.skel(?:--[a-z]+)?\s*\{([^}]*)\}/g)].map((m) => m[1]!);
    expect(rules.length).toBeGreaterThan(3);
    for (const r of rules) {
      expect(r).not.toMatch(/gradient/);
      const height = /(?:^|;)\s*height:\s*([^;]+);/.exec(r)?.[1];
      if (height !== undefined) expect(height, "a skeleton height in px would not follow .xa-co-chu--*").toMatch(/em$/);
    }
    const pulse = /@keyframes skel-pulse\s*\{([\s\S]*?)\n\}/.exec(css)?.[1] ?? "";
    expect(pulse).toContain("opacity");
    expect(pulse).not.toMatch(/transform|background/);
  });

  it("shared app: the loading sentence stays the one visible status; the empty list gets the drawing and the hint", () => {
    const onMo = () => {};
    const loading = renderToStaticMarkup(createElement(ThanTinXa, { ds: TIN_DAU, onMo, onTai: onMo }));
    expect(loading).toContain(`<div class="cd-tai"><p class="cd-cau" role="status">${TIN_XA.dang_tai}</p><span class="cd-skeleton cd-skeleton--rows" aria-hidden="true">`);
    expect(loading.match(/role="status"/g)).toHaveLength(1);
    const empty = renderToStaticMarkup(
      createElement(ThanTinXa, { ds: { ...TIN_DAU, da_co_trang_dau: true, dang_tai: false }, onMo, onTai: onMo }),
    );
    expect(empty).toMatch(
      new RegExp(`^<div class="cd-trong"><svg class="cd-illus"[^>]*aria-hidden="true"[\\s\\S]*</svg><p class="cd-cau">${TIN_XA.trong}</p><p class="cd-ghi-chu">${TIN_XA.empty_hint}</p></div>$`),
    );
  });

  it("each drawing is a JSX svg, aria-hidden, with no text in it", () => {
    for (const kind of ["news", "petition", "directory", "generic"] as const) {
      const svg = renderToStaticMarkup(createElement(EmptyIllustration, { kind, className: "cd-illus" }));
      expect(svg).toMatch(/^<svg class="cd-illus"[^>]*aria-hidden="true" focusable="false">/);
      expect(svg).not.toMatch(/<text|<title/);
    }
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
