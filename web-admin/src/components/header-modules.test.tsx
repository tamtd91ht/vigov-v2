import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { pendingMarkerLabel } from "./ui/pending-feature";
import { HeaderModules, SETTINGS_LABEL, SettingsButton, splitSettings } from "./header-modules";
import { MENU_ICONS } from "./menu-icons";
import { locMenu, NHOM_MENU, PENDING_SCREENS, type MucMenu, type NhomMenu } from "./muc-menu";

/**
 * The header's module row — the icon-only replacement of the text sidebar (ADR 0068 §Sửa đổi
 * 05/10/2026 (lần 2) #6). WHICH items a session gets is `muc-menu.test.ts`'s job; these tests pin
 * that the row draws exactly what `locMenu` returned, that an icon with no visible word still has a
 * name, and that the current page is marked for a screen reader, not only by a tint.
 */
const ALL_PERMISSIONS = NHOM_MENU.flatMap((n) =>
  n.muc.flatMap((m) => (m.khoa === null ? [] : typeof m.khoa === "string" ? [m.khoa] : [...m.khoa])),
);
const ALL_ITEMS = NHOM_MENU.flatMap((g) => g.muc);

/** React escapes `&` in attributes and text; "Văn bản & Đơn thư" carries one. */
const esc = (s: string) => s.replace(/&/g, "&amp;");

function row(permissions: readonly string[] | null, pathname = "/danh-ba") {
  const { modules } = splitSettings(locMenu(NHOM_MENU, permissions));
  return renderToStaticMarkup(<HeaderModules groups={modules} pathname={pathname} />);
}

/** Every `<a …>` opening tag in the markup. */
const links = (html: string) => html.match(/<a [^>]*>/g) ?? [];

/** The `<a …>` opening tag named `label` (attribute order is React's, so match the tag, not a string). */
const linkNamed = (html: string, label: string) => links(html).find((a) => a.includes(`aria-label="${esc(label)}"`)) ?? "";

describe("menu icons", () => {
  it("every item of NHOM_MENU has its own icon — a new item cannot fall back silently", () => {
    for (const m of ALL_ITEMS) expect(MENU_ICONS[m.nhan], m.nhan).toBeDefined();
  });
});

describe("module row — what it draws", () => {
  it("every permission: one icon link per item, settings excepted (it sits on the right)", () => {
    const html = row(ALL_PERMISSIONS);
    expect(links(html)).toHaveLength(ALL_ITEMS.length - 1);
    expect(html).not.toContain(`aria-label="${SETTINGS_LABEL}"`);
    expect(html).toContain('<nav class="header-modules" aria-label="Điều hướng chính">');
  });

  it("EVERY module icon has an accessible name: aria-label = the menu's Vietnamese label", () => {
    const html = row(ALL_PERMISSIONS);
    const names = links(html).map((a) => /aria-label="([^"]*)"/.exec(a)?.[1]);
    expect(names.every((n) => n !== undefined && n.trim() !== "")).toBe(true);
    expect(names).toEqual(ALL_ITEMS.filter((m) => m.nhan !== SETTINGS_LABEL).map((m) => esc(m.nhan)));
  });

  it("icon only: no visible text label in the row (the word lives in aria-label and the tooltip)", () => {
    const html = row(ALL_PERMISSIONS);
    for (const m of ALL_ITEMS) expect(html).not.toContain(`>${esc(m.nhan)}<`);
  });

  it("the groups of NHOM_MENU stay as runs of icons — one list per group, Tổng quan's untitled", () => {
    const html = row(ALL_PERMISSIONS);
    expect(html.match(/<ul class="header-module-group"/g)).toHaveLength(NHOM_MENU.length);
    for (const g of NHOM_MENU.filter((x) => x.ten !== "")) expect(html).toContain(`aria-label="${g.ten}"`);
  });

  it("only `report.read`: exactly Tổng quan and Báo cáo", () => {
    const html = row(["report.read"]);
    expect(links(html).map((a) => /href="([^"]*)"/.exec(a)?.[1])).toEqual(["/tong-quan", "/bao-cao"]);
  });

  it("DENIED: no permission, or the session not read yet → no module icon at all, no empty nav", () => {
    for (const p of [[], null]) {
      const html = row(p);
      expect(html).toBe("");
    }
  });
});

