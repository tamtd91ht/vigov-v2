import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { NationalEmblem, RootTabHeader } from "./khung-xa";

/**
 * The root tabs' left slot (customer sheet 12/10/2026, row 5: "đổi icon ở góc trái thành icon ngôi sao giống
 * prototype"): the prototype's `NationalEmblem` (`AppHeader.tsx:98-142`) drawn inline, replacing the commune logo
 * ADR 0069 put there. Server-rendered markup, as in `commune-screens.test.tsx` — no DOM here.
 */

const html = (el: Parameters<typeof renderToStaticMarkup>[0]) => renderToStaticMarkup(el);

describe("the star emblem", () => {
  const svg = html(createElement(NationalEmblem));

  it("is the prototype's drawing: 32px, red disc, gold ring, twelve strokes, the star, the base", () => {
    expect(svg).toContain('<svg width="32" height="32" viewBox="0 0 32 32"');
    expect(svg).toContain('<circle cx="16" cy="16" r="15" fill="#b8000c"></circle>');
    expect(svg).toContain('<circle cx="16" cy="16" r="12.5" fill="none" stroke="#f5c518" stroke-width="1.6"></circle>');
    expect(svg.match(/<line /g)).toHaveLength(12);
    expect(svg).toContain('d="m16 8.4 2.2 4.6 5 .7-3.6 3.6.9 5-4.5-2.4-4.5 2.4.9-5-3.6-3.6 5-.7L16 8.4Z" fill="#f5c518"');
    expect(svg).toContain('<rect x="12" y="23" width="8" height="1.8" rx="0.9" fill="#f5c518"></rect>');
  });

  it("is decoration, not a tap target: aria-hidden, no button, no picture to load", () => {
    expect(svg).toMatch(/^<span class="xa-emblem" aria-hidden="true"><svg/);
    expect(svg).not.toContain("<button");
    expect(svg).not.toContain("<img");
  });

  it("renders the same markup twice (coordinates rounded once, so server markup is stable)", () => {
    expect(html(createElement(NationalEmblem))).toBe(svg);
  });
});

describe("every root-tab header carries it", () => {
  it("left slot = the emblem; the commune's name stays in words beside it", () => {
    const out = html(createElement(RootTabHeader, { title: "Xã Thử" }));
    expect(out).toContain('<div class="xa-header__side"><span class="xa-emblem" aria-hidden="true">');
    expect(out).toContain(">Xã Thử</h1>");
  });

  it("the commune half no longer reads the profile for the header (its only reader left with the logo)", () => {
    const src = Object.values(import.meta.glob("./TrangXa.tsx", { query: "?raw", import: "default", eager: true }))[0] as string;
    expect(src).not.toMatch(/communeProfiles\(|useCommuneProfile|logoUrl/);
    expect(src.match(/<DauTab tieu_de=\{[^}]+\} \/>/g)).toHaveLength(3);
  });
});
