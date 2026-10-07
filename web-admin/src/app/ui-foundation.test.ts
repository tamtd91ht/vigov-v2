import { readdirSync, readFileSync } from "node:fs";
import { extname, join, relative } from "node:path";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

/**
 * Guards for the 02/10/2026 UI foundation. Each one is a decision that, undone, breaks nothing
 * visible on the developer's machine and turns no other test red:
 *
 *   - the old CSS must stay INSIDE `@layer legacy`, or every utility class silently loses to it;
 *   - Tailwind's preflight must stay OUT until every screen has moved, or it restyles them all;
 *   - the font must stay self-hosted (no `next/font/google`, no font CDN);
 *   - dark mode is defined but NOT enabled (no `prefers-color-scheme` switch);
 *   - the page grid keeps `minmax(0, 1fr)`, the fix for the page-wide horizontal scroll;
 *   - "Sắp có" never appears: an authority's menu does not promise dates (owner, 02/10/2026);
 *   - new UI code uses none of the patterns rule 13 bans.
 */
const SRC = fileURLToPath(new URL("..", import.meta.url));
const CSS = readFileSync(join(SRC, "app", "globals.css"), "utf8");

/**
 * Comments removed before scanning — the files EXPLAIN the banned strings ("never 'Sắp có'", "no
 * injected `<style>`"), and a scan that reads its own warnings goes red, then gets switched off.
 * Same trade-off as `ranh-gioi-nguon.test.ts`: only block comments and whole `//` lines are cut.
 */