describe("module row — the current page", () => {
  it("marks the current module with aria-current=\"page\" and the active fill, once", () => {
    const html = row(ALL_PERMISSIONS, "/danh-ba");
    expect(html.match(/aria-current="page"/g)).toHaveLength(1);
    const a = linkNamed(html, "Danh bạ cán bộ");
    expect(a).toContain('href="/danh-ba"');
    expect(a).toContain('aria-current="page"');
    expect(a).toContain('class="header-icon-button is-active"');
  });

  it("a parent route stays lit on its child route — the rule of `dangChon`, unchanged", () => {
    const html = row(ALL_PERMISSIONS, "/giai-ngan/thu-chi");
    expect(linkNamed(html, "Giải ngân")).toContain('aria-current="page"');
    expect(linkNamed(html, "Thu - Chi ngân sách")).toContain('aria-current="page"');
  });

  it("no module matches the path → nothing marked", () => {
    expect(row(ALL_PERMISSIONS, "/doi-mat-khau")).not.toContain("aria-current");
  });
});

describe("settings button — the `Cấu hình` item, moved to the right, same route and key set", () => {
  it("`Cấu hình` is a NHOM_MENU item; splitSettings takes it out of the row and keeps it", () => {
    expect(ALL_ITEMS.map((m) => m.nhan)).toContain(SETTINGS_LABEL);
    const { modules, settings } = splitSettings(locMenu(NHOM_MENU, ALL_PERMISSIONS));
    expect(settings?.duong).toBe("/cau-hinh");
    expect(modules.flatMap((g) => g.muc).map((m) => m.nhan)).not.toContain(SETTINGS_LABEL);
  });

  it("only `admin.sla`: settings present, and its group (left with nothing else) disappears from the row", () => {
    const { modules, settings } = splitSettings(locMenu(NHOM_MENU, ["admin.sla"]));
    expect(settings?.nhan).toBe(SETTINGS_LABEL);
    expect(modules).toHaveLength(0);
  });

  it("DENIED: no key of KHOA_MO_CAU_HINH → no settings button", () => {
    expect(splitSettings(locMenu(NHOM_MENU, ["task.read", "admin.user", "admin.user.delete"])).settings).toBeNull();
  });

  it("draws the settings icon with the item's name and route; current on /cau-hinh", () => {
    const item = ALL_ITEMS.find((m) => m.nhan === SETTINGS_LABEL)!;
    const html = renderToStaticMarkup(<SettingsButton item={item} pathname="/cau-hinh" />);
    const a = linkNamed(html, "Cấu hình");
    expect(a).toContain('href="/cau-hinh"');
    expect(a).toContain('aria-current="page"');
    expect(html).toContain("lucide-settings");
  });
});

describe("items with no screen (ADR 0068 §14)", () => {
  it("no item is unbuilt today, and PENDING_SCREENS keeps no stale entry", () => {
    expect(ALL_ITEMS.filter((m) => m.duong === null)).toEqual([]);
    expect(Object.keys(PENDING_SCREENS)).toEqual([]);
    expect(row(ALL_PERMISSIONS)).not.toContain("data-pending-marker");
  });

  it("never say 'Sắp có' — a public authority does not promise a date nobody set", () => {
    expect(row(ALL_PERMISSIONS)).not.toContain("Sắp có");
  });

  it("the report and the economic map are BUILT: real links, no '?'", () => {
    const html = row(ALL_PERMISSIONS);
    expect(linkNamed(html, "Báo cáo")).toContain('href="/bao-cao"');
    expect(linkNamed(html, "Bản đồ kinh tế số")).toContain('href="/ban-do"');
  });

  it("an item listed with `duong: null` is a DISABLED icon: no link, aria-disabled, still named, '?' beside it", () => {
    const item: MucMenu = { nhan: "Báo cáo", duong: null, khoa: null };
    const groups: NhomMenu[] = [{ ten: "HỆ THỐNG", muc: [item] }];
    // The marker needs a description to open; the table is empty today, so give it one for the test.
    (PENDING_SCREENS as Record<string, { ten: string; viSao: string }>)["Báo cáo"] = { ten: "Báo cáo", viSao: "x" };
    try {
      const html = renderToStaticMarkup(<HeaderModules groups={groups} pathname="/danh-ba" />);
      expect(html).not.toContain("<a ");
      expect(html).toContain('aria-disabled="true"');
      expect(html).toContain('<span class="an-thi-giac">Báo cáo</span>');
      // The ONE focusable thing is the "?", and its name says the whole sentence.
      expect(html.match(/<button/g)).toHaveLength(1);
      expect(html).toContain(`aria-label="${pendingMarkerLabel("Báo cáo")}"`);
    } finally {
      delete (PENDING_SCREENS as Record<string, unknown>)["Báo cáo"];
    }
  });
});
