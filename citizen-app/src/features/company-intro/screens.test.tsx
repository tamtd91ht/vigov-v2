import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { App } from "../../App";
import { TabBar } from "../../components/TabBar";
import {
  BRAND_STATEMENTS,
  CERTIFICATES,
  COMPANY,
  CONTACT,
  ECOSYSTEM,
  GROUP,
  GROUP_STATS,
  MEMBER_UNITS,
  OFFICES,
  SOLUTIONS,
  TECH_KEYWORDS,
} from "../../content/company-profile";
import { AboutScreen } from "./AboutScreen";
import { ContactScreen } from "./ContactScreen";
import { HomeScreen } from "./HomeScreen";
import { DEFAULT_SCREEN_ID, findScreen, SCREENS } from "./screens";
import { SolutionsScreen } from "./SolutionsScreen";

/**
 * WHY PINNING THE CONTENT FILE IS NOT ENOUGH:
 *
 *   company-profile.test.ts proves the sourced facts are stored correctly. It cannot prove any
 *   of them reaches a screen. Deleting `{GROUP.figuresOwnerNote}` from HomeScreen leaves every
 *   one of those tests green while the app publishes the PARENT's figures with no owner named —
 *   exactly the overstatement a reviewer is right to reject. The guarantee is "the attribution
 *   is on the screen, next to the figures", and only rendering can check that.
 *
 * WHY NOT A SNAPSHOT:
 *
 *   A whole-screen snapshot goes red on every wording change, and a suite that goes red for
 *   harmless reasons gets updated without being read. Each assertion below names one fact that
 *   must be visible, so a failure says what was lost.
 */

const render = (element: Parameters<typeof renderToStaticMarkup>[0]) =>
  renderToStaticMarkup(element);

/** Text content only — assertions are about what a citizen reads, not about markup. */
const ENTITIES: Record<string, string> = {
  "&amp;": "&",
  "&lt;": "<",
  "&gt;": ">",
  "&quot;": '"',
  "&#x27;": "'",
  "&#39;": "'",
};

/**
 * Entities are decoded because the assertions are about what a CITIZEN reads. "Messaging &
 * Voice" leaves the renderer as `Messaging &amp; Voice`; comparing that against the published
 * sentence fails for a reason that has nothing to do with the sentence, and a test that is red
 * for a false reason is a test somebody relaxes.
 */
