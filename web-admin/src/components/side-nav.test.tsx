import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { PENDING_HOVER_TEXT } from "./ui/pending-feature";
import { MENU_ICONS } from "./menu-icons";
import { flattenMenu, isMenuParent, locMenu, NHOM_MENU, PENDING_SCREENS, type MucMenu, type NhomMenu } from "./muc-menu";
import {
  APP_VERSION_LABEL,
  PRODUCT_MARK,
  PRODUCT_NAME,
  PRODUCT_TAGLINE,
  SIDE_NAV_LABEL,
  SideNav,
  SIDEBAR_COLLAPSE_LABEL,
  SIDEBAR_EXPAND_LABEL,
} from "./side-nav";

/**
 * The left sidebar (owner, 05/10/2026: module navigation back on the left, vertical, icon + word; ADR 0068
 * lần 5: the prototype's two groups, order, labels and one level of children). WHICH items a session gets
 * is `muc-menu.test.ts`'s job; these tests pin that the sidebar draws exactly what `locMenu` returned,
 * that an item is never nameless (collapsed included), and that the current page is marked for a screen
 * reader, not only by a tint. The collapsed flyout's children are in `side-nav-flyout.test.tsx` (jsdom).
 */
const ALL_PERMISSIONS = flattenMenu(NHOM_MENU).flatMap((m) =>
  m.khoa === null ? [] : typeof m.khoa === "string" ? [m.khoa] : [...m.khoa],
);
const ALL_ITEMS = flattenMenu(NHOM_MENU);
const BUILT = ALL_ITEMS.filter((m) => m.duong !== null);
/** Every row label, parents included — each needs an icon. */
const ALL_ROWS = NHOM_MENU.flatMap((g) => g.muc.flatMap((m) => (isMenuParent(m) ? [m.nhan, ...m.children.map((c) => c.nhan)] : [m.nhan])));
const PARENT = "Người dùng & Phân quyền";

/** React escapes `&` in attributes and text; "Văn bản & Đơn thư" carries one. */
const esc = (s: string) => s.replace(/&/g, "&amp;");

function sidebar(permissions: readonly string[] | null, { pathname = "/mini-app", collapsed = false } = {}) {
  return renderToStaticMarkup(
    <SideNav groups={locMenu(NHOM_MENU, permissions)} pathname={pathname} collapsed={collapsed} onToggle={() => {}} />,
  );
}

/** Every `<a …>…</a>` in the markup. */
const links = (html: string) => html.match(/<a [^>]*>.*?<\/a>/g) ?? [];
const hrefs = (html: string) => links(html).map((a) => /href="([^"]*)"/.exec(a)?.[1]);
const linkTo = (html: string, href: string) => links(html).find((a) => a.includes(`href="${href}"`)) ?? "";
/** The `<ul>` of a parent's children, by its accessible name. */
const childList = (html: string) => new RegExp(`<ul class="side-nav-children" aria-label="${esc(PARENT)}">(.*?)</ul>`).exec(html)?.[1] ?? null;

describe("menu icons", () => {
  it("every row of NHOM_MENU, parent and children included, has its own icon — none falls back silently", () => {
    for (const label of ALL_ROWS) expect(MENU_ICONS[label], label).toBeDefined();
  });
});

