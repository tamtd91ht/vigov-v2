import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { PhienProvider } from "@/features/phien/phien-hien-tai";

import { CauHinhXaProvider } from "./cau-hinh-xa";
import { AUTHORITY_KIND } from "./commune-identity";
import { DauTrang } from "./dau-trang";
import { userInitials } from "./user-initials";

/**
 * The commune block moved to the sidebar (owner decision 02/10/2026). The topbar prints it again
 * ONLY on pages with no sidebar (`withCommune`), so no signed-in page is without the body's name —
 * and no page prints it twice. Verbatim-name rules are pinned in `sidebar-view.test.tsx`.
 */
function render(displayName: string, parentAuthority: string, withCommune = false) {
  return renderToStaticMarkup(
    <CauHinhXaProvider giaTri={{ displayName, parentAuthority }}>
      <PhienProvider>
        <DauTrang withCommune={withCommune} />
      </PhienProvider>
    </CauHinhXaProvider>,
  );
}

describe("DauTrang — commune block", () => {
  it("next to a sidebar: does not repeat the commune name", () => {
    const html = render("Xã Tân Phú", "Tỉnh Đồng Nai");
    expect(html).not.toContain("Xã Tân Phú");
    expect(html).not.toContain(AUTHORITY_KIND);
  });

  it("on a page with no sidebar: prints the authority kind, then displayName verbatim, then the province", () => {
    const html = render("UBND xã Tân Phú", "Tỉnh Đồng Nai", true);
    expect(html.indexOf(AUTHORITY_KIND)).toBeLessThan(html.indexOf("UBND xã Tân Phú"));
    expect(html).toContain('<p class="ten-co-quan">UBND xã Tân Phú</p>');
    expect(html).not.toContain("UBND UBND");
    expect(html).toContain('<p class="co-quan-cap-tren">Tỉnh Đồng Nai</p>');
    expect(html).toContain("lucide-landmark");
  });

  it("while the session is unread, draws no person and no avatar — never a fallback name", () => {
    const html = render("Xã Tân Phú", "Tỉnh Đồng Nai");
    expect(html).not.toContain("topbar-avatar");
    expect(html).not.toContain("ho-ten");
  });

  it("while the session is unread, draws no role pill — never a guessed role", () => {
    expect(render("Xã Tân Phú", "Tỉnh Đồng Nai")).not.toContain("role-pill");
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
