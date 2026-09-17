/**
 * PHASE 1 CONTENT — the VihatSoftware company introduction.
 *
 * WHY EVERY STRING LIVES HERE AND NOWHERE ELSE:
 *
 *   This app ships in two phases on ONE Zalo App ID (README, ADR 0018). Phase 1 is this
 *   introduction, submitted so the OA `VihatSoftware` can verify the app. Phase 2 replaces
 *   this content with the commune/citizen surface. When that happens the edit must land in
 *   ONE file, not in eight components.
 *
 * WHY NOTHING MAY BE ADDED HERE WITHOUT A SOURCE:
 *
 *   The app carries the name of a real legal entity. A founding year, a customer name, an
 *   award or a phone number that nobody sourced is a false statement published under that
 *   name, and somebody has to answer for it. Every field below was taken from
 *   vihatsoftware.com (metadata) and vihatgroup.com (rendered content) on 2026-09-17.
 *   Missing facts are LEFT OUT, never filled in. See README §"Open content questions".
 *
 * WHY THE GROUP FIGURES CARRY AN EXPLICIT OWNER LABEL:
 *
 *   12 years, 100K+ customers, 500+ partners, 300+ staff and 100+ countries are figures of
 *   ViHAT GROUP, the parent. VihatSoftware is one member unit of six. Printing them without
 *   saying whose they are would attribute the parent's scale to the subsidiary — the kind of
 *   overstatement a reviewer is right to reject.
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

export type MemberUnit = {
  name: string;
  /** True for the single unit that owns this app. Drives a text badge, never colour alone. */
  ownsThisApp: boolean;
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

/** Brand navy, taken from the website's `mask-icon` and `msapplication-TileColor`. */
export const BRAND_NAVY = "#1e3150";

export const COMPANY = {
  /**
   * Display name, settled 2026-09-17: `VihatSoftware`, one spelling everywhere.
   *
   * The sources disagree — vihatsoftware.com writes `VihatSoftware`, vihatgroup.com writes it
   * with a space. The verifying Official Account is named `VihatSoftware` (ADR 0018), and an app
   * whose own name is spelled one way on screen and another way on the account that vouches for
   * it gives a reviewer a discrepancy to ask about. One spelling, and it is the account's.
   */
  name: "VihatSoftware",
  /** Verbatim English positioning line from vihatsoftware.com. Not translated on purpose. */
  positioning: "Outsourcing Software Development Company",
  /** Verbatim Vietnamese description from vihatgroup.com. */
  description:
    "Cung cấp giải pháp phần mềm, ứng dụng và giải pháp trên điện thoại di động cho cá nhân và doanh nghiệp.",
  website: "https://vihatsoftware.com",
} as const;

export const GROUP = {
  name: "ViHAT Group",
  /** Every screen that shows a GROUP figure prints this line next to it. */
  figuresOwnerNote: "Số liệu dưới đây là của Tập đoàn ViHAT Group, công ty mẹ của VihatSoftware.",
} as const;

export const GROUP_STATS: readonly Stat[] = [
  { value: "12 năm", label: "Hành trình phát triển" },
  { value: "100K+", label: "Khách hàng" },
  { value: "500+", label: "Đối tác" },
  { value: "300+", label: "Nhân sự" },
  { value: "Hơn 100", label: "Quốc gia kết nối" },
];

export const MEMBER_UNITS: readonly MemberUnit[] = [
  { name: "ViHAT Solutions", ownsThisApp: false },
  { name: "VihatSoftware", ownsThisApp: true },
  { name: "ViHAT Global", ownsThisApp: false },
  { name: "ViHAT Cambodia", ownsThisApp: false },
  { name: "OMI JSC", ownsThisApp: false },
  { name: "Vboss", ownsThisApp: false },
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
 * Corporate contact points of ViHAT GROUP, published on the group website.
 *
 * These are business contact details, NOT personal data under Decree 13/2023 — no individual
 * is identified by them. No individual's number belongs in this app, and VihatSoftware has
 * no separate published hotline, so none is invented here.
 */
export const CONTACT = {
  hotlineLabel: "0287 1010 898",
  /** `tel:` target. Digits only: spaces in a tel URI are not reliably handled by dialers. */
  hotlineDialable: "02871010898",
  email: "contact@vihat.vn",
  ownerNote: "Hotline và email là đầu mối liên hệ chung của Tập đoàn ViHAT Group.",
} as const;
