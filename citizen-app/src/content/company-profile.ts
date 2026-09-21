/**
 * PHASE 1 CONTENT — the ViHAT Group introduction.
 *
 * WHY EVERY STRING LIVES HERE AND NOWHERE ELSE:
 *
 *   This app ships in two phases on ONE Zalo App ID (README, ADR 0018). Phase 1 is this
 *   introduction, submitted so the OA `Vihat` can verify the app. Phase 2 replaces this
 *   content with the commune/citizen surface. When that happens the edit must land in ONE
 *   file, not in eight components.
 *
 * WHY THERE IS ONE ENTITY HERE AND NO LONGER TWO — owner's decision, 2026-09-21:
 *
 *   Ownership of this app moved from VihatSoftware to ViHAT GROUP, the parent, and the
 *   verifying OA moved with it. `COMPANY` and `GROUP` used to be two constants for exactly one
 *   reason: the subsidiary published the app, so the parent's figures, ecosystem and contact
 *   points had to carry a note saying whose they were. The publisher IS the parent now, so
 *   those three notes describe a relationship that no longer exists, and two constants holding
 *   one entity is the split-that-splits-nothing trap this repository has already paid for twice
 *   (see `chinh-sach-rieng-tu.ts`, §"một cơ chế tách đôi").
 *
 *   VihatSoftware stays in `MEMBER_UNITS`: it is still one of the six member units. What it no
 *   longer is, is the publisher of this app — so no unit carries a publisher flag any more.
 *
 * ⚠ THE THREE FIELDS THAT WERE ABSENT ARE NOW FILLED — 2026-09-21, FROM vihatgroup.com.
 *
 *   `positioning`, `description` and `website` used to hold VihatSoftware's published wording,
 *   were emptied when the app changed hands, and are filled again here from the GROUP's own
 *   site, read on 2026-09-21. Each one carries its source at the declaration, verbatim.
 *
 *   TWO FIELDS OF THE LEGAL ENTITY ARE STILL ABSENT AND STAY ABSENT: the tax code
 *   (`mã số thuế`) and the legal representative. Neither is published on either site, and
 *   neither may be typed from memory into a submission Zalo reviews.
 *
 * WHY NOTHING MAY BE ADDED HERE WITHOUT A SOURCE:
 *
 *   The app carries the name of a real legal entity. A founding year, a customer name, an
 *   award or a phone number that nobody sourced is a false statement published under that
 *   name, and somebody has to answer for it. Every field below was taken from
 *   vihatsoftware.com (metadata) and vihatgroup.com (rendered content) on 2026-09-17.
 *   Missing facts are LEFT OUT, never filled in.
 */

export type Stat = {
  /** Shown large. Kept as a string: "100K+" and ">100" are not numbers. */
  value: string;
  label: string;
};

export type NamedText = {
  title: string;
  body: string;
};

export type Office = {
  name: string;
  address: string;
};

/**
 * A technology line of the ViHAT Group ecosystem, shown as one card.
 *
 * WHY `product` IS OPTIONAL: the published source names the product on some lines and not on
 * others. A card whose product name was inferred ("this must be Vboss") would attribute a
 * product to a company on nothing but a guess, under the name of a real legal entity.
 */
export type Solution = {
  /** Picks the decorative glyph in SolutionsScreen. Never rendered as text. */
  id: SolutionId;
  /** Only filled where the source names the product. Left out otherwise, never inferred. */
  product?: string;
  /** Verbatim line as published. */
  headline: string;
  /** Second verbatim line, where the source publishes one for the same product. */
  note?: string;
};

export type SolutionId = "messaging" | "voice-ai" | "crm" | "namecard";

/** Picks the decorative glyph on a keyword chip. Never rendered as text. */
export type KeywordId = "cloud" | "cpaas" | "ai" | "crm" | "messaging";

export type Keyword = {
  id: KeywordId;
  label: string;
};

/**
 * A member unit of the group, shown as one chip in a list of six.
 *
 * NO `ownsThisApp` FLAG ANY MORE: it marked the one unit that published this app, and since
 * 2026-09-21 the publisher is the group itself, not a unit. A flag that is false for every row
 * is a flag that invites somebody to set it true again for the wrong reason.
 */
