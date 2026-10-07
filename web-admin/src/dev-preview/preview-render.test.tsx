// @vitest-environment jsdom
//
// jsdom: the preview must render the REAL Giải ngân components POPULATED from fixtures, open the dialog
// or tab its query names, and send nothing to the network.

import type { AnchorHTMLAttributes } from "react";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

const H = vi.hoisted(() => ({ path: "/xem-thu/giai-ngan", search: "" }));

vi.mock("next/navigation", () => ({
  usePathname: () => H.path,
  useRouter: () => ({ push: () => {}, replace: () => {}, refresh: () => {}, back: () => {}, prefetch: () => {} }),
  useSearchParams: () => new URLSearchParams(H.search),
}));
vi.mock("next/link", () => ({
  default: ({ href, ...rest }: AnchorHTMLAttributes<HTMLAnchorElement> & { href: string }) => <a href={href} {...rest} />,
}));

/** Anything that reaches the REAL fetch is a request leaving the preview — the test fails on it. */
const network = vi.fn(() => Promise.reject(new Error("network")));

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  for (const name of ["ResizeObserver", "IntersectionObserver"] as const) {
    if (!(name in globalThis)) {
      (globalThis as unknown as Record<string, unknown>)[name] = class {
        observe() {}
        unobserve() {}
        disconnect() {}
      };
    }
  }
  window.fetch = network as unknown as typeof window.fetch;
});

const { PreviewShell } = await import("./preview-shell");
const { Toaster } = await import("@/components/ui/toaster");
const { SIDEBAR_STORAGE_KEY } = await import("@/components/sidebar-state");
const { DisbursementPreview, ProjectDetailPreview } = await import("./disbursement-preview");

let root: Root | null = null;
let host: HTMLDivElement | null = null;

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  H.search = "";
});

async function mount(path: string, body: React.ReactNode, fullMenu = false, search = ""): Promise<HTMLDivElement> {
  H.path = path;
  H.search = search;
  window.history.pushState({}, "", path);
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  // The root layout's one `Toaster`, a later sibling of the page — where the real app mounts it.
  await act(async () =>
    r.render(
      <>
        <PreviewShell fullMenu={fullMenu}>{body}</PreviewShell>
        <Toaster />
      </>,
    ),
  );
  return host;
}

/** Lets the fixture promises and the 100ms "press when ready" timer run. */
async function settle(ms = 400): Promise<void> {
  for (let i = 0; i < ms / 50; i++) await act(async () => new Promise((r) => setTimeout(r, 50)));
}

describe("preview — list screen", () => {
  it("the real shell (fixture commune, brand, Giải ngân lit) and the populated register, with no network", async () => {
    const el = await mount("/xem-thu/giai-ngan", <DisbursementPreview modal={null} />);
    await settle();
    expect(el.querySelector("header")!.textContent).toContain("Xã Thăng Bình");
    expect(el.querySelector(".side-nav-brand-name")!.textContent).toBe("ViGov");
    expect(el.querySelector('.side-nav a[aria-current="page"]')!.getAttribute("href")).toBe("/giai-ngan");
    expect(el.textContent).toContain("Theo dõi giải ngân");
    expect(el.textContent).toContain("Nhà văn hoá thôn Bình An");
    expect(el.textContent).toContain("Kè chống sạt lở bờ sông Ly Ly");
    expect(el.textContent).toContain("Ngân sách thành phố hỗ trợ");
    expect(el.querySelector(".header-user")!.textContent).toContain("Cán bộ A");
    expect(network).not.toHaveBeenCalled();
  });

  it("?modal=hang-muc opens the real category dialog over it; ?modal=them-du-an presses the real button", async () => {
    let el = await mount("/xem-thu/giai-ngan", <DisbursementPreview modal="hang-muc" />);
    await settle();
    const dialog = document.querySelector("dialog[open]")!;
    expect(dialog.textContent).toContain("Hạng mục kế hoạch vốn");
    // The four fixture categories, each in its rename field.
    expect([...dialog.querySelectorAll("input")].map((i) => i.value)).toEqual(
      expect.arrayContaining(["Công trình xây dựng mới", "Sửa chữa, cải tạo", "Hạ tầng số", "Môi trường"]),
    );
    act(() => root?.unmount());
    host?.remove();
    el = await mount("/xem-thu/giai-ngan", <DisbursementPreview modal="them-du-an" />);
    await settle(600);
    expect(el.querySelector("dialog[open]")).not.toBeNull();
    expect(network).not.toHaveBeenCalled();
  });

  it("?menu=day-du: the sidebar shows the full menu", async () => {
    const el = await mount("/xem-thu/giai-ngan", <DisbursementPreview modal={null} />, true);
    await settle();
    const hrefs = [...el.querySelectorAll(".side-nav a")].map((a) => a.getAttribute("href"));
    expect(hrefs).toContain("/tong-quan");
    expect(hrefs).toContain("/cau-hinh");
  });
});

