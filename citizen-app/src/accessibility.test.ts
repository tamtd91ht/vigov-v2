/// <reference types="vite/client" />
import { describe, expect, it } from "vitest";

import indexHtml from "../index.html?raw";
import { TABS } from "./features/company-intro/screens";
import { MUC_MENU_NHANH } from "./features/company-intro/MenuNhanh";

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

    // HAI NÚT THÊM VÀO 21/09/2026.
    //
    //   `.solution--mo` là cả một thẻ giải pháp, nay bấm được. Nó — chứ không phải `.solution`
    //   bao ngoài — là đích chạm, nên nó là thứ phải đo; đo trên thẻ bao ngoài là đo nhầm ô và
    //   xanh vì lý do sai.
    //
    //   `.quay-lai` là đường về DUY NHẤT của một màn con: nút back của Zalo đóng cả Mini App chứ
    //   không quay về danh sách. Bấm trượt ở đây là mất chỗ đang đứng, và người lớn tuổi bấm
    //   trượt một nút thoát thường không tìm lại đường vào.
    expect(styles).toMatch(/\.solution--mo\s*\{[^}]*min-height:\s*var\(--tap-min\)/);
    expect(styles).toMatch(/\.quay-lai\s*\{[^}]*min-height:\s*var\(--tap-min\)/);

    // HAI NÚT THÊM VÀO 21/09/2026 (tối).
    //
    //   `.menu-nhanh__nut` là sáu ô của menu nhanh trên màn chủ — thứ đầu tiên người dùng bấm sau
    //   khi mở app, và sáu đích chạm đứng SÁT NHAU trong một lưới. Chỗ nào đích chạm sát nhau thì
    //   bấm trượt không dẫn tới "không có gì xảy ra" mà dẫn tới "mở nhầm màn".
    //
    //   `.nut-chat` nổi trên MỌI màn, nằm ngay trên thanh tab. Một nút nổi hẹp hơn ngón tay, đặt
    //   cạnh một thanh tab, là một nút mà mỗi lần bấm trượt là một lần đổi tab ngoài ý muốn.
    expect(styles).toMatch(/\.menu-nhanh__nut\s*\{[^}]*min-height:\s*var\(--tap-min\)/);
    expect(styles).toMatch(/\.nut-chat\s*\{[^}]*min-height:\s*var\(--tap-min\)/);

    // NÚT THÊM VÀO 21/09/2026 (khuya): "Xem tất cả" của bản mẫu, nằm trên hàng đầu mục.
    //
    //   Nó là đích chạm nằm SÁT MÉP PHẢI màn hình và nó ngắn — đúng hình dạng của một liên kết
    //   chữ, và đúng chỗ người ta hay dựng một liên kết chữ cao 20px. Một dòng chữ xanh bấm được
    //   là một đích chạm mà ngón cái phải ngắm, và ở mép phải thì ngắm trượt là chuyện thường.
    expect(styles).toMatch(/\.hang-tieu-de__them\s*\{[^}]*min-height:\s*var\(--tap-min\)/);
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

    // DẢI LỊCH SỬ · THẺ GIẢI PHÁP BẤM ĐƯỢC · TRANG CHI TIẾT · MÀN QUẢN LÝ QUYỀN (21/09/2026).
    // Không một màu mới nào — đúng bảng màu đã đo ở trên. Vẫn liệt kê riêng, vì cùng lý do như
    // mọi khối trên: một cặp màu chỉ được bảo vệ khi có TÊN NÓ trong danh sách này, và đổi
    // `.moc__nam` sang --brand-blue thì không dòng nào ở trên đỏ lên.
    ["the year on a history milestone", token("navy"), token("surface")],
    ["what happened in that year", token("ink"), token("surface")],
    ["the back-to-list control of a sub-screen", token("navy"), token("surface")],
    ["the arrow marking a solution card as tappable", token("navy"), token("surface")],
    ["the headline on a solution detail page", token("navy"), token("surface")],
    ["the title of a solution detail page", "#ffffff", token("navy")],
    ["the title of a solution detail page at its blue glow", "#ffffff", token("panel-glow-blue")],

    // MÀN QUẢN LÝ QUYỀN. Đây là chữ người dùng đọc để quyết định có chia sẻ dữ liệu của mình hay
    // không, và là chữ người duyệt của Zalo đọc để đối chiếu từng quyền với chỗ nó được dùng.
    ["the name of a feature that uses a permission", token("navy"), token("surface")],
    ["which screen and which half uses it", token("ink-muted"), token("surface")],
    ["what the permission is for", token("ink"), token("surface")],
    ["the sentence saying whether Zalo asks first", token("ink"), token("surface-tint")],
    ["the sentence saying what leaves the device", token("ink"), token("surface-tint")],
    ["the technical API name a reviewer matches against the console", token("ink-muted"), token("surface")],

    /**
     * ⚠ MỌI THẺ SÁNG NAY CÓ NỀN CHUYỂN SẮC — 21/09/2026, hướng thị giác của chủ dự án.
     *
     *   `--nen-the` đi từ `--surface` (trắng) xuống `--surface-tint`, `--nen-the-luc` xuống
     *   `--surface-tint-green`. Mọi cặp ở trên đo chữ trên `--surface` — tức đo ĐẦU SÁNG của dải,
     *   chỗ dễ đọc nhất. **Chữ ở đáy thẻ đứng trên đầu kia.**
     *
     *   Một dải chuyển sắc là chỗ độ tương phản chết đầu tiên, và nó chết ở phía DƯỚI thẻ — chỗ
     *   mắt đã mỏi khi đọc tới, và chỗ không ai soi khi duyệt một ảnh chụp màn hình. Nên cả ba
     *   màu chữ được đo lại trên CẢ HAI đầu kia, và ba dòng này là thứ giữ cho "nền đẹp" không
     *   bao giờ được đổi thành "nền đẹp, chữ khó đọc".
     */
    ["card text where the card gradient ends blue", token("ink"), token("surface-tint")],
    ["card secondary text where the card gradient ends blue", token("ink-muted"), token("surface-tint")],
    ["card titles where the card gradient ends blue", token("navy"), token("surface-tint")],
    /**
     * MÀN CHỦ SAU LƯỢT 21/09/2026 (tối): hai câu mới trong hero, menu nhanh, giải pháp nổi bật,
     * khối tin, nút chat nổi.
     *
     * KHÔNG MỘT MÀU MỚI NÀO — đúng bảng đã đo ở trên. Vẫn liệt kê riêng, vì cùng lý do như mọi
     * khối trước: một cặp màu chỉ được bảo vệ khi có TÊN NÓ trong danh sách này. Đổi `.tin__ngay`
     * sang `--brand-blue` cho "nhẹ mắt" thì không dòng nào ở trên đỏ lên, và ngày đăng — thứ duy
     * nhất nói cho người đọc biết tin này cũ hay mới — là chữ mất trước nhất khi ra nắng.
     */
    ["the prototype slogan in the hero", "#ffffff", token("navy")],
    ["the prototype slogan at the blue glow", "#ffffff", token("panel-glow-blue")],
    ["the prototype slogan crossing a blue node line", "#ffffff", token("panel-line-blue")],
    ["the founding-year line above the entity name", token("on-panel"), token("navy")],
    ["the founding-year line at the blue glow", token("on-panel"), token("panel-glow-blue")],

    /**
     * ⚠ HERO CHUYỂN SẮC — 21/09/2026 (khuya), theo bản mẫu của PM. ĐÂY LÀ CHỖ ĐỘ TƯƠNG PHẢN CHẾT
     * ĐẦU TIÊN CỦA CẢ ỨNG DỤNG, và bảy dòng dưới đây là thứ duy nhất đứng chắn.
     *
     *   Hero nay là một dải đi từ `--hero-sang` xuống `--navy-deep`, mang TOÀN BỘ chữ mở đầu của
     *   app: tên pháp nhân, câu slogan lớn nhất màn hình, câu sản phẩm, câu định vị, và năm chỉ
     *   số. Mọi dòng ấy đứng trên cả dải, nên chúng được đo ở ĐẦU SÁNG — `--hero-sang` — chứ
     *   không ở điểm trung bình và cũng không ở đầu tối. "Đầu tối thì đạt" là đúng phép kiểm nửa
     *   vời sẽ được nộp lên.
     *
     *   BẢN MẪU VẼ ĐẦU SÁNG LÀ `--brand-blue` (#00aef4). Trắng trên nó là 2,5:1. Nếu ai đó đổi
     *   `--hero-sang` về đúng màu bản mẫu cho "giống ảnh", hai dòng đầu dưới đây đỏ ngay — và đó
     *   chính xác là cuộc trò chuyện phải xảy ra TRƯỚC, không phải sau khi nộp.
     */
    ["the entity name at the bright end of the hero", "#ffffff", token("hero-sang")],
    ["the prototype slogan at the bright end of the hero", "#ffffff", token("hero-sang")],
    ["the product sentence at the bright end of the hero", token("on-panel"), token("hero-sang")],
    ["the founding-year line at the bright end of the hero", token("on-panel"), token("hero-sang")],
    ["a figure in the hero at its bright end", "#ffffff", token("hero-sang")],
    ["a figure label in the hero at its bright end", token("on-panel"), token("hero-sang")],
    // Huy hiệu "Vi": chữ navy trên `--surface-tint`, đúng cặp của mọi ô biểu tượng mềm khác.
    ["the two-letter badge in the hero", token("navy"), token("surface-tint")],

    ["a quick-menu label", token("navy"), token("surface")],
    ["a quick-menu label where its card gradient ends", token("navy"), token("surface-tint")],
    ["the line saying where a quick-menu item leads", token("ink-muted"), token("surface")],
    [
      "that line where the card gradient ends",
      token("ink-muted"),
      token("surface-tint"),
    ],
    ["a quick-menu label on the green pressed state", token("navy"), token("surface-tint-green")],
    // Glyph trong ô biểu tượng xen kẽ: `--navy` trên hai màu pastel, cả hai đã đo ở trên. Vẫn có
    // tên riêng ở đây, vì một cặp màu chỉ được bảo vệ khi có TÊN NÓ trong danh sách này.
    ["a quick-menu glyph on its blue pastel tile", token("navy"), token("surface-tint")],
    ["a quick-menu glyph on its green pastel tile", token("navy"), token("surface-tint-green")],
    // "Xem tất cả" trên nền trang — hai đầu của nền ấy, vì nó là một dải.
    ["the see-all control beside a section title", token("navy"), token("surface-alt")],
    ["the see-all control over the blue wash", token("navy"), token("surface-tint")],
    ["the see-all control over the green wash", token("navy"), token("surface-tint-green")],
    // Nhãn tab: ô đang xem (`--navy`) và ô không xem (`--ink-muted`), trên nền trắng thanh tab.
    ["the label of the current tab", token("navy"), token("surface")],
    ["the label of a tab that is not current", token("ink-muted"), token("surface")],

    ["a featured product name on the home screen", token("navy"), token("surface")],
    ["a featured solution line on the home screen", token("ink"), token("surface")],
    ["a featured solution line where its card gradient ends", token("ink"), token("surface-tint")],

    ["the publication date of a news item", token("ink-muted"), token("surface")],
    ["the headline of a news item", token("navy"), token("surface")],
    ["the excerpt of a news item", token("ink"), token("surface")],
    ["the excerpt where the news card gradient ends", token("ink"), token("surface-tint")],
    ["the note saying the news block is a snapshot", token("ink-muted"), token("surface-alt")],
    ["that note over the blue wash", token("ink-muted"), token("surface-tint")],

    ["the floating chat label", "#ffffff", token("navy")],
    ["the floating chat label at its blue glow", "#ffffff", token("panel-glow-blue")],
    ["the 'opens only inside Zalo' line under the chat button", token("ink"), token("surface-alt")],

    /**
     * DẢI CHIẾN DỊCH TRÊN MÀN CHỦ (22/09/2026) — hai dòng chữ trên `--surface-tint`.
     *
     * Không một màu mới nào: dải dùng lại đúng nền xanh nhạt và hai màu chữ đã đo ở trên. Vẫn có
     * tên riêng ở đây, vì cùng lý do như mọi khối trước — một cặp màu chỉ được bảo vệ khi có TÊN
     * NÓ trong danh sách này. Đây là hai dòng đầu tiên một người tới từ mã QR đọc được.
     */
    ["the campaign strip title", token("navy"), token("surface-tint")],
    ["the campaign strip greeting", token("ink"), token("surface-tint")],

    /**
     * BỘ CHỌN BA BƯỚC "GỢI Ý GIẢI PHÁP" (22/09/2026).
     *
     * Cũng không một màu mới nào — màn này dùng lại `.banner`, `.action`, `.card` và
     * `.section-title`. Liệt kê riêng vì cùng lý do: đổi `.gygp-ds__nut` sang một màu khác thì
     * không dòng nào ở trên đỏ lên, và đây là chữ người dùng đọc để CHỌN — đọc sai một dòng là
     * chọn sai một bước.
     */
    ["a wizard question", token("navy"), token("surface-alt")],
    ["a wizard question over the blue wash", token("navy"), token("surface-tint")],
    ["the step counter above a wizard question", token("ink-muted"), token("surface-alt")],
    ["a wizard option label", token("ink"), token("surface")],
    ["a wizard option label where its card gradient ends", token("ink"), token("surface-tint")],
    ["the answer summary label", token("ink-muted"), token("surface")],
    ["the answer summary value", token("ink"), token("surface")],

    ["card text where the card gradient ends green", token("ink"), token("surface-tint-green")],
    [
      "card secondary text where the card gradient ends green",
      token("ink-muted"),
      token("surface-tint-green"),
    ],
    ["card titles where the card gradient ends green", token("navy"), token("surface-tint-green")],
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
 *   Thanh tab có BỐN ô lại từ 21/09/2026 (khuya) — màn Danh thiếp bỏ tab, theo bản mẫu — và trên
 *   một máy 320px mỗi ô chỉ còn khoảng 72px chữ. Một nhãn dài hơn thế thì hoặc bị cắt mất đuôi,
 *   hoặc đẩy thanh tab vỡ hàng. Người gặp chuyện đó là người dùng một chiếc điện thoại cũ — đúng
 *   người ít có khả năng báo lại nhất.
 *
 *   PHÉP ĐO ĐỌC `TABS`, KHÔNG ĐỌC `SCREENS`: sổ màn hình có năm màn, thanh tab vẽ bốn. Đo trên
 *   `SCREENS` thì mẫu số sai về phía DỄ — nó chia bề rộng cho năm và kết luận chật hơn thực tế —
 *   và ngày ai đó trả tab thứ năm về, con số vẫn "đúng" nên không gì đỏ lên.
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
/**
 * NỀN CHUYỂN SẮC CHỈ ĐO ĐƯỢC KHI MỌI ĐIỂM DỪNG CỦA NÓ LÀ MỘT MÀU CÓ TÊN.
 *
 *   Bảng cặp màu ở trên đo `--surface`, `--surface-tint` và `--surface-tint-green`. Điều đó chỉ
 *   bảo vệ được các thẻ **chừng nào dải chuyển sắc của thẻ đi giữa đúng những màu ấy**. Đổi một
 *   điểm dừng thành `#fafcff` thì mọi cặp ở trên vẫn xanh — chúng đo token, không đo dải — và chữ
 *   ở đáy thẻ rơi xuống một màu chưa ai đo. Đó là cách một phép kiểm về độ tương phản chết trong
 *   im lặng, và nó chết đúng vào lúc ai đó đang làm cho app "hiện đại hơn".
 */