export type MemberUnit = {
  name: string;
};

export type Certificate = {
  name: string;
  /**
   * Optional on purpose. The published source gives a scope for the two ISO certificates and
   * none for the Zalo one — so this app gives none either, instead of writing a plausible
   * sentence about what an accreditation covers.
   */
  scope?: string;
};

/**
 * The dark brand colour — `#144a80`, the `--color-primary` token of vihatgroup.com.
 *
 * ⚠ IT CHANGED ON 2026-09-21, AND THE REASON IS THE SAME AS FOR THE THREE STRINGS ABOVE: the old
 * value `#1e3150` was read off vihatsoftware.com's `mask-icon` / `msapplication-TileColor`, i.e.
 * it was the SUBSIDIARY's frame colour. The app is published by the group, and the group declares
 * its own.
 *
 * FOUR PLACES HOLD THIS VALUE AND `bundle-for-zalo.test.ts` PINS THREE OF THEM TOGETHER: here,
 * `app-config.json` (`headerColor`, the native Zalo header), `index.html` (`theme-color`) and
 * `--navy` in styles.css. The platform header sits directly above ours — two of four updated is a
 * two-tone bar that reads as a rendering fault.
 *
 * The app icons are NOT a fifth place: `tools/logo.py` takes its colours from the `fill`
 * attributes of the logo vector (brand blue and green), and never from this value.
 */
export const BRAND_NAVY = "#144a80";

/**
 * The legal entity this app is published under. One entity, one constant.
 *
 * The three optional fields are optional because the app must keep working while they are
 * missing, NOT because they are decorative — see the header note. Every render site tests them.
 */
export type Company = {
  name: string;
  /**
   * ⚠ POSITIONING LINE — VIETNAMESE, AND THAT CHANGE OF LANGUAGE IS A DECISION TAKEN ON THE
   * CUSTOMER'S BEHALF ON 2026-09-21. IT IS MARKED HERE SO THEY CAN OVERRULE IT.
   *
   *   The field used to be declared as a "short ENGLISH positioning line", because the value it
   *   held — "Outsourcing Software Development Company" — was English. That string is the
   *   SUBSIDIARY's, inherited from vihatsoftware.com, and it went with the ownership transfer.
   *
   *   The group publishes NO English positioning line anywhere this repository has read. So the
   *   choice was: leave the field empty and ship a home screen with no lead line, invent an
   *   English sentence for a real legal entity, or take the group's own Vietnamese hero line.
   *   The third is the only one that publishes nothing nobody wrote. The declared type of the
   *   field therefore changes with it: this field is Vietnamese now.
   */
  positioning?: string;
  /** One-paragraph Vietnamese description, verbatim as published. */
  description?: string;
  /** Official website, `https://`, as the entity prints it on its own pages. */
  website?: string;
};

export const COMPANY: Company = {
  /**
   * Display name, settled 2026-09-21: `ViHAT Group`, one spelling everywhere.
   *
   * The sources disagree on the subsidiary's spelling, and the same discipline applies to the
   * parent: one spelling on screen, and it is the one the group publishes for itself.
   * The verifying Official Account is `Vihat`; the account name and the legal entity's name are
   * two different strings and neither is edited to match the other.
   */
  name: "ViHAT Group",

  /** Verbatim hero line of the "Về ViHAT" page on vihatgroup.com, read 2026-09-21. */
  positioning: "Chúng tôi xây dựng hệ sinh thái công nghệ toàn diện",

  /**
   * Verbatim one-paragraph description from vihatgroup.com, read 2026-09-21.
   *
   * ⚠ IT OPENS WITH "Hơn 12 năm" AND THAT NUMBER IS INSIDE A QUOTE, NOT A FIGURE THIS APP
   * COMPUTES. The figure the app computes is `soNamHoatDong()` below, and it is derived from the
   * founding date precisely so it cannot go stale. This sentence is the source's own wording and
   * is reproduced unchanged; editing it would publish a sentence the group never wrote.
   *
   *   CONSEQUENCE, STATED RATHER THAN HIDDEN: on 2026-12-06 the derived figure becomes 13 while
   *   this quote still says 12. That is a discrepancy inside a QUOTED paragraph, and the fix is
   *   for the customer to publish a new paragraph — not for this file to rewrite theirs.
   */
  description:
    "Hơn 12 năm kinh nghiệm trong lĩnh vực công nghệ, cùng đội ngũ 300+ nhân sự và 6 dự án lớn, ViHAT Group cam kết mang đến hệ thống thông minh, linh hoạt, giúp doanh nghiệp tăng trưởng bền vững và dẫn đầu thị trường",

  /** Verbatim: the site prints "vihatgroup.com" for itself. `https://` is the scheme it serves. */
  website: "https://vihatgroup.com",
};

