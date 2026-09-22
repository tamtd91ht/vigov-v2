import { describe, expect, it } from "vitest";

import * as profile from "./company-profile";
import {
  BRAND_NAVY,
  BRAND_STATEMENTS,
  CAU_SAN_PHAM,
  CERTIFICATES,
  COMPANY,
  COMPANY_STATS,
  CONTACT,
  MEMBER_UNITS,
  MOC_LICH_SU,
  NGAY_THANH_LAP,
  NHAN_LOAI_HINH,
  nhanLoaiHinhVaNam,
  NHAN_SO_NAM,
  OFFICES,
  SLOGAN_HERO,
  soNamHoatDong,
  SOLUTIONS,
  TECH_KEYWORDS,
} from "./company-profile";

import {
  DEFAULT_SCREEN_ID,
  findScreen,
  MAN_GIOI_THIEU,
  SCREENS,
} from "../features/company-intro/screens";

/**
 * WHY A STATIC INTRODUCTION SCREEN IS WORTH TESTING AT ALL:
 *
 *   This app publishes statements under the name of a real legal entity. The failure mode is
 *   not a crash — it is a sentence that quietly changes, an attribution that gets dropped while
 *   someone tidies the wording, or a figure of the PARENT company reading as a figure of the
 *   subsidiary. None of those turn a screen red. Pinning them here is the only thing that
 *   notices.
 */

/**
 * Every string the app can render, flattened — used by the "nothing personal" sweep below.
 *
 * The three optional `COMPANY` fields are filtered rather than listed: they are unset until the
 * customer supplies the group's own wording, and the sweeps below must cover them the day they
 * arrive without anyone remembering to come back here.
 */
