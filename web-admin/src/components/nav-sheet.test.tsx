// @vitest-environment jsdom
//
// jsdom: the sheet must OPEN from the header's menu button, show every permitted item with its WORD,
// and close when a link is followed.

import type { AnchorHTMLAttributes } from "react";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import type { KhoiNguoiDung } from "@/features/phien/khoi-nguoi-dung";

// A plain anchor: the App Router's Link needs a router this test does not mount, and what is tested
// here is which links exist and what they say, not client navigation.
vi.mock("next/link", () => ({
  default: ({ href, ...rest }: AnchorHTMLAttributes<HTMLAnchorElement> & { href: string }) => <a href={href} {...rest} />,
}));

const { locMenu, NHOM_MENU } = await import("./muc-menu");
const { NAV_SHEET_CLOSE_LABEL, NAV_SHEET_OPEN_LABEL, NavSheet, NavSheetContent } = await import("./nav-sheet");

const ALL_PERMISSIONS = NHOM_MENU.flatMap((n) =>
  n.muc.flatMap((m) => (m.khoa === null ? [] : typeof m.khoa === "string" ? [m.khoa] : [...m.khoa])),
);
const PERSON: KhoiNguoiDung = { hien: true, hoTen: "Nguyễn Văn Hùng", chucVu: "Chuyên viên" };
const NOT_READ: KhoiNguoiDung = { hien: false, vi: "dang-doc" };

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

let root: Root | null = null;
let host: HTMLDivElement | null = null;

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
});

function content(permissions: readonly string[] | null, person: KhoiNguoiDung, roleName: string | null = null, pathname = "/nhiem-vu") {
  return renderToStaticMarkup(
    <NavSheetContent
      groups={locMenu(NHOM_MENU, permissions)}
      pathname={pathname}
      person={person}
      roleName={roleName}
      search={null}
      onNavigate={() => {}}
    />,
  );
}

const esc = (s: string) => s.replace(/&/g, "&amp;");

describe("nav sheet content — small screens show TEXT labels (touch has no tooltip)", () => {
  it("every permitted item, settings included, with its visible word and its group heading", () => {
    const html = content(ALL_PERMISSIONS, PERSON);
    for (const g of NHOM_MENU) {
      if (g.ten !== "") expect(html).toContain(`<p class="nav-sheet-group-label">${g.ten}</p>`);
      for (const m of g.muc) expect(html, m.nhan).toContain(`<span class="nav-sheet-label">${esc(m.nhan)}</span>`);
    }
  });

  it("marks the current page with aria-current, once", () => {
    const html = content(ALL_PERMISSIONS, PERSON, null, "/van-ban");
    expect(html.match(/aria-current="page"/g)).toHaveLength(1);
    expect(html).toMatch(/<a href="\/van-ban" aria-current="page" class="nav-sheet-link is-active"/);
  });

  it("DENIED: only `task.read` → the three task screens and nothing else", () => {
    const html = content(["task.read"], PERSON);
    const hrefs = [...html.matchAll(/<a href="([^"]*)"/g)].map((m) => m[1]);
    expect(hrefs).toEqual(["/nhiem-vu", "/nhiem-vu/bien-ban", "/nhiem-vu/so-tay", "/doi-mat-khau"]);
  });

  it("session not read: no menu item, no person, no change-password link — only the sign-out control", () => {
    const html = content(null, NOT_READ);
    expect(html).not.toContain("nav-sheet-nav");
    expect(html).not.toContain("ho-ten");
    expect(html).not.toContain('href="/doi-mat-khau"');
    expect(html).toContain("khoi-dang-xuat");
  });

  it("the person block collapses in: name, position, role, change password — no identity in the URL", () => {
    const html = content(["task.read"], PERSON, "Văn thư");
    expect(html).toContain('<p class="ho-ten">Nguyễn Văn Hùng</p>');
    expect(html).toContain('<p class="chuc-vu">Chuyên viên</p>');
    expect(html).toContain("<strong>Văn thư</strong>");
    expect(html).toMatch(/<a href="\/doi-mat-khau"/);
    expect(html).not.toMatch(/href="\/doi-mat-khau[?#/]/);
  });
});

describe("nav sheet — opening and closing", () => {
  function mount(permissions: readonly string[]) {
    host = document.createElement("div");
    document.body.append(host);
    const r = createRoot(host);
    root = r;
    act(() =>
      r.render(
        <NavSheet groups={locMenu(NHOM_MENU, permissions)} pathname="/nhiem-vu" person={PERSON} roleName={null} search={null} />,
      ),
    );
    return host;
  }

  it("closed by default: the menu button only, named, aria-expanded=false", () => {
    const el = mount(ALL_PERMISSIONS);
    const button = el.querySelector<HTMLButtonElement>(`button[aria-label="${NAV_SHEET_OPEN_LABEL}"]`);
    expect(button).not.toBeNull();
    expect(button!.getAttribute("aria-expanded")).toBe("false");
    expect(el.querySelector("dialog")).toBeNull();
  });

  it("the button opens a dialog with text labels; following a link closes it", () => {
    const el = mount(["task.read"]);
    act(() => el.querySelector<HTMLButtonElement>(`button[aria-label="${NAV_SHEET_OPEN_LABEL}"]`)!.click());
    const dialog = el.querySelector("dialog");
    expect(dialog).not.toBeNull();
    expect(dialog!.hasAttribute("open")).toBe(true);
    expect(dialog!.getAttribute("aria-labelledby")).toBe("nav-sheet-title");
    const labels = [...dialog!.querySelectorAll(".nav-sheet-nav .nav-sheet-label")].map((s) => s.textContent);
    expect(labels).toEqual(["Nhiệm vụ", "Biên bản họp", "Sổ tay lãnh đạo"]);
    act(() => dialog!.querySelector<HTMLAnchorElement>('a[href="/nhiem-vu/so-tay"]')!.click());
    expect(el.querySelector("dialog")).toBeNull();
  });

  it("the close button closes it", () => {
    const el = mount(["task.read"]);
    act(() => el.querySelector<HTMLButtonElement>(`button[aria-label="${NAV_SHEET_OPEN_LABEL}"]`)!.click());
    act(() => el.querySelector<HTMLButtonElement>(`button[aria-label="${NAV_SHEET_CLOSE_LABEL}"]`)!.click());
    expect(el.querySelector("dialog")).toBeNull();
  });
});