describe("nền chuyển sắc của thẻ chỉ đi qua những màu đã đo", () => {
  const CHO_PHEP = ["--surface", "--surface-tint", "--surface-tint-green"];

  /**
   * ⚠ DẢI HERO — CÙNG MỘT LUẬT, VÀ Ở ĐÂY NÓ ĐẮT HƠN HẲN.
   *
   *   `--nen-hero` là nền của khối mang chữ lớn nhất ứng dụng, và nó là nền TỐI mang chữ TRẮNG —
   *   tức là ngược chiều với các thẻ sáng ở trên: chỗ hỏng không phải đầu tối mà là ĐẦU SÁNG. Một
   *   điểm dừng viết thẳng `#00aef4` cho "giống bản mẫu" thì mọi cặp màu ở trên vẫn xanh (chúng
   *   đo token, không đo dải), và tiêu đề của app rơi xuống 2,5:1.
   */
  it("--nen-hero chỉ dừng ở những màu đã đo, và không có điểm dừng viết thẳng", () => {
    const gia_tri = token("nen-hero");
    expect(gia_tri, "--nen-hero không còn là một linear-gradient").toContain("linear-gradient");
    expect(gia_tri, "--nen-hero chứa một mã màu viết thẳng, không phải một token đã đo").not.toMatch(
      /#[0-9a-fA-F]{3,8}|\brgba?\(|\bhsla?\(/,
    );
    expect(gia_tri, "--nen-hero hoà qua transparent").not.toContain("transparent");
    const bien = [...gia_tri.matchAll(/var\(\s*(--[a-z-]+)\s*\)/g)].map((m) => m[1]!);
    expect(bien.length, "--nen-hero không tham chiếu token nào").toBeGreaterThan(1);
    for (const b of bien) {
      expect(
        ["--hero-sang", "--navy", "--navy-deep", "--panel-glow-blue"],
        `--nen-hero dừng ở ${b}, một màu không có trong bảng cặp màu của hero`,
      ).toContain(b);
    }
    // Và hero PHẢI dùng chính token ấy, không tự dựng một dải riêng trong luật `.hero`.
    const luat = /\.hero\s*\{([^}]*)\}/.exec(styles)?.[1] ?? "";
    expect(luat, ".hero không còn đọc --nen-hero").toContain("var(--nen-hero)");
    expect(luat, ".hero tự dựng một dải chuyển sắc thay vì dùng --nen-hero").not.toMatch(
      /background(-image)?:\s*(linear|radial)-gradient/,
    );
  });

  for (const ten of ["nen-the", "nen-the-luc"]) {
    it(`--${ten} không mang một màu nào ngoài bảng đã đo`, () => {
      const gia_tri = token(ten);
      expect(gia_tri, `--${ten} không còn là một linear-gradient`).toContain("linear-gradient");
      expect(gia_tri, `--${ten} chứa một mã màu viết thẳng, không phải một token đã đo`).not.toMatch(
        /#[0-9a-fA-F]{3,8}|\brgba?\(|\bhsla?\(/,
      );
      const bien = [...gia_tri.matchAll(/var\(\s*(--[a-z-]+)\s*\)/g)].map((m) => m[1]!);
      expect(bien.length, `--${ten} không tham chiếu token nào`).toBeGreaterThan(1);
      for (const b of bien) {
        expect(CHO_PHEP, `--${ten} dừng ở ${b}, một màu không có trong bảng cặp màu`).toContain(b);
      }
      // `transparent` là cách một dải đi qua những màu không ai đo được: nó hoà với bất cứ thứ gì
      // nằm dưới, và thứ nằm dưới là nền trang — vốn cũng là một dải.
      expect(gia_tri, `--${ten} hoà qua transparent`).not.toContain("transparent");
    });
  }

  it("các thẻ dùng token nền ấy, không tự dựng một dải riêng", () => {
    // Một `.moc { background: linear-gradient(...#fff...) }` viết thẳng đi vòng qua cả ba ca trên.
    // Ca này buộc mọi bề mặt thẻ đi qua đúng một cái tên.
    // BA LỚP THÊM VÀO 21/09/2026 (tối) — ô menu nhanh, thẻ giải pháp nổi bật, thẻ tin. Cả ba là
    // bề mặt thẻ SÁNG có chữ đứng trên, nên cả ba phải đi qua cùng một cái tên như sáu lớp kia.
    for (const lop of [
      ".card",
      ".solution",
      ".unit",
      ".cert",
      ".office",
      ".action",
      ".moc",
      ".quyen",
      ".menu-nhanh__nut",
      ".gp-noi-bat",
      ".tin",
      // Thẻ trắng đè lên đáy hero (21/09/2026, khuya) — bề mặt sáng có sáu ô chữ đứng trên.
      ".menu-the",
    ]) {
      const luat = new RegExp(`\\${lop}\\s*\\{([^}]*)\\}`).exec(styles)?.[1] ?? "";
      expect(luat, `${lop} không còn khai nền`).toMatch(/background:/);
      expect(luat, `${lop} tự dựng một dải chuyển sắc thay vì dùng --nen-the`).not.toMatch(
        /background:\s*linear-gradient/,
      );
    }
  });
});

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
    const cho_chu = BE_RONG_MAY / TABS.length - demNgang();
    expect(TABS.length, "thanh tab rỗng — phép đo này sẽ xanh vì lý do sai").toBeGreaterThan(0);

    for (const man of TABS) {
      for (const tu of man.cho.nhan.split(/\s+/)) {
        const rong = tu.length * co_chu * RONG_MOI_EM;
        expect(
          rong,
          `nhãn tab "${man.cho.nhan}": từ "${tu}" cần ~${rong.toFixed(0)}px, tab chỉ còn ` +
            `${cho_chu.toFixed(0)}px trên máy ${BE_RONG_MAY}px. Rút ngắn nhãn, đừng thu nhỏ chữ.`,
        ).toBeLessThanOrEqual(cho_chu);
      }
    }
  });

  /**
   * HÌNH ĐỨNG TRÊN CHỮ, KHÔNG ĐỨNG CẠNH CHỮ — và đó là điều kiện để phép đo ngay trên còn đúng.
   *
   *   Xếp hình cạnh chữ thì bề rộng chữ của mỗi ô mất đi cả hình lẫn khe: ở 320px còn khoảng
   *   72 − 22 − 2 = 48px, và "Trang" (~50px) bị cắt đuôi. Phép đo ở trên KHÔNG nhìn thấy điều đó,
   *   vì nó chia bề rộng ô mà không biết trong ô còn gì khác. Nên hướng xếp thành một phép kiểm
   *   riêng, thay vì một giả định nằm im trong phép kiểm kia.
   */
  it("mỗi ô tab xếp DỌC — hình trên chữ, nên cả bề rộng ô là bề rộng chữ", () => {
    const luat = /\.tabbar__item\s*\{([^}]*)\}/.exec(styles)?.[1] ?? "";
    expect(luat, ".tabbar__item không còn xếp dọc — hình đang chiếm mất bề rộng của nhãn").toMatch(
      /flex-direction\s*:\s*column/,
    );
  });

  it("không ai đặt `white-space: nowrap` lên tab — đó là thứ biến xuống dòng thành tràn", () => {
    const luat = /\.tabbar__item\s*\{([^}]*)\}/.exec(styles)?.[1] ?? "";
    expect(luat).not.toMatch(/white-space\s*:\s*nowrap/);
  });
});