const ALL_CONTENT_STRINGS: string[] = [
  COMPANY.name,
  COMPANY.positioning,
  COMPANY.description,
  COMPANY.website,
  // Câu slogan của bản mẫu PM, và DÒNG GHI NGUỒN đi kèm nó. Cả hai vào lượt quét như mọi chuỗi
  // khác: `nguon` không hiện lên màn hình nào, nhưng nó nằm trong bundle, và lượt quét dưới đây
  // cố ý mù với chuyện một chuỗi có được đọc bởi người dùng hay không.
  SLOGAN_HERO.cau,
  SLOGAN_HERO.nguon,
  CAU_SAN_PHAM.cau,
  CAU_SAN_PHAM.nguon,
  NHAN_LOAI_HINH,
  ...COMPANY_STATS.flatMap((stat) => [stat.value, stat.label]),
  NHAN_SO_NAM,
  ...MOC_LICH_SU.flatMap((moc) => [moc.nam, moc.viec]),
  ...MEMBER_UNITS.map((unit) => unit.name),
  ...BRAND_STATEMENTS.flatMap((statement) => [statement.title, statement.body]),
  ...CERTIFICATES.flatMap((certificate) =>
    certificate.scope ? [certificate.name, certificate.scope] : [certificate.name],
  ),
  ...OFFICES.flatMap((office) => [office.name, office.address]),
  CONTACT.hotlineLabel,
  CONTACT.hotlineDialable,
  CONTACT.email,
  // `id` is swept too: it is an exported string, and the sweep below is deliberately blind to
  // whether a string was meant to be read by a citizen or by a lookup table.
  ...SOLUTIONS.flatMap((solution) =>
    [solution.id, solution.product, solution.headline, solution.note].filter(
      (value): value is string => typeof value === "string",
    ),
  ),
  ...TECH_KEYWORDS.flatMap((keyword) => [keyword.id, keyword.label]),
  BRAND_NAVY,
].filter((value): value is string => typeof value === "string");

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
  /**
   * THE THREE FIELDS THE OWNERSHIP TRANSFER EMPTIED, AND THAT WERE FILLED AGAIN ON 2026-09-21.
   *
   *   Between 2026-09-21 morning and this commit these three were `undefined`, and the case here
   *   pinned them as `undefined` so that whoever filled one in had to say where it came from.
   *   That is exactly what happened, so the case turns into what it was always meant to become:
   *   the three strings pinned WORD FOR WORD, with the source named.
   *
   *   SOURCE: vihatgroup.com, read 2026-09-21.
   *     · positioning — the hero line of the "Về ViHAT" page. VIETNAMESE, and the change of
   *       language is a decision taken on the customer's behalf: the group publishes no English
   *       positioning line anywhere, and the English one this field used to hold was the
   *       SUBSIDIARY's. Marked in `company-profile.ts` for the customer to overrule.
   *     · description — the one-paragraph description, verbatim including its own "Hơn 12 năm".
   *     · website — the site prints "vihatgroup.com" for itself.
   *
   *   STILL MISSING, AND NOT INVENTED: tax code and legal representative. Neither is published.
   */
  it("prints the three group strings word for word, as read from vihatgroup.com", () => {
    expect(COMPANY.positioning).toBe("Chúng tôi xây dựng hệ sinh thái công nghệ toàn diện");
    expect(COMPANY.description).toBe(
      "Hơn 12 năm kinh nghiệm trong lĩnh vực công nghệ, cùng đội ngũ 300+ nhân sự và 6 dự án lớn, ViHAT Group cam kết mang đến hệ thống thông minh, linh hoạt, giúp doanh nghiệp tăng trưởng bền vững và dẫn đầu thị trường",
    );
    expect(COMPANY.website).toBe("https://vihatgroup.com");
  });

  it("carries no tax code and no legal representative — neither is published anywhere", () => {
    // The two fields a company profile is most often "completed" with from memory. Both are
    // statements about a legal entity that a reviewer can check against the business register,
    // and both are absent from both sites. A field that does not exist cannot be filled wrongly.
    expect(Object.keys(COMPANY).sort()).toEqual(["description", "name", "positioning", "website"]);
    const swept = collectStrings(profile).join("\n");
    expect(swept, "a tax code appeared in the content module").not.toMatch(/[Mm]ã số thuế|MST/);
    expect(swept, "a legal representative appeared in the content module").not.toMatch(
      /[Nn]gười đại diện|[Đđ]ại diện theo pháp luật/,
    );
  });

  it("does not carry VihatSoftware's wording under the group's name", () => {
    // The cheapest wrong fix is to move the old strings across: they are already written, they
    // read well, and nothing else in the suite would notice. They are the SUBSIDIARY's.
    const swept = collectStrings(profile).join("\n");
    expect(swept, "VihatSoftware's positioning line is being published as the group's").not.toContain(
      "Outsourcing Software Development Company",
    );
    expect(swept, "vihatsoftware.com is being published as the group's address").not.toContain(
      "vihatsoftware.com",
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

  /**
   * THE ECOSYSTEM LINES ARE QUOTES, NOT DESCRIPTIONS.
   *
   *   Each one is published on vihatgroup.com and is printed under the name of a real legal
   *   entity. "Tidying" one produces a sentence the company never published, and a sentence
   *   about a product is a claim about what that product does. Pinned whole, like the vision
   *   and the mission above.
   */
  it("quotes each ecosystem line exactly as published", () => {
    expect(SOLUTIONS.map((solution) => solution.headline)).toEqual([
      "Giải pháp CPaaS toàn cầu: Messaging & Voice",
      "Tổng đài đa kênh ứng dụng AI hàng đầu Việt Nam",
      "Contact Center tích hợp CRM",
      "Giải pháp networking và quản lý danh thiếp số cho cá nhân và doanh nghiệp",
    ]);
    expect(SOLUTIONS.map((solution) => solution.note)).toEqual([
      "Hệ sinh thái giải pháp nâng cao trải nghiệm khách hàng đa kênh",
      undefined,
      undefined,
      undefined,
    ]);
  });

  it("names a product only where the source names one", () => {
    // The source attributes two of these lines to a product and leaves two unattributed. Filling
    // the blanks with the member unit that seems likeliest would credit a company with a product
    // on a guess — the same failure as inventing a certificate scope.
    expect(SOLUTIONS.map((solution) => solution.product)).toEqual([
      "eSMS",
      "OMICall",
      undefined,
      undefined,
    ]);
  });

  it("keeps every technology chip a term taken from the source", () => {
    expect(TECH_KEYWORDS.map((keyword) => keyword.label)).toEqual([
      "CPaaS",
      "Messaging & Voice",
      "Tổng đài ảo",
      "AI",
      "CRM",
    ]);
  });

  /**
   * THE THREE ATTRIBUTION NOTES ARE GONE, AND MUST STAY GONE.
   *
   *   "Số liệu … của Tập đoàn ViHAT Group, công ty mẹ của VihatSoftware", the same for the
   *   ecosystem, and "Hotline và email là đầu mối liên hệ chung của Tập đoàn" each existed for
   *   one reason: a SUBSIDIARY published this app and could not appear to own its parent's
   *   scale, products or contact points. The parent publishes it now.
   *
   *   Why this is a test and not just a deletion: the sentences are polite, they read like
   *   diligence, and the next person to tidy this file will not know the premise died. A
   *   disclaimer nobody needs tells a reviewer the publisher is standing behind something.
   */
  it("keeps no parent/subsidiary disclaimer anywhere in the content module", () => {
    const swept = collectStrings(profile);
    for (const value of swept) {
      expect(value, `a parent/subsidiary disclaimer survived: ${value}`).not.toMatch(
        /công ty mẹ|công ty con/,
      );
    }
  });

  it("carries the head office and both branches", () => {
    expect(OFFICES).toHaveLength(3);
    for (const office of OFFICES) {
      expect(office.address.trim().length).toBeGreaterThan(0);
    }
  });
});

describe("the published figures and who they belong to", () => {
  it("keeps every figure as published, including the approximate ones", () => {
    expect(COMPANY_STATS.map((stat) => stat.value)).toEqual(["100K+", "500+", "300+", "Hơn 100"]);
  });

  /**
   * THE YEARS FIGURE IS DERIVED, AND THIS IS THE CASE THAT STOPS IT BECOMING A STRING AGAIN.
   *
   *   "12 năm" was a stored value until 2026-09-21. It is correct today, wrong from 2026-12-06,
   *   and wrong in the one way nobody catches: nothing turns red, no screen breaks, the app just
   *   quietly understates the age of the entity it is published under. Derived from a date it
   *   cannot drift — but the cheapest "fix" for anything awkward here is to type the number back
   *   in, so the absence is pinned.
   */
  it("stores no hardcoded age anywhere in the content module", () => {
    for (const value of collectStrings(profile)) {
      expect(value, `a hardcoded age is stored again: ${value}`).not.toMatch(
        /^\s*(?:hơn\s+)?\d{1,2}\s*năm\s*$/i,
      );
    }
    expect(COMPANY_STATS.map((stat) => stat.label)).not.toContain(NHAN_SO_NAM);
  });

  it("counts the years from the founding date, in completed years, at a fixed instant", () => {
    // Fixed instants, not `new Date()`: a case that asks today's date passes on the day it is
    // written and says nothing about the day the boundary is crossed.
    expect(NGAY_THANH_LAP.toISOString()).toBe("2013-12-06T00:00:00.000Z");
    expect(soNamHoatDong(new Date("2026-09-21T00:00:00Z"))).toBe(12);
    expect(soNamHoatDong(new Date("2026-12-05T23:59:59Z")), "rounds its own age up").toBe(12);
    expect(soNamHoatDong(new Date("2026-12-06T00:00:00Z"))).toBe(13);
    expect(soNamHoatDong(new Date("2027-01-01T00:00:00Z"))).toBe(13);
  });

  /**
   * DÒNG "Tập đoàn công nghệ · từ 2013" LÀ CON SỐ THỨ HAI SUY RA TỪ NGÀY THÀNH LẬP.
   *
   *   Bản mẫu giao hàng ghi **2012**, và chủ dự án chốt theo NGUỒN: 06/12/2013. Một "2013" gõ
   *   thẳng vào JSX thì mọi phép kiểm đọc tệp nội dung đều không nhìn thấy — đó chính là cách con
   *   số 2012 của bản mẫu suýt đi vào màn hình mà không có gì đỏ lên.
   */
  it("ghép dòng loại hình với năm ĐỌC TỪ ngày thành lập, không gõ vào", () => {
    expect(nhanLoaiHinhVaNam()).toBe(`${NHAN_LOAI_HINH} · từ 2013`);
    expect(nhanLoaiHinhVaNam(), "năm của bản mẫu quay lại").not.toContain("2012");
    // Và cái nhãn tự nó KHÔNG mang năm nào: mang năm ở đây là dựng chỗ thứ hai để lệch.
    expect(NHAN_LOAI_HINH, "nhãn loại hình tự mang một năm viết cứng").not.toMatch(/\d{4}/);
  });
});

/**
 * HAI CÂU TRONG HERO, HAI NGUỒN — VÀ CA NÀY CANH ĐÚNG CHỖ CHÚNG BỊ LẪN VÀO NHAU.
 *
 *   `COMPANY.positioning` là câu hero ĐÃ CÔNG BỐ trên vihatgroup.com và được ghim nguyên văn ở
 *   trên. `SLOGAN_HERO.cau` là chữ của BẢN MẪU PM: chủ dự án duyệt dùng, nhưng nó chưa từng xuất
 *   hiện trên trang nào.
 *
 *   Cách hỏng không ai thấy: sáu tuần nữa có người đọc màn chủ, thấy hai câu cạnh nhau, và "dọn"
 *   bằng cách bỏ một câu hoặc sửa câu đã công bố cho khớp câu kia — tức sửa một văn bản đã công
 *   bố của một pháp nhân có thật. Hoặc trích câu slogan lại cho khách như chữ của chính họ.
 *
 *   Nên `nguon` là một TRƯỜNG, không phải một chú thích: chú thích tách khỏi giá trị được, trường
 *   thì không.
 */
describe("câu slogan của bản mẫu, và nguồn đi kèm nó", () => {
  it("giữ nguyên văn câu bản mẫu", () => {
    expect(SLOGAN_HERO.cau).toBe("Hạ tầng giao tiếp khách hàng cho doanh nghiệp Việt");
  });

  it("mang theo lời khai nguồn, và lời khai ấy nói rõ KHÔNG phải nguồn trang web", () => {
    expect(SLOGAN_HERO.nguon, "không nói nguồn là bản mẫu").toMatch(/[Bb]ản mẫu/);
    expect(SLOGAN_HERO.nguon, "không phủ định nguồn trang web").toMatch(/KHÔNG phải/);
    expect(SLOGAN_HERO.nguon).toContain("vihatgroup.com");
  });

  /**
   * CÂU THỨ HAI CỦA BẢN MẪU — CÙNG MỘT LUẬT, VÀ NÓ CÒN DỄ BỊ ĐỌC NHẦM HƠN CÂU THỨ NHẤT.
   *
   *   Câu này LIỆT KÊ NĂM DÒNG SẢN PHẨM. Nó trông y hệt một câu trích từ trang giới thiệu hệ sinh
   *   thái, mà `SOLUTIONS` — thứ thật sự đọc từ vihatgroup.com — chỉ có BỐN dòng và gọi tên khác
   *   hẳn. Người sau đọc hai chỗ, thấy lệch, và "sửa cho khớp" một trong hai. Nếu chỗ bị sửa là
   *   `SOLUTIONS` thì đó là sửa văn bản đã công bố của một pháp nhân có thật.
   */
  it("câu sản phẩm cũng là chữ bản mẫu, và mang theo lời khai nguồn của nó", () => {
    expect(CAU_SAN_PHAM.cau).toBe(
      "SMS, Zalo, Voice, Contact Center và Marketing Automation trên một nền tảng.",
    );
    expect(CAU_SAN_PHAM.nguon, "không nói nguồn là bản mẫu").toMatch(/[Bb]ản mẫu/);
    expect(CAU_SAN_PHAM.nguon, "không phủ định nguồn trang web").toMatch(/KHÔNG phải/);
    expect(CAU_SAN_PHAM.nguon).toContain("vihatgroup.com");
  });

  it("câu sản phẩm KHÔNG bị nhầm thành một câu đã công bố", () => {
    expect(CAU_SAN_PHAM.cau).not.toBe(COMPANY.positioning);
    expect(CAU_SAN_PHAM.cau).not.toBe(COMPANY.description);
    expect(CAU_SAN_PHAM.cau).not.toBe(SLOGAN_HERO.cau);
    // Và không dòng nào của `SOLUTIONS` — thứ đọc từ trang web — bị viết lại thành câu này.
    for (const giai_phap of SOLUTIONS) {
      expect(giai_phap.headline).not.toBe(CAU_SAN_PHAM.cau);
    }
  });

  it("KHÔNG thay câu định vị đã công bố, và không trùng với nó", () => {
    // Câu đã công bố giữ chỗ của nó. Ngày ai đó gán `positioning = SLOGAN_HERO.cau` cho gọn, ca
    // này đỏ — và đó là một lượt sửa văn bản đã công bố, không phải một lượt dọn mã.
    expect(COMPANY.positioning).toBe("Chúng tôi xây dựng hệ sinh thái công nghệ toàn diện");
    expect(SLOGAN_HERO.cau).not.toBe(COMPANY.positioning);
  });
});

describe("the published history strip", () => {
  /**
   * ⚠ NINE MILESTONES, EARLIEST 2013 — AND THE "2012" IN THE PROTOTYPE IS THE POINT OF THIS CASE.
   *
   *   The prototype handed over drew a strip starting at 2012. The source says the group was
   *   founded on 06/12/2013. A history strip that starts a year before the entity existed is a
   *   false statement about a real legal entity, printed under that entity's own name, in a
   *   submission a reviewer reads. Pinned so that following the prototype turns the suite red.
   */
  it("starts no earlier than the founding year, and never before it", () => {
    const som_nhat = Math.min(...MOC_LICH_SU.map((moc) => moc.tu));
    expect(som_nhat).toBe(NGAY_THANH_LAP.getUTCFullYear());
    expect(som_nhat, "a milestone predates the entity — the prototype's 2012 is back").toBe(2013);
  });

  it("carries the nine milestones, in chronological order", () => {
    expect(MOC_LICH_SU).toHaveLength(9);
    const nam = MOC_LICH_SU.map((moc) => moc.tu);
    expect([...nam].sort((a, b) => a - b), "the strip is out of order").toEqual(nam);
    expect(nam).toEqual([2013, 2018, 2019, 2021, 2021, 2021, 2022, 2022, 2024]);
  });

  it("writes every milestone as a sentence, not as a bare year", () => {
    for (const moc of MOC_LICH_SU) {
      expect(moc.nam.trim().length, "a milestone has no year label").toBeGreaterThan(0);
      expect(moc.viec.trim().length, `${moc.nam}: no event`).toBeGreaterThan(10);
      // The label may be a span ("2022 - 2023"); it must still begin with the year it sorts by.
      expect(moc.nam.startsWith(String(moc.tu)), `${moc.nam} does not start with ${moc.tu}`).toBe(
        true,
      );
    }
  });

  it("spells the subsidiary one way across the whole module", () => {
    // The history strip on the source writes "ViHAT Software" with a space; vihatsoftware.com
    // writes it as one word, and so does `MEMBER_UNITS` and the privacy policy — where a spelling
    // change would be an edit to a legal declaration. One spelling on screen.
    const swept = collectStrings(profile).join("\n");
    expect(swept, "two spellings of the subsidiary are on screen at once").not.toContain(
      "ViHAT Software",
    );
    expect(swept).toContain("VihatSoftware");
  });
});

describe("the six member units", () => {
  /**
   * SIX, NOT FIVE — and the source disagrees with ITSELF about that, on one page.
   *
   *   The footer lists six units; the "5 DỰ ÁN LỚN" block lists five while the paragraph above it
   *   says "6 dự án lớn". Six is what this app publishes (see the note on `MEMBER_UNITS`). Pinned
   *   here so that the next person who opens the source and finds five does not quietly drop one.
   */
  it("publishes six units, the count the footer states", () => {
    expect(MEMBER_UNITS).toHaveLength(6);
  });

  /**
   * NO MEMBER UNIT PUBLISHES THIS APP ANY MORE — the group does, and the group is not one of the
   * six. The unit that used to carry the flag is still in the list: it is still a member unit.
   * Removing it to "clean up" would delete a published fact about the group's structure.
   */
  it("carries no publisher flag on any member unit, and still lists VihatSoftware as one of six", () => {
    for (const unit of MEMBER_UNITS) {
      expect(
        Object.keys(unit),
        `member unit ${unit.name} carries a field beyond its name — a publisher flag is back`,
      ).toEqual(["name"]);
    }
    expect(MEMBER_UNITS.map((unit) => unit.name)).toContain("VihatSoftware");
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

  it("opens the website over https — whenever there is a website to open", () => {
    // Written as a conditional rather than deleted: the field is unset today, and the day the
    // customer supplies it this is the check that stops an `http://` or a bare domain shipping.
    if (COMPANY.website !== undefined) {
      expect(COMPANY.website.startsWith("https://")).toBe(true);
    }
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
  it("declares four introduction screens with unique ids", () => {
    const ids = MAN_GIOI_THIEU.map((screen) => screen.id);
    expect(ids).toHaveLength(4);
    expect(new Set(ids).size).toBe(4);
  });

  it("sổ màn hình là bốn màn ấy CỘNG bốn màn ngoài tab, ở MỌI biến thể", () => {
    // Không còn phụ thuộc biến thể: ba quyền nay thuộc về chính ứng dụng sản phẩm, nên màn
    // "Danh thiếp" có mặt ở cả bản nộp lẫn bản demo; "Gợi ý giải pháp" (22/09/2026, giai đoạn A)
    // và hai màn của bề mặt yêu cầu (22/09/2026, giai đoạn B) cũng vậy.
    //
    // GHIM TÁM Ở ĐÂY, VÀ GHIM BỐN Ở CA TRÊN — hai con số đo hai thứ khác nhau, và khoảng cách
    // giữa chúng chính là số màn KHÔNG có ô trên thanh tab. Ca trên (`MAN_GIOI_THIEU`) đỏ lên nếu
    // ai thêm tab thứ năm mà quên đo lại bề rộng 320px (`accessibility.test.ts`); ca này đỏ lên
    // nếu ai thêm một màn mà quên khai nó vào sổ.
    const ids = SCREENS.map((screen) => screen.id);
    expect(new Set(ids).size, "hai màn trùng id — một tab sẽ dẫn sang màn kia").toBe(ids.length);
    expect(SCREENS).toHaveLength(8);
    for (const id of ["danh-thiep", "goi-y", "tu-van", "yeu-cau"]) {
      expect(ids, `sổ màn hình thiếu màn ngoài tab: ${id}`).toContain(id);
    }
    for (const man of MAN_GIOI_THIEU) expect(ids).toContain(man.id);
  });

  it("resolves every declared screen — a tab must never lead nowhere", () => {
    for (const screen of SCREENS) {
      expect(findScreen(screen.id)).toBe(screen);
    }
  });

  it("opens on a screen that exists", () => {
    expect(() => findScreen(DEFAULT_SCREEN_ID)).not.toThrow();
  });

  /**
   * MỖI MÀN PHẢI TRẢ LỜI ĐỦ HAI CÂU: "tiêu đề của tôi là gì" và "CHỖ CỦA TÔI TRÊN THANH TAB".
   *
   * Câu thứ hai mới từ 21/09/2026 (khuya), khi màn "Danh thiếp" bỏ tab theo bản mẫu. Một màn
   * không có tab mà cũng không khai ô cha là một màn mà đứng ở đó thanh tab không sáng ô nào:
   * công dân mất dấu vị trí của mình, và không có gì đỏ lên — vì bốn màn kia vẫn đúng.
   */
  it("gives every screen a header title and a declared place on the tab bar", () => {
    for (const screen of SCREENS) {
      expect(screen.headerTitle.trim().length).toBeGreaterThan(0);
      const cho = screen.cho;
      if (cho.kieu === "tab") {
        expect(cho.nhan.trim().length, `${screen.id}: tab không có nhãn chữ`).toBeGreaterThan(0);
        expect(typeof cho.glyph, `${screen.id}: tab không có hình`).toBe("function");
      } else {
        const cha = SCREENS.find((man) => man.id === cho.tabSangLen);
        expect(cha, `${screen.id} khai ô cha là một màn không có trong sổ`).toBeDefined();
        expect(cha!.cho.kieu, `${screen.id} khai ô cha là một màn cũng không có tab`).toBe("tab");
      }
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
    expect(COMPANY_STATS.map((stat) => stat.label)).toEqual([
      "Khách hàng",
      "Đối tác",
      "Nhân sự",
      "Quốc gia kết nối",
    ]);
    // The fifth label belongs to the DERIVED figure and is held separately, so that a figure
    // computed at render time still has exactly one place its wording is written down.
    expect(NHAN_SO_NAM).toBe("Hành trình phát triển");
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

  it("names the publishing entity the way it is written", () => {
    // ONE entity since 2026-09-21. The spelling is pinned because it is printed in the native
    // Zalo header, in the page title and on the vCard people save into their contacts
    // (`bundle-for-zalo.test.ts` reads it from here), and three places drifting apart is the
    // discrepancy a reviewer asks about.
    expect(COMPANY.name).toBe("ViHAT Group");
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
