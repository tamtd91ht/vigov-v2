/// <reference types="vite/client" />
import { describe, expect, it } from "vitest";

import indexHtml from "../index.html?raw";
import { SCREENS } from "./features/company-intro/screens";

/**
 * The stylesheet is read from disk, not imported: vitest resolves a CSS import to an empty
 * string unless CSS processing is turned on, and a sweep over an empty string passes every
 * assertion for the wrong reason. The specifier is held in a variable because this package
 * carries no `@types/node` — it ships to a browser, and the one test that needs a file read is
 * not a reason to add node typings to it.
 */
const nodeFs = "node:fs";
const { readFileSync } = (await import(/* @vite-ignore */ nodeFs)) as {
  readFileSync: (path: URL, encoding: "utf8") => string;
};
const styles = readFileSync(new URL("./styles.css", import.meta.url), "utf8");

/**
 * HTML comments are removed before scanning: index.html EXPLAINS in prose that it never sets
 * `user-scalable=no`, and scanning the comment would fail the rule the comment describes.
 */
const shell = indexHtml.replace(/<!--[\s\S]*?-->/g, " ");

/**
 * WHY A STYLESHEET IS WORTH TESTING HERE AND ALMOST NOWHERE ELSE:
 *
 *   Citizens do not choose this software. Not being able to read it means not being able to
 *   reach a public service, which makes body size, tap size and contrast a rights question
 *   rather than a preference (README §Non-negotiables #6).
 *
 *   These four thresholds live in four CSS custom properties. Anyone can lower one to make a
 *   layout fit — the app still renders, every other test stays green, and the only person who
 *   notices is an ageing citizen holding the phone at arm's length in sunlight, who has no way
 *   to report it. That is a silent failure, so it gets a test.
 */

/** Reads a `--token: value;` declaration out of the stylesheet. */
function token(name: string): string {
  const match = new RegExp(`--${name}:\\s*([^;]+);`).exec(styles);
  expect(match, `stylesheet no longer declares --${name}`).not.toBeNull();
  return match![1]!.trim();
}

const pixels = (name: string): number => Number.parseFloat(token(name));

/** Relative luminance per WCAG 2.1, from a `#rrggbb` string. */
function luminance(hex: string): number {
  const channels = [1, 3, 5].map((offset) => Number.parseInt(hex.slice(offset, offset + 2), 16) / 255);
  const linear = channels.map((channel) =>
    channel <= 0.03928 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4,
  );
  return 0.2126 * linear[0]! + 0.7152 * linear[1]! + 0.0722 * linear[2]!;
}

function contrast(foreground: string, background: string): number {
  const [lighter, darker] = [luminance(foreground), luminance(background)].sort((a, b) => b - a);
  return (lighter! + 0.05) / (darker! + 0.05);
}

