import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { pendingMarkerLabel } from "./ui/pending-feature";
import { MENU_ICONS } from "./menu-icons";
import { locMenu, NHOM_MENU, PENDING_SCREENS, type MucMenu, type NhomMenu } from "./muc-menu";
import { SIDE_NAV_LABEL, SideNav, SIDEBAR_COLLAPSE_LABEL, SIDEBAR_EXPAND_LABEL } from "./side-nav";

/**
 * The left sidebar (owner, 05/10/2026: module navigation back on the left, vertical, icon + word).
 * WHICH items a session gets is `muc-menu.test.ts`'s job; these tests pin that the sidebar draws exactly
 * what `locMenu` returned, that an item is never nameless (collapsed included), and that the current page
 * is marked for a screen reader, not only by a tint.
 */
const ALL_PERMISSIONS = NHOM_MENU.flatMap((n) =>
  n.muc.flatMap((m) => (m.khoa === null ? [] : typeof m.khoa === "string" ? [m.khoa] : [...m.khoa])),
);
const ALL_ITEMS = NHOM_MENU.flatMap((g) => g.muc);

/** React escapes `&` in attributes and text; "Văn bản & Đơn thư" carries one. */
const esc = (s: string) => s.replace(/&/g, "&amp;");

function sidebar(permissions: readonly string[] | null, { pathname = "/danh-ba", collapsed = false } = {}) {
  return renderToStaticMarkup(
    <SideNav groups={locMenu(NHOM_MENU, permissions)} pathname={pathname} collapsed={collapsed} onToggle={() => {}} />,
  );
}

/** Every `<a …>…</a>` in the markup. */
const links = (html: string) => html.match(/<a [^>]*>.*?<\/a>/g) ?? [];
const hrefs = (html: string) => links(html).map((a) => /href="([^"]*)"/.exec(a)?.[1]);
const linkTo = (html: string, href: string) => links(html).find((a) => a.includes(`href="${href}"`)) ?? "";

describe("menu icons", () => {
  it("every item of NHOM_MENU has its own icon — a new item cannot fall back silently", () => {
    for (const m of ALL_ITEMS) expect(MENU_ICONS[m.nhan], m.nhan).toBeDefined();
  });
});

describe("sidebar — what it draws (expanded)", () => {
  it("every permission: one link per item, in NHOM_MENU order, Cấu hình included, each with icon + visible word", () => {
    const html = sidebar(ALL_PERMISSIONS);
    expect(hrefs(html)).toEqual(ALL_ITEMS.map((m) => m.duong));
    for (const m of ALL_ITEMS) {
      const a = linkTo(html, m.duong!);
      expect(a, m.nhan).toContain(`<span class="side-nav-label">${esc(m.nhan)}</span>`);
      expect(a, m.nhan).toContain("<svg");
    }
    expect(html).toContain(`<nav class="side-nav-nav" aria-label="${SIDE_NAV_LABEL}">`);
  });

  it("group headings of NHOM_MENU, visible; the untitled Tổng quan group has none", () => {
    const html = sidebar(ALL_PERMISSIONS);
    const headings = [...html.matchAll(/<p class="side-nav-group-label">([^<]*)<\/p>/g)].map((m) => m[1]);
    expect(headings).toEqual(NHOM_MENU.filter((g) => g.ten !== "").map((g) => g.ten));
    expect(headings).toEqual(["CÔNG VIỆC", "TÀI CHÍNH", "NGƯỜI DÂN", "HỆ THỐNG"]);
  });

  it("HỆ THỐNG carries Người dùng and Phân quyền, each its own link and key", () => {
    expect(hrefs(sidebar(["admin.user"]))).toEqual(["/danh-ba", "/nguoi-dung"]);
    expect(hrefs(sidebar(["admin.role"]))).toEqual(["/nguoi-dung/phan-quyen"]);
  });

  it("only `report.read`: exactly Tổng quan and Báo cáo", () => {
    expect(hrefs(sidebar(["report.read"]))).toEqual(["/tong-quan", "/bao-cao"]);
  });

  it("only `task.read`: the three task screens, nothing else", () => {
    expect(hrefs(sidebar(["task.read"]))).toEqual(["/nhiem-vu", "/nhiem-vu/bien-ban", "/nhiem-vu/so-tay"]);
  });

  it("`admin.sla` alone opens Cấu hình (one key of the tab set)", () => {
    expect(hrefs(sidebar(["admin.sla"]))).toEqual(["/cau-hinh"]);
  });

  it("DENIED: no key, or the session not read yet → no link and no nav landmark; the frame and its toggle stay", () => {
    for (const p of [[], null, ["admin.user.delete", "report.export"]]) {
      const html = sidebar(p);
      expect(html).not.toContain("<a ");
      expect(html).not.toContain("<nav");
      expect(html).toContain('class="side-nav"');
      expect(html).toContain(`aria-label="${SIDEBAR_COLLAPSE_LABEL}"`);
    }
  });
});

