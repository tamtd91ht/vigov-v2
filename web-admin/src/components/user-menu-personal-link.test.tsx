// @vitest-environment jsdom
//
// jsdom: the person's menu is a popover — its links exist only once it is opened.

import type { AnchorHTMLAttributes } from "react";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({ usePathname: () => "/nhiem-vu", useRouter: () => ({ replace: vi.fn() }) }));
vi.mock("next/link", () => ({
  default: ({ href, ...rest }: AnchorHTMLAttributes<HTMLAnchorElement> & { href: string }) => <a href={href} {...rest} />,
}));

const { PERSONAL_PAGE_PATH, UserMenu } = await import("./user-menu");

/** `/ca-nhan` (ADR 0074, Zalo reminders) is reached from the person's own menu, beside Đổi mật khẩu. */

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

describe("UserMenu — Hồ sơ cá nhân", () => {
  it("closed: no link; opened: Hồ sơ cá nhân → /ca-nhan, before Đổi mật khẩu, no identity in the URL", () => {
    host = document.createElement("div");
    document.body.append(host);
    const r = createRoot(host);
    root = r;
    act(() => r.render(<UserMenu fullName="Nguyễn Văn Hùng" position="Chuyên viên" roleName="Văn thư" />));
    expect(document.querySelector(`a[href="${PERSONAL_PAGE_PATH}"]`)).toBeNull();

    act(() => host!.querySelector<HTMLButtonElement>("button.header-user")!.click());
    const menu = document.body.querySelector<HTMLElement>(".user-menu")!;
    const links = [...menu.querySelectorAll<HTMLAnchorElement>("a.user-menu-item")];
    expect(PERSONAL_PAGE_PATH).toBe("/ca-nhan");
    expect(links.map((a) => [a.getAttribute("href"), a.textContent?.trim()])).toEqual([
      ["/ca-nhan", "Hồ sơ cá nhân"],
      ["/doi-mat-khau", "Đổi mật khẩu"],
    ]);
  });

  it("the trigger shows the position under the name, as the prototype's topbar does; the role stays in the menu", () => {
    host = document.createElement("div");
    document.body.append(host);
    const r = createRoot(host);
    root = r;
    act(() => r.render(<UserMenu fullName="Nguyễn Văn Hùng" position="Chuyên viên" roleName="Văn thư" />));
    const trigger = host.querySelector<HTMLButtonElement>("button.header-user")!;
    expect(trigger.querySelector(".header-user-role")?.textContent).toBe("Chuyên viên");
    expect(trigger.textContent).not.toContain("Văn thư");

    act(() => trigger.click());
    expect(document.body.querySelector(".user-menu")!.textContent).toContain("Văn thư");
  });

  // Sheet web-admin-131026: "Thiếu vị trí/chức vụ của user đang login". Accounts created by an
  // administrator or operator carry no position; the line under the name then shows the role they hold.
  function triggerOf(position: string, roleName: string | null): HTMLButtonElement {
    host = document.createElement("div");
    document.body.append(host);
    const r = createRoot(host);
    root = r;
    act(() => r.render(<UserMenu fullName="Nguyễn Văn Hùng" position={position} roleName={roleName} />));
    return host.querySelector<HTMLButtonElement>("button.header-user")!;
  }

  it("no position: the line under the name is the role the session holds", () => {
    expect(triggerOf("", "Quản trị đơn vị").querySelector(".header-user-role")?.textContent).toBe("Quản trị đơn vị");
  });

  it("a blank position counts as none", () => {
    expect(triggerOf("   ", "Văn thư").querySelector(".header-user-role")?.textContent).toBe("Văn thư");
  });

  it("neither position nor role: no second line at all — never an invented title", () => {
    expect(triggerOf("", null).querySelector(".header-user-role")).toBeNull();
  });
});