/**
 * ⚠ THE HERO SLOGAN — AND ITS SOURCE IS **THE PM'S PROTOTYPE**, NOT vihatgroup.com.
 *
 *   Every other sentence in this file is a QUOTE: it was read off a published page, and
 *   company-profile.test.ts pins several of them word for word precisely so that nobody edits a
 *   published statement. This one is not. It was written for the handover prototype on
 *   2026-09-21, the owner approved using it, and it has never appeared on either website.
 *
 *   THAT DIFFERENCE HAS TO TRAVEL WITH THE VALUE, WHICH IS WHY THIS IS AN OBJECT AND NOT A BARE
 *   STRING. A slogan sitting among sourced quotes reads as one more sourced quote; six weeks from
 *   now somebody "corrects" the published positioning line to match it, or cites it back to the
 *   customer as their own published wording. The `nguon` field is unreadable from any screen and
 *   deliberately inseparable from the sentence — deleting it is a visible edit, forgetting it is
 *   not possible.
 *
 *   `COMPANY.positioning` KEEPS ITS PLACE. It is the group's own published hero line and it still
 *   renders — on the home screen under this slogan, and as the lead of the solutions screen. Two
 *   lines, two provenances, and the app does not merge them.
 */
export const SLOGAN_HERO = {
  cau: "Hạ tầng giao tiếp khách hàng cho doanh nghiệp Việt",
  nguon:
    "Bản mẫu giao diện của PM, 21/09/2026 — KHÔNG phải câu đã công bố trên vihatgroup.com. Chủ dự án duyệt dùng.",
} as const;

/**
 * ⚠ CÂU SẢN PHẨM DƯỚI SLOGAN — CŨNG LÀ CHỮ CỦA BẢN MẪU PM, KHÔNG PHẢI CHỮ ĐÃ CÔNG BỐ.
 *
 *   Nó là câu thứ HAI trong tệp này mang một `nguon` thay vì một chú thích, và vì đúng lý do của
 *   câu thứ nhất (`SLOGAN_HERO`): một câu chưa công bố đứng giữa những câu đã công bố sẽ được đọc
 *   thành một câu đã công bố, rồi được trích ngược lại cho khách như chữ của chính họ.
 *
 *   NÓ LIỆT KÊ NĂM DÒNG SẢN PHẨM, VÀ ĐÓ LÀ MỘT KHẲNG ĐỊNH VỀ NĂNG LỰC. `SOLUTIONS` bên dưới —
 *   thứ đọc từ vihatgroup.com — có BỐN dòng, và cách gọi tên khác hẳn. Hai danh sách ấy KHÔNG
 *   được trộn vào nhau: câu này là chữ của bản mẫu, danh sách kia là chữ của trang web, và ngày
 *   ai đó "đồng bộ" chúng là ngày một trong hai nguồn bị sửa mà không ai xin phép chủ của nó.
 */
export const CAU_SAN_PHAM = {
  cau: "SMS, Zalo, Voice, Contact Center và Marketing Automation trên một nền tảng.",
  nguon:
    "Bản mẫu giao diện của PM, 21/09/2026 — KHÔNG phải câu đã công bố trên vihatgroup.com. Chủ dự án duyệt dùng.",
} as const;

/** Nhãn đứng trên tên pháp nhân ở màn chủ. Năm ghép từ `NGAY_THANH_LAP`, xem `NHAN_TU_NAM`. */
export const NHAN_LOAI_HINH = "Tập đoàn công nghệ";

