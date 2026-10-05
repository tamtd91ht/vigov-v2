// @vitest-environment jsdom
//
// jsdom: the person's menu is a popover — its actions exist only once it is opened.

import type { AnchorHTMLAttributes } from "react";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import type { PhienDaDoc } from "@/features/phien/phien-hien-tai";

/**
 * The header with a READ session: the person block, the menu it opens (`Đổi mật khẩu`, `Đăng xuất`),
 * and which module icons and settings the session's permissions give. "Đổi mật khẩu" is the only
 * voluntary way into `/doi-mat-khau` — without it the page opens only when the server forces a change.
 * It belongs to the signed-in person, so it is drawn only once the session is read, and it carries no
 * identity of its own.
 */
const H = vi.hoisted(() => ({ phien: null as PhienDaDoc, path: "/nhiem-vu" }));

vi.mock("@/features/phien/phien-hien-tai", () => ({ usePhien: () => H.phien }));
vi.mock("next/navigation", () => ({ usePathname: () => H.path }));
// The bell needs the App Router context; it is not what this file tests.
vi.mock("./notification-bell", () => ({ NotificationBell: () => null }));
vi.mock("next/link", () => ({
  default: ({ href, ...rest }: AnchorHTMLAttributes<HTMLAnchorElement> & { href: string }) => <a href={href} {...rest} />,
}));

const { CauHinhXaProvider } = await import("./cau-hinh-xa");
const { DauTrang } = await import("./dau-trang");

function session(permissions: string[], role: { code: string; name: string; is_leader: boolean } | null = null): PhienDaDoc {
  return {
    ok: true,
    duLieu: { staff: { full_name: "Nguyễn Văn Hùng", position: "Chuyên viên" }, role, permissions },
  } as unknown as PhienDaDoc;
}

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
  H.phien = null;
  H.path = "/nhiem-vu";
});

function mount(phien: PhienDaDoc, navigation = true): HTMLDivElement {
  H.phien = phien;
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() =>
    r.render(
      <CauHinhXaProvider giaTri={{ displayName: "Xã Tân Phú", parentAuthority: "Tỉnh Đồng Nai", logoUrl: "", webAdminBannerUrl: "" }}>
        <DauTrang navigation={navigation} />
      </CauHinhXaProvider>,
    ),
  );
  return host;
}

const header = (el: HTMLElement) => el.querySelector("header")!;
const moduleNames = (el: HTMLElement) =>
  [...header(el).querySelectorAll(".header-modules a")].map((a) => a.getAttribute("aria-label"));

describe("DauTrang — person block and its menu", () => {
  it("shows the person's name and role on the trigger; no change-password link until the menu opens", () => {
    const el = mount(session([], { code: "van-thu", name: "Văn thư", is_leader: false }));
    const trigger = header(el).querySelector<HTMLButtonElement>("button.header-user")!;
    expect(trigger.textContent).toContain("Nguyễn Văn Hùng");
    expect(trigger.textContent).toContain("Văn thư");
    expect(trigger.getAttribute("aria-expanded")).toBe("false");
    expect(document.querySelector('a[href="/doi-mat-khau"]')).toBeNull();
  });

  it("opening it: change password (no identity in the URL), sign out, the role pill", () => {
    const el = mount(session([], { code: "van-thu", name: "Văn thư", is_leader: false }));
    const trigger = header(el).querySelector<HTMLButtonElement>("button.header-user")!;
    act(() => trigger.click());
    expect(trigger.getAttribute("aria-expanded")).toBe("true");
    const menu = document.body.querySelector<HTMLElement>(".user-menu")!;
    expect(menu).not.toBeNull();
    const link = menu.querySelector<HTMLAnchorElement>('a[href="/doi-mat-khau"]');
    expect(link?.textContent).toContain("Đổi mật khẩu");
    expect(link?.getAttribute("href")).toBe("/doi-mat-khau");
    expect(menu.querySelector(".khoi-dang-xuat")).not.toBeNull();
    expect(menu.querySelector(".role-pill")?.textContent).toContain("Văn thư");
    expect(menu.querySelector(".chuc-vu")?.textContent).toBe("Chuyên viên");
  });

  it("before the session is read, or when it cannot be read: no person block, no link — the bare sign-out stays", () => {
    for (const phien of [null, { ok: false } as unknown as PhienDaDoc]) {
      const el = mount(phien);
      expect(header(el).querySelector("button.header-user")).toBeNull();
      expect(document.querySelector('a[href="/doi-mat-khau"]')).toBeNull();
      expect(header(el).querySelector(".header-logout .khoi-dang-xuat")).not.toBeNull();
      act(() => root?.unmount());
      host?.remove();
      root = null;
    }
  });
});

describe("DauTrang — module icons follow the session's permissions (UX; the server still checks)", () => {
  it("`task.read`: the three task screens, the current one marked, no settings", () => {
    const el = mount(session(["task.read"]));
    expect(moduleNames(el)).toEqual(["Nhiệm vụ", "Biên bản họp", "Sổ tay lãnh đạo"]);
    const current = header(el).querySelectorAll('.header-modules [aria-current="page"]');
    expect([...current].map((a) => a.getAttribute("aria-label"))).toEqual(["Nhiệm vụ"]);
    expect(header(el).querySelector('a[aria-label="Cấu hình"]')).toBeNull();
  });

  it("`admin.sla`: the settings button on the right, to /cau-hinh, and no module icon", () => {
    const el = mount(session(["admin.sla"]));
    expect(moduleNames(el)).toEqual([]);
    const settings = header(el).querySelector<HTMLAnchorElement>('.header-end a[aria-label="Cấu hình"]');
    expect(settings?.getAttribute("href")).toBe("/cau-hinh");
  });

  it("DENIED: a read session with no key → no module icon, no settings", () => {
    const el = mount(session(["admin.user.delete", "report.export"]));
    expect(header(el).querySelector(".header-modules")).toBeNull();
    expect(header(el).querySelector('a[aria-label="Cấu hình"]')).toBeNull();
  });

  it("`navigation={false}` (/doi-mat-khau): no menu even with every key — the person block stays", () => {
    const el = mount(session(["task.read", "admin.sla", "report.read"]), false);
    expect(header(el).querySelector(".header-modules")).toBeNull();
    expect(header(el).querySelector('a[aria-label="Cấu hình"]')).toBeNull();
    expect(header(el).querySelector("button.header-user")).not.toBeNull();
  });
});
