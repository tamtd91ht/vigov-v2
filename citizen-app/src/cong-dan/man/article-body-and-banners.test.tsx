/// <reference types="vite/client" />
import { createElement, isValidElement, type ReactElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import { communeBanners } from "../api/goi-vigov";
import { bannersAddress, type BodyBlock, type CommuneBannerItem, readBanners, readBodyBlocks, readHttpsLink } from "../api/hop-dong-cong-khai";
import { type BaiTinXa as ArticleData, docBaiTin, DUONG_DAN_TIN_XA } from "../api/hop-dong-cong-khai"; // vi-name-ok: existing contract names

import { ArticleBody } from "./article-body";
import { askToLeave, externalOpenFailed, LeaveAppDialog, linkHost } from "./leave-app";
import { XA_TN } from "./noi-dung"; // vi-name-ok: existing strings module
import { NewsArticle } from "./TinTucAppXa"; // vi-name-ok: existing screen file
import { BaiTin } from "./TinTucXaScreen"; // vi-name-ok: existing shared-app article component
import { BannerStrip, bannerScreen, bannerTap, HomeBanner } from "./TrangXa"; // vi-name-ok: existing screen file

/**
 * ADR 0067 §1 and §5 in the commune app (01/10/2026): the article body drawn from `body_blocks`, links in it that
 * ask before leaving the app, and the home banner strip. Rendered with `react-dom/server` and pure functions, as
 * the other commune-app tests are; a tap is exercised by EXPANDING the pure component tree (`expand`) and calling
 * the `onClick` a finger would reach. Every URL and domain here is fake.
 */

const html = (el: Parameters<typeof renderToStaticMarkup>[0]) => renderToStaticMarkup(el);
const noop = () => {};

/**
 * Every host element under `node`, with function components called through. Only for components that use NO
 * hooks — the body, the dialog and the strip are written that way on purpose, so this works.
 */
function expand(node: unknown, out: ReactElement[] = []): ReactElement[] {
  if (Array.isArray(node)) {
    for (const n of node) expand(n, out);
    return out;
  }
  if (!isValidElement(node)) return out;
  if (typeof node.type === "function") return expand((node.type as (p: unknown) => unknown)(node.props), out);
  out.push(node);
  expand((node.props as { children?: unknown }).children, out);
  return out;
}

const buttons = (node: unknown) =>
  expand(node).filter((e) => e.type === "button") as ReactElement<{ onClick: () => void; className?: string }>[];

const LINK = "https://thongtin.example.vn/thong-bao/lich-tiem?dot=2";

/** The server's shape (`tin_xa_cong_khai.go` `bodyBlockOut`), every kind and mark once. */
const WIRE_BLOCKS = [
  { kind: "heading", level: 2, runs: [{ text: "Lịch tiêm chủng tháng 10" }] },
  {
    kind: "paragraph",
    runs: [
      { text: "Thông báo tới " },
      { text: "toàn thể bà con", bold: true },
      { text: ", nhất là ", italic: true },
      { text: "người cao tuổi", bold: true, italic: true },
      { text: ".\nMang theo sổ tiêm. Xem " },
      { text: "lịch chi tiết", href: LINK },
      { text: "." },
    ],
  },
  { kind: "heading", level: 3, runs: [{ text: "Địa điểm" }] },
  { kind: "bullet_list", items: [{ runs: [{ text: "Trạm y tế xã" }] }, { runs: [{ text: "Nhà văn hoá thôn" }] }] },
  { kind: "ordered_list", items: [{ runs: [{ text: "Đăng ký" }] }, { runs: [{ text: "Tiêm" }] }] },
];

const ARTICLE: ArticleData = {
  id: "tin-1",
  tieu_de: "Tiêm chủng",
  tom_tat: "",
  ngay_dang: "2026-10-01",
  chuyen_muc: "Y tế",
  type: "thong-bao",
  noi_dung: "Bản văn bản thuần.\n\nĐoạn hai.",
};

describe("body_blocks — the parser keeps what it can draw, and nothing else", () => {
  it("reads all four kinds, both marks, links, and keeps `\\n` inside the text", () => {
    const blocks = readBodyBlocks(WIRE_BLOCKS)!;
    expect(blocks.map((b) => b.kind)).toEqual(["heading", "paragraph", "heading", "bullet_list", "ordered_list"]);
    expect(blocks[0]).toEqual({ kind: "heading", level: 2, runs: [{ text: "Lịch tiêm chủng tháng 10", bold: false, italic: false }] });
    expect(blocks[2]).toMatchObject({ kind: "heading", level: 3 });
    const p = blocks[1] as Extract<BodyBlock, { kind: "paragraph" }>;
    expect(p.runs[1]).toEqual({ text: "toàn thể bà con", bold: true, italic: false });
    expect(p.runs[3]).toEqual({ text: "người cao tuổi", bold: true, italic: true });
    expect(p.runs[4]!.text).toBe(".\nMang theo sổ tiêm. Xem ");
    expect(p.runs[5]).toEqual({ text: "lịch chi tiết", bold: false, italic: false, href: LINK });
    expect(blocks[3]).toEqual({
      kind: "bullet_list",
      items: [[{ text: "Trạm y tế xã", bold: false, italic: false }], [{ text: "Nhà văn hoá thôn", bold: false, italic: false }]],
    });
  });

  it("an unknown kind — `script`, `image`, a later server's block — is skipped, the rest stays", () => {
    const blocks = readBodyBlocks([
      { kind: "script", runs: [{ text: "alert(1)" }] },
      { kind: "image", src: "https://anh.example.vn/x.jpg" },
      { kind: "iframe", runs: [{ text: "x" }] },
      { kind: "paragraph", runs: [{ text: "Còn lại." }] },
      null,
      "paragraph",
    ]);
    expect(blocks).toEqual([{ kind: "paragraph", runs: [{ text: "Còn lại.", bold: false, italic: false }] }]);
  });

  it("only an absolute https link with no user part is a link; anything else is the same words as plain text", () => {
    for (const href of [
      "http://thongtin.example.vn/",
      "javascript:alert(1)",
      "data:text/html,x",
      "/tin-tuc",
      "//thongtin.example.vn",
      "https://gov.vn@lua-dao.example/",
      "https://user:pass@thongtin.example.vn/",
      "",
      42,
    ]) {
      const [b] = readBodyBlocks([{ kind: "paragraph", runs: [{ text: "bấm đây", href }] }])!;
      expect(b, String(href)).toEqual({ kind: "paragraph", runs: [{ text: "bấm đây", bold: false, italic: false }] });
    }
    expect(readHttpsLink(LINK)).toBe(LINK);
  });

  it("not an array, empty, or nothing readable → absent (the screen shows `body`)", () => {
    for (const v of [undefined, null, "x", {}, [], [{ kind: "paragraph", runs: [] }], [{ kind: "bullet_list", items: [] }]]) {
      expect(readBodyBlocks(v), JSON.stringify(v)).toBeUndefined();
    }
    // A run with an empty or non-string text is skipped; a heading level that is not 3 draws as 2.
    expect(readBodyBlocks([{ kind: "heading", level: 7, runs: [{ text: "" }, { text: 5 }, { text: "Đầu mục" }] }])).toEqual([
      { kind: "heading", level: 2, runs: [{ text: "Đầu mục", bold: false, italic: false }] },
    ]);
  });

  it("the detail carries them as `bodyBlocks`; without `body_blocks` the key is absent, and `body` stays required", () => {
    const wire = { id: "tin-1", title: "T", summary: "", published_on: "2026-10-01", category_name: "", body: "Văn bản." };
    expect(docBaiTin({ ...wire, body_blocks: WIRE_BLOCKS })?.bodyBlocks).toHaveLength(5);
    const old = docBaiTin(wire)!;
    expect("bodyBlocks" in old).toBe(false);
    expect(old.noi_dung).toBe("Văn bản.");
    // A broken `body_blocks` never costs the article: the plain body is there.
    expect(docBaiTin({ ...wire, body_blocks: "<p>x</p>" })?.noi_dung).toBe("Văn bản.");
    expect(docBaiTin({ ...wire, body_blocks: WIRE_BLOCKS, body: undefined })).toBeNull();
  });
});

describe("the body drawn — React elements, never markup", () => {
  const blocks = readBodyBlocks(WIRE_BLOCKS)!;
  const render = (onLink?: (h: string) => void) =>
    html(createElement(ArticleBody, { blocks, text: "không dùng", paragraphClass: "xa-bai__doan", onLink }));

  it("headings under the title's h2 (h3, h4), real lists, bold and italic, a <br> per line break", () => {
    const page = render(noop);
    expect(page).toContain('<h3 class="xa-body-heading xa-body-heading--2">Lịch tiêm chủng tháng 10</h3>');
    expect(page).toContain('<h4 class="xa-body-heading xa-body-heading--3">Địa điểm</h4>');
    expect(page).toContain('<ul class="xa-body-list"><li>Trạm y tế xã</li><li>Nhà văn hoá thôn</li></ul>');
    expect(page).toContain('<ol class="xa-body-list"><li>Đăng ký</li><li>Tiêm</li></ol>');
    expect(page).toContain("<strong>toàn thể bà con</strong>");
    expect(page).toContain("<em>, nhất là </em>");
    expect(page).toContain("<strong><em>người cao tuổi</em></strong>");
    expect(page).toContain(".<br/>Mang theo sổ tiêm. Xem ");
    expect(page).not.toContain("không dùng"); // the plain body is not drawn beside the blocks
  });

  it("with an opener: the link is a button in the text, in its own words; without one: the same words, plain", () => {
    expect(render(noop)).toContain('Xem <button type="button" class="xa-body-link">lịch chi tiết</button>.');
    const plain = render(undefined);
    expect(plain).not.toContain("<button");
    expect(plain).toContain("Xem lịch chi tiết.");
    // The URL itself is never printed or put in an attribute — it reaches only the handler.
    expect(render(noop)).not.toContain("thongtin.example.vn");
  });

  it("a tap hands EXACTLY the link's URL to the handler, once", () => {
    const taps: string[] = [];
    const tree = ArticleBody({ blocks, text: "", paragraphClass: "xa-bai__doan", onLink: (h) => taps.push(h) });
    const [link] = buttons(tree);
    expect(link!.props.className).toBe("xa-body-link");
    link!.props.onClick();
    expect(taps).toEqual([LINK]);
  });

  it("text that looks like markup stays text", () => {
    const evil = readBodyBlocks([{ kind: "paragraph", runs: [{ text: '<script>alert("x")</script><img src=x onerror=alert(1)>' }] }]);
    const page = html(createElement(ArticleBody, { blocks: evil, text: "", paragraphClass: "xa-bai__doan" }));
    expect(page).not.toMatch(/<script\b|<img\b/i);
    expect(page).toContain("&lt;script&gt;");
  });

  it("FALLBACK: no blocks → the plain body, split into paragraphs, exactly as before", () => {
    expect(html(createElement(ArticleBody, { blocks: undefined, text: ARTICLE.noi_dung, paragraphClass: "xa-bai__doan" }))).toBe(
      '<p class="xa-bai__doan">Bản văn bản thuần.</p><p class="xa-bai__doan">Đoạn hai.</p>',
    );
  });

  it("the commune article draws the blocks; the shared app's article draws them too, with links as plain words", () => {
    const withBlocks = { ...ARTICLE, bodyBlocks: blocks };
    const commune = html(createElement(NewsArticle, { bai: withBlocks, coverFailed: false, onCoverFail: noop, ds: [], onLink: noop }));
    expect(commune).toContain('<div class="xa-ke"></div><h3 class="xa-body-heading xa-body-heading--2">');
    expect(commune).toContain('class="xa-body-link"');
    expect(commune).not.toContain("Bản văn bản thuần.");
    // No `onLink` → no button.
    expect(html(createElement(NewsArticle, { bai: withBlocks, coverFailed: false, onCoverFail: noop, ds: [] }))).not.toContain(
      "xa-body-link",
    );
    const shared = html(createElement(BaiTin, { bai: withBlocks }));
    expect(shared).toContain("<strong>toàn thể bà con</strong>");
    expect(shared).toContain("Xem lịch chi tiết.");
    expect(shared).not.toContain("<button");
    // And an older server: the shared app's paragraphs keep their class.
    expect(html(createElement(BaiTin, { bai: ARTICLE }))).toContain('<p class="cd-tin__doan">Bản văn bản thuần.</p>');
  });
});

describe("leaving the app — the question first, naming the host", () => {
  it("names the host read from the link; refuses to offer anything that is not https", () => {
    expect(linkHost(LINK)).toBe("thongtin.example.vn");
    expect(askToLeave(LINK)).toEqual({ url: LINK, host: "thongtin.example.vn", failed: false });
    for (const bad of ["http://thongtin.example.vn", "javascript:alert(1)", "/tin-tuc", "https://a@b.example"]) {
      expect(askToLeave(bad), bad).toBeNull();
    }
    expect(XA_TN.leave_app_question("thongtin.example.vn")).toBe("Bạn sắp rời ứng dụng để mở thongtin.example.vn");
  });

  it("the dialog: an alertdialog with the question, a note, 'Mở trang' and 'Ở lại ứng dụng' — and nothing opens without a tap", () => {
    const page = html(createElement(LeaveAppDialog, { host: "thongtin.example.vn", failed: false, onOpen: noop, onStay: noop }));
    expect(page).toContain('role="alertdialog" aria-modal="true" aria-labelledby="xa-leave-app-question"');
    expect(page).toContain(">Bạn sắp rời ứng dụng để mở thongtin.example.vn</h2>");
    expect(page).toContain(XA_TN.leave_app_note);
    expect(page).toContain(`<button type="button" class="xa-nut">${XA_TN.leave_app_open}</button>`);
    expect(page).toContain(`class="xa-nut xa-nut--phu"`);
    expect(page).toContain(XA_TN.leave_app_stay);
    expect(page).not.toContain('role="alert"');
  });

  it("each button reaches its own handler", () => {
    const calls: string[] = [];
    const tree = LeaveAppDialog({ host: "h.example", failed: false, onOpen: () => calls.push("open"), onStay: () => calls.push("stay") });
    const [open, stay] = buttons(tree);
    open!.props.onClick();
    stay!.props.onClick();
    expect(calls).toEqual(["open", "stay"]);
  });

  it("a failed open: one sentence saying what to do next — words, no code", () => {
    const page = html(createElement(LeaveAppDialog, { host: "h.example", failed: true, onOpen: noop, onStay: noop }));
    expect(page).toContain(`<p class="xa-error-box" role="alert">${XA_TN.leave_app_failed}</p>`);
    expect(XA_TN.leave_app_failed).toMatch(/bấm “Mở trang” lần nữa/);
    expect(XA_TN.leave_app_failed).not.toMatch(/\d{3}|lỗi|error|demo|trải nghiệm/i);
  });

  it("'Mở trang' calls the opener with EXACTLY the link; failed = it did not open; a rejection is a failure", async () => {
    const calls: string[] = [];
    const opener = (answer: boolean) => async (url: string) => {
      calls.push(url);
      return answer;
    };
    expect(await externalOpenFailed(opener(true), LINK)).toBe(false);
    expect(await externalOpenFailed(opener(false), LINK)).toBe(true);
    expect(await externalOpenFailed(() => Promise.reject(new Error("x")), LINK)).toBe(true);
    expect(calls).toEqual([LINK, LINK]);
  });
});

/* ════════════════════════════════════════ BANNERS ════════════════════════════════════════ */

const IMG = (n: number) => `https://cdn.example.vn/t_TENANT/banner-${n}.jpg`;

describe("banners — `?type=banner`, read as sent", () => {
  it("the address carries host and type=banner, nothing else", () => {
    const url = new URL(bannersAddress("xa-thu.vigov.example"));
    expect(url.pathname).toBe(DUONG_DAN_TIN_XA);
    expect([...url.searchParams.entries()].sort()).toEqual([
      ["host", "xa-thu.vigov.example"],
      ["type", "banner"],
    ]);
  });

  it("keeps the server's ORDER; title becomes the alt; link_to kept as a path or an https URL", () => {
    const list = readBanners({
      items: [
        { id: "b2", type: "banner", title: " Ngày hội ", image_url: IMG(2), link_to: "/su-kien" },
        { id: "b1", type: "banner", title: "Tiêm chủng", image_url: IMG(1), link_to: LINK },
        { id: "b3", type: "banner", title: "Ảnh", image_url: IMG(3) },
      ],
      has_more: false,
    });
    expect(list).toEqual([
      { id: "b2", title: "Ngày hội", imageUrl: IMG(2), linkTo: "/su-kien" },
      { id: "b1", title: "Tiêm chủng", imageUrl: IMG(1), linkTo: LINK },
      { id: "b3", title: "Ảnh", imageUrl: IMG(3) },
    ]);
  });

  it("no usable picture → that banner is dropped; a bad link_to → a picture with no tap; no title → no tap", () => {
    const list = readBanners({
      items: [
        { id: "a", title: "A", image_url: "http://cdn.example.vn/a.jpg" },
        { id: "b", title: "B" },
        { id: "c", title: "C", image_url: IMG(3), link_to: "//lua-dao.example/x" },
        { id: "d", title: "D", image_url: IMG(4), link_to: "http://thongtin.example.vn" },
        { id: "e", title: "E", image_url: IMG(5), link_to: "https://gov.vn@lua-dao.example/" },
        { id: "f", title: "  ", image_url: IMG(6), link_to: "/tin-tuc" },
        { id: "c", title: "C again", image_url: IMG(7) },
      ],
    })!;
    expect(list.map((b) => b.id)).toEqual(["c", "d", "e", "f"]);
    expect(list.every((b) => b.linkTo === undefined)).toBe(true);
  });

  it("a field of the wrong type, or not a list → `null` (the bundled picture stays)", () => {
    expect(readBanners({ items: [{ id: 1, title: "x", image_url: IMG(1) }] })).toBeNull();
    expect(readBanners({ items: [{ id: "x", title: "x", image_url: 5 }] })).toBeNull();
    expect(readBanners({ items: [{ id: "x", title: "x", image_url: IMG(1), link_to: 5 }] })).toBeNull();
    expect(readBanners({})).toBeNull();
    expect(readBanners({ items: [] })).toEqual([]);
  });
});

describe("banners — the call", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("GET the comms list route with type=banner, no bearer; a 500 is a branch, not an exception", async () => {
    const calls: Array<{ url: string; init: RequestInit }> = [];
    let answer: { status: number; ok: boolean; json: () => Promise<unknown> } = {
      status: 200,
      ok: true,
      json: async () => ({ items: [{ id: "b1", title: "T", image_url: IMG(1) }], has_more: false }),
    };
    vi.stubGlobal("fetch", (url: string, init: RequestInit) => {
      calls.push({ url, init });
      return Promise.resolve(answer);
    });
    const ok = await communeBanners("xa-thu.vigov.example");
    expect(ok).toEqual({ kieu: "xong", gia_tri: [{ id: "b1", title: "T", imageUrl: IMG(1) }] });
    expect(new URL(calls[0]!.url).searchParams.get("type")).toBe("banner");
    expect(JSON.stringify(calls[0]!.init.headers ?? {})).not.toMatch(/Authorization/i);
    answer = { status: 500, ok: false, json: async () => ({}) };
    expect((await communeBanners("xa-thu.vigov.example")).kieu).toBe("loi-may-chu");
    // A malformed domain never reaches the network.
    const before = calls.length;
    expect((await communeBanners("not a domain")).kieu).toBe("khong-hop-le");
    expect(calls.length).toBe(before);
  });
});