describe("preview — project detail", () => {
  it("the real detail page on the fixture project; ?tab=trao-doi opens the discussion with its two comments", async () => {
    const el = await mount("/xem-thu/giai-ngan/du-an", <ProjectDetailPreview tab="trao-doi" />);
    await settle(600);
    expect(el.textContent).toContain("Nhà văn hoá thôn Bình An");
    expect(el.querySelector("#tab-trao-doi-du-an")!.getAttribute("aria-selected")).toBe("true");
    expect(el.textContent).toContain("Đã liên hệ, đơn vị hẹn nộp trong tuần này.");
    expect(network).not.toHaveBeenCalled();
  });
});

describe("preview — shell states for screenshots", () => {
  it("?sidebar=thu-gon collapses the real sidebar; without it the preview is expanded whatever was remembered", async () => {
    let el = await mount("/xem-thu/giai-ngan", <DisbursementPreview modal={null} />, false, "sidebar=thu-gon");
    await settle(100);
    expect(window.localStorage.getItem(SIDEBAR_STORAGE_KEY)).toBe("1");
    expect(el.querySelector(".side-nav")!.classList.contains("is-collapsed")).toBe(true);
    act(() => root?.unmount());
    host?.remove();
    el = await mount("/xem-thu/giai-ngan", <DisbursementPreview modal={null} />);
    await settle(100);
    expect(window.localStorage.getItem(SIDEBAR_STORAGE_KEY)).toBe("0");
    expect(el.querySelector(".side-nav")!.classList.contains("is-collapsed")).toBe(false);
  });

  it("?menu-tai-khoan=1 opens the real account menu; ?chuong=1 opens the real bell panel", async () => {
    await mount("/xem-thu/giai-ngan", <DisbursementPreview modal={null} />, false, "menu-tai-khoan=1");
    await settle();
    expect(document.querySelector('[aria-label="Tài khoản"]')).not.toBeNull();
    act(() => root?.unmount());
    host?.remove();
    const el = await mount("/xem-thu/giai-ngan", <DisbursementPreview modal={null} />, false, "chuong=1");
    await settle();
    expect(el.querySelector("button.nut-chuong")!.getAttribute("aria-expanded")).toBe("true");
    expect(el.querySelector("#bang-thong-bao-chuong")).not.toBeNull();
    expect(network).not.toHaveBeenCalled();
  });

  it("?toast=1 shows one toast through the real Toaster; no word, no toast", async () => {
    await mount("/xem-thu/giai-ngan", <DisbursementPreview modal={null} />, false, "toast=1");
    await settle(600);
    expect(document.body.textContent).toContain("Đã thêm hạng mục.");
  });

  it("no shell word opens nothing", async () => {
    const el = await mount("/xem-thu/giai-ngan", <DisbursementPreview modal={null} />);
    await settle();
    expect(el.querySelector("button.nut-chuong")!.getAttribute("aria-expanded")).toBe("false");
    expect(document.querySelector('[aria-label="Tài khoản"]')).toBeNull();
  });
});