const textOf = (markup: string) =>
  markup
    .replace(/<[^>]*>/g, " ")
    .replace(/&(?:amp|lt|gt|quot|#x27|#39);/g, (entity) => ENTITIES[entity] ?? entity)
    .replace(/\s+/g, " ");

/** The `<li>` elements of a rendered screen, each still a complete element. */
const listItems = (markup: string) =>
  markup
    .split("<li")
    .slice(1)
    .map((item) => `<li${item}`);

const SCREEN_MARKUP = [
  { id: "home", markup: render(<HomeScreen />) },
  { id: "solutions", markup: render(<SolutionsScreen />) },
  { id: "about", markup: render(<AboutScreen />) },
  { id: "contact", markup: render(<ContactScreen />) },
] as const;

describe("the attribution reaches the screen, not just the content file", () => {
  it("shows the owner note on the same screen as the figures", () => {
    const text = textOf(render(<HomeScreen />));
    expect(text).toContain(GROUP.figuresOwnerNote);
  });

  it("puts the owner note ABOVE the first figure", () => {
    // A reviewer reads top to bottom. The note under the numbers is a note read too late:
    // by then the subsidiary has already appeared to claim the parent's scale.
    const text = textOf(render(<HomeScreen />));
    const noteAt = text.indexOf(GROUP.figuresOwnerNote);
    const firstFigureAt = text.indexOf(GROUP_STATS[0]!.value);
    expect(noteAt).toBeGreaterThanOrEqual(0);
    expect(firstFigureAt).toBeGreaterThanOrEqual(0);
    expect(noteAt, "the group figures are printed before anyone is told whose they are").toBeLessThan(
      firstFigureAt,
    );
  });

  it("prints every figure it stores — a dropped one is a changed claim", () => {
    const text = textOf(render(<HomeScreen />));
    for (const stat of GROUP_STATS) {
      expect(text, `figure missing from the home screen: ${stat.label}`).toContain(stat.value);
      expect(text).toContain(stat.label);
    }
  });

  it("repeats the parent/subsidiary relation on the screen that carries the group's statements", () => {
    const text = textOf(render(<AboutScreen />));
    expect(text).toContain(GROUP.name);
    expect(text).toContain("công ty mẹ của VihatSoftware");
  });

  it("marks the publishing unit with words, never with a colour alone", () => {
    // Accessibility non-negotiable #6 and the only thing that says WHICH of six units published
    // this app. A badge reduced to a coloured dot says nothing to a screen reader.
    const items = listItems(render(<SolutionsScreen />));
    const badged = items.filter((item) => item.includes("Đơn vị phát hành ứng dụng này"));
    expect(badged).toHaveLength(1);
    expect(badged[0]).toContain("VihatSoftware");
  });

  it("lists all six member units", () => {
    const text = textOf(render(<SolutionsScreen />));
    for (const unit of MEMBER_UNITS) {
      expect(text, `member unit missing: ${unit.name}`).toContain(unit.name);
    }
  });

  it("prints every ecosystem line, with the product name the source gives it", () => {
    const text = textOf(render(<SolutionsScreen />));
    for (const solution of SOLUTIONS) {
      expect(text, `solution line missing: ${solution.id}`).toContain(solution.headline);
      if (solution.product) expect(text).toContain(solution.product);
      if (solution.note) expect(text).toContain(solution.note);
    }
  });

  it("puts the ecosystem owner note ABOVE the first solution line", () => {
    // Same reason as the figures on the home screen: these are the PARENT's products. A note
    // read after the product list is a note read once the subsidiary already appeared to own it.
    const text = textOf(render(<SolutionsScreen />));
    const noteAt = text.indexOf(ECOSYSTEM.ownerNote);
    const firstLineAt = text.indexOf(SOLUTIONS[0]!.headline);
    expect(noteAt).toBeGreaterThanOrEqual(0);
    expect(firstLineAt).toBeGreaterThanOrEqual(0);
    expect(noteAt, "the group's products are listed before anyone is told whose they are").toBeLessThan(
      firstLineAt,
    );
  });

  it("prints every technology chip as WORDS", () => {
    // The chips exist to say "cloud, AI, CRM" at a glance. A glyph carries none of that to a
    // screen reader — the glyphs are aria-hidden precisely because the word next to them is the
    // content, and a chip reduced to its icon would be an empty pill.
    const text = textOf(render(<SolutionsScreen />));
    for (const keyword of TECH_KEYWORDS) {
      expect(text, `keyword chip missing: ${keyword.id}`).toContain(keyword.label);
    }
  });

  it("prints the three brand statements in full", () => {
    const text = textOf(render(<AboutScreen />));
    for (const statement of BRAND_STATEMENTS) {
      expect(text, `statement missing: ${statement.title}`).toContain(statement.title);
      expect(text, `statement body truncated: ${statement.title}`).toContain(statement.body);
    }
  });

  it("prints each certificate, and no scope for the one published without one", () => {
    const items = listItems(render(<AboutScreen />));
    for (const certificate of CERTIFICATES) {
      const item = items.find((candidate) => candidate.includes(certificate.name));
      expect(item, `certificate missing: ${certificate.name}`).toBeDefined();
    }
    const zalo = items.find((item) => item.includes("Chứng nhận đại lý chính thức của Zalo"));
    // Anything after the name inside that list item would be an invented scope.
    expect(textOf(zalo ?? "").trim()).toBe("Chứng nhận đại lý chính thức của Zalo");
  });

  it("shows every office address", () => {
    const text = textOf(render(<ContactScreen />));
    for (const office of OFFICES) {
      expect(text, `office missing: ${office.name}`).toContain(office.address);
    }
  });
});

describe("contact actions do what they say", () => {
  const markup = render(<ContactScreen />);

  it("dials the number it displays", () => {
    // The displayed label and the tel: target are two separate fields. Editing one and not the
    // other sends a citizen's call to a number the screen never showed — and nobody notices
    // until a call lands somewhere it should not.
    const href = /href="tel:([^"]+)"/.exec(markup)?.[1];
    expect(href, "no tel: link on the contact screen").toBeDefined();
    expect(href).toBe(CONTACT.hotlineLabel.replace(/\D/g, ""));
    expect(textOf(markup)).toContain(CONTACT.hotlineLabel);
  });

  it("mails the address it displays", () => {
    const href = /href="mailto:([^"]+)"/.exec(markup)?.[1];
    expect(href).toBe(CONTACT.email);
    expect(textOf(markup)).toContain(CONTACT.email);
  });

  it("opens the company website over https, with the opener detached", () => {
    // `target="_blank"` without `rel="noopener"` hands the opened page a handle on this app's
    // window. Cheap to add, and the kind of thing a security review of a government-adjacent
    // app asks about.
    const anchor = /<a[^>]*href="https:\/\/[^"]*"[^>]*>/.exec(markup)?.[0] ?? "";
    expect(anchor).toContain(COMPANY.website);
    expect(anchor).toContain('rel="noopener noreferrer"');
  });

  it("offers no form, no field and no upload — phase 1 collects nothing", () => {
    for (const { id, markup: screenMarkup } of SCREEN_MARKUP) {
      expect(screenMarkup, `${id} renders an input control`).not.toMatch(
        /<(form|input|textarea|select)\b/,
      );
    }
  });
});