describe("sidebar — what it draws (expanded)", () => {
  it("every permission: the prototype's order; the parent row leads to its first child, then its children", () => {
    const html = sidebar(ALL_PERMISSIONS);
    expect(hrefs(html)).toEqual([
      "/tong-quan",
      "/nhiem-vu",
      "/nhiem-vu/so-tay",
      "/nhiem-vu/bien-ban",
      "/van-ban",
      "/giai-ngan",
      "/giai-ngan/thu-chi",
      "/thong-bao",
      "/phan-anh",
      "/ban-do",
      "/mini-app",
      "/bao-cao",
      "/nguoi-dung", // the parent row
      "/nguoi-dung",
      "/nguoi-dung/phan-quyen",
      "/cau-hinh",
    ]);
    for (const m of BUILT) {
      const a = links(html).find((x) => x.includes(`href="${m.duong!}"`) && x.includes(`<span class="side-nav-label">${esc(m.nhan)}</span>`));
      expect(a, m.nhan).toBeDefined();
      expect(a, m.nhan).toContain("<svg");
    }
    expect(html).toContain(`<nav class="side-nav-nav" aria-label="${SIDE_NAV_LABEL}">`);
  });

  it("the prototype's two group headings, in order", () => {
    const html = sidebar(ALL_PERMISSIONS);
    const headings = [...html.matchAll(/<p class="side-nav-group-label">([^<]*)<\/p>/g)].map((m) => m[1]);
    expect(headings).toEqual(["Điều hành", "Quản trị"]);
  });

  it("children sit INDENTED in their own list under the parent row, labelled by the parent", () => {
    const html = sidebar(ALL_PERMISSIONS);
    expect(html).toContain(`<span class="side-nav-label">${esc(PARENT)}</span>`);
    const list = childList(html);
    expect(list).not.toBeNull();
    expect(hrefs(list!)).toEqual(["/nguoi-dung", "/nguoi-dung/phan-quyen"]);
    expect(list).toContain('class="side-nav-item is-child"');
  });

  it("only `admin.user`: the parent with Người dùng only; only `admin.role`: the parent leads to Phân quyền", () => {
    const user = sidebar(["admin.user"]);
    // `admin.user` also opens "Nội dung Mini App": its Danh bạ cán bộ tab is gated by that key.
    expect(hrefs(user)).toEqual(["/mini-app", "/nguoi-dung", "/nguoi-dung"]);
    expect(hrefs(childList(user)!)).toEqual(["/nguoi-dung"]);
    const role = sidebar(["admin.role"]);
    expect(hrefs(role)).toEqual(["/nguoi-dung/phan-quyen", "/nguoi-dung/phan-quyen"]);
    expect(hrefs(childList(role)!)).toEqual(["/nguoi-dung/phan-quyen"]);
  });

  it("DENIED: neither child key → no parent row and no children list", () => {
    for (const p of [["report.read"], ["admin.user.delete", "admin.sla"], [], null]) {
      const html = sidebar(p);
      expect(html).not.toContain(esc(PARENT));
      expect(html).not.toContain("side-nav-children");
    }
  });

  it("only `content.read`: exactly Nội dung Mini App, at /mini-app", () => {
    expect(hrefs(sidebar(["content.read"]))).toEqual(["/mini-app"]);
  });

  it("only `report.read`: exactly Tổng quan and Báo cáo", () => {
    expect(hrefs(sidebar(["report.read"]))).toEqual(["/tong-quan", "/bao-cao"]);
  });

  it("only `task.read`: the three task screens, nothing else, in the prototype's order", () => {
    expect(hrefs(sidebar(["task.read"]))).toEqual(["/nhiem-vu", "/nhiem-vu/so-tay", "/nhiem-vu/bien-ban"]);
  });

  it("`admin.sla` alone opens Cấu hình (one key of the tab set)", () => {
    expect(hrefs(sidebar(["admin.sla"]))).toEqual(["/cau-hinh"]);
  });

  it("DENIED: no key, or the session not read yet → no link at all; only the unbuilt items, disabled; the toggle stays", () => {
    for (const p of [[], null, ["admin.user.delete", "report.export"]]) {
      const html = sidebar(p);
      expect(html).not.toContain("<a ");
      expect(html.match(/aria-disabled="true"/g)).toHaveLength(3);
      expect(html).toContain('class="side-nav"');
      expect(html).toContain(`aria-label="${SIDEBAR_COLLAPSE_LABEL}"`);
    }
  });
});

