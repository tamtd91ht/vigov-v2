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
  });

  it("keeps pinch-zoom available", () => {
    // Disabling zoom is the single most common way a mobile page becomes unusable, and it is
    // usually added to protect a layout, by someone who can read the page fine.
    expect(shell).not.toMatch(/user-scalable\s*=\s*no/);
    expect(shell).not.toMatch(/maximum-scale/);
  });
});

describe("every colour pair the app actually renders clears 4.5:1", () => {
  const pairs: ReadonlyArray<[string, string, string]> = [
    ["body text on the page", token("ink"), token("surface-alt")],
    ["body text on a card", token("ink"), token("surface")],
    ["secondary text on the page", token("ink-muted"), token("surface-alt")],
    ["secondary text on a card", token("ink-muted"), token("surface")],
    ["header and primary action text", "#ffffff", token("navy")],
    ["card titles and figures", token("navy"), token("surface")],
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
