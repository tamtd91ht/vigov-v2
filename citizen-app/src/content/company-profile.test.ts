import { describe, expect, it } from "vitest";

import * as profile from "./company-profile";
import {
  BRAND_NAVY,
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
  BRAND_NAVY,
];

/**
 * Every string reachable from the module, collected from the module object rather than typed
 * out. The hand-written list above is what the sweeps below read; this one is what keeps that
 * list honest. Adding `AWARDS` to the content file and forgetting the list would otherwise mean
 * the new strings are published without ever passing the "nothing personal" sweep — a hole that
 * opens silently, which is the only kind that survives review.
 */
function collectStrings(value: unknown, into: string[] = []): string[] {
  if (typeof value === "string") {
    into.push(value);
  } else if (Array.isArray(value)) {
    for (const item of value) collectStrings(item, into);
  } else if (value && typeof value === "object") {
    for (const item of Object.values(value)) collectStrings(item, into);
  }
  return into;
}

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
      "VihatSoftware",
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
    expect(owners.map((unit) => unit.name)).toEqual(["VihatSoftware"]);
  });

  it("attributes the hotline and email to the group, not to VihatSoftware alone", () => {
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

  /**
   * The two sweeps above only see what ALL_CONTENT_STRINGS lists, and that list is written by
   * hand. A new export — an awards list, a founder quote — would be published without ever being
   * swept, and nothing would say so. This is what makes the list self-maintaining.
   */
  it("sweeps every string the content module exports, including ones added later", () => {
    const exported = collectStrings(profile);
    const swept = new Set(ALL_CONTENT_STRINGS);
    const missed = exported.filter((value) => !swept.has(value));
    expect(
      missed,
      "new content strings are not covered by the personal-data sweep — add them to ALL_CONTENT_STRINGS",
    ).toEqual([]);
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

describe("published sentences stay word for word", () => {
  /**
   * WHY LENGTH > 0 IS NOT ENOUGH:
   *
   *   A vision statement, an office address and a certificate scope are STATEMENTS ABOUT A REAL
   *   LEGAL ENTITY. "Tidying the wording" of a published vision, or correcting an address to the
   *   one somebody remembers, produces a sentence the company never published — under the
   *   company's own name, inside a submission Zalo has reviewed. A non-empty check passes
   *   through every one of those edits.
   */

  it("prints the brand philosophy exactly as published", () => {
    expect(BRAND_STATEMENTS[0]?.body).toBe(
      "Đặt lợi ích và sự hài lòng của khách hàng lên hàng đầu. Đối tác lâu dài, tin cậy. Luôn khuyến khích sự sáng tạo, đổi mới.",
    );
  });

  it("prints the vision exactly as published", () => {
    expect(BRAND_STATEMENTS[1]?.body).toBe(
      "Trở thành tập đoàn có tầm ảnh hưởng toàn cầu với những sản phẩm, dịch vụ công nghệ thiết thực cho mọi doanh nghiệp, cùng nhau tạo ra nhiều giá trị cho xã hội phát triển.",
    );
  });

  it("prints the mission exactly as published", () => {
    expect(BRAND_STATEMENTS[2]?.body).toBe(
      "Giúp các doanh nghiệp, cá nhân ứng dụng những giải pháp công nghệ tiên tiến một cách đơn giản, hiệu quả và tiết kiệm nhất phù hợp với mọi doanh nghiệp từ nhỏ đến lớn.",
    );
  });

  it("gives each office the address that was published for it", () => {
    expect(OFFICES.map((office) => [office.name, office.address])).toEqual([
      [
        "Trụ sở chính",
        "140 – 142 đường số 2, KDC Vạn Phúc, Phường Hiệp Bình Phước, TP Hồ Chí Minh",
      ],
      ["Chi nhánh Hà Nội", "85 – 87 Hoàng Quốc Việt, P Nghĩa Đô, Cầu Giấy, Hà Nội"],
      [
        "Chi nhánh Cambodia",
        "Thida Rath #154 St.33MC, Sangkat Steung Meanchey, Khan Mean Chey Phnom Penh",
      ],
    ]);
  });

  it("labels each figure with what it counts", () => {
    expect(GROUP_STATS.map((stat) => stat.label)).toEqual([
      "Hành trình phát triển",
      "Khách hàng",
      "Đối tác",
      "Nhân sự",
      "Quốc gia kết nối",
    ]);
  });

  /**
   * A certificate scope is a claim about what an audit covered. The published source gives a
   * scope for the two ISO certificates and none for the Zalo accreditation, so this app gives
   * none either. Filling that blank with a plausible sentence invents an audit finding.
   */
  it("keeps the ISO scopes as published and invents none for the Zalo accreditation", () => {
    expect(CERTIFICATES.map((certificate) => certificate.scope)).toEqual([
      "Quản lý chất lượng",
      "An toàn thông tin",
      undefined,
    ]);
  });

  it("names both entities the way they are written", () => {
    expect(COMPANY.name).toBe("VihatSoftware");
    expect(GROUP.name).toBe("ViHAT Group");
    expect(COMPANY.website).toBe("https://vihatsoftware.com");
  });
});

describe("attribution notes say WHOSE the figures are", () => {
  /**
   * `toContain("ViHAT Group")` above is satisfied by a sentence that reverses the ownership —
   * "Số liệu của VihatSoftware, công ty con của ViHAT Group" contains the string and states the
   * opposite of the truth. The attribution is the whole point of these two sentences, so they
   * are pinned whole.
   */
  it("attributes the figures to the parent, in the published wording", () => {
    expect(GROUP.figuresOwnerNote).toBe(
      "Số liệu dưới đây là của Tập đoàn ViHAT Group, công ty mẹ của VihatSoftware.",
    );
  });

  it("attributes the contact points to the parent, in the published wording", () => {
    expect(CONTACT.ownerNote).toBe(
      "Hotline và email là đầu mối liên hệ chung của Tập đoàn ViHAT Group.",
    );
  });
});

describe("the Vietnamese is stored the way it will be rendered", () => {
  /**
   * TWO SILENT ENCODING FAILURES, NEITHER OF WHICH THE COMPILER SEES:
   *
   *   1. Decomposed text (NFD). "ế" pasted from some editors arrives as "ê" + a combining acute.
   *      It looks identical in a diff and in most editors, renders with drifting marks on some
   *      Android fonts, and breaks every `toBe` comparison for reasons nobody can see.
   *   2. Mojibake. A UTF-8 file re-saved as Latin-1 turns "ế" into "áº¿". That reaches the screen
   *      of a government-adjacent app as visible gibberish — the incident somebody answers for.
   */
  it("stores every string in precomposed form (NFC)", () => {
    for (const value of ALL_CONTENT_STRINGS) {
      expect(value.normalize("NFC"), `decomposed Vietnamese in: ${value}`).toBe(value);
    }
  });

  it("carries no mis-decoded characters", () => {
    // `[ÃÄÆ]` followed by a UTF-8 continuation byte, `â€`, or U+FFFD: the three shapes a
    // wrongly decoded Vietnamese file takes. Legitimate Vietnamese never produces them —
    // "XÃ" in capitals is followed by a space, not by a continuation byte.
    const mojibake = /[ÃÄÆ][-¿]|â€|�/;
    for (const value of ALL_CONTENT_STRINGS) {
      expect(value, `mis-decoded characters in: ${value}`).not.toMatch(mojibake);
    }
  });
});
