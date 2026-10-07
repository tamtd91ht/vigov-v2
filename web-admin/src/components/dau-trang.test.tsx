import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { PhienProvider } from "@/features/phien/phien-hien-tai";

import { CauHinhXaProvider } from "./cau-hinh-xa";
import { AUTHORITY_KIND } from "./commune-identity";
import { DauTrang } from "./dau-trang";
import { userInitials } from "./user-initials";

/**
 * The WHITE header carries the commune's identity on EVERY signed-in page (spec 01, ADR 0068 §Sửa đổi
 * 07/10/2026 lần 6 #2, #11): one place, so no page is without the body's name and none prints it twice.
 * The failure guarded is silent and public: a string-built "UBND " + name reads right for "xã Tân Phú"
 * and wrong — "UBND UBND xã Tân Phú" — for a commune that declared the prefix.
 */
const COMMUNE = { displayName: "Xã Tân Phú", parentAuthority: "Tỉnh Đồng Nai", logoUrl: "", webAdminBannerUrl: "" };

function render(commune: Partial<typeof COMMUNE> = {}, navigation = true) {
  return renderToStaticMarkup(
    <CauHinhXaProvider giaTri={{ ...COMMUNE, ...commune }}>
      <PhienProvider>
        <DauTrang navigation={navigation} />
      </PhienProvider>
    </CauHinhXaProvider>,
  );
}

const headerOf = (html: string) => html.slice(html.indexOf("<header"), html.indexOf("</header>"));
const sidebarOf = (html: string) => html.slice(html.indexOf('<aside class="side-nav"'), html.indexOf("</aside>"));

describe("DauTrang — commune identity", () => {
  it("the authority kind and displayName as TWO elements on one line, verbatim, upper-cased by CSS; then the province", () => {
    for (const navigation of [true, false]) {
      const header = headerOf(render({ displayName: "UBND xã Tân Phú" }, navigation));
      const line = /<p class="([^"]*)"><span class="commune-authority-kind">([^<]*)<\/span> <span class="ten-co-quan">([^<]*)<\/span><\/p>/.exec(header);
      expect(line, "one line, two elements").not.toBeNull();
      expect(line![1]).toContain("uppercase");
      expect(line![2]).toBe(AUTHORITY_KIND);
      // Printed as declared — the capitals are CSS, never a rebuilt string.
      expect(line![3]).toBe("UBND xã Tân Phú");
      expect(header.match(/UBND xã Tân Phú/g)).toHaveLength(1);
      expect(header).not.toContain("UBND UBND");
      expect(header).toMatch(/<p class="co-quan-cap-tren[^"]*">Tỉnh Đồng Nai<\/p>/);
      expect(header.indexOf("commune-identity")).toBeLessThan(header.indexOf("header-end"));
    }
  });

  // ADR 0068 lần 6 #3 lifted §13 for the sidebar's brand block ONLY: the product name is required there
  // and still absent from the header, where the commune's identity is.
  it("the product name sits in the sidebar's brand block, and nowhere in the header (ADR 0068 lần 6 #3)", () => {
    const html = render();
    const sidebar = sidebarOf(html);
    expect(sidebar).toContain('<span class="side-nav-brand-name">ViGov</span>');
    expect(sidebar).toContain('<span class="side-nav-brand-tagline">Điều hành số cấp xã</span>');
    const header = headerOf(html);
    for (const s of ["ViGov", "ViHAT", "OMICALL", "Omicall", "Điều hành số cấp xã", "VG<"]) expect(header).not.toContain(s);
    // `navigation={false}` draws no sidebar, so no product name at all.
    for (const s of ["ViGov", "Điều hành số cấp xã"]) expect(render({}, false)).not.toContain(s);
  });

  it("a commune that declared no parent authority gets no line — never a guess", () => {
    expect(render({ parentAuthority: "" })).not.toContain("co-quan-cap-tren");
  });

  // Presentation pin moved (lần 6 #8): spec 01's header has no tile; the sidebar's "VG" tile is the brand.
  it("no logo tile and no image in the shell, uploaded logo or not", () => {
    for (const logoUrl of ["", "https://media.example.test/vigov-public/t_01JXA/logo-512.png"]) {
      const html = render({ logoUrl });
      expect(html).not.toContain("commune-emblem");
      expect(html).not.toContain("<img");
    }
  });
});

