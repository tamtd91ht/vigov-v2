import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { PhienProvider } from "@/features/phien/phien-hien-tai";

import { CauHinhXaProvider } from "./cau-hinh-xa";
import { AUTHORITY_KIND } from "./commune-identity";
import { DauTrang } from "./dau-trang";
import { userInitials } from "./user-initials";

/**
 * The navy header carries the commune's identity on EVERY signed-in page since it replaced the text
 * sidebar (ADR 0068 §Sửa đổi 05/10/2026 (lần 2)): one place, so no page is without the body's name and
 * none prints it twice. The failure guarded is silent and public: a string-built "UBND " + name reads
 * right for "xã Tân Phú" and wrong — "UBND UBND xã Tân Phú" — for a commune that declared the prefix.
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

describe("DauTrang — commune identity", () => {
  it("prints the authority kind, then displayName verbatim, then the province — once, with and without menu", () => {
    for (const navigation of [true, false]) {
      const html = render({ displayName: "UBND xã Tân Phú" }, navigation);
      expect(html.indexOf(AUTHORITY_KIND)).toBeLessThan(html.indexOf("UBND xã Tân Phú"));
      expect(html.match(/<p class="ten-co-quan">UBND xã Tân Phú<\/p>/g)).toHaveLength(1);
      expect(html).not.toContain("UBND UBND");
      expect(html).toContain('<p class="co-quan-cap-tren">Tỉnh Đồng Nai</p>');
      expect(html.indexOf("commune-identity")).toBeLessThan(html.indexOf("header-end"));
    }
  });

  it("names no product and no vendor (ADR 0068 §13, §Sửa đổi lần 2 #9)", () => {
    const html = render();
    for (const s of ["ViGov", "ViHAT", "OMICALL", "Omicall", "Điều hành số cấp xã"]) expect(html).not.toContain(s);
  });

  it("a commune that declared no parent authority gets no line — never a guess", () => {
    expect(render({ parentAuthority: "" })).not.toContain("co-quan-cap-tren");
  });

  it("no logo uploaded: the building icon in the tile, no image (ADR 0069 #7)", () => {
    const html = render();
    expect(html).toContain('<span class="commune-emblem" aria-hidden="true">');
    expect(html).toContain("lucide-landmark");
    expect(html).not.toContain("<img");
  });

  it("logo uploaded: the commune's own image replaces the icon, contained, decorative beside the name", () => {
    const url = "https://media.example.test/vigov-public/t_01JXA/logo-512.png";
    const html = render({ logoUrl: url });
    expect(html).toContain('<span class="commune-emblem has-logo" aria-hidden="true">');
    expect(html.match(/<img /g)).toHaveLength(1);
    expect(html).toContain(`src="${url}"`);
    expect(html).toContain('alt=""');
    expect(html).not.toContain("lucide-landmark");
  });
});

describe("DauTrang — session not read yet", () => {
  it("draws no person, no avatar, no role pill — never a fallback name or a guessed role", () => {
    const html = render();
    expect(html).not.toContain("topbar-avatar");
    expect(html).not.toContain("ho-ten");
    expect(html).not.toContain("role-pill");
  });

  it("draws the sidebar frame with no item (every item needs a key) — the menu never shows then withdraws", () => {
    const html = render();
    expect(html).toContain('class="side-nav"');
    expect(html).not.toContain('<nav class="side-nav-nav"');
    expect(html).not.toContain("Cấu hình");
  });

  it("keeps the sign-out control — the way out needs no name", () => {
    expect(render()).toContain("khoi-dang-xuat");
  });

  it("keeps the narrow-screen menu button", () => {
    expect(render()).toContain('aria-label="Mở menu"');
  });
});

/**
 * ADR 0069 #5: the commune's web-admin banner is a strip under the header on EVERY signed-in page —
 * drawn here, once, so no page can forget it. No banner = no strip at all (ADR 0069 #7).
 */
describe("DauTrang — banner strip", () => {
  const URL_BANNER = "https://media.example.test/vigov-public/t_01JXA/banner-1600.jpg";

  it("no banner: no strip, no image, no empty frame", () => {
    for (const navigation of [true, false]) {
      const html = render({}, navigation);
      expect(html).not.toContain("commune-banner");
      expect(html).not.toContain("<img");
    }
  });

  it("a banner: one decorative image AFTER the header (sibling, so the sticky header does not carry it)", () => {
    const html = render({ webAdminBannerUrl: URL_BANNER });
    expect(html.match(/<img /g)).toHaveLength(1);
    expect(html).toContain(`src="${URL_BANNER}"`);
    expect(html.indexOf('<div class="commune-banner">')).toBeGreaterThan(html.indexOf("</header>"));
  });
});

/**
 * Owner, 05/10/2026: module navigation lives in ONE place, the left sidebar — the header's horizontal
 * icon row is gone. The sidebar is a SIBLING after the header and banner (the shell grid places it).
 */
describe("DauTrang — navigation lives in the left sidebar, not in the header", () => {
  it("the header holds no module navigation", () => {
    const html = render();
    const header = html.slice(html.indexOf("<header"), html.indexOf("</header>"));
    expect(header).not.toContain("<nav");
    expect(header).not.toContain("header-modules");
    expect(header).not.toContain("side-nav");
  });

  it("the sidebar follows the header (and the banner), outside it", () => {
    const html = render({ webAdminBannerUrl: "https://media.example.test/vigov-public/t_01JXA/banner-1600.jpg" });
    expect(html.indexOf('class="side-nav"')).toBeGreaterThan(html.indexOf("</header>"));
    expect(html.indexOf('class="side-nav"')).toBeGreaterThan(html.indexOf('<div class="commune-banner">'));
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