describe("sidebar — the current page", () => {
  it("marks the current item with aria-current=\"page\" and the active class, once", () => {
    const html = sidebar(ALL_PERMISSIONS, { pathname: "/danh-ba" });
    expect(html.match(/aria-current="page"/g)).toHaveLength(1);
    const a = linkTo(html, "/danh-ba");
    expect(a).toContain('aria-current="page"');
    expect(a).toContain('class="side-nav-link is-active"');
  });

  it("a parent route stays lit on its child route — the rule of `dangChon`, unchanged", () => {
    const html = sidebar(ALL_PERMISSIONS, { pathname: "/giai-ngan/thu-chi" });
    expect(linkTo(html, "/giai-ngan")).toContain('aria-current="page"');
    expect(linkTo(html, "/giai-ngan/thu-chi")).toContain('aria-current="page"');
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
    for (const m of ALL_ITEMS) expect(linkTo(html, m.duong!), m.nhan).toContain(`<span class="an-thi-giac">${esc(m.nhan)}</span>`);
  });

  it("group headings become dividers between groups (none above the first)", () => {
    const html = sidebar(ALL_PERMISSIONS, { collapsed: true });
    expect(html).not.toContain("side-nav-group-label");
    expect(html.match(/class="side-nav-divider"/g)).toHaveLength(NHOM_MENU.length - 1);
  });

  it("same items either way — collapsing changes the look, never who sees what", () => {
    for (const p of [ALL_PERMISSIONS, ["task.read"], ["report.read"], []]) {
      expect(hrefs(sidebar(p, { collapsed: true }))).toEqual(hrefs(sidebar(p)));
    }
  });
});

describe("items with no screen (ADR 0068 §14)", () => {
  it("no item is unbuilt today, and PENDING_SCREENS keeps no stale entry", () => {
    expect(ALL_ITEMS.filter((m) => m.duong === null)).toEqual([]);
    expect(Object.keys(PENDING_SCREENS)).toEqual([]);
    expect(sidebar(ALL_PERMISSIONS)).not.toContain("data-pending-marker");
  });

  it("never say 'Sắp có' — a public authority does not promise a date nobody set", () => {
    expect(sidebar(ALL_PERMISSIONS)).not.toContain("Sắp có");
  });

  it("an item listed with `duong: null` is DISABLED: no link, aria-disabled, its word, '?' beside it — both widths", () => {
    const item: MucMenu = { nhan: "Báo cáo", duong: null, khoa: null };
    const groups: NhomMenu[] = [{ ten: "HỆ THỐNG", muc: [item] }];
    // The marker needs a description to open; the table is empty today, so give it one for the test.
    (PENDING_SCREENS as Record<string, { ten: string; viSao: string }>)["Báo cáo"] = { ten: "Báo cáo", viSao: "x" };
    try {
      for (const collapsed of [false, true]) {
        const html = renderToStaticMarkup(<SideNav groups={groups} pathname="/danh-ba" collapsed={collapsed} onToggle={() => {}} />);
        expect(html).not.toContain("<a ");
        expect(html).toContain('aria-disabled="true"');
        expect(html).toContain(collapsed ? '<span class="an-thi-giac">Báo cáo</span>' : '<span class="side-nav-label">Báo cáo</span>');
        // Focusable: the toggle, and the "?" whose name says the whole sentence.
        expect(html.match(/<button/g)).toHaveLength(2);
        expect(html).toContain(`aria-label="${pendingMarkerLabel("Báo cáo")}"`);
      }
    } finally {
      delete (PENDING_SCREENS as Record<string, unknown>)["Báo cáo"];
    }
  });
});