describe("text and targets stay usable for an ageing eye", () => {
  it("never drops body text below 16px", () => {
    expect(pixels("text-body")).toBeGreaterThanOrEqual(16);
    expect(pixels("text-small")).toBeGreaterThanOrEqual(16);
  });

  it("keeps every tap target at 44px or more", () => {
    expect(pixels("tap-min")).toBeGreaterThanOrEqual(44);
  });

  it("applies the tap minimum to the controls that are actually tapped", () => {
    // A token nothing references protects nothing. The tab bar buttons and the contact actions
    // are the only tappable things in phase 1.
    expect(styles).toMatch(/\.tabbar__item\s*\{[^}]*min-height:\s*var\(--tap-min\)/);
    expect(styles).toMatch(/\.action\s*\{[^}]*min-height:\s*var\(--tap-min\)/);

    // Lớp khám phá (ADR 0005): nút xác nhận xã, từng dòng trong danh mục xã, và nút đổi xã trên
    // header. Đây là những nút một người lớn tuổi bấm khi đang đứng ở trụ sở xã, một tay cầm
    // điện thoại — và bấm trượt ở đây nghĩa là gửi hồ sơ cho một xã khác.
    expect(styles).toMatch(/\.goi-y__nut\s*\{[^}]*min-height:\s*var\(--tap-min\)/);
    expect(styles).toMatch(/\.chon-xa__dong\s*\{[^}]*min-height:\s*var\(--tap-min\)/);
    expect(styles).toMatch(/\.app-header__doi-xa\s*\{[^}]*min-height:\s*var\(--tap-min\)/);

    // Trang xã: mỗi mục dịch vụ là một nút. Chúng là những nút duy nhất trên màn ấy, và người
    // bấm chúng là người vừa đứng dậy khỏi ghế chờ ở trụ sở xã.
    expect(styles).toMatch(/\.dich-vu__nut\s*\{[^}]*min-height:\s*var\(--tap-min\)/);

    // Ba tính năng: nút xin quyền, nút "Quét mã khác", và nút hành động dùng chung (Gọi · Gửi
    // email · Mở liên kết · Chỉ đường). Bấm trượt ở đây thì hoặc mở nhầm máy ảnh, hoặc — tệ hơn —
    // bấm vào một nút xin dữ liệu mà người dùng không định bấm; ở thẻ danh thiếp thì là gọi nhầm
    // số của một người thật.
    expect(styles).toMatch(/\.tn__nut\s*\{[^}]*min-height:\s*var\(--tap-min\)/);
    expect(styles).toMatch(/\.tn__nut-phu\s*\{[^}]*min-height:\s*var\(--tap-min\)/);
    expect(styles).toMatch(/\.tn-hanh-dong\s*\{[^}]*min-height:\s*var\(--tap-min\)/);

    // Nút bật/tắt giữ màn hình sáng. Người bấm nó đang đứng đối diện một người khác, một tay
    // cầm máy chìa mã ra — đúng tư thế bấm trượt nhất — và bấm trượt ở đây là tắt mất thứ đang
    // giữ màn hình sáng giữa lúc người kia quét.
    expect(styles).toMatch(/\.tn__nut-giu\s*\{[^}]*min-height:\s*var\(--tap-min\)/);
  });

  it("animates nothing outside a reduced-motion guard", () => {
    // Motion is decoration here — a drifting constellation behind a dark panel. For a citizen
    // with a vestibular disorder or a migraine it is not decoration, and the phone already
    // carries their answer. An `animation:` declared outside the guard ignores that answer, and
    // it is invisible to every other check in this package.
    const opener = "@media (prefers-reduced-motion: no-preference) {";
    const start = styles.indexOf(opener);
    let end = styles.length;
    if (start >= 0) {
      let depth = 0;
      for (let at = start + opener.length - 1; at < styles.length; at += 1) {
        if (styles[at] === "{") depth += 1;
        if (styles[at] === "}") {
          depth -= 1;
          if (depth === 0) {
            end = at;
            break;
          }
        }
      }
    }
    for (const match of styles.matchAll(/animation(?:-name)?\s*:/g)) {
      expect(
        match.index! > start && match.index! < end && start >= 0,
        `an animation is declared outside @media (prefers-reduced-motion: no-preference) at offset ${match.index}`,
      ).toBe(true);
    }
  });

  it("keeps pinch-zoom available", () => {
    // Disabling zoom is the single most common way a mobile page becomes unusable, and it is
    // usually added to protect a layout, by someone who can read the page fine.
    expect(shell).not.toMatch(/user-scalable\s*=\s*no/);
    expect(shell).not.toMatch(/maximum-scale/);
  });
});

/**
 * WHY A GRADIENT CAN BE CHECKED AT ALL:
 *
 *   A ratio can only be computed against a colour that is written down. The dark panels and the
 *   page wash are therefore built from SOLID stops declared as tokens — `--panel-glow-blue` and
 *   `--panel-glow-green` are the lightest points of the panel gradients, `--surface-tint`,
 *   `--surface-tint-green` and `--grid-line` are the extremes of the page background. Every pair
 *   below is text that actually lands on one of them. A gradient fading to `transparent` would
 *   pass through colours nobody measured, and this file could say nothing about it.
 *
 *   The brand pair is here for the opposite reason: #00aef4 and #78bd1a are LIGHT (2.5:1 and
 *   2.3:1 against white), so the two cases pinned are the dark glyph that is allowed to sit on
 *   them. If someone ever puts white text on a brand tile, nothing renders wrong — it just
 *   becomes unreadable outdoors, for the person least able to report it.
 */
