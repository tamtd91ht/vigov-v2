// @vitest-environment jsdom
//
// jsdom: a collapsed parent's children exist only once its flyout is opened (a Radix popover).

import type { AnchorHTMLAttributes } from "react";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

vi.mock("next/link", () => ({
  default: ({ href, ...rest }: AnchorHTMLAttributes<HTMLAnchorElement> & { href: string }) => <a href={href} {...rest} />,
}));

const { locMenu, NHOM_MENU } = await import("./muc-menu");
const { SideNav } = await import("./side-nav");

const PARENT = "Người dùng & Phân quyền";

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  if (!("ResizeObserver" in globalThis)) {
    (globalThis as unknown as { ResizeObserver: unknown }).ResizeObserver = class {
      observe() {}
      unobserve() {}
      disconnect() {}
    };
  }
});

let root: Root | null = null;
let host: HTMLDivElement | null = null;

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
});

function mount(permissions: readonly string[], pathname: string) {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() => r.render(<SideNav groups={locMenu(NHOM_MENU, permissions)} pathname={pathname} collapsed onToggle={() => {}} />));
  return host;
}

function parentButton(el: HTMLElement): HTMLButtonElement {
  const button = [...el.querySelectorAll<HTMLButtonElement>("button.side-nav-parent")].find((b) => b.textContent === PARENT);
  expect(button).toBeDefined();
  return button!;
}

const flyoutLinks = () =>
  [...document.body.querySelectorAll<HTMLAnchorElement>(".side-nav-flyout a")].map((a) => [
    a.getAttribute("href"),
    a.textContent,
    a.getAttribute("aria-current"),
  ]);

describe("collapsed sidebar — the parent's flyout", () => {
  it("closed: no child link anywhere; pressing the parent lists every visible child with its word", () => {
    const el = mount(["admin.user", "admin.role"], "/nguoi-dung/phan-quyen");
    expect(document.body.querySelector('a[href="/nguoi-dung/phan-quyen"]')).toBeNull();
    const button = parentButton(el);
    expect(button.getAttribute("aria-expanded")).toBe("false");

    act(() => button.click());
    expect(button.getAttribute("aria-expanded")).toBe("true");
    expect(document.body.querySelector(".side-nav-flyout-title")?.textContent).toBe(PARENT);
    // The current child is marked there too, and only it.
    expect(flyoutLinks()).toEqual([
      ["/nguoi-dung", "Người dùng", null],
      ["/nguoi-dung/phan-quyen", "Phân quyền", "page"],
    ]);
  });

  it("following a child link closes the flyout", () => {
    const el = mount(["admin.user", "admin.role"], "/cau-hinh");
    act(() => parentButton(el).click());
    act(() => document.body.querySelector<HTMLAnchorElement>('.side-nav-flyout a[href="/nguoi-dung"]')!.click());
    expect(document.body.querySelector(".side-nav-flyout")).toBeNull();
  });

  it("DENIED child: only `admin.role` → the flyout lists Phân quyền alone", () => {
    const el = mount(["admin.role"], "/cau-hinh");
    act(() => parentButton(el).click());
    expect(flyoutLinks()).toEqual([["/nguoi-dung/phan-quyen", "Phân quyền", null]]);
  });
});