describe("banners — where a tap goes", () => {
  it("known in-app paths open their screen; one trailing slash is fine; /tin-tuc/<id> opens that article", () => {
    expect(bannerScreen("/tin-tuc")).toEqual({ kieu: "tab", tab: "tin-tuc" });
    expect(bannerScreen("/tin-tuc/")).toEqual({ kieu: "tab", tab: "tin-tuc" });
    expect(bannerScreen("/phan-anh")).toEqual({ kieu: "tab", tab: "phan-anh" });
    expect(bannerScreen("/ca-nhan")).toEqual({ kieu: "tab", tab: "ca-nhan" });
    expect(bannerScreen("/gui-phan-anh")).toEqual({ kieu: "gui" });
    expect(bannerScreen("/danh-ba")).toEqual({ kieu: "danh-ba" });
    expect(bannerScreen("/su-kien")).toEqual({ kieu: "su-kien" });
    expect(bannerScreen("/truyen-thanh")).toEqual({ kieu: "truyen-thanh" });
    expect(bannerScreen("/video")).toEqual({ kieu: "video" });
    expect(bannerScreen("/tin-tuc/01JABC%20X")).toEqual({ kieu: "bai", id: "01JABC X", tu: "trang-chu" });
  });

  it("anything else is NOT tappable — unknown, home, a query, a fragment, `//`, a bad escape, a deeper path", () => {
    for (const p of [
      "/",
      "/khong-co",
      "/tin-tuc?x=1",
      "/tin-tuc#a",
      "//lua-dao.example",
      "/tin-tuc/%E0%A4%A",
      "/tin-tuc/a/b",
      "/tin-tuc/%20",
      "tin-tuc",
    ]) {
      expect(bannerScreen(p), p).toBeNull();
    }
  });

  it("an https target is tappable only with an opener; a path never needs one", () => {
    expect(bannerTap(undefined, true)).toBeNull();
    expect(bannerTap(LINK, true)).toEqual({ kind: "web", url: LINK });
    expect(bannerTap(LINK, false)).toBeNull();
    expect(bannerTap("/danh-ba", false)).toEqual({ kind: "screen", screen: { kieu: "danh-ba" } });
    expect(bannerTap("/khong-co", true)).toBeNull();
  });
});