/**
 * Founding date of ViHAT Group — 06/12/2013, from the history strip published on vihatgroup.com.
 *
 * ⚠ THIS IS WHY THERE IS NO "12 năm" STRING IN THE FIGURE LIST BELOW ANY MORE.
 *
 *   A hardcoded "12 năm" is correct today, becomes wrong on 2026-12-06, and is wrong in the one
 *   way nobody catches: no test turns red, no screen breaks, the app simply understates the
 *   entity it publishes under. Derived from a date, it cannot drift.
 *
 * ISO form, parsed as UTC: a date literal without a zone is parsed in the RUNNER's zone, and the
 * same expression then answers differently on a CI box in UTC and on a laptop in UTC+7.
 */
export const NGAY_THANH_LAP = new Date("2013-12-06T00:00:00Z");

/**
 * Years since founding, counted in COMPLETED years — the app never rounds its own age upward.
 *
 * `moc` is a parameter so the calculation is testable at a fixed instant instead of only on the
 * day the suite happens to run.
 */
export function soNamHoatDong(moc: Date = new Date()): number {
  let nam = moc.getUTCFullYear() - NGAY_THANH_LAP.getUTCFullYear();
  const truoc_ngay_gio =
    moc.getUTCMonth() < NGAY_THANH_LAP.getUTCMonth() ||
    (moc.getUTCMonth() === NGAY_THANH_LAP.getUTCMonth() &&
      moc.getUTCDate() < NGAY_THANH_LAP.getUTCDate());
  if (truoc_ngay_gio) nam -= 1;
  return nam;
}

/** Label of the derived figure. Held here so the screen prints no sentence of its own. */
export const NHAN_SO_NAM = "Hành trình phát triển";

/**
 * Dòng đứng trên tên pháp nhân ở màn chủ: "Tập đoàn công nghệ · từ 2013".
 *
 * ⚠ NĂM LẤY TỪ `NGAY_THANH_LAP`, KHÔNG GÕ VÀO. Đây là con số thứ HAI trong app suy ra từ ngày
 * thành lập, và nó suy ra vì đúng lý do của con số thứ nhất (`soNamHoatDong`): một "2013" gõ
 * thẳng vào JSX là một khẳng định về một pháp nhân có thật, nằm ngoài tầm mọi phép kiểm đọc tệp
 * nội dung — và bản mẫu giao hàng ghi **2012**, tức sai đúng ở chỗ này. Suy ra từ một ngày thì
 * không có hai chỗ để lệch nhau.
 */
export function nhanLoaiHinhVaNam(): string {
  return `${NHAN_LOAI_HINH} · từ ${NGAY_THANH_LAP.getUTCFullYear()}`;
}

/**
 * THE FOUR PUBLISHED FIGURES. The fifth one — the years — is DERIVED, see above.
 *
 * Every value is a string because "100K+" and "Hơn 100" are not numbers, and rewriting them as
 * numbers would publish figures the group never printed.
 */
export const COMPANY_STATS: readonly Stat[] = [
  { value: "100K+", label: "Khách hàng" },
  { value: "500+", label: "Đối tác" },
  { value: "300+", label: "Nhân sự" },
  { value: "Hơn 100", label: "Quốc gia kết nối" },
];

/**
 * THE ECOSYSTEM SOLUTIONS ARE THE PUBLISHER'S OWN.
 *
 * ⚠ THERE ARE NOT 15 SOLUTIONS AND THERE ARE NOT 3 BUSINESS GROUPS. Checked 2026-09-21.
 *
 *   The prototype handed over asked for "15 giải pháp, 3 nhóm nghiệp vụ". Neither site publishes
 *   that: vihatgroup.com shows FIVE unit cards, vihatsoftware.com shows SIX services, and no page
 *   groups anything into three. The number 15 matches the subsidiary's count of PROJECTS, which
 *   is a different thing from a solution. Padding this list to fifteen would mean inventing
 *   eleven product lines for a real legal entity, so the list stays at what the source publishes.
 *
 * ⚠ AND THERE IS NO AI SECTION, DELIBERATELY. Everything the two sites publish about AI is: the
 *   ONE line below attached to OMICall, one unlabelled "AI" logo tile (which is the `TECH_KEYWORDS`
 *   chip), and one 2023 blog post titled "Giới thiệu về AI Callbot". That is the whole source.
 *   Writing a paragraph about the group's AI capability would be writing a capability claim
 *   nobody published. More than this needs omicall.com opened, and that is the owner's call.
 *
 * eSMS, OMICall and the rest are published on vihatgroup.com as the group's ecosystem, and the
 * group is who publishes this app. The `ECOSYSTEM.ownerNote` that used to stand above this list
 * existed to stop a subsidiary reading as the owner of its parent's product line; with one
 * entity there is nobody left to disclaim against, so the note is gone rather than reworded.
 */