/**
 * MÀN CHỦ SAU BẢN MẪU CỦA PM — HERO TRÀN NGANG, THẺ MENU ĐÈ LÊN NÓ, CHỈ SỐ NẰM TRONG HERO.
 *
 * ⚠ BA PHÉP ĐO DƯỚI ĐÂY LÀ BA CHỖ MỘT CHỒNG LỚP HỎNG MÀ KHÔNG GÌ ĐỎ LÊN.
 *
 *   Một khối tràn ngang dựng bằng LỀ ÂM, một thẻ đè lên nó dựng bằng LỀ ÂM khác, và một lưới bốn
 *   cột trong khối ấy: cả ba đều hiện đúng trên máy của người viết chúng, và hỏng ở 320px. Hỏng
 *   theo hai kiểu — hoặc đẻ ra một thanh cuộn NGANG (thứ trên điện thoại không ai tìm ra nguyên
 *   nhân), hoặc thẻ trắng trùm mất hàng chỉ số cuối cùng của hero.
 *
 *   Ba con số ấy nằm ở ba chỗ cách xa nhau trong styles.css. Chú thích thì không đếm được gì.
 */
describe("chồng lớp của màn chủ, đo ở 320px", () => {
  const BE_RONG_MAY = 320;
  const RONG_MOI_EM = 0.62;

  /** Giá trị `px` thứ `chi_so` (0-based) của một thuộc tính trong một luật CSS. */
  function px(lop: string, thuoc_tinh: string, chi_so: number): number {
    const luat = new RegExp(`\\${lop}\\s*\\{([^}]*)\\}`).exec(styles);
    expect(luat, `stylesheet không còn khai ${lop}`).not.toBeNull();
    const gia_tri = new RegExp(`(?:^|;)\\s*${thuoc_tinh}:\\s*([^;]+);`).exec(luat![1]!)?.[1]?.trim();
    expect(gia_tri, `${lop} không còn khai ${thuoc_tinh}`).toBeDefined();
    const phan = gia_tri!.split(/\s+/);
    const mot = phan[chi_so];
    expect(mot, `${lop} { ${thuoc_tinh} } không có giá trị thứ ${chi_so}`).toBeDefined();
    return Number.parseFloat(mot!);
  }

  it("hero tràn đúng bằng đệm trang — không hụt một vệt, không đẻ ra thanh cuộn ngang", () => {
    // `.app-main { padding: 16px 16px calc(...) }` — giá trị thứ hai là đệm NGANG.
    const dem_trang = px(".app-main", "padding", 1);
    // `.hero { margin: 0 -16px }` — giá trị thứ hai là lề ngang, âm.
    const le_hero = px(".hero", "margin", 1);
    expect(
      -le_hero,
      `hero tràn ${-le_hero}px mỗi bên nhưng trang đệm ${dem_trang}px: lớn hơn thì trang có thanh ` +
        `cuộn ngang, nhỏ hơn thì còn một vệt nền lộ ra hai bên hero.`,
    ).toBe(dem_trang);
  });

  it("đệm đáy hero lớn hơn phần thẻ menu trèo lên — không trùm mất hàng chỉ số", () => {
    // `.hero { padding: 22px 20px 76px }` — giá trị thứ ba là đệm đáy.
    const dem_day = px(".hero", "padding", 2);
    // `.menu-the { margin: -56px 0 18px }` — giá trị đầu là phần trèo lên, âm.
    const treo_len = -px(".menu-the", "margin", 0);
    expect(treo_len, "thẻ menu không còn đè lên hero — bản mẫu vẽ nó đè lên").toBeGreaterThan(0);
    expect(
      treo_len,
      `thẻ menu trèo lên ${treo_len}px trong khi hero chỉ chừa ${dem_day}px ở đáy: hàng chỉ số ` +
        `cuối cùng của hero nằm dưới thẻ trắng.`,
    ).toBeLessThan(dem_day);
  });

  /** Số cột của một lưới, ĐỌC TỪ CSS. Viết cứng "4" ở đây là để phép đo xanh khi lưới đổi cột. */
  function soCot(lop: string): number {
    const luat = new RegExp(`\\${lop}\\s*\\{([^}]*)\\}`).exec(styles);
    expect(luat, `stylesheet không còn khai ${lop}`).not.toBeNull();
    const co = /grid-template-columns:\s*repeat\(\s*(\d+)\s*,/.exec(luat![1]!)?.[1];
    expect(co, `${lop} không còn là một lưới repeat(N, …)`).toBeDefined();
    return Number.parseInt(co!, 10);
  }

  it("bốn cột chỉ số và ba cột menu đều đủ chỗ cho từ dài nhất ở 320px", () => {
    const co_chu = pixels("text-small");

    // CHỈ SỐ: hero rộng cả màn (nhờ lề âm), trừ hai đệm ngang của chính nó.
    const dem_hero = px(".hero", "padding", 1);
    const cot_chi_so = (BE_RONG_MAY - 2 * dem_hero) / soCot(".stat-grid");
    for (const nhan of ["Khách hàng", "Đối tác", "Nhân sự", "Quốc gia kết nối"]) {
      for (const tu of nhan.split(/\s+/)) {
        const rong = tu.length * co_chu * RONG_MOI_EM;
        expect(
          rong,
          `nhãn chỉ số "${nhan}": từ "${tu}" cần ~${rong.toFixed(0)}px, mỗi cột chỉ còn ` +
            `${cot_chi_so.toFixed(0)}px ở ${BE_RONG_MAY}px.`,
        ).toBeLessThanOrEqual(cot_chi_so);
      }
    }

    // MENU NHANH: bề rộng màn − đệm trang hai bên − đệm thẻ hai bên − hai khe, chia ba.
    const dem_trang = px(".app-main", "padding", 1);
    const dem_the = px(".menu-the", "padding", 0);
    const khe = px(".menu-nhanh", "gap", 0);
    const dem_o = px(".menu-nhanh__nut", "padding", 1);
    const cot = soCot(".menu-nhanh");
    const cot_menu = (BE_RONG_MAY - 2 * dem_trang - 2 * dem_the - (cot - 1) * khe) / cot - 2 * dem_o;
    expect(MUC_MENU_NHANH.length, "menu rỗng — phép đo này sẽ xanh vì lý do sai").toBeGreaterThan(0);
    for (const muc of MUC_MENU_NHANH) {
      for (const tu of `${muc.nhan} ${muc.phu}`.split(/\s+/)) {
        const rong = tu.length * co_chu * RONG_MOI_EM;
        expect(
          rong,
          `ô menu "${muc.nhan}": từ "${tu}" cần ~${rong.toFixed(0)}px, mỗi ô chỉ còn ` +
            `${cot_menu.toFixed(0)}px ở ${BE_RONG_MAY}px. Rút ngắn nhãn, đừng thu nhỏ chữ.`,
        ).toBeLessThanOrEqual(cot_menu);
      }
    }
  });
});