describe("DauTrang — session not read yet", () => {
  it("draws no person, no avatar, no role pill — never a fallback name or a guessed role", () => {
    const html = render();
    expect(html).not.toContain("topbar-avatar");
    expect(html).not.toContain("ho-ten");
    expect(html).not.toContain("role-pill");
  });

  it("draws the sidebar frame with no screen link (every screen needs a key) — the menu never shows then withdraws", () => {
    const sidebar = sidebarOf(render());
    expect(sidebar).toContain('class="side-nav"');
    // Only the unbuilt placeholders: they open no data, so they need no key.
    expect(sidebar).not.toContain("<a ");
    expect(sidebar.match(/class="side-nav-item is-pending"/g)).toHaveLength(3);
    expect(sidebar).not.toContain("Cấu hình");
  });

  it("keeps the sign-out control — the way out needs no name", () => {
    expect(render()).toContain("khoi-dang-xuat");
  });

  it("keeps the narrow-screen menu button", () => {
    expect(render()).toContain('aria-label="Mở menu"');
  });
});

/**
 * ADR 0068 lần 6 #8: the commune's web-admin banner strip (ADR 0069 #5) is gone from the shell — the
 * prototype has none. A banner the commune uploaded must not reappear under the header.
 */
describe("DauTrang — no banner strip", () => {
  it("draws no banner, with or without one configured", () => {
    for (const webAdminBannerUrl of ["", "https://media.example.test/vigov-public/t_01JXA/banner-1600.jpg"]) {
      for (const navigation of [true, false]) {
        const html = render({ webAdminBannerUrl }, navigation);
        expect(html).not.toContain("banner");
        expect(html).not.toContain("<img");
      }
    }
  });
});

/**
 * Navigation lives in ONE place, the left sidebar. Spec 01: the sidebar runs the full height beside the
 * header, so it comes FIRST (prototype DOM order) and the shell grid places it (`.khung-trang`).
 */
describe("DauTrang — navigation lives in the left sidebar, not in the header", () => {
  it("the header holds no module navigation", () => {
    const header = headerOf(render());
    expect(header).not.toContain("<nav");
    expect(header).not.toContain("header-modules");
    expect(header).not.toContain("side-nav");
  });

  it("the sidebar comes before the header, outside it", () => {
    const html = render();
    expect(html.indexOf('<aside class="side-nav"')).toBe(0);
    expect(html.indexOf("</aside>")).toBeLessThan(html.indexOf("<header"));
  });

  it("the header is spec 01's: white, 64px, sticky, hairline, the blurred 95% white", () => {
    const tag = /<header class="([^"]*)"/.exec(render())![1]!.split(" ");
    for (const c of ["dau-trang", "sticky", "top-0", "z-30", "h-16", "border-b", "border-line", "bg-white/95", "backdrop-blur", "md:px-7"])
      expect(tag, c).toContain(c);
  });

  it("`navigation={false}`: no sidebar at all", () => {
    expect(render({}, false)).not.toContain("side-nav");
  });
});

describe("userInitials", () => {
  it("takes the first letters of the last two words", () => {
    expect(userInitials("Nguyễn Văn An")).toBe("VA");
    expect(userInitials("Trần Bình")).toBe("TB");
    expect(userInitials("  lê   thị   đào ")).toBe("TĐ");
  });

  it("one word gives one letter; a blank name gives nothing to draw", () => {
    expect(userInitials("Admin")).toBe("A");
    expect(userInitials("   ")).toBe("");
  });
});
