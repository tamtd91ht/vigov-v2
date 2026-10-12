import type { ReactNode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

/**
 * `/he-thong` (owner 12/10/2026, S27): the commune is resolved first, and `?tab=` only picks the starting
 * tab — and only when it names one of this screen's three.
 */

let tenantResolves = true;
const received: (string | null | undefined)[] = [];

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
vi.mock("@/features/cau-hinh/khung-tab-cau-hinh", () => ({
  SystemTabFrame: ({ initialTab }: { initialTab?: string | null }) => {
    received.push(initialTab);
    return null;
  },
}));

const page = await import("./page");

const open = async (params: Record<string, string | string[] | undefined>) =>
  renderToStaticMarkup(await page.default({ searchParams: Promise.resolve(params) }));

afterEach(() => {
  received.length = 0;
  tenantResolves = true;
});

describe("/he-thong", () => {
  it("title Hệ thống, h1 Hệ thống", async () => {
    expect(await page.generateMetadata()).toEqual({ title: "Hệ thống" });
    expect(await open({})).toContain(">Hệ thống</h1>");
  });

  it("?tab=<one of the three> is passed down as the starting tab", async () => {
    await open({ tab: "nhat-ky-he-thong" });
    await open({ tab: "nhan-dien-xa" });
    expect(received).toEqual(["nhat-ky-he-thong", "nhan-dien-xa"]);
  });

  it("no tab, a /cau-hinh tab, a look-alike or a repeated tab → no starting tab", async () => {
    for (const tab of [undefined, "danh-muc", "constructor", "Nhan-Dien-Xa", ["nhan-dien-xa", "lich-lam-viec"]]) {
      await open({ tab });
    }
    expect(received).toEqual([null, null, null, null, null]);
  });

  it("an unknown Host 404s before anything renders", async () => {
    tenantResolves = false;
    await expect(open({ tab: "nhan-dien-xa" })).rejects.toThrow("NEXT_NOT_FOUND");
    expect(received).toEqual([]);
  });
});
