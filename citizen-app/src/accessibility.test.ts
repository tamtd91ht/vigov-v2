/// <reference types="vite/client" />
import { describe, expect, it } from "vitest";

import indexHtml from "../index.html?raw";

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
