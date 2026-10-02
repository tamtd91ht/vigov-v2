import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { MENU_ICONS } from "./menu-icons";
import { CHUA_CO_MAN, locMenu, NHOM_MENU } from "./muc-menu";
import { AUTHORITY_KIND } from "./commune-identity";
import { COLLAPSE_LABEL, EXPAND_LABEL, NOT_BUILT_BADGE, SidebarView } from "./sidebar-view";

/**
 * The sidebar's DRAWING, in both states. The collapsed state can only be seen here: the server
 * render (and so any page-level string test) always draws the expanded menu, because
 * `localStorage` does not exist there.
 *
 * Which items appear is `muc-menu.test.ts`'s job; these tests pass the full menu (every permission)
 * so that every item is drawn and every icon is exercised.
 */
const ALL_PERMISSIONS = NHOM_MENU.flatMap((n) => n.muc.flatMap((m) => (m.khoa === null ? [] : typeof m.khoa === "string" ? [m.khoa] : [...m.khoa])));
const GROUPS = locMenu(NHOM_MENU, ALL_PERMISSIONS);
const ITEM_COUNT = NHOM_MENU.reduce((n, g) => n + g.muc.length, 0);
const NOT_BUILT = NHOM_MENU.flatMap((g) => g.muc).filter((m) => m.duong === null);

/** React escapes `&` in text; menu labels such as "Văn bản & Đơn thư" carry one. */
const esc = (s: string) => s.replace(/&/g, "&amp;");

const COMMUNE = { displayName: "Xã Tân Phú", parentAuthority: "Tỉnh Đồng Nai" };

function view(collapsed: boolean, pathname = "/danh-ba", commune = COMMUNE) {
  return renderToStaticMarkup(
    <SidebarView commune={commune} groups={GROUPS} pathname={pathname} collapsed={collapsed} onToggleCollapsed={() => {}} />,
  );
}

/**
 * The sidebar's top is the commune's People's Committee, not the vendor's product (owner decision
 * 02/10/2026). The failure guarded is silent and public: a string-built "UBND " + name reads right
 * for "xã Tân Phú" and wrong — "UBND UBND xã Tân Phú" — for a commune that declared the prefix.
 */
describe("commune identity at the top", () => {
  it("prints displayName exactly as configured, under the authority kind, with no product name", () => {
    const html = view(false, "/danh-ba", { displayName: "UBND xã Tân Phú", parentAuthority: "Tỉnh Đồng Nai" });
    expect(html).toContain('<p class="ten-co-quan">UBND xã Tân Phú</p>');
    expect(html).not.toContain("UBND UBND");
    expect(html).toContain('<p class="co-quan-cap-tren">Tỉnh Đồng Nai</p>');
    expect(html.indexOf(AUTHORITY_KIND)).toBeLessThan(html.indexOf("UBND xã Tân Phú"));
    expect(html.indexOf("commune-identity")).toBeLessThan(html.indexOf("thanh-ben-muc"));
    expect(html).not.toContain("ViGov");
    expect(html).not.toContain("Điều hành số cấp xã");
  });

  it("a commune that declared no parent authority gets no line — never a guess", () => {
    const html = view(false, "/danh-ba", { displayName: "Xã Tân Phú", parentAuthority: "" });
    expect(html).not.toContain("co-quan-cap-tren");
  });

  it("collapsed: the emblem stays, the name stays in the DOM for screen readers", () => {
    const html = view(true);
    expect(html).toContain("commune-emblem");
    expect(html).toContain('<p class="ten-co-quan">Xã Tân Phú</p>');
  });
});

describe("groups by business area (spec v2 §5)", () => {
  it("Tổng quan alone with no heading, then CÔNG VIỆC · TÀI CHÍNH · NGƯỜI DÂN · HỆ THỐNG", () => {
    expect(NHOM_MENU.map((g) => g.ten)).toEqual(["", "CÔNG VIỆC", "TÀI CHÍNH", "NGƯỜI DÂN", "HỆ THỐNG"]);
    expect(NHOM_MENU[0]!.muc.map((m) => m.nhan)).toEqual(["Tổng quan"]);
    const html = view(false);
    // Four headings drawn, none of them empty.
    expect(html.match(/<p class="thanh-ben-nhan-nhom">/g)).toHaveLength(4);
    expect(html).not.toContain('<p class="thanh-ben-nhan-nhom"></p>');
    expect(html.indexOf("Tổng quan")).toBeLessThan(html.indexOf("CÔNG VIỆC"));
  });

  it("the internal announcement book sits under CÔNG VIỆC, not NGƯỜI DÂN", () => {
    const group = (label: string) => NHOM_MENU.find((g) => g.muc.some((m) => m.nhan === label))?.ten;
    expect(group("Thông báo")).toBe("CÔNG VIỆC");
    expect(group("Phản ánh người dân")).toBe("NGƯỜI DÂN");
  });
});

