import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

/**
 * `/mini-app` resolves the commune from `Host` first (404 on none), then hands the requested tab to the
 * workspace. Which tab SHOWS is the workspace's call (`mini-app-workspace.test.tsx`).
 */

let tenantResolves = true;
const seen: { requested?: string } = {};

vi.mock("@/lib/tenant.server", () => ({
  layCauHinhXa: async () => {
    if (!tenantResolves) throw new Error("NEXT_NOT_FOUND");
    return {};
  },
  communePageMetadata: async (screen: string) => ({ title: screen }),
}));
vi.mock("@/lib/cau-hinh-xa-hien-thi", () => ({ phanHienThi: () => ({}) }));
vi.mock("@/components/cau-hinh-xa", () => ({
  CauHinhXaProvider: ({ children }: { children: ReactNode }) => <>{children}</>,
}));
vi.mock("@/features/phien/phien-hien-tai", () => ({
  PhienProvider: ({ children }: { children: ReactNode }) => <>{children}</>,
}));
vi.mock("@/components/dau-trang", () => ({ DauTrang: () => null }));
vi.mock("@/features/noi-dung/so-noi-dung", () => ({ SoNoiDung: () => null }));
vi.mock("@/features/danh-ba/staff-directory-tab", () => ({ StaffDirectoryTab: () => null }));
vi.mock("@/features/mini-app/mini-app-workspace", () => ({
  MiniAppWorkspace: (p: { requested: string }) => {
    seen.requested = p.requested;
    return null;
  },
}));

const { renderToStaticMarkup } = await import("react-dom/server");
const MiniAppPage = (await import("./page")).default;

afterEach(() => {
  tenantResolves = true;
  delete seen.requested;
});

async function open(params: Record<string, string | string[] | undefined>) {
  renderToStaticMarkup(await MiniAppPage({ searchParams: Promise.resolve(params) }));
  return seen.requested;
}

describe("/mini-app", () => {
  it("?tab=danh-ba asks for the directory; anything else for the content tab", async () => {
    expect(await open({ tab: "danh-ba" })).toBe("danh-ba");
    expect(await open({})).toBe("noi-dung");
    expect(await open({ tab: "x" })).toBe("noi-dung");
  });

  it("the page's own padding is off (`p-0`): the tab band is flush, the workspace pads its body", async () => {
    const html = renderToStaticMarkup(await MiniAppPage({ searchParams: Promise.resolve({}) }));
    expect(html).toContain('<main class="than-trang p-0">');
  });

  it("unknown Host: 404, nothing rendered", async () => {
    tenantResolves = false;
    await expect(MiniAppPage({ searchParams: Promise.resolve({ tab: "danh-ba" }) })).rejects.toThrow("NEXT_NOT_FOUND");
    expect(seen.requested).toBeUndefined();
  });
});