export const SOLUTIONS: readonly Solution[] = [
  {
    id: "messaging",
    product: "eSMS",
    headline: "Giải pháp CPaaS toàn cầu: Messaging & Voice",
    note: "Hệ sinh thái giải pháp nâng cao trải nghiệm khách hàng đa kênh",
  },
  {
    id: "voice-ai",
    product: "OMICall",
    headline: "Tổng đài đa kênh ứng dụng AI hàng đầu Việt Nam",
  },
  // No product name on these two: the published description states the capability without
  // naming which member unit ships it, and naming one here would be an invention.
  { id: "crm", headline: "Contact Center tích hợp CRM" },
  {
    id: "namecard",
    headline: "Giải pháp networking và quản lý danh thiếp số cho cá nhân và doanh nghiệp",
  },
];

/**
 * Sourced technology terms, shown as chips. Every label is a term taken from the published
 * ecosystem description — none of them is a claim this file invents.
 *
 * "Tổng đài ảo" is the one label whose typography differs from the source: the source uses the
 * phrase mid-sentence in lower case, and a chip starts a line. The words are unchanged.
 */
export const TECH_KEYWORDS: readonly Keyword[] = [
  { id: "cpaas", label: "CPaaS" },
  { id: "messaging", label: "Messaging & Voice" },
  { id: "cloud", label: "Tổng đài ảo" },
  { id: "ai", label: "AI" },
  { id: "crm", label: "CRM" },
];

/**
 * SIX MEMBER UNITS — AND THE SOURCE CONTRADICTS ITSELF ABOUT THAT NUMBER. Resolved 2026-09-21.
 *
 *   The footer of vihatgroup.com lists SIX units (ViHAT Cambodia included). The "5 DỰ ÁN LỚN"
 *   block on the "Về ViHAT" page lists FIVE — while the paragraph directly above that same block
 *   says "6 dự án lớn". So the page disagrees with itself on the same screen.
 *
 *   SIX is what this app publishes, for two reasons: the footer is the structural statement (the
 *   block is a highlight reel), and six is what this repository has been carrying since
 *   2026-09-17, so keeping it changes nothing anybody has already reviewed. Recorded rather than
 *   silently picked, because the next person to open the source will hit the same contradiction.
 *
 *   Note this is also a different count from "solutions": five/six UNITS is not four ecosystem
 *   LINES, and neither is the "15 projects" figure the prototype carried (see `SOLUTIONS`).
 */
export const MEMBER_UNITS: readonly MemberUnit[] = [
  { name: "ViHAT Solutions" },
  { name: "VihatSoftware" },
  { name: "ViHAT Global" },
  { name: "ViHAT Cambodia" },
  { name: "OMI JSC" },
  { name: "Vboss" },
];

export const BRAND_STATEMENTS: readonly NamedText[] = [
  {
    title: "Triết lý thương hiệu",
    body: "Đặt lợi ích và sự hài lòng của khách hàng lên hàng đầu. Đối tác lâu dài, tin cậy. Luôn khuyến khích sự sáng tạo, đổi mới.",
  },
  {
    title: "Tầm nhìn",
    body: "Trở thành tập đoàn có tầm ảnh hưởng toàn cầu với những sản phẩm, dịch vụ công nghệ thiết thực cho mọi doanh nghiệp, cùng nhau tạo ra nhiều giá trị cho xã hội phát triển.",
  },
  {
    title: "Sứ mệnh",
    body: "Giúp các doanh nghiệp, cá nhân ứng dụng những giải pháp công nghệ tiên tiến một cách đơn giản, hiệu quả và tiết kiệm nhất phù hợp với mọi doanh nghiệp từ nhỏ đến lớn.",
  },
];