describe("what every screen owes the reader", () => {
  it("gives each screen exactly one first-level heading", () => {
    for (const { id, markup } of SCREEN_MARKUP) {
      const headings = markup.match(/<h1\b/g) ?? [];
      expect(headings.length, `${id} has ${headings.length} <h1> — a screen reader needs one`).toBe(
        1,
      );
    }
  });

  it("hides every decorative glyph from the screen reader", () => {
    // A drawn icon that is announced is noise for the citizen who depends on the announcement
    // most, and an icon that is NOT decoration would mean information carried by a picture alone
    // (README §Non-negotiables #6). Either way the answer is the same: the words carry it.
    for (const { id, markup } of SCREEN_MARKUP) {
      const svgs = markup.match(/<svg[\s>][^>]*>/g) ?? [];
      expect(svgs.length, `${id} renders no glyph at all`).toBeGreaterThan(0);
      for (const svg of svgs) {
        expect(svg, `${id} renders an <svg> a screen reader will announce`).toContain(
          'aria-hidden="true"',
        );
      }
      // No <title>/<desc> either: both are announced, and neither is content this app owns.
      expect(markup, `${id} puts text inside a decorative glyph`).not.toMatch(/<(title|desc)[\s>]/);
    }
  });

  it("renders no personal data, even hardcoded straight into a screen", () => {
    // The content-file sweep cannot see a number typed directly into JSX. This one can.
    for (const { id, markup } of SCREEN_MARKUP) {
      const digits = textOf(markup).replace(/\s/g, "");
      expect(digits, `${id} renders a Vietnamese mobile number`).not.toMatch(
        /(^|\D)0[35789]\d{8}(\D|$)/,
      );
      expect(digits, `${id} renders a 12-digit identity number`).not.toMatch(/(^|\D)\d{12}(\D|$)/);
    }
  });

  it("names the publisher and the current screen in the header", () => {
    // The header slot phase 2 fills with the COMMUNE name (README non-negotiable #2). If it can
    // go empty in phase 1 it can go empty in phase 2, and then a citizen submits to a commune
    // they were never shown.
    const text = textOf(render(<App />));
    expect(text).toContain(COMPANY.name);
    expect(text).toContain(findScreen(DEFAULT_SCREEN_ID).headerTitle);
  });
});

describe("tab bar", () => {
  it("marks exactly one tab as current, for every screen", () => {
    for (const screen of SCREENS) {
      const markup = render(<TabBar current={screen.id} onSelect={() => {}} />);
      const current = markup.match(/aria-current="page"/g) ?? [];
      expect(current.length, `${screen.id}: ${current.length} tabs marked current`).toBe(1);
    }
  });

  it("offers a button for every declared screen and nothing else", () => {
    const markup = render(<TabBar current={DEFAULT_SCREEN_ID} onSelect={() => {}} />);
    const buttons = markup.match(/<button\b/g) ?? [];
    expect(buttons).toHaveLength(SCREENS.length);
    for (const screen of SCREENS) {
      expect(textOf(markup)).toContain(screen.tabLabel);
    }
  });

  it("announces the current tab through aria-current, not through colour", () => {
    const markup = render(<TabBar current="contact" onSelect={() => {}} />);
    const currentButton = markup.split("<button").find((item) => item.includes('aria-current="page"'));
    expect(textOf(currentButton ?? "")).toContain(findScreen("contact").tabLabel);
  });
});