describe("banners — the strip on the home screen", () => {
  const ITEMS: CommuneBannerItem[] = [
    { id: "b2", title: "Ngày hội", imageUrl: IMG(2), linkTo: "/su-kien" },
    { id: "b1", title: "Tiêm chủng", imageUrl: IMG(1), linkTo: LINK },
    { id: "b3", title: "Ảnh xã", imageUrl: IMG(3) },
  ];
  const strip = (items: readonly CommuneBannerItem[], canOpenWeb = true) =>
    html(createElement(BannerStrip, { items, canOpenWeb, onScreen: noop, onWeb: noop, onFail: noop }));

  it("pictures in the server's order, alt = title, a labelled list; tappable ones are buttons, the rest plain pictures", () => {
    const page = strip(ITEMS);
    expect([...page.matchAll(/alt="([^"]*)"/g)].map((m) => m[1])).toEqual(["Ngày hội", "Tiêm chủng", "Ảnh xã"]);
    expect(page).toContain(`<ul class="xa-banner-strip xa-banner-strip--many" aria-label="${XA_TN.banner_strip}">`);
    expect(page.match(/<button type="button" class="xa-banner-strip__tap">/g)).toHaveLength(2);
    expect(page).toContain(`<li class="xa-banner-strip__item"><img class="xa-banner__anh" src="${IMG(3)}" alt="Ảnh xã"`);
    // The bundled picture is NOT drawn beside a posted strip.
    expect(page).not.toContain("banner-xa.png");
    // No opener → the https banner is a picture only.
    expect(strip(ITEMS, false).match(/<button/g)).toHaveLength(1);
    // One picture fills the width: no `--many`.
    expect(strip([ITEMS[2]!])).toContain('<ul class="xa-banner-strip" ');
  });

  it("a tap: an in-app path opens its screen; an https target goes to the question, never straight out", () => {
    const screens: unknown[] = [];
    const webs: string[] = [];
    const tree = BannerStrip({ items: ITEMS, canOpenWeb: true, onScreen: (m) => screens.push(m), onWeb: (u) => webs.push(u), onFail: noop });
    const [toEvents, toLink] = buttons(tree);
    toEvents!.props.onClick();
    toLink!.props.onClick();
    expect(screens).toEqual([{ kieu: "su-kien" }]);
    expect(webs).toEqual([LINK]);
  });

  it("FALLBACK: loading / failed / none posted → the bundled picture, never a blank band", () => {
    const bundled = '<div class="xa-banner"><img class="xa-banner__anh" src="./banner-xa.png" alt=""/></div>';
    const home = (items: readonly CommuneBannerItem[] | null) =>
      html(createElement(HomeBanner, { items, canOpenWeb: true, onScreen: noop, onWeb: noop }));
    expect(home(null)).toContain(bundled);
    expect(home([])).toContain(bundled);
    const posted = home(ITEMS);
    expect(posted).not.toContain("banner-xa.png");
    expect(posted).toContain('class="xa-banner-strip');
  });
});

describe("wiring — the commune app only, through the declared destination, and nothing recorded", () => {
  const raw = import.meta.glob(
    ["../../App.tsx", "./TinTucXaScreen.tsx", "./leave-app.tsx", "./article-body.tsx", "./TrangXa.tsx", "./TinTucAppXa.tsx"],
    { query: "?raw", import: "default", eager: true },
  ) as Record<string, string>;
  const code = (p: string) => raw[p]!.replace(/\/\*[\s\S]*?\*\//g, " ").replace(/(?<!:)\/\/[^\n]*/g, " ");

  it("AppRieng injects `moRaNgoai(\"lien-ket-xa\", …)`; AppChung does not; the shared news screen takes no opener", () => {
    const app = raw["../../App.tsx"]!;
    const own = app.slice(app.indexOf("export function AppRieng("), app.indexOf("function AppChung("));
    expect(own).toMatch(/openLink=\{openCommuneLink\}/);
    expect(app).toMatch(/const openCommuneLink: OpenExternal = \(url\) => moRaNgoai\("lien-ket-xa", url\);/);
    expect(app.slice(app.indexOf("function AppChung("))).not.toMatch(/openLink|openCommuneLink/);
    expect(code("./TinTucXaScreen.tsx")).not.toMatch(/onLink|openLink|useLeaveApp/);
  });

  it("the state half opens nothing and records nothing on a tap: no door, no SDK, no console, no network", () => {
    for (const p of ["./leave-app.tsx", "./article-body.tsx", "./TrangXa.tsx", "./TinTucAppXa.tsx"]) {
      expect(code(p).length, p).toBeGreaterThan(500);
      expect(code(p), p).not.toMatch(/moRaNgoai|moTrangWeb|zmp-sdk|window\s*\.\s*open|console\s*\.|fetch\s*\(|location\s*\.\s*href/);
    }
  });

  it("the body link is underlined as well as coloured, and its tap area grows to ≥ 44px", async () => {
    const nodeFs = "node:fs";
    const { readFileSync } = (await import(/* @vite-ignore */ nodeFs)) as {
      readFileSync: (path: URL, encoding: "utf8") => string;
    };
    const css = readFileSync(new URL("../../styles.css", import.meta.url), "utf8");
    const rule = /\.xa-body-link \{([^}]*)\}/.exec(css)![1]!;
    expect(rule).toMatch(/text-decoration: underline;/);
    expect(rule).toMatch(/color: var\(--xa-blue-ink\);/);
    // ~20px of text (17px body, the font's content height) + 2 × 12px ≥ 44px.
    const pad = Number(/padding: (\d+)px/.exec(rule)![1]);
    expect(2 * pad + 20).toBeGreaterThanOrEqual(44);
    expect(css).toMatch(/\.xa-leave-app \{[^}]*position: fixed;/);
  });
});
