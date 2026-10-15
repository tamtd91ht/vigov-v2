// @vitest-environment jsdom
//
// jsdom: the shell is mounted, unmounted and mounted again — what a `<Link>` between two screens does.

import type { AnchorHTMLAttributes } from "react";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import type { PhienDaDoc } from "@/features/phien/phien-hien-tai";

/**
 * Bug sheet 15/10/2026: moving to another screen from the left menu redrew the whole screen, and the
 * menu came back scrolled to its top — "phải kéo left menu xuống dưới để click tiếp các menu phía dưới".
 * Every screen draws its own shell, so each move mounts a NEW sidebar under a NEW, not-yet-read session
 * (`sidebar-state.ts`). These tests mount that sequence and check what the second screen's sidebar shows
 * before its own session read answers.
 */
const H = vi.hoisted(() => ({ phien: null as PhienDaDoc, path: "/tong-quan" }));

vi.mock("@/features/phien/phien-hien-tai", () => ({ usePhien: () => H.phien }));
vi.mock("next/navigation", () => ({ usePathname: () => H.path }));
vi.mock("./notification-bell", () => ({ NotificationBell: () => null }));
vi.mock("next/link", () => ({
  default: ({ href, ...rest }: AnchorHTMLAttributes<HTMLAnchorElement> & { href: string }) => <a href={href} {...rest} />,
}));

const { CauHinhXaProvider } = await import("./cau-hinh-xa");
const { DauTrang } = await import("./dau-trang");
const { rememberNavScroll, restoreNavScroll } = await import("./sidebar-state");

function session(permissions: string[]): PhienDaDoc {
  return {
    ok: true,
    duLieu: { staff: { full_name: "Cán bộ thử", position: "Chuyên viên" }, role: null, permissions },
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

function unmount() {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
}

afterEach(() => {
  unmount();
  H.phien = null;
  window.localStorage.clear();
});

/** One screen: a fresh shell, as each `page.tsx` draws it. */
function mountScreen(phien: PhienDaDoc, path: string): HTMLDivElement {
  H.phien = phien;
  H.path = path;
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() =>
    r.render(
      <CauHinhXaProvider giaTri={{ displayName: "Xã Tân Phú", parentAuthority: "Tỉnh Đồng Nai", logoUrl: "", webAdminBannerUrl: "" }}>
        <DauTrang />
      </CauHinhXaProvider>,
    ),
  );
  return host;
}

const sideHrefs = (el: HTMLElement) => [...el.querySelectorAll(".side-nav a")].map((a) => a.getAttribute("href"));

// ORDER MATTERS: the memory is module state, as it is in the browser tab. The first test is the tab's
// first screen.
describe("the sidebar across a move between screens", () => {
  it("DENIED before any read in this tab: the first screen lists no keyed item until its session answers", () => {
    const el = mountScreen(null, "/tong-quan");
    expect(sideHrefs(el)).toEqual([]);
  });

  it("the next screen lists the previous screen's items while its own read is in flight — no empty-then-refill", () => {
    mountScreen(session(["task.read", "feedback.read"]), "/nhiem-vu");
    unmount();
    const el = mountScreen(null, "/phan-anh");
    expect(sideHrefs(el)).toEqual(["/nhiem-vu", "/nhiem-vu/so-tay", "/nhiem-vu/bien-ban", "/phan-anh"]);
    expect(el.querySelector('.side-nav [aria-current="page"]')?.getAttribute("href")).toBe("/phan-anh");
  });

  it("this screen's own answer replaces the remembered one — a key withdrawn meanwhile hides its item", () => {
    mountScreen(session(["task.read", "feedback.read"]), "/nhiem-vu");
    unmount();
    const el = mountScreen(session(["task.read"]), "/nhiem-vu");
    expect(sideHrefs(el)).toEqual(["/nhiem-vu", "/nhiem-vu/so-tay", "/nhiem-vu/bien-ban"]);
    unmount();
    // …and the screen after that starts from THAT answer.
    expect(sideHrefs(mountScreen(null, "/nhiem-vu"))).toEqual(["/nhiem-vu", "/nhiem-vu/so-tay", "/nhiem-vu/bien-ban"]);
  });

  it("the menu list is put back at the scroll position the previous screen's list was left at", () => {
    const el = mountScreen(session(["feedback.read"]), "/tong-quan");
    const nav = el.querySelector<HTMLElement>(".side-nav-nav")!;
    expect(nav).not.toBeNull();
    rememberNavScroll({ currentTarget: { scrollTop: 240 } as HTMLElement });
    unmount();
    const next = mountScreen(null, "/phan-anh").querySelector<HTMLElement>(".side-nav-nav")!;
    expect(next.scrollTop).toBe(240);
    // Wired to the list itself, not only exported.
    const probe = { scrollTop: 0 } as HTMLElement;
    restoreNavScroll(probe);
    expect(probe.scrollTop).toBe(240);
  });
});