describe("sidebar — the current page", () => {
  it("marks the current item with aria-current=\"page\" and the active class, once", () => {
    const html = sidebar(ALL_PERMISSIONS, { pathname: "/mini-app" });
    expect(html.match(/aria-current="page"/g)).toHaveLength(1);
    const a = linkTo(html, "/mini-app");
    expect(a).toContain('aria-current="page"');
    expect(a).toContain('class="side-nav-link is-active"');
  });

  it("on /giai-ngan/thu-chi ONLY Thu - Chi is lit, not Giải ngân too (customer bug sheet row 46)", () => {
    const html = sidebar(ALL_PERMISSIONS, { pathname: "/giai-ngan/thu-chi" });
    expect(html.match(/aria-current="page"/g)).toHaveLength(1);
    expect(linkTo(html, "/giai-ngan")).not.toContain("aria-current");
    expect(linkTo(html, "/giai-ngan/thu-chi")).toContain('aria-current="page"');
  });

  it("on /nhiem-vu/so-tay ONLY Sổ tay is lit, not Nhiệm vụ too", () => {
    const html = sidebar(ALL_PERMISSIONS, { pathname: "/nhiem-vu/so-tay" });
    expect(html.match(/aria-current="page"/g)).toHaveLength(1);
    expect(linkTo(html, "/nhiem-vu")).not.toContain("aria-current");
    expect(linkTo(html, "/nhiem-vu/so-tay")).toContain('aria-current="page"');
  });

  it("a page with no item of its own still lights its section by segment", () => {
    const html = sidebar(ALL_PERMISSIONS, { pathname: "/giai-ngan/du-an/abc" });
    expect(html.match(/aria-current="page"/g)).toHaveLength(1);
    expect(linkTo(html, "/giai-ngan")).toContain('aria-current="page"');
  });

  it("on a child: ONLY that child is current (most specific match); the parent row is marked open, not current", () => {
    const html = sidebar(ALL_PERMISSIONS, { pathname: "/nguoi-dung/phan-quyen" });
    expect(html.match(/aria-current="page"/g)).toHaveLength(1);
    expect(linkTo(html, "/nguoi-dung/phan-quyen")).toContain('aria-current="page"');
    expect(html).toContain('class="side-nav-link side-nav-parent is-open"');
    expect(hrefs(childList(html)!)).toEqual(["/nguoi-dung", "/nguoi-dung/phan-quyen"]);
  });

  it("on /nguoi-dung: the child Người dùng is current; the parent row (same href) is not", () => {
    const html = sidebar(ALL_PERMISSIONS, { pathname: "/nguoi-dung" });
    expect(html.match(/aria-current="page"/g)).toHaveLength(1);
    expect(childList(html)).toContain('aria-current="page"');
    const row = links(html).find((a) => a.includes("side-nav-parent"));
    expect(row).toContain('class="side-nav-link side-nav-parent is-open"');
    expect(row).toContain('href="/nguoi-dung"');
    expect(row).not.toContain("aria-current");
  });

  it("outside the branch: the parent row is neither open nor current", () => {
    const html = sidebar(ALL_PERMISSIONS, { pathname: "/cau-hinh" });
    expect(html).not.toContain("is-open");
  });

  it("no item matches the path → nothing marked", () => {
    expect(sidebar(ALL_PERMISSIONS, { pathname: "/doi-mat-khau" })).not.toContain("aria-current");
  });

  it("collapsed keeps the current mark", () => {
    const html = sidebar(ALL_PERMISSIONS, { pathname: "/phan-anh", collapsed: true });
    expect(linkTo(html, "/phan-anh")).toContain('aria-current="page"');
  });
});

describe("sidebar — collapsed to icons", () => {
  it("the toggle names what it will do and states the expanded state", () => {
    const open = sidebar(ALL_PERMISSIONS);
    expect(open).toMatch(new RegExp(`<button type="button" class="side-nav-toggle" aria-label="${SIDEBAR_COLLAPSE_LABEL}" aria-expanded="true"`));
    expect(open).toContain('class="side-nav"');
    const shut = sidebar(ALL_PERMISSIONS, { collapsed: true });
    expect(shut).toMatch(new RegExp(`<button type="button" class="side-nav-toggle" aria-label="${SIDEBAR_EXPAND_LABEL}" aria-expanded="false"`));
    expect(shut).toContain('class="side-nav is-collapsed"');
  });

  it("no visible word, but every link keeps its name in visually hidden text (the tooltip only repeats it)", () => {
    const html = sidebar(ALL_PERMISSIONS, { collapsed: true });
    expect(html).not.toContain("side-nav-label");
    const outsideParent = NHOM_MENU.flatMap((g) => g.muc).filter((m): m is MucMenu => !isMenuParent(m) && m.duong !== null);
    for (const m of outsideParent) expect(linkTo(html, m.duong!), m.nhan).toContain(`<span class="an-thi-giac">${esc(m.nhan)}</span>`);
  });

  it("the parent becomes ONE named button opening its flyout; its children are not drawn in the strip", () => {
    const html = sidebar(ALL_PERMISSIONS, { collapsed: true });
    expect(html).toMatch(new RegExp(`<button type="button" class="side-nav-link side-nav-parent"[^>]*aria-haspopup="dialog"[^>]*>.*?<span class="an-thi-giac">${esc(PARENT)}</span></button>`));
    expect(html).not.toContain("side-nav-children");
    expect(hrefs(html)).not.toContain("/nguoi-dung/phan-quyen");
  });

  it("the parent button is lit when one of its children is the current page", () => {
    expect(sidebar(ALL_PERMISSIONS, { collapsed: true, pathname: "/nguoi-dung/phan-quyen" })).toContain(
      'class="side-nav-link side-nav-parent is-active"',
    );
    expect(sidebar(ALL_PERMISSIONS, { collapsed: true, pathname: "/cau-hinh" })).not.toContain("side-nav-parent is-active");
  });

  // Presentation pin moved 07/10/2026 (ADR 0068 lần 6, spec 01 / prototype `AppSidebar.tsx`): one divider
  // per group, the first included — it replaces the heading the group had.
  it("group headings become one divider per group", () => {
    const html = sidebar(ALL_PERMISSIONS, { collapsed: true });
    expect(html).not.toContain("side-nav-group-label");
    expect(html.match(/class="side-nav-divider"/g)).toHaveLength(NHOM_MENU.length);
  });

  it("same items either way — collapsing changes the look, never who sees what (children: the flyout test)", () => {
    const notInBranch = (h: (string | undefined)[]) => h.filter((x) => x === undefined || !x.startsWith("/nguoi-dung"));
    for (const p of [ALL_PERMISSIONS, ["task.read"], ["report.read"], []]) {
      expect(hrefs(sidebar(p, { collapsed: true }))).toEqual(notInBranch(hrefs(sidebar(p))));
    }
  });
});

