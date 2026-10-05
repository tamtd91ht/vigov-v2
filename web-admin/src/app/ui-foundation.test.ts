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
    for (const cls of [".nut-chinh {", ".nut-phu {", ".bang-cuon {", ".khung-trang {", ".dau-trang {"]) {
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
  // Presentation pins (ADR 0068 §5): rows and motion follow the OMICALL tokens of 05/10/2026.
  it("declares the v2 tokens: 240/72px sidebar, 60px topbar, 64px rows, 4/8px spacing, motion", () => {
    for (const decl of [
      "--sidebar-w: 240px;",
      "--sidebar-w-collapsed: 72px;",
      "--topbar-h: 60px;",
      "--row-h: 64px;",
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

  it("no frosted glass anywhere: no backdrop-filter, no blur()", () => {
    expect(stripComments(CSS)).not.toMatch(/backdrop-filter|blur\(/);
    for (const f of NEW_UI) expect(f.text, f.path).not.toMatch(/backdrop-blur|backdrop-filter/);
  });

  it("no gradient in the shared components (the PageHeader tile is flat)", () => {
    for (const f of NEW_UI) expect(f.text, f.path).not.toMatch(/bg-linear|bg-gradient|from-brand-|linear-gradient/);
  });

  // Presentation pins (guide §7, §8.9; ADR 0068 §Sửa đổi 05/10/2026 lần 2).
  it("app shell: grey outside, 8px margin, radius 16, `--bg` inside, corners clipped without breaking sticky", () => {
    const shell = RULES.find((r) => r.selector === ".khung-trang")?.body ?? "";
    for (const decl of ["margin: 8px;", "border-radius: var(--r-lg);", "background: var(--bg);", "overflow: clip;", "min-height: calc(100dvh - 16px);"])
      expect(shell, decl).toContain(decl);
    expect(RULES.find((r) => r.selector.includes("body:has(.khung-trang)"))?.body).toContain("background: var(--bg-outer);");
  });

  it("header: 68px navy, sticky, header shadow, WHITE focus ring on navy (the app ring is 3.18:1 there)", () => {
    const h = RULES.find((r) => r.selector === ".dau-trang")?.body ?? "";
    for (const decl of ["position: sticky;", "top: 0;", "height: 68px;", "background: var(--brand-600);", "box-shadow: var(--shadow-header);"])
      expect(h, decl).toContain(decl);
    expect(RULES.find((r) => r.selector === ".dau-trang :focus-visible")?.body).toContain("outline: 2px solid #fff;");
    const icon = RULES.find((r) => r.selector === ".header-icon-button")?.body ?? "";
    for (const decl of ["width: 36px;", "height: 36px;", "border-radius: var(--r-md);"]) expect(icon, decl).toContain(decl);
  });

  it("the old white text sidebar stays gone, and no shell rule draws a gradient", () => {
    expect(CSS).not.toMatch(/\.thanh-ben/);
    expect(CSS).not.toMatch(/\.header-modules/);
    for (const r of RULES.filter((x) => /dau-trang|header-|nav-sheet|side-nav|khung-trang|user-menu/.test(x.selector)))
      expect(r.body, r.selector).not.toMatch(/gradient/);
  });

  // Presentation pins (owner 05/10/2026; ADR 0068 lần 2 #1): the sidebar is hidden below 768px (the nav
  // sheet takes over), the accent #00B1FF never becomes its text colour, and it sticks under the header.
  it("left sidebar: hidden by default, 240/72px from 768px, sticky under the 68px header, accent never text", () => {
    expect(RULES.find((r) => r.selector === ".side-nav" && r.body.includes("display: none;"))).toBeDefined();
    const wide = RULES.find((r) => r.selector === ".side-nav" && r.body.includes("display: block;"))?.body ?? "";
    expect(wide).toContain("width: var(--sidebar-w);");
    expect(RULES.find((r) => r.selector === ".side-nav.is-collapsed")?.body).toContain("width: var(--sidebar-w-collapsed);");
    const inner = RULES.find((r) => r.selector === ".side-nav-inner")?.body ?? "";
    for (const decl of ["position: sticky;", "top: 68px;"]) expect(inner, decl).toContain(decl);
    for (const r of RULES.filter((x) => /side-nav/.test(x.selector)))
      expect(r.body, r.selector).not.toMatch(/(^|[\s;])color:\s*(var\(--accent-500\)|#00b1ff)/i);
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

  it("is the controlClass look: 36px, line-strong hairline, control radius, chevron, no native arrow", () => {
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
    // Presentation pin (ADR 0068 §5): the OMICALL tokens of 05/10/2026 use Roboto 400/500/600.
    expect(layout).toContain('import "@fontsource/roboto/400.css";');
    expect(layout).toContain('import "@fontsource/roboto/600.css";');
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

// Tester screenshot 06/10/2026: Kanban cards spilled over the next column at ≥1280px. The legacy
// `.cot-kanban { min-width: 15rem }` is unlayered, so it beat Tailwind's `xl:min-w-0` and each column
// grew past its 1fr track. The rule must stay bounded below the 1280px grid breakpoint.
describe("kanban board does not overlap at the grid breakpoint", () => {
  it("the fixed-width column rule is limited to 768–1279px", () => {
    const i = CSS.indexOf("min-width: 15rem;");
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
