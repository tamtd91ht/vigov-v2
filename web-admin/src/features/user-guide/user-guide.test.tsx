import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { flattenMenu, NHOM_MENU } from "@/components/muc-menu";
import type { CauHinhXaHienThi } from "@/lib/cau-hinh-xa-hien-thi";
import { createHref } from "@/lib/create-link";

import { GUIDE_SECTIONS } from "./guide-content";
import { guideAccess, visibleGuide, type GuideSession } from "./guide-visibility";
import { UserGuideView } from "./user-guide";

/**
 * The guide page (`/huong-dan`, owner 17/10/2026): open to every signed-in account, but each section and
 * each button shows only when the account can open what it leads to. The denied cases are the ones that
 * matter here — an account with every key never sees them (definition of done, admin web builder).
 */

const COMMUNE: CauHinhXaHienThi = {
  displayName: "Xã Kiểm Thử",
  parentAuthority: "Tỉnh Kiểm Thử",
  logoUrl: "",
  webAdminBannerUrl: "",
};

/** Every key the menu, the create buttons and the tab gates read — the "full-permission" account. */
const MENU_KEYS = flattenMenu(NHOM_MENU).flatMap((m) => (m.khoa === null ? [] : typeof m.khoa === "string" ? [m.khoa] : [...m.khoa]));
const ALL_KEYS = [
  ...MENU_KEYS,
  "task.create",
  "petition.create",
  "announcement.create",
  "citizen.update",
  "budget.update",
  "feedback.create",
  "content.update",
  "admin.citizen_field",
  "admin.audit",
];

/** A read session holding exactly `permissions` — the shape `usePhien` returns. */
function sessionWith(permissions: readonly string[]): GuideSession {
  return { ok: true, duLieu: { permissions: [...permissions] } } as unknown as GuideSession;
}

function render(permissions: readonly string[] | null, commune: CauHinhXaHienThi = COMMUNE): string {
  const session = permissions === null ? null : sessionWith(permissions);
  const sections = visibleGuide(GUIDE_SECTIONS, guideAccess(permissions, session));
  return renderToStaticMarkup(<UserGuideView commune={commune} sections={sections} />);
}

const sectionIds = (html: string) => [...html.matchAll(/<section id="([^"]+)"/g)].map((m) => m[1]);
const hrefs = (html: string) => [...html.matchAll(/<a [^>]*href="([^"]*)"/g)].map((m) => m[1]!.replace(/&amp;/g, "&"));
/** The `<a>` whose visible words are `label`, or undefined. */
const linkNamed = (html: string, label: string) =>
  [...html.matchAll(/<a [^>]*>(.*?)<\/a>/g)].find((m) => m[1]!.replace(/<[^>]+>/g, "") === label)?.[0];

