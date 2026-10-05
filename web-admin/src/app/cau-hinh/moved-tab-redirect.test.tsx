import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

/**
 * `/cau-hinh?tab=nguoi-dung|phan-quyen` → the screens those tabs became (05/10/2026). Links are saved
 * and shared; one that silently opens on "Sơ đồ tổ chức" reads as "the screen is gone".
 */

const calls: string[] = [];
let tenantResolves = true;

vi.mock("next/navigation", () => ({
  redirect: (to: string) => {
    calls.push(`redirect:${to}`);
    throw new Error(`NEXT_REDIRECT ${to}`);
  },
}));
vi.mock("@/lib/tenant.server", () => ({
  layCauHinhXa: async () => {
    calls.push("tenant");
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
vi.mock("@/features/cau-hinh/khung-tab-cau-hinh", () => ({ KhungTabCauHinh: () => null }));

const ConfigPage = (await import("./page")).default;

const open = (params: Record<string, string | string[] | undefined>) =>
  ConfigPage({ searchParams: Promise.resolve(params) });

afterEach(() => {
  calls.length = 0;
  tenantResolves = true;
});

describe("/cau-hinh with an old tab link", () => {
  it("?tab=nguoi-dung → /nguoi-dung", async () => {
    await expect(open({ tab: "nguoi-dung" })).rejects.toThrow("NEXT_REDIRECT /nguoi-dung");
  });

  it("?tab=phan-quyen → /nguoi-dung/phan-quyen", async () => {
    await expect(open({ tab: "phan-quyen" })).rejects.toThrow("NEXT_REDIRECT /nguoi-dung/phan-quyen");
  });

  it("no tab, or a tab still here → the screen renders, no redirect", async () => {
    for (const params of [{}, { tab: "danh-muc" }, { tab: "x" }]) {
      await expect(open(params)).resolves.toBeTruthy();
    }
    expect(calls.some((c) => c.startsWith("redirect:"))).toBe(false);
  });

  it("the commune is resolved BEFORE any redirect: an unknown Host 404s, never redirects", async () => {
    tenantResolves = false;
    await expect(open({ tab: "nguoi-dung" })).rejects.toThrow("NEXT_NOT_FOUND");
    expect(calls).toEqual(["tenant"]);
  });
});