describe("every colour pair the app actually renders clears 4.5:1", () => {
  const pairs: ReadonlyArray<[string, string, string]> = [
    ["body text on the page", token("ink"), token("surface-alt")],
    ["body text on a card", token("ink"), token("surface")],
    ["secondary text on the page", token("ink-muted"), token("surface-alt")],
    ["secondary text on a card", token("ink-muted"), token("surface")],
    ["header and primary action text", "#ffffff", token("navy")],
    ["card titles and figures", token("navy"), token("surface")],

    // Page background: the blue wash, the green wash and the graph-paper rule drawn over both.
    ["body text on the blue wash", token("ink"), token("surface-tint")],
    ["secondary text on the blue wash", token("ink-muted"), token("surface-tint")],
    ["chip text on its tinted pill", token("ink"), token("surface-tint")],
    ["body text on the green wash", token("ink"), token("surface-tint-green")],
    ["secondary text on the green wash", token("ink-muted"), token("surface-tint-green")],
    ["secondary text crossing a grid line", token("ink-muted"), token("grid-line")],
    ["section titles on the washes", token("navy"), token("surface-tint")],

    // Dark panels: hero, banner, statistics band, primary action. Checked at BOTH ends of each
    // gradient, because "the darkest end is fine" is exactly the half-check that ships.
    ["panel text at the navy end", "#ffffff", token("navy")],
    ["panel text at the deep end", "#ffffff", token("navy-deep")],
    ["panel text at the blue glow", "#ffffff", token("panel-glow-blue")],
    ["panel text at the green glow", "#ffffff", token("panel-glow-green")],
    ["figure labels on the navy panel", token("on-panel"), token("navy")],
    ["figure labels at the blue glow", token("on-panel"), token("panel-glow-blue")],
    ["figure labels at the green glow", token("on-panel"), token("panel-glow-green")],

    // The drawn constellation passes BEHIND the words. A 1px node line is thin, and a thin light
    // line under a letter is exactly what an ageing eye loses the letter in, so the line is a
    // background colour like any other: these two are the stroke at the opacity it is drawn with,
    // over the lightest point of the panel.
    ["panel text crossing a blue node line", "#ffffff", token("panel-line-blue")],
    ["panel text crossing a green node line", "#ffffff", token("panel-line-green")],
    ["figure labels crossing a blue node line", token("on-panel"), token("panel-line-blue")],
    ["figure labels crossing a green node line", token("on-panel"), token("panel-line-green")],

    // Brand surfaces. The glyph is dark BECAUSE the brand colours are light.
    ["the glyph on the blue end of a tile", token("tile-glyph"), token("brand-blue")],
    ["the glyph on the green end of a tile", token("tile-glyph"), token("brand-green")],

    // LỚP KHÁM PHÁ (ADR 0005). Đây là những chữ quyết định công dân gửi hồ sơ cho xã nào, và
    // chúng được đọc ở ngoài trời, trước cổng trụ sở xã, bởi một người đang cầm điện thoại xa
    // mắt. Chúng dùng lại đúng bảng màu đã đo ở trên — nhưng liệt kê riêng, vì một cặp màu chỉ
    // được bảo vệ khi có tên nó trong danh sách này: đổi `.xa-the__ten` sang --brand-blue thì
    // không có dòng nào ở trên đỏ lên.
    ["the commune name on the confirm card", token("navy"), token("surface")],
    ["the province line under the commune name", token("ink-muted"), token("surface")],
    ["the confirm button label", "#ffffff", token("navy")],
    ["the confirm button label at its blue glow", "#ffffff", token("panel-glow-blue")],
    ["the reason line above the commune list", token("ink"), token("surface")],
    ["a commune row in the picker", token("navy"), token("surface")],
    ["the province of a commune row", token("ink-muted"), token("surface")],
    ["the demo-data footnote", token("ink-muted"), token("surface-alt")],
    ["the demo-data footnote on the blue wash", token("ink-muted"), token("surface-tint")],
    ["the change-commune control in the header", "#ffffff", token("navy-deep")],
    ["the change-commune control at the header glow", "#ffffff", token("navy")],

    // TRANG XÃ. Cùng bảng màu, liệt kê riêng vì cùng một lý do như trên: đổi `.trang-xa__so`
    // sang --brand-green thì không dòng nào ở trên đỏ lên, và số điện thoại trực — thứ người
    // dân đọc rồi bấm sang máy — là chữ mất trước nhất khi ra nắng.
    ["the commune name on its own page", token("navy"), token("surface-alt")],
    ["the commune name on its own page, over the blue wash", token("navy"), token("surface-tint")],
    [
      "the commune name on its own page, over the green wash",
      token("navy"),
      token("surface-tint-green"),
    ],
    ["the province line on the commune page", token("ink-muted"), token("surface-alt")],
    ["the one-line introduction of the commune", token("ink"), token("surface-alt")],
    ["the duty-phone label", token("ink-muted"), token("surface")],
    ["the duty-phone number", token("navy"), token("surface")],
    ["the working hours under it", token("ink-muted"), token("surface")],
    ["a service name on the commune page", token("navy"), token("surface")],
    ["the 'not open yet' wording under a service", token("ink-muted"), token("surface")],
    ["the under-construction note a tapped service opens", token("ink"), token("surface")],
    ["the demo-data footnote on the commune page", token("ink-muted"), token("surface-alt")],

    // BA TÍNH NĂNG THẬT. Liệt kê riêng vì cùng lý do như trên: một cặp màu chỉ được bảo vệ khi có
    // tên nó trong danh sách này. Đây là chữ người dùng đọc TRƯỚC KHI quyết định có chia sẻ số
    // điện thoại hay vị trí của mình hay không — đọc không ra thì họ hoặc bấm bừa, hoặc bỏ đi;
    // cả hai đều tệ hơn một chữ to.
    ["the feature title on its card", token("navy"), token("surface")],
    ["the feature lead line", token("ink-muted"), token("surface")],
    ["the reason the app needs this permission", token("ink"), token("surface")],
    ["the permission button label", "#ffffff", token("navy")],
    ["the permission button label at its blue glow", "#ffffff", token("panel-glow-blue")],
    ["the permission button label while waiting", "#ffffff", token("navy-deep")],
    ["the scan-again button label", token("navy"), token("surface")],
    ["a call / mail / directions button label", token("navy"), token("surface")],
    ["the 'token received' line", token("navy"), token("surface")],
    ["the measured token figures", token("ink"), token("surface")],
    ["the note saying the real data never reaches the device", token("ink"), token("surface-alt")],
    ["the sentence naming what this build cannot do yet", token("ink"), token("surface-alt")],
    ["the sentence shown when the user declines", token("ink"), token("surface-alt")],
    ["the reminder that a scanned card is someone else's data", token("ink-muted"), token("surface")],
    ["the promise that nothing leaves this screen", token("ink-muted"), token("surface")],

    // KHỐI ĐĂNG NHẬP (ADR 0020). Không một MÀU MỚI nào ở đây — khối này dùng lại đúng những lớp
    // của năm khối kia (`tn__xong` · `tn__do` · `tn__giai-thich` · `tn__ranh-gioi` · `tn__loi`).
    // Vẫn liệt kê riêng, vì cùng lý do như trên: một cặp màu chỉ được bảo vệ khi có tên nó ở
    // đây, và đây là chữ người dùng đọc ở đúng khoảnh khắc họ quyết định có chia sẻ số điện
    // thoại của mình hay không.
    ["the 'you are signed in' line", token("navy"), token("surface")],
    ["the session expiry line under it", token("ink"), token("surface")],
    ["the note that the session is kept in memory only", token("ink"), token("surface-alt")],
    ["the 'opening your session' line", token("ink"), token("surface-alt")],
    ["the sentence saying the sign-in code expired", token("ink"), token("surface-alt")],
    ["the sentence saying Zalo did not answer", token("ink"), token("surface-alt")],

    // THẺ DANH THIẾP quét được. Đây là tên, số điện thoại và email của một người thật, đọc trong
    // mười giây sau khi bắt tay ở một hội thảo — đọc nhầm một ký tự là gọi nhầm một người.
    ["the scanned card title", token("navy"), token("surface-alt")],
    ["a field label on the scanned card", token("ink-muted"), token("surface-alt")],
    ["a field value on the scanned card", token("ink"), token("surface-alt")],
    ["the verbatim content of a non-vCard code", token("ink"), token("surface")],

    // BA VĂN PHÒNG. Địa chỉ là thứ người ta đọc rồi gõ vào bản đồ, hoặc đọc cho tài xế nghe.
    ["an office name", token("navy"), token("surface")],
    ["an office address", token("ink-muted"), token("surface")],
    ["an office address over the blue wash", token("ink-muted"), token("surface-tint")],

    // BA TÍNH NĂNG THÊM VÀO: kiểm tra đường truyền · danh thiếp của chúng tôi · số hoá thiếp
    // giấy. Cùng bảng màu, liệt kê riêng vì cùng lý do như trên — một cặp màu chỉ được bảo vệ
    // khi có tên nó ở đây.
    ["the network-type answer", token("navy"), token("surface")],
    ["what that network type means for a call", token("ink"), token("surface-alt")],
    ["the sentence saying no speed was measured", token("ink"), token("surface-alt")],
    ["the keep-screen-on toggle label", token("navy"), token("surface")],
    ["the note shown while the screen is held awake", token("ink"), token("surface-alt")],
    ["a field label on OUR namecard", token("ink-muted"), token("surface")],
    ["a field value on OUR namecard", token("ink"), token("surface")],
    ["the namecard-in-the-code heading", token("navy"), token("surface")],
    ["the camera-permission answer", token("navy"), token("surface")],
    ["the sentence shown when camera permission is declined", token("ink"), token("surface-alt")],
    ["the caption under the chosen photo", token("ink-muted"), token("surface")],
    ["the sentence saying this build reads no text from the photo", token("ink"), token("surface-alt")],

    /**
     * MÃ QR — CẶP MÀU DUY NHẤT Ở ĐÂY KHÔNG PHẢI VỀ CHỮ, VÀ LÀ CẶP CÓ HẬU QUẢ RÕ NHẤT.
     *
     * Máy quét đọc mã bằng ngưỡng sáng/tối. Một mã vẽ bằng `--navy` trên `--surface-alt` cho
     * "hợp thương hiệu" vẫn trông hoàn hảo trên màn hình trong nhà và KHÔNG QUÉT ĐƯỢC ngoài
     * nắng — tính năng hỏng đúng ở chỗ nó được dùng, và người phát hiện ra là khách hàng đang
     * đứng trước mặt nhân viên kinh doanh. 21:1 là ngưỡng thật sự cần ở đây, không phải 4,5:1.
     */
    ["the QR modules against their quiet zone", token("ma-qr-toi"), token("ma-qr-sang")],
  ];

  for (const [what, foreground, background] of pairs) {
    it(`keeps ${what} readable`, () => {
      expect(
        contrast(foreground, background),
        `${what}: ${foreground} on ${background} is below the 4.5:1 this app committed to`,
      ).toBeGreaterThanOrEqual(4.5);
    });
  }
});

