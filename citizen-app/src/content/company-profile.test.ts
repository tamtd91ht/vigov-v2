import { describe, expect, it } from "vitest";

import {
  BRAND_STATEMENTS,
  CERTIFICATES,
  COMPANY,
  CONTACT,
  GROUP,
  GROUP_STATS,
  MEMBER_UNITS,
  OFFICES,
} from "./company-profile";
import { DEFAULT_SCREEN_ID, findScreen, SCREENS } from "../features/company-intro/screens";

/**
 * WHY A STATIC INTRODUCTION SCREEN IS WORTH TESTING AT ALL:
 *
 *   This app publishes statements under the name of a real legal entity. The failure mode is
 *   not a crash — it is a sentence that quietly changes, an attribution that gets dropped while
 *   someone tidies the wording, or a figure of the PARENT company reading as a figure of the
 *   subsidiary. None of those turn a screen red. Pinning them here is the only thing that
 *   notices.
 */

/** Every string the app can render, flattened — used by the "nothing personal" sweep below. */
const ALL_CONTENT_STRINGS: string[] = [
  COMPANY.name,
  COMPANY.positioning,
  COMPANY.description,
  COMPANY.website,
  GROUP.name,
  GROUP.figuresOwnerNote,
  ...GROUP_STATS.flatMap((stat) => [stat.value, stat.label]),
  ...MEMBER_UNITS.map((unit) => unit.name),
  ...BRAND_STATEMENTS.flatMap((statement) => [statement.title, statement.body]),
  ...CERTIFICATES.flatMap((certificate) =>
    certificate.scope ? [certificate.name, certificate.scope] : [certificate.name],
  ),
  ...OFFICES.flatMap((office) => [office.name, office.address]),
  CONTACT.hotlineLabel,
  CONTACT.hotlineDialable,
  CONTACT.email,
  CONTACT.ownerNote,
];

describe("sourced facts stay exactly as published", () => {
  it("keeps the English positioning line untranslated", () => {
    expect(COMPANY.positioning).toBe("Outsourcing Software Development Company");
  });

  it("keeps the Vietnamese description verbatim, diacritics included", () => {
    expect(COMPANY.description).toBe(
      "Cung cấp giải pháp phần mềm, ứng dụng và giải pháp trên điện thoại di động cho cá nhân và doanh nghiệp.",
    );
  });

  it("publishes the three brand statements under their published titles", () => {
    expect(BRAND_STATEMENTS.map((statement) => statement.title)).toEqual([
      "Triết lý thương hiệu",
      "Tầm nhìn",
      "Sứ mệnh",
    ]);
    for (const statement of BRAND_STATEMENTS) {
      expect(statement.body.length).toBeGreaterThan(0);
    }
  });

  it("names the three certificates and nothing beyond them", () => {
    expect(CERTIFICATES.map((certificate) => certificate.name)).toEqual([
      "ISO 9001:2015",
      "ISO 27001:2022",
      "Chứng nhận đại lý chính thức của Zalo",
    ]);
  });

  it("lists all six member units of the group", () => {
    expect(MEMBER_UNITS.map((unit) => unit.name)).toEqual([
      "ViHAT Solutions",
      "ViHAT Software",
      "ViHAT Global",
      "ViHAT Cambodia",
      "OMI JSC",
      "Vboss",
    ]);
  });

  it("carries the head office and both branches", () => {
    expect(OFFICES).toHaveLength(3);
    for (const office of OFFICES) {
      expect(office.address.trim().length).toBeGreaterThan(0);
    }
  });
});

describe("attribution of the parent company's figures", () => {
  /**
   * 12 years / 100K+ / 500+ / 300+ / 100+ countries belong to ViHAT GROUP. Dropping the note
   * that says so turns published parent figures into an unsourced claim by the subsidiary.
   */
  it("names ViHAT Group in the note that accompanies the figures", () => {
    expect(GROUP.figuresOwnerNote).toContain("ViHAT Group");
  });

  it("keeps every figure as published, including the approximate ones", () => {
    expect(GROUP_STATS.map((stat) => stat.value)).toEqual([
      "12 năm",
      "100K+",
      "500+",
      "300+",
      "Hơn 100",
    ]);
  });

  it("marks exactly one member unit as the publisher of this app", () => {
    const owners = MEMBER_UNITS.filter((unit) => unit.ownsThisApp);
    expect(owners.map((unit) => unit.name)).toEqual(["ViHAT Software"]);
  });

  it("attributes the hotline and email to the group, not to ViHAT Software alone", () => {
    expect(CONTACT.ownerNote).toContain("ViHAT Group");
  });
});

describe("contact links are actually usable", () => {
  it("gives the dialer digits only — a tel: URI with spaces is not reliably dialled", () => {
    expect(CONTACT.hotlineDialable).toMatch(/^[0-9]+$/);
  });

  it("dials the same number it displays", () => {
    expect(CONTACT.hotlineDialable).toBe(CONTACT.hotlineLabel.replace(/\s/g, ""));
  });

  it("uses the published corporate mailbox", () => {
    expect(CONTACT.email).toBe("contact@vihat.vn");
  });

  it("opens the website over https", () => {
    expect(COMPANY.website.startsWith("https://")).toBe(true);
  });
});

describe("phase 1 publishes no personal data", () => {
  /**
   * Rule 3. Phase 1 has no form, no `getPhoneNumber` and no backend call, so the only way
   * personal data could appear is by someone typing it into the content file. A Vietnamese
   * mobile number is the shape most likely to arrive that way — an individual's direct line
   * added "so customers can reach him". The corporate landline is a business contact point and
   * stays; a mobile number is a person.
   */
  it("contains no Vietnamese mobile number", () => {
    const mobilePattern = /(^|\D)(0[35789])\d{8}(\D|$)/;
    for (const value of ALL_CONTENT_STRINGS) {
      expect(value.replace(/\s/g, ""), `personal-looking number in: ${value}`).not.toMatch(
        mobilePattern,
      );
    }
  });

  it("contains no national identity number", () => {
    for (const value of ALL_CONTENT_STRINGS) {
      expect(value.replace(/\s/g, ""), `12-digit run in: ${value}`).not.toMatch(/(^|\D)\d{12}(\D|$)/);
    }
  });
});

describe("screen registry", () => {
  it("declares four screens with unique ids", () => {
    const ids = SCREENS.map((screen) => screen.id);
    expect(ids).toHaveLength(4);
    expect(new Set(ids).size).toBe(4);
  });

  it("resolves every declared screen — a tab must never lead nowhere", () => {
    for (const screen of SCREENS) {
      expect(findScreen(screen.id)).toBe(screen);
    }
  });

  it("opens on a screen that exists", () => {
    expect(() => findScreen(DEFAULT_SCREEN_ID)).not.toThrow();
  });

  it("gives every screen a tab label and a header title", () => {
    for (const screen of SCREENS) {
      expect(screen.tabLabel.trim().length).toBeGreaterThan(0);
      expect(screen.headerTitle.trim().length).toBeGreaterThan(0);
    }
  });
});