function stripComments(text: string): string {
  return text
    .replace(/\/\*[\s\S]*?\*\//g, "")
    .split("\n")
    .filter((line) => !/^\s*\/\//.test(line))
    .join("\n");
}

function sourceFiles(dir: string): { path: string; text: string }[] {
  const out: { path: string; text: string }[] = [];
  for (const e of readdirSync(dir, { withFileTypes: true })) {
    const p = join(dir, e.name);
    if (e.isDirectory()) out.push(...sourceFiles(p));
    else if ([".ts", ".tsx", ".css"].includes(extname(e.name)) && !/\.test\.tsx?$/.test(e.name))
      out.push({ path: relative(SRC, p).replace(/\\/g, "/"), text: stripComments(readFileSync(p, "utf8")) });
  }
  return out;
}

const ALL = sourceFiles(SRC);

/** The files this foundation added — the ban below is checked on exactly these. */
const NEW_UI = ALL.filter(
  (f) =>
    f.path.startsWith("components/ui/") ||
    [
      "components/side-nav.tsx",
      "components/sidebar-state.ts",
      "components/nav-sheet.tsx",
      "components/user-menu.tsx",
      "components/dau-trang.tsx",
      "components/menu-icons.ts",
      "components/user-initials.ts",
      "components/role-pill.tsx",
      "lib/cn.ts",
    ].includes(f.path),
);

/** Innermost `selector { body }` pairs of the comment-stripped CSS (nested @media included). */
const RULES = [...stripComments(CSS).matchAll(/([^{}]+)\{([^{}]*)\}/g)].map((m) => ({
  selector: m[1]!.trim(),
  body: m[2]!,
}));

describe("globals.css layering", () => {
  it("declares the layer order with legacy BELOW the utilities", () => {
    expect(CSS).toContain("@layer theme, base, legacy, components, utilities;");
  });

  it("imports Tailwind theme and utilities, and NOT preflight", () => {
    expect(CSS).toContain('@import "tailwindcss/theme.css" layer(theme);');
    expect(CSS).toContain('@import "tailwindcss/utilities.css" layer(utilities);');
    expect(CSS).not.toMatch(/@import\s+["']tailwindcss(\/preflight(\.css)?)?["']/);
  });

  it("keeps every legacy rule inside one `@layer legacy { … }` block", () => {
    const open = CSS.indexOf("@layer legacy {");
    const close = CSS.lastIndexOf("} /* end @layer legacy */");
    expect(open).toBeGreaterThan(-1);
    expect(close).toBeGreaterThan(open);
    // Nothing but whitespace after the legacy block: a rule appended below it would be outside.
    expect(CSS.slice(close + "} /* end @layer legacy */".length).trim()).toBe("");
    for (const cls of [".nut-chinh {", ".nut-phu {", ".bang-cuon {", ".khung-trang {", ".side-nav {"]) {
      const at = CSS.indexOf("\n" + cls);
      expect(at, cls).toBeGreaterThan(open);
      expect(at, cls).toBeLessThan(close);
    }
  });

  // Presentation pin (owner 05/10/2026: the left sidebar is back). One column by default, a sidebar
  // column beside the content from 768px. The fix it guards is unchanged — the content column may shrink.
  it("the page grid lets its content column shrink (no page-wide horizontal scroll)", () => {
    expect(CSS).toMatch(/\.khung-trang \{[^}]*grid-template-columns: minmax\(0, 1fr\);/);
    expect(CSS).not.toMatch(/grid-template-columns: auto 1fr;/);
    expect(RULES.find((r) => r.selector === ".khung-trang")?.body).not.toMatch(/grid-template-columns: 1fr;/);
    expect(RULES.find((r) => r.selector === ".khung-trang:has(> .side-nav)")?.body).toContain(
      "grid-template-columns: auto minmax(0, 1fr);",
    );
  });

  it("the primary button is no longer full width by default", () => {
    const rule = /\n\.nut-chinh \{([^}]*)\}/.exec(CSS)?.[1] ?? "";
    expect(rule).not.toBe("");
    expect(rule).not.toContain("width: 100%");
  });

  it("aliases the old variables to the spec tokens instead of keeping a second palette", () => {
    for (const [oldVar, token] of [
      ["--nen", "--bg"],
      ["--xanh", "--brand-600"],
      ["--xam-chu", "--ink-500"],
      ["--xam-vien", "--line-strong"],
      ["--do-loi", "--red-600"],
      ["--navy", "--ink-900"],
      ["--navy-nhat", "--ink-700"],
    ]) {
      expect(CSS, oldVar).toContain(`${oldVar}: var(${token});`);
    }
  });

  it("defines dark tokens but never switches them on by itself", () => {
    expect(CSS).toContain(':root[data-theme="dark"]');
    expect(CSS).not.toMatch(/prefers-color-scheme\s*:\s*dark/);
    const setters = ALL.filter((f) => !f.path.endsWith(".css") && /data-theme|dataset\.theme/.test(f.text));
    expect(setters.map((f) => f.path)).toEqual([]);
  });

  it("honours reduced motion", () => {
    expect(CSS).toMatch(/@media \(prefers-reduced-motion: reduce\)/);
  });
});

/**
 * Spec v2 (ADR 0068 §11): "modern = less friction, not decoration". Each guard below is a look that
 * nothing else would catch coming back — a blur or a gradient tile is invisible to every other test.
 */
describe("spec v2 shell", () => {
  // Presentation pins (ADR 0068 §5). Rows: spec 00 §5 (ADR 0068 lần 6) — a `p-2` cell floored at the
  // `h-10` header, replacing the 64px OMICALL row of 05/10/2026. Sidebar 240 / 64 (`w-60` / `w-16`) and
  // header 64 (`h-16`): spec 01. Motion unchanged.
  it("declares the tokens: 240/64px sidebar, 64px topbar, 40px rows, 4/8px spacing, motion", () => {
    for (const decl of [
      "--sidebar-w: 240px;",
      "--sidebar-w-collapsed: 64px;",
      "--topbar-h: 64px;",
      "--row-h: 40px;",
      "--space-1: 4px;",
      "--space-6: 32px;",
      "--dur-fast: 250ms;",
      "--dur: 250ms;",
      "--dur-slow: 250ms;",
      "--ease: cubic-bezier(0.4, 0, 0.2, 1);",
    ])
      expect(CSS, decl).toContain(decl);
  });

  it("zeroes the motion tokens under reduced motion", () => {
    const block = /@media \(prefers-reduced-motion: reduce\) \{([\s\S]*?)\n {2}\}/.exec(CSS)?.[1] ?? "";
    expect(block).toContain("--dur-fast: 0.01ms;");
    expect(block).toContain("--dur-slow: 0.01ms;");
  });

  // ADR 0068 lần 6 #3 lifted §11's blur ban for exactly two places, both spec text: the modal's own
  // overlay (spec 00 §5, `bg-black/10 backdrop-blur-xs`) and the white header (spec 01, `bg-white/95
  // backdrop-blur`). Everywhere else the ban still holds, so a blur that turns up in another component is
  // a decision nobody took.
  it("no frosted glass except the modal overlay and the header: no backdrop-filter in CSS, one blur class each", () => {
    expect(stripComments(CSS)).not.toMatch(/backdrop-filter|blur\(/);
    for (const f of NEW_UI) {
      const blurs = f.text.match(/[\w:-]*backdrop-blur[\w-]*|backdrop-filter/g) ?? [];
      if (f.path === "components/ui/modal-dialog.tsx") expect(blurs, f.path).toEqual(["backdrop:backdrop-blur-xs"]);
      else if (f.path === "components/dau-trang.tsx") expect(blurs, f.path).toEqual(["backdrop-blur"]);
      else expect(blurs, f.path).toEqual([]);
    }
  });

  it("no gradient in the shared components (the PageHeader tile is flat)", () => {
    for (const f of NEW_UI) expect(f.text, f.path).not.toMatch(/bg-linear|bg-gradient|from-brand-|linear-gradient/);
  });

  // Presentation pins (spec 01; ADR 0068 §Sửa đổi 07/10/2026 lần 6 #2): edge to edge on the page colour —
  // no grey frame, margin or radius any more — and still no `overflow` on the shell, or the sticky header
  // and sidebar would stick to a box that never scrolls.
  it("app shell: edge to edge, `--bg`, full height, no frame, nothing that breaks sticky", () => {
    const shell = RULES.find((r) => r.selector === ".khung-trang")?.body ?? "";
    for (const decl of ["background: var(--bg);", "min-height: 100dvh;"]) expect(shell, decl).toContain(decl);
    for (const banned of [/margin/, /border-radius/, /overflow/]) expect(shell).not.toMatch(banned);
    expect(CSS).not.toMatch(/:has\(\.khung-trang\)/);
  });

  // The header's look is spec 01's utilities on the element (`dau-trang.test.tsx` pins them); no legacy
  // rule may draw it again — a `.dau-trang { background }` outside them would win nothing (layer order)
  // but would put the navy header's values back in front of the next reader.
  it("header: no legacy rule draws it; the menu button matches the bell (40px, hairline, page fill)", () => {
    expect(RULES.find((r) => r.selector === ".dau-trang")).toBeUndefined();
    expect(CSS).not.toMatch(/--shadow-header/);
    for (const sel of [".header-icon-button", ".nut-chuong"]) {
      const body = RULES.find((r) => r.selector === sel)?.body ?? "";
      for (const decl of ["width: 40px;", "height: 40px;", "border: 1px solid var(--line);", "background: var(--bg);"]) expect(body, `${sel} ${decl}`).toContain(decl);
    }
  });

  it("the old white text sidebar stays gone, and no shell rule draws a gradient", () => {
    expect(CSS).not.toMatch(/\.thanh-ben/);
    expect(CSS).not.toMatch(/\.header-modules/);
    expect(CSS).not.toMatch(/\.commune-banner/);
    for (const r of RULES.filter((x) => /dau-trang|header-|nav-sheet|side-nav|khung-trang|user-menu/.test(x.selector)))
      expect(r.body, r.selector).not.toMatch(/gradient/);
  });

  // Presentation pins (spec 01): hidden below 768px (the nav sheet takes over); from 768px navy, the
  // full height of the window, sticky at the top, beside the header (first column, every row), 240/64px.
  // Z 30: under the economic map's full-screen view and Tổng quan's presentation overlay (both z-40).
  it("left sidebar: hidden by default; from 768px navy, full height, sticky, first column over every row, z under 40", () => {
    expect(RULES.find((r) => r.selector === ".side-nav" && r.body.includes("display: none;"))).toBeDefined();
    const wide = RULES.find((r) => r.selector === ".side-nav" && r.body.includes("display: flex;"))?.body ?? "";
    for (const decl of [
      "width: var(--sidebar-w);",
      "background: var(--sidebar);",
      "position: sticky;",
      "top: 0;",
      "height: 100dvh;",
      "grid-column: 1;",
      "grid-row: 1 / -1;",
      "z-index: 30;",
    ])
      expect(wide, decl).toContain(decl);
    expect(RULES.find((r) => r.selector === ".side-nav.is-collapsed")?.body).toContain("width: var(--sidebar-w-collapsed);");
    expect(RULES.find((r) => r.selector === ".khung-trang:has(> .side-nav) > .dau-trang")?.body).toContain("grid-column: 2;");
    expect(RULES.find((r) => r.selector === ".khung-trang:has(> .side-nav) > .than-trang")?.body).toContain("grid-column: 2;");
    const active = RULES.find((r) => r.selector === ".side-nav .side-nav-link.is-active::before")?.body ?? "";
    for (const decl of ["left: -12px;", "width: 3px;", "background: var(--sidebar-primary);"]) expect(active, decl).toContain(decl);
  });

  it("page width classes exist: data none, form 880px, detail 1120px", () => {
    expect(CSS).toMatch(/\.page--data \{[^}]*max-width: none;/);
    expect(CSS).toMatch(/\.page--form \{[^}]*max-width: 880px;/);
    expect(CSS).toMatch(/\.page--detail \{[^}]*max-width: 1120px;/);
  });
});

/**
 * The global select frame (owner, 02/10/2026: "quá hẹp, view xấu, lạc hậu"). It must stay at ZERO
 * specificity so contextual rules (44px touch targets, filter rows) keep winning, and nothing may
 * erase its chevron: an `appearance: none` select whose chevron image was wiped by a `background:`
 * shorthand has no arrow at all and no longer looks like something to open.
 */
describe("global select frame", () => {
  const frame = RULES.find((r) => r.selector === ":where(select:not([multiple], [size]))" && r.body.includes("appearance"));

  it("exists, at zero specificity, inside the legacy layer", () => {
    expect(frame).toBeDefined();
    const at = CSS.indexOf(":where(select:not([multiple], [size])) {");
    expect(at).toBeGreaterThan(CSS.indexOf("@layer legacy {"));
    expect(at).toBeLessThan(CSS.lastIndexOf("} /* end @layer legacy */"));
  });

  it("is the controlClass look: control height, line-strong hairline, control radius, chevron, no native arrow", () => {
    const b = frame?.body ?? "";
    for (const decl of [
      "height: var(--control-h);",
      "border: 1px solid var(--line-strong);",
      "border-radius: var(--r-control);",
      "appearance: none;",
      "background-image: url(\"data:image/svg+xml,",
    ])
      expect(b, decl).toContain(decl);
  });

  it("no rule that targets a select uses the `background` shorthand", () => {
    const offenders = RULES.filter((r) => /\bselect\b/.test(r.selector) && /(^|[\s;])background\s*:/.test(r.body));
    expect(offenders.map((r) => r.selector)).toEqual([]);
  });

  it("Field, which draws its own chevron icon, switches the frame's image off", () => {
    const field = readFileSync(join(SRC, "components", "ui", "field.tsx"), "utf8");
    expect(field).toContain('kind === "select" && "[&_select]:bg-none"');
  });

  it("puts the label ABOVE every label-beside-select shape, never on tick-box rows", () => {
    expect(CSS).toMatch(/\.o-chon:has\(> select\) \{[^}]*flex-direction: column;/);
    expect(CSS).toMatch(/:where\(p, div\):where\(:not\(\.o-nhap, \.o-chon, \.chon-nam, \.chon-hang-muc\)\):where\(:has\(> label \+ select\)\) \{[^}]*flex-direction: column;/);
  });
});

describe("font", () => {
  it("is self-hosted: no next/font/google, no font CDN anywhere in src", () => {
    for (const f of ALL) {
      expect(f.text, f.path).not.toContain("next/font/google");
      expect(f.text, f.path).not.toMatch(/fonts\.googleapis|fonts\.gstatic|cdn\.jsdelivr/);
    }
    const layout = readFileSync(join(SRC, "app", "layout.tsx"), "utf8");
    // Presentation pin (ADR 0068 §5): spec 00 §1 (ADR 0068 lần 6) — Inter, the five weights the spec's
    // classes use, self-hosted through @fontsource (lần 6 #6), replacing Roboto 400/500/600.
    for (const w of [400, 500, 600, 700, 800]) expect(layout).toContain(`import "@fontsource/inter/${w}.css";`);
    expect(layout).not.toContain("@fontsource/roboto");
  });
});

describe("wording", () => {
  it("never shows 'Sắp có' anywhere in the app", () => {
    for (const f of ALL) expect(f.text, f.path).not.toContain("Sắp có");
  });
});

describe("new UI code", () => {
  it("is actually scanned", () => {
    expect(NEW_UI.length).toBeGreaterThanOrEqual(14);
  });

  it("uses none of the patterns rule 13 and the source boundary ban", () => {
    const banned: [string, RegExp][] = [
      ["Math.random", /Math\s*\.\s*random/],
      ["eval", /\beval\s*\(/],
      ["new Function", /new\s+Function\s*\(/],
      ["dangerouslySetInnerHTML", /dangerouslySetInnerHTML/],
      ["innerHTML =", /\.\s*innerHTML\s*=/],
      ["document.cookie", /document\s*\.\s*cookie/],
      ["inline <style>", /<style[\s>]/],
      ["NEXT_PUBLIC_", new RegExp(["NEXT", "PUBLIC"].join("_") + "_")],
    ];
    for (const f of NEW_UI) for (const [name, re] of banned) expect(re.test(f.text), `${name} in ${f.path}`).toBe(false);
  });
});

/** The body of the FIRST unlayered `<selector> {` rule. */
function ruleBody(selector: string): string {
  const start = CSS.indexOf(`\n${selector} {`);
  expect(start, selector).toBeGreaterThan(-1);
  return CSS.slice(start, CSS.indexOf("}", start));
}

// Tester screenshot 06/10/2026: Kanban cards spilled over the next column at ≥1280px. An unlayered
// fixed-width column rule beats Tailwind's layered `xl:grid-cols-5`, so it must stay bounded below
// the 1280px grid breakpoint.
describe("kanban board does not overlap at the grid breakpoint", () => {
  it("the fixed-width (232px) column rule is limited to 768–1279px", () => {
    const i = CSS.indexOf("grid-auto-columns: 232px;");
    expect(i).toBeGreaterThan(-1);
    const media = CSS.lastIndexOf("@media", i);
    expect(CSS.slice(media, i)).toMatch(/@media \(min-width: 768px\) and \(max-width: 1279\.98px\)/);
  });

  it("the card list lets a card shrink below its one-line holder text", () => {
    const list = CSS.slice(CSS.indexOf(".danh-sach-the {"), CSS.indexOf("}", CSS.indexOf(".danh-sach-the {")));
    expect(list).toContain("grid-template-columns: minmax(0, 1fr)");
    expect(CSS).toMatch(/\.danh-sach-the > li \{\s*min-width: 0;/);
  });
});

// Tester 07/10/2026: "khoảng hở giữa 2 cột". These rules are unlayered, so whatever they say wins
// over the prototype's utilities on the element — they must carry the prototype's values themselves
// (`vigov-require/.../TaskKanbanBoard.tsx:106,146`, `TaskCard.tsx:42`).
describe("kanban board carries the prototype's layout values", () => {
  it("board: 14px gap, no padding of its own", () => {
    const board = ruleBody(".bang-kanban");
    expect(board).toContain("gap: 0.875rem;");
    expect(board).toContain("padding: 0;");
  });

  it("column: page-colour panel, hairline border, 12px radius, 12px padding, 180px tall", () => {
    const column = ruleBody(".cot-kanban");
    expect(column).toContain("background: var(--nen);");
    expect(column).toContain("border: 1px solid var(--line);");
    expect(column).toContain("border-radius: 12px;");
    expect(column).toContain("padding: 0.75rem;");
    expect(column).toContain("min-height: 11.25rem;");
    expect(column).not.toContain("var(--xam-vien)");
  });

  it("cards 10px apart; a card has no padding and no thick left border", () => {
    expect(ruleBody(".danh-sach-the")).toContain("gap: 0.625rem;");
    const card = ruleBody(".the-nhiem-vu");
    expect(card).toContain("padding: 0;");
    expect(card).toContain("border-radius: 10px;");
    expect(card).not.toContain("border-left");
  });
});

// ADR 0068 lần 6 review round 1 — each one a rendered defect a class-level test could not see.
describe("lần 6 review: hidden, tables, bell badge", () => {
  // T1-16: without preflight a `flex` class beat the UA's `[hidden]`. The rule must sit in `base` (its
  // `!important` then beats every later layer) and keep `until-found`.
  it("[hidden] hides, from the base layer, with !important", () => {
    const base = CSS.slice(CSS.indexOf("@layer base {"), CSS.indexOf("@layer legacy {"));
    expect(base).toMatch(/\[hidden\]:where\(:not\(\[hidden="until-found"\]\)\) \{\s*display: none !important;/);
  });

  // T1-17: shadcn `Table` is `w-full`; footer cells take the body cell's box; a scroller inside a card is
  // not a second frame, and nothing on it sets a shadow.
  it("data-table is full width; tfoot cells padded like body cells", () => {
    expect(RULES.find((r) => r.selector === ".data-table")?.body).toContain("width: 100%;");
    const foot = RULES.find((r) => r.selector === ".data-table tfoot th,\n.data-table tfoot td")?.body ?? "";
    expect(foot).toContain("padding: 0.5rem;");
    expect(foot).toContain("vertical-align: middle;");
  });

  it("bang-cuon: framed standing alone, frameless inside a card, never a box-shadow", () => {
    const own = RULES.find((r) => r.selector === ".bang-cuon")?.body ?? "";
    expect(own).toContain("overflow-x: auto;");
    expect(own).toContain("border: 1px solid var(--line);");
    const inCard = RULES.find((r) => r.selector === ':is([data-slot="card"], .shadow-card) .bang-cuon')?.body ?? "";
    expect(inCard).toContain("border: 0;");
    expect(inCard).toContain("border-radius: 0;");
    for (const r of RULES.filter((x) => /\.bang-cuon\b(?!-)/.test(x.selector))) expect(r.body, r.selector).not.toContain("box-shadow");
    expect(readFileSync(join(SRC, "components", "ui", "card.tsx"), "utf8")).toContain('data-slot="card"');
  });

  // T2-12: the prototype's badge (`NotificationBell.tsx:57`) in the spec's brand colour.
  it("bell badge: brand fill, 18px round, white 2px ring, -6px offsets, 10.5px bold", () => {
    const b = RULES.find((r) => r.selector === ".huy-hieu-chuong")?.body ?? "";
    for (const decl of [
      "top: -6px;",
      "right: -6px;",
      "min-width: 18px;",
      "height: 18px;",
      "border: 2px solid #fff;",
      "border-radius: 9999px;",
      "background: var(--brand-500);",
      "color: #fff;",
      "font-size: 10.5px;",
      "font-weight: 700;",
    ])
      expect(b, decl).toContain(decl);
  });
});