/**
 * THANH TAB TRÊN MÁY HẸP NHẤT CÒN BÁN ĐƯỢC — 320px.
 *
 * VÌ SAO ĐÂY LÀ MỘT PHÉP KIỂM CHỨ KHÔNG PHẢI MỘT CHÚ THÍCH:
 *
 *   Thanh tab nay có NĂM tab, không còn bốn. Trên một máy 320px mỗi tab chỉ còn khoảng 56px
 *   chữ, và một nhãn dài hơn thế thì hoặc bị cắt mất đuôi, hoặc đẩy thanh tab vỡ hàng. Người
 *   gặp chuyện đó là người dùng một chiếc điện thoại cũ — đúng người ít có khả năng báo lại nhất.
 *
 *   `screens.ts` vẫn giữ một dòng chú thích về bề rộng 320px từ khi còn bốn tab. Một chú thích
 *   không đếm được gì: thêm tab thứ năm là đủ làm nó sai mà không có gì đỏ lên. Nên nó thành một
 *   phép đo.
 *
 * ĐO THEO TỪ, KHÔNG THEO CẢ NHÃN — và đó là phép đo đúng, không phải phép đo dễ:
 *
 *   `.tabbar__item` không đặt `white-space: nowrap`, nên "Trang chủ" xuống hai dòng một cách
 *   bình thường và không tràn đi đâu. Thứ KHÔNG xuống dòng được là một TỪ. Nên ngưỡng đặt ở từ
 *   dài nhất của mỗi nhãn, và có một ca riêng khẳng định `nowrap` chưa bị ai thêm vào.
 *
 * HỆ SỐ 0,62em LÀ CẬN TRÊN, CÓ CHỦ ĐÍCH: chữ thường của Inter rộng khoảng 0,5em, chữ hoa khoảng
 * 0,7em, và tab đang xem còn in đậm. Đo bằng cận trên thì phép kiểm sai về phía AN TOÀN — nó
 * kêu sớm hơn thực tế, chứ không im lặng cho tới lúc một người nhìn thấy nhãn mất đuôi.
 */
