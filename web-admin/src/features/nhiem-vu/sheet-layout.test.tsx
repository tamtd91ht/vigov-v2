// Customer bug sheet 10/10/2026 — the layout fixes no behaviour test can see: a select's right padding
// against its chevron, the filter row's fixed widths, the move menu's size, the reserved scrollbar and
// the page lock under a modal. Read from the class strings and `globals.css` they live in.

import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { SELECT_CLASS as BUDGET_SELECT_CLASS } from "@/features/giai-ngan/spec-classes";

import { HangLoc, type DanhMucNhiemVu } from "./so-nhiem-vu"; // vi-name-ok: existing names, imported not declared
import { FILTER_SELECT_CLASS, FORM_SELECT_CLASS } from "./task-spec";

const CSS = readFileSync(fileURLToPath(new URL("../../app/globals.css", import.meta.url)), "utf8");
const CATALOGUES: DanhMucNhiemVu = { loai: [], mucUuTien: [], khoi: [], boPhan: [] };

describe("row 2 — a long chosen value ends in '…' and never runs under the chevron", () => {
  it("the selects keep the global `2.25rem` right padding (`pr-9`) — no `px-*` overriding it", () => {
    for (const cls of [FILTER_SELECT_CLASS, FORM_SELECT_CLASS, BUDGET_SELECT_CLASS]) {
      expect(cls).toContain("pr-9");
      expect(cls).not.toMatch(/(^|\s)px-/);
    }
    // The global frame still ellipsises and draws the chevron in that padding.
    expect(CSS).toMatch(/:where\(select:not\(\[multiple\], \[size\]\)\) \{[^}]*padding: 0 2\.25rem 0 0\.75rem;[^}]*text-overflow: ellipsis;/s);
  });
});

describe("row 11 — the filter row does not jump when a card is ticked", () => {
  it("`Mọi loại nhiệm vụ` · `Mọi khối` · `Mọi nguồn giao` have fixed narrow widths", () => {
    const html = renderToStaticMarkup(
      <HangLoc loc={{}} tim="" datTim={() => {}} datLoc={() => {}} danhMuc={CATALOGUES} danhBa={null} />,
    );
    const cls = (id: string) => new RegExp(`<select id="${id}"[^>]*class="([^"]*)"`).exec(html)?.[1] ?? "";
    expect(cls("loc-loai")).toContain("w-40");
    expect(cls("loc-khoi")).toContain("w-28");
    expect(cls("loc-nguon-giao")).toContain("w-36");
    for (const id of ["loc-loai", "loc-khoi", "loc-nguon-giao"]) expect(cls(id)).toContain("min-w-0");
    // The first option names the filter, as the prototype (sheet row 9).
    expect(html).toMatch(/<select id="loc-loai"[^>]*><option value=""[^>]*>Mọi loại nhiệm vụ<\/option>/);
  });
});

describe("rows 7, 13, 16 — globals.css", () => {
  it("row 7: the Nhiệm vụ page reserves the scrollbar's gutter", () => {
    expect(CSS).toMatch(/html:has\(\.man-nhiem-vu\),\s*html:has\(dialog:modal\) \{\s*scrollbar-gutter: stable;/);
  });

  it("row 16: the page behind an open modal dialog does not scroll", () => {
    expect(CSS).toMatch(/html:has\(dialog:modal\) \{\s*overflow: hidden;/);
  });

  it("row 13: the card's move menu uses the app's menu size (36px rows, 14px text)", () => {
    const rule = /\.menu-noi \.menu-chuyen-cot button \{([^}]*)\}/.exec(CSS)?.[1] ?? "";
    expect(rule).toContain("min-height: 2.25rem;");
    expect(rule).toContain("font-size: 0.875rem;");
  });
});