/**
 * A milestone on the published history strip. One row, one year, one sentence.
 *
 * `nam` is the label as the strip prints it, so a span ("2022 - 2023") survives as written; `tu`
 * is the year it SORTS by. Two fields because a label is for reading and a key is for ordering,
 * and collapsing them would either break the span or break the order.
 */
export type MocLichSu = {
  nam: string;
  tu: number;
  viec: string;
};

/**
 * THE PUBLISHED HISTORY STRIP OF ViHAT GROUP — nine milestones, read from vihatgroup.com on
 * 2026-09-21.
 *
 * ⚠ CONFIDENCE: MEDIUM. The strip is rendered as an image on the source page, so these nine rows
 * were read off a picture rather than copied out of text. The years and the events are legible;
 * the exact typography of each caption is not guaranteed word for word, which is why these
 * sentences are NOT pinned as verbatim quotes the way the vision and the mission are.
 *
 * ⚠ THE EARLIEST MILESTONE IS 2013, NOT 2012. The prototype handed over showed a strip starting
 * at 2012. The source says the group was founded on 06/12/2013, and `NGAY_THANH_LAP` above is
 * that date. A history strip that starts a year before the entity existed is a false statement
 * about a real legal entity, so the prototype was not followed here.
 *
 * ⚠ SPELLING: `VihatSoftware`, one word, as everywhere else in this app. The source's history
 * strip writes "ViHAT Software" with a space while vihatsoftware.com writes it as one word. One
 * spelling on screen — the same rule applied to the parent's own name — and the one chosen is the
 * one already standing in `MEMBER_UNITS` and in the privacy policy, where a spelling change would
 * be an edit to a legal declaration.
 */
export const MOC_LICH_SU: readonly MocLichSu[] = [
  { nam: "2013", tu: 2013, viec: "Thành lập ViHAT Group, ra mắt eSMS.VN" },
  { nam: "2018", tu: 2018, viec: "Thành lập ViHAT Cambodia" },
  { nam: "2019", tu: 2019, viec: "Ra mắt OMICall" },
  { nam: "2021", tu: 2021, viec: "Trở thành đại lý chính thức của Zalo" },
  { nam: "2021", tu: 2021, viec: "Thành lập VihatSoftware" },
  { nam: "2021", tu: 2021, viec: "Thành lập OMI JSC" },
  { nam: "2022", tu: 2022, viec: "Đạt chứng nhận ISO 9001 và ISO 27001" },
  { nam: "2022 - 2023", tu: 2022, viec: "OMICall và eSMS nhận giải Sao Khuê" },
  { nam: "2024", tu: 2024, viec: "Thành lập ViHAT Solutions" },
];

export const CERTIFICATES: readonly Certificate[] = [
  { name: "ISO 9001:2015", scope: "Quản lý chất lượng" },
  { name: "ISO 27001:2022", scope: "An toàn thông tin" },
  { name: "Chứng nhận đại lý chính thức của Zalo" },
];

export const OFFICES: readonly Office[] = [
  {
    name: "Trụ sở chính",
    address: "140 – 142 đường số 2, KDC Vạn Phúc, Phường Hiệp Bình Phước, TP Hồ Chí Minh",
  },
  {
    name: "Chi nhánh Hà Nội",
    address: "85 – 87 Hoàng Quốc Việt, P Nghĩa Đô, Cầu Giấy, Hà Nội",
  },
  {
    name: "Chi nhánh Cambodia",
    address: "Thida Rath #154 St.33MC, Sangkat Steung Meanchey, Khan Mean Chey Phnom Penh",
  },
];

/**
 * Corporate contact points of ViHAT Group, published on the group website.
 *
 * These are business contact details, NOT personal data under Decree 13/2023 — no individual
 * is identified by them. No individual's number belongs in this app, so no mobile number is
 * invented here to look more reachable.
 *
 * `ownerNote` is gone with the other two attribution notes: it told the reader these were the
 * PARENT's contact points, which only meant something while a subsidiary published the app.
 */
export const CONTACT = {
  hotlineLabel: "0287 1010 898",
  /** `tel:` target. Digits only: spaces in a tel URI are not reliably handled by dialers. */
  hotlineDialable: "02871010898",
  email: "contact@vihat.vn",
} as const;