describe("thanh tab không tràn trên máy 320px", () => {
  const BE_RONG_MAY = 320;
  const RONG_MOI_EM = 0.62;

  /** Đệm ngang của một tab, đọc thẳng từ `.tabbar__item { padding: 10px 4px; }`. */
  const demNgang = (): number => {
    const luat = /\.tabbar__item\s*\{([^}]*)\}/.exec(styles);
    expect(luat, "stylesheet no longer declares .tabbar__item").not.toBeNull();
    const padding = /padding:\s*([^;]+);/.exec(luat![1]!)?.[1]?.trim().split(/\s+/) ?? [];
    expect(padding.length, ".tabbar__item no longer declares a two-value padding").toBe(2);
    return Number.parseFloat(padding[1]!) * 2;
  };

  it("mỗi từ của mỗi nhãn tab lọt trong phần bề rộng của tab ấy", () => {
    // Cỡ chữ của tab là `--text-small`, thứ đã được ghim ≥16px ở trên. Đọc lại từ chính CSS chứ
    // không viết cứng số 16: đổi token mà phép kiểm này vẫn dùng số cũ là một phép kiểm nói dối.
    expect(styles).toMatch(/\.tabbar__item\s*\{[^}]*font-size:\s*var\(--text-small\)/);

    const co_chu = pixels("text-small");
    const cho_chu = BE_RONG_MAY / SCREENS.length - demNgang();
    expect(SCREENS.length, "sổ màn hình rỗng — phép đo này sẽ xanh vì lý do sai").toBeGreaterThan(0);

    for (const man of SCREENS) {
      for (const tu of man.tabLabel.split(/\s+/)) {
        const rong = tu.length * co_chu * RONG_MOI_EM;
        expect(
          rong,
          `nhãn tab "${man.tabLabel}": từ "${tu}" cần ~${rong.toFixed(0)}px, tab chỉ còn ` +
            `${cho_chu.toFixed(0)}px trên máy ${BE_RONG_MAY}px. Rút ngắn nhãn, đừng thu nhỏ chữ.`,
        ).toBeLessThanOrEqual(cho_chu);
      }
    }
  });

  it("không ai đặt `white-space: nowrap` lên tab — đó là thứ biến xuống dòng thành tràn", () => {
    const luat = /\.tabbar__item\s*\{([^}]*)\}/.exec(styles)?.[1] ?? "";
    expect(luat).not.toMatch(/white-space\s*:\s*nowrap/);
  });
});