describe("user guide — full-permission account", () => {
  const html = render(ALL_KEYS);

  it("renders every section, in guide order, each with its TOC entry", () => {
    expect(sectionIds(html)).toEqual(GUIDE_SECTIONS.map((s) => s.id));
    for (const s of GUIDE_SECTIONS) expect(html, s.id).toContain(`href="#${s.id}"`);
  });

  it("numbers the sections 1..n with no gap", () => {
    const numbers = [...html.matchAll(/<h2 id="[^"]+"[^>]*><span[^>]*>(\d+)\.<\/span>/g)].map((m) => Number(m[1]));
    expect(numbers).toEqual(GUIDE_SECTIONS.map((_, i) => i + 1));
  });

  it("create buttons open the screen's create dialog: href is exactly createHref(route)", () => {
    const expected: [string, string][] = [
      ["Giao việc mới", createHref("/nhiem-vu")],
      ["Nhập biên bản", createHref("/nhiem-vu/bien-ban")],
      ["Vào sổ đơn thư", createHref("/van-ban")],
      ["Thêm dự án", createHref("/giai-ngan")],
      ["Soạn thông báo", createHref("/thong-bao")],
      ["Thêm người dân", createHref("/nguoi-dan")],
      ["Nhập hộ phản ánh", createHref("/phan-anh")],
      ["Thêm nội dung", createHref("/mini-app?tab=noi-dung")],
      ["Thêm cán bộ", createHref("/nguoi-dung")],
    ];
    for (const [label, href] of expected) {
      const a = linkNamed(html, label);
      expect(a, label).toBeDefined();
      expect(hrefs(a!), label).toEqual([href]);
    }
    expect(createHref("/mini-app?tab=noi-dung")).toBe("/mini-app?tab=noi-dung&create=1");
  });

  it("Gửi tin is a plain link (the screen is not a dialog); every Cấu hình and Hệ thống tab has its link", () => {
    expect(hrefs(linkNamed(html, "Soạn tin")!)).toEqual(["/gui-thong-bao"]);
    for (const tab of [
      "so-do-to-chuc",
      "thon-to-dan-pho",
      "danh-muc",
      "truong-ban-do",
      "loi-he-thong",
      "thoi-han-xu-ly",
      "tu-dong-hoa",
      "may-chu-thu",
      "kenh-zalo",
      "kenh-sms",
      "thuoc-tinh-danh-ba",
    ]) {
      expect(hrefs(html), tab).toContain(`/cau-hinh?tab=${tab}`);
    }
    for (const tab of ["lich-lam-viec", "nhat-ky-he-thong", "nhan-dien-xa"]) expect(hrefs(html), tab).toContain(`/he-thong?tab=${tab}`);
  });

  it("the commune's name comes from the configuration; no hardcoded commune, no demo wording, no key shown", () => {
    expect(html).toContain('<span class="guide-commune">Xã Kiểm Thử</span>');
    expect(html).toContain("Tỉnh Kiểm Thử");
    const other = render(ALL_KEYS, { ...COMMUNE, displayName: "Phường Khác", parentAuthority: "" });
    expect(other).toContain("Phường Khác");
    expect(other).not.toContain("Xã Kiểm Thử");
    for (const word of ["Tư Nghĩa", "Quảng Ngãi", "demo", "trải nghiệm"]) {
      expect(html.toLowerCase(), word).not.toContain(word.toLowerCase());
    }
    for (const key of ALL_KEYS) expect(html, key).not.toContain(key);
  });

  it("never injects markup: the bold marker becomes <strong>, nothing else is parsed", () => {
    expect(html).not.toContain("**");
    expect(html).toContain('<strong class="font-semibold text-navy">Giao việc mới</strong>');
  });
});

describe("user guide — DENIED cases", () => {
  it("menu visible but no `task.create`: Nhiệm vụ shows, its list link shows, `Giao việc mới` does not", () => {
    const html = render(["task.read"]);
    expect(sectionIds(html)).toContain("nhiem-vu");
    expect(linkNamed(html, "Mở danh sách nhiệm vụ")).toBeDefined();
    expect(linkNamed(html, "Giao việc mới")).toBeUndefined();
    expect(hrefs(html)).not.toContain(createHref("/nhiem-vu"));
  });

  it("no Nhiệm vụ menu (no `task.read`), even holding `task.create`: no Nhiệm vụ section and no link into it", () => {
    const html = render(["task.create", "report.read"]);
    expect(sectionIds(html)).not.toContain("nhiem-vu");
    expect(sectionIds(html)).not.toContain("so-tay");
    expect(sectionIds(html)).not.toContain("bien-ban");
    expect(hrefs(html).some((h) => h.startsWith("/nhiem-vu"))).toBe(false);
    expect(html).not.toContain('href="#nhiem-vu"');
  });

  it("no key at all: only the sections tied to no menu — login, own account, FAQ", () => {
    expect(sectionIds(render([]))).toEqual(["dang-nhap", "tai-khoan", "hoi-dap"]);
    expect(hrefs(render([])).filter((h) => !h.startsWith("#"))).toEqual(["/ca-nhan", "/doi-mat-khau"]);
  });

  it("session not read yet (null): the same three sections — gated ones can only appear later, never vanish", () => {
    expect(sectionIds(render(null))).toEqual(["dang-nhap", "tai-khoan", "hoi-dap"]);
  });

  it("only `admin.sms`: Cấu hình shows with its Kênh SMS link only — tabs follow the screen's own gates", () => {
    const html = render(["admin.sms"]);
    expect(sectionIds(html)).toContain("cau-hinh");
    const configLinks = hrefs(html).filter((h) => h.startsWith("/cau-hinh"));
    // The three tabs with no gate (read routes open to every account) plus the one this key opens.
    expect(configLinks).toEqual([
      "/cau-hinh?tab=so-do-to-chuc",
      "/cau-hinh?tab=thon-to-dan-pho",
      "/cau-hinh?tab=danh-muc",
      "/cau-hinh?tab=kenh-sms",
    ]);
  });

  it("Nội dung Mini App with `content.read` only: no `Thêm nội dung`, no Danh bạ cán bộ tab link", () => {
    const html = render(["content.read"]);
    expect(sectionIds(html)).toContain("mini-app");
    expect(linkNamed(html, "Thêm nội dung")).toBeUndefined();
    expect(linkNamed(html, "Cảnh báo thiên tai")).toBeDefined();
    expect(linkNamed(html, "Danh bạ cán bộ")).toBeUndefined();
  });

  it("only `admin.role`: Người dùng & Phân quyền shows with Phân quyền only — no Thêm cán bộ", () => {
    const html = render(["admin.role"]);
    expect(sectionIds(html)).toContain("nguoi-dung");
    expect(hrefs(linkNamed(html, "Phân quyền")!)).toEqual(["/nguoi-dung/phan-quyen"]);
    expect(linkNamed(html, "Thêm cán bộ")).toBeUndefined();
  });

  it("numbering is counted over VISIBLE sections: no gap when sections are hidden", () => {
    const html = render(["report.read"]);
    const numbers = [...html.matchAll(/<h2 id="[^"]+"[^>]*><span[^>]*>(\d+)\.<\/span>/g)].map((m) => Number(m[1]));
    expect(numbers).toEqual(numbers.map((_, i) => i + 1));
    expect(sectionIds(html)).toEqual(["dang-nhap", "tong-quan", "bao-cao", "tai-khoan", "hoi-dap"]);
  });
});
