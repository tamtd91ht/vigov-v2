import { afterEach, describe, expect, it, vi } from "vitest";

/**
 * `/noi-dung` and `/danh-ba` became the two tabs of `/mini-app` (06/10/2026, ADR 0068 lần 5). Links are
 * saved and shared, so both old paths redirect — and only AFTER the commune resolved from `Host`: an
 * unknown domain 404s, it never learns that the route exists.
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

const ContentPage = (await import("../noi-dung/page")).default;
const DirectoryPage = (await import("../danh-ba/page")).default;

afterEach(() => {
  calls.length = 0;
  tenantResolves = true;
});

describe("/noi-dung → /mini-app", () => {
  it("no query → /mini-app", async () => {
    await expect(ContentPage({ searchParams: Promise.resolve({}) })).rejects.toThrow("NEXT_REDIRECT /mini-app");
    expect(calls).toEqual(["tenant", "redirect:/mini-app"]);
  });

  it("a query is kept (but `tab`, the content tab being the default)", async () => {
    await expect(
      ContentPage({ searchParams: Promise.resolve({ type: "tin-tuc", tab: "danh-ba" }) }),
    ).rejects.toThrow("NEXT_REDIRECT /mini-app?type=tin-tuc");
  });

  it("unknown Host: 404 before any redirect", async () => {
    tenantResolves = false;
    await expect(ContentPage({ searchParams: Promise.resolve({ type: "x" }) })).rejects.toThrow("NEXT_NOT_FOUND");
    expect(calls).toEqual(["tenant"]);
  });
});

describe("/danh-ba → /mini-app?tab=danh-ba", () => {
  it("redirects to the directory tab", async () => {
    await expect(DirectoryPage()).rejects.toThrow("NEXT_REDIRECT /mini-app?tab=danh-ba");
    expect(calls).toEqual(["tenant", "redirect:/mini-app?tab=danh-ba"]);
  });

  it("unknown Host: 404 before any redirect", async () => {
    tenantResolves = false;
    await expect(DirectoryPage()).rejects.toThrow("NEXT_NOT_FOUND");
    expect(calls).toEqual(["tenant"]);
  });
});
