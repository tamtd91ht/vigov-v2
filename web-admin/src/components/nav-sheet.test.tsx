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

const { flattenMenu, isMenuParent, locMenu, NHOM_MENU } = await import("./muc-menu");
const { NAV_SHEET_CLOSE_LABEL, NAV_SHEET_OPEN_LABEL, NavSheet, NavSheetContent } = await import("./nav-sheet");

const ALL_PERMISSIONS = flattenMenu(NHOM_MENU).flatMap((m) =>
  m.khoa === null ? [] : typeof m.khoa === "string" ? [m.khoa] : [...m.khoa],
);
const PARENT = "Người dùng & Phân quyền";
const PENDING = ["Danh bạ người dân", "Gửi tin ZNS / SMS", "Hướng dẫn sử dụng"];
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
      for (const m of g.muc) {
        expect(html, m.nhan).toContain(`<span class="nav-sheet-label">${esc(m.nhan)}</span>`);
        if (isMenuParent(m)) {
          for (const c of m.children) expect(html, c.nhan).toContain(`<span class="nav-sheet-label">${esc(c.nhan)}</span>`);
        }
      }
    }
  });

  it("the same structure as the sidebar: two groups in order, the parent row then its children indented", () => {
    const html = content(ALL_PERMISSIONS, PERSON, null, "/nguoi-dung/phan-quyen");
    const headings = [...html.matchAll(/<p class="nav-sheet-group-label">([^<]*)<\/p>/g)].map((m) => m[1]);
    expect(headings).toEqual(["Điều hành", "Quản trị"]);
    const labels = [...html.matchAll(/<span class="nav-sheet-label">([^<]*)<\/span>/g)].map((m) => m[1]);
    expect(labels.slice(0, -1)).toEqual(
      NHOM_MENU.flatMap((g) => g.muc.flatMap((m) => (isMenuParent(m) ? [m.nhan, ...m.children.map((c) => c.nhan)] : [m.nhan]))).map(esc),
    );
    const children = new RegExp(`<ul class="nav-sheet-children" aria-label="${esc(PARENT)}">(.*?)</ul>`).exec(html)?.[1] ?? "";
    expect([...children.matchAll(/<a href="([^"]*)"/g)].map((m) => m[1])).toEqual(["/nguoi-dung", "/nguoi-dung/phan-quyen"]);
    // On a child: that child alone is current; the parent row is marked open.
    expect(html.match(/aria-current="page"/g)).toHaveLength(1);
    expect(children).toMatch(/<a href="\/nguoi-dung\/phan-quyen" aria-current="page"/);
    expect(html).toContain('class="nav-sheet-link nav-sheet-parent is-open"');
  });

  it("DENIED: neither child key → no parent row in the sheet", () => {
    for (const p of [["task.read"], [], null]) expect(content(p, PERSON)).not.toContain(esc(PARENT));
  });

  // ADR 0068 lần 6 #11: as in the sidebar — muted, the hover sentence, no "?".
  it("the three unbuilt items: disabled, their word, the hover sentence, no '?'", () => {
    const html = content([], PERSON);
    expect(html).not.toContain("data-pending-marker");
    expect(html.match(/<span aria-disabled="true" class="nav-sheet-link" title="Tính năng đang phát triển">/g)).toHaveLength(3);
    for (const nhan of PENDING) expect(html).toContain(`<span class="nav-sheet-label">${nhan}</span>`);
    expect(html).not.toMatch(/<a href="(?!\/doi-mat-khau)/);
  });

  it("opens on the sidebar's brand row (navy sheet = the sidebar below 768px); the dialog is still named 'Menu'", () => {
    const html = content([], PERSON);
    expect(html).toMatch(/^<div class="nav-sheet-body"><div class="side-nav-brand">.*ViGov.*Điều hành số cấp xã.*aria-label="Đóng menu"/);
    expect(html).toContain('<h2 id="nav-sheet-title" class="an-thi-giac">Menu</h2>');
  });

  it("marks the current page with aria-current, once", () => {
    const html = content(ALL_PERMISSIONS, PERSON, null, "/van-ban");
    expect(html.match(/aria-current="page"/g)).toHaveLength(1);
    expect(html).toMatch(/<a href="\/van-ban" aria-current="page" class="nav-sheet-link is-active"/);
  });

  it("DENIED: only `task.read` → the three task screens and nothing else", () => {
    const html = content(["task.read"], PERSON);
    const hrefs = [...html.matchAll(/<a href="([^"]*)"/g)].map((m) => m[1]);
    expect(hrefs).toEqual(["/nhiem-vu", "/nhiem-vu/so-tay", "/nhiem-vu/bien-ban", "/doi-mat-khau"]);
  });

  it("session not read: no menu link (only the unbuilt placeholders), no person, no change-password link", () => {
    const html = content(null, NOT_READ);
    expect(html).not.toMatch(/<a href="(?!\/dang-nhap)/);
    expect(html.match(/class="nav-sheet-item is-pending"/g)).toHaveLength(3);
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
    expect(labels).toEqual(["Nhiệm vụ", "Sổ tay lãnh đạo", "Biên bản họp", "Danh bạ người dân", "Gửi tin ZNS / SMS", "Hướng dẫn sử dụng"]);
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