/**
 * ADR 0068 lần 6 #11 (owner did not object, 07/10/2026): the unbuilt items are muted, not clickable,
 * hover "Tính năng đang phát triển" — and carry NO "?" any more (the "?" stays on unbuilt controls
 * inside a screen, §14). Their word stays, so a screen reader still hears the whole menu.
 */
describe("items with no screen", () => {
  const PENDING = ["Danh bạ người dân", "Gửi tin ZNS / SMS", "Hướng dẫn sử dụng"];

  it("the prototype's three unbuilt items: disabled, their word, the hover sentence, no '?' — expanded and collapsed", () => {
    for (const collapsed of [false, true]) {
      const html = sidebar(ALL_PERMISSIONS, { collapsed });
      expect(html).not.toContain("data-pending-marker");
      expect(html.match(/class="side-nav-item is-pending"/g)).toHaveLength(3);
      for (const nhan of PENDING) {
        expect(html, nhan).toContain(collapsed ? `<span class="an-thi-giac">${nhan}</span>` : `<span class="side-nav-label">${nhan}</span>`);
        const title = collapsed ? `${nhan} — ${PENDING_HOVER_TEXT}` : PENDING_HOVER_TEXT;
        expect(html, nhan).toContain(`aria-disabled="true" class="side-nav-link" title="${title}"`);
      }
    }
  });

  it("never say 'Sắp có' — a public authority does not promise a date nobody set", () => {
    expect(sidebar(ALL_PERMISSIONS)).not.toContain("Sắp có");
    for (const info of Object.values(PENDING_SCREENS)) expect(info.viSao).not.toContain("Sắp có");
  });

  it("an item listed with `duong: null` is DISABLED: no link, aria-disabled, its word, not focusable — both widths", () => {
    const item: MucMenu = { nhan: "Báo cáo", duong: null, khoa: null };
    const groups: NhomMenu[] = [{ ten: "Quản trị", muc: [item] }];
    for (const collapsed of [false, true]) {
      const html = renderToStaticMarkup(<SideNav groups={groups} pathname="/mini-app" collapsed={collapsed} onToggle={() => {}} />);
      expect(html).not.toContain("<a ");
      expect(html).toContain('aria-disabled="true"');
      expect(html).toContain(collapsed ? '<span class="an-thi-giac">Báo cáo</span>' : '<span class="side-nav-label">Báo cáo</span>');
      // The only focusable thing left is the collapse toggle.
      expect(html.match(/<button/g)).toHaveLength(1);
      expect(html).not.toContain("tabindex");
    }
  });
});

/**
 * Spec 01 / ADR 0068 lần 6 #3, #8: the brand block "VG · ViGov · Điều hành số cấp xã" on top, the
 * version footer at the bottom ("Môi trường phát triển" dropped). Collapsed: the tile alone, the toggle
 * under it, no footer — the prototype's `AppSidebar.tsx`.
 */
describe("sidebar — brand block and footer", () => {
  it("expanded: tile, product name, tagline, the toggle at the row's end; the version footer — and no environment line", () => {
    const html = sidebar(ALL_PERMISSIONS);
    expect(html).toMatch(
      new RegExp(
        `^<aside class="side-nav"><div class="side-nav-brand"><span class="side-nav-brand-mark" aria-hidden="true">${PRODUCT_MARK}</span>` +
          `<span class="side-nav-brand-text"><span class="side-nav-brand-name">${PRODUCT_NAME}</span>` +
          `<span class="side-nav-brand-tagline">${PRODUCT_TAGLINE}</span></span><button type="button" class="side-nav-toggle"`,
      ),
    );
    expect(html).toMatch(new RegExp(`<p class="side-nav-footer">${APP_VERSION_LABEL}</p></aside>$`));
    expect(APP_VERSION_LABEL).toBe("Phiên bản 0.1.0");
    expect(html).not.toContain("Môi trường");
  });

  it("collapsed: the tile alone in the brand row, the toggle right after it, no footer", () => {
    const html = sidebar(ALL_PERMISSIONS, { collapsed: true });
    expect(html).toContain(
      `<div class="side-nav-brand is-collapsed"><span class="side-nav-brand-mark" aria-hidden="true">${PRODUCT_MARK}</span></div><button type="button" class="side-nav-toggle"`,
    );
    expect(html).not.toContain(PRODUCT_NAME);
    expect(html).not.toContain("side-nav-footer");
  });
});