describe("menu icons", () => {
  it("every item of NHOM_MENU has its own icon — a new item cannot fall back silently", () => {
    for (const g of NHOM_MENU) for (const m of g.muc) expect(MENU_ICONS[m.nhan], m.nhan).toBeDefined();
  });

  it("the full menu is drawn: one icon per item, same count as NHOM_MENU", () => {
    const html = view(false);
    expect(GROUPS.reduce((n, g) => n + g.muc.length, 0)).toBe(ITEM_COUNT);
    // One icon per item, plus the commune emblem tile and the collapse control.
    expect(html.match(/<svg/g)).toHaveLength(ITEM_COUNT + 2);
    expect(html).toContain("lucide-layout-dashboard");
    expect(html).toContain("lucide-contact-round");
  });
});

describe("items with no screen", () => {
  it("carry the 'Chưa có' badge INSIDE the item and the full sentence as title", () => {
    const html = view(false);
    expect(html.match(new RegExp(`<span class="thanh-ben-dau-chua-co">${NOT_BUILT_BADGE}</span>`, "g"))).toHaveLength(
      NOT_BUILT.length,
    );
    expect(html.match(new RegExp(`title="${CHUA_CO_MAN}"`, "g"))).toHaveLength(NOT_BUILT.length);
  });

  it("never say 'Sắp có' — a public authority does not promise a date nobody set", () => {
    expect(view(false)).not.toContain("Sắp có");
    expect(view(true)).not.toContain("Sắp có");
  });

  it("stay non-clickable: no link, no button, aria-disabled", () => {
    const html = view(false);
    for (const m of NOT_BUILT) {
      const at = html.indexOf(`<span class="thanh-ben-nhan">${m.nhan}</span>`);
      const li = html.slice(html.lastIndexOf("<li", at), html.indexOf("</li>", at));
      expect(li, m.nhan).toContain('aria-disabled="true"');
      expect(li, m.nhan).not.toContain("<a ");
      expect(li, m.nhan).not.toContain("<button");
    }
  });
});

describe("active item", () => {
  it("marks the current link with aria-current and the legacy `dang-chon` class", () => {
    const html = view(false, "/danh-ba");
    expect(html.match(/aria-current="page"/g)).toHaveLength(1);
    expect(html).toContain('<a aria-current="page" class="dang-chon" href="/danh-ba">');
  });

  it("a parent route stays lit on its child route — the rule of `dangChon`, unchanged", () => {
    const html = view(false, "/giai-ngan/thu-chi");
    expect(html).toContain('<a aria-current="page" class="dang-chon" href="/giai-ngan">');
    expect(html).toContain('<a aria-current="page" class="dang-chon" href="/giai-ngan/thu-chi">');
  });
});

describe("collapse control", () => {
  it("expanded: sits at the bottom, names the action it will take", () => {
    const html = view(false);
    expect(html).toContain('<nav class="thanh-ben" aria-label="Điều hướng chính">');
    expect(html.indexOf("thanh-ben-chan")).toBeGreaterThan(html.lastIndexOf("thanh-ben-muc"));
    expect(html).toContain(`aria-expanded="true" aria-label="${COLLAPSE_LABEL}"`);
  });

  it("collapsed: the `thu-gon` class, the opposite action, labels still in the DOM", () => {
    const html = view(true);
    expect(html).toContain('<nav class="thanh-ben thu-gon" aria-label="Điều hướng chính">');
    expect(html).toContain(`aria-expanded="false" aria-label="${EXPAND_LABEL}"`);
    // Labels are hidden from the eye by CSS only — a screen reader and the keyboard keep them.
    for (const g of NHOM_MENU) for (const m of g.muc) expect(html).toContain(`<span class="thanh-ben-nhan">${esc(m.nhan)}</span>`);
  });

  it("collapsed: an item with no screen names itself in its tooltip, not only 'Chưa có màn hình'", () => {
    const html = view(true);
    for (const m of NOT_BUILT) expect(html).toContain(`title="${m.nhan} — ${CHUA_CO_MAN}"`);
  });
});
