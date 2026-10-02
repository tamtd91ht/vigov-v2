import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { PhienProvider } from "@/features/phien/phien-hien-tai";

import { CauHinhXaProvider } from "./cau-hinh-xa";
import { AUTHORITY_KIND, DauTrang } from "./dau-trang";
import { userInitials } from "./user-initials";

/**
 * The topbar's commune block (owner decision 02/10/2026): a small "Ủy ban nhân dân" line, then
 * `displayName` VERBATIM, then `parentAuthority`. The failure this guards is silent and public: a
 * string-built "UBND " + name reads right for a commune configured as "xã Tân Phú" and wrong —
 * "UBND UBND xã Tân Phú" — for one configured with the prefix already in its name.
 */
function render(displayName: string, parentAuthority: string) {
  return renderToStaticMarkup(
    <CauHinhXaProvider giaTri={{ displayName, parentAuthority }}>
      <PhienProvider>
        <DauTrang />
      </PhienProvider>
    </CauHinhXaProvider>,
  );
}

describe("DauTrang — commune block", () => {
  it("prints displayName exactly as configured, with no prefix built in code", () => {
    const html = render("UBND xã Tân Phú", "Tỉnh Đồng Nai");
    expect(html).toContain('<p class="ten-co-quan">UBND xã Tân Phú</p>');
    expect(html).not.toContain("UBND UBND");
    expect(html).toContain('<p class="co-quan-cap-tren">Tỉnh Đồng Nai</p>');
  });

  it("shows the authority-kind line above the name, and the Landmark icon", () => {
    const html = render("Xã Tân Phú", "Tỉnh Đồng Nai");
    expect(html.indexOf(AUTHORITY_KIND)).toBeLessThan(html.indexOf("Xã Tân Phú"));
    expect(html).toContain("lucide-landmark");
  });

  it("while the session is unread, draws no person and no avatar — never a fallback name", () => {
    const html = render("Xã Tân Phú", "Tỉnh Đồng Nai");
    expect(html).not.toContain("topbar-avatar");
    expect(html).not.toContain("ho-ten");
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
