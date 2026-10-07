import { NextRequest } from "next/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import ProjectDetailPreviewPage from "@/app/(dev)/xem-thu/giai-ngan/du-an/page";
import DisbursementPreviewPage from "@/app/(dev)/xem-thu/giai-ngan/page";
import proxy from "@/proxy";

import { answer, PREVIEW_WRITE_REFUSAL } from "./fixture-fetch";
import { isDevPreviewPath } from "./preview-gate";

/**
 * The dev-only screenshot preview (ADR 0068 lần 6 #10) must not exist in production. Two independent
 * locks, each tested on its DENIED side, because each one alone would look fine on a developer's
 * machine while the other was broken:
 *   1. the pages 404 when NODE_ENV is "production" (what every `next build` inlines);
 *   2. the session guard (`proxy.ts`) lets the path through without a cookie ONLY outside production.
 */
afterEach(() => {
  vi.unstubAllEnvs();
});

const PAGES = [
  ["/xem-thu/giai-ngan", DisbursementPreviewPage],
  ["/xem-thu/giai-ngan/du-an", ProjectDetailPreviewPage],
] as const;

const params = (q: Record<string, string> = {}) => ({ searchParams: Promise.resolve(q) });

describe("preview pages", () => {
  it("DENIED in production: every preview page is a 404 — notFound(), before anything renders", async () => {
    vi.stubEnv("NODE_ENV", "production");
    for (const [path, Page] of PAGES) {
      const err = await Page(params({ modal: "hang-muc", tab: "chung-tu" })).then(
        () => null,
        (e: unknown) => e as { digest?: string },
      );
      expect(err, path).not.toBeNull();
      expect(err!.digest, path).toBe("NEXT_HTTP_ERROR_FALLBACK;404");
    }
  });

  it("in development: the page renders (the real shell around the preview body)", async () => {
    vi.stubEnv("NODE_ENV", "development");
    for (const [path, Page] of PAGES) expect(await Page(params()), path).toBeTruthy();
  });
});

describe("session guard (proxy.ts) on the preview path", () => {
  const request = (path: string, cookie?: string) => {
    const req = new NextRequest(new URL(`https://xa.example.test${path}`));
    if (cookie !== undefined) req.cookies.set(cookie, "x");
    return req;
  };

  it("DENIED in production: no cookie → redirected to sign-in like every other page", () => {
    vi.stubEnv("NODE_ENV", "production");
    for (const [path] of PAGES) {
      const res = proxy(request(path));
      expect(res.status, path).toBe(307);
      const to = new URL(res.headers.get("location")!);
      expect(to.pathname, path).toBe("/dang-nhap");
      expect(to.searchParams.get("tiep-tuc"), path).toBe(path);
    }
  });

  it("in development: the preview opens without a cookie", () => {
    vi.stubEnv("NODE_ENV", "development");
    for (const [path] of PAGES) {
      const res = proxy(request(path));
      expect(res.status, path).toBe(200);
      expect(res.headers.get("x-middleware-next"), path).toBe("1");
    }
  });

  it("in development, a real page is still guarded — the exception is the preview path only", () => {
    vi.stubEnv("NODE_ENV", "development");
    for (const path of ["/giai-ngan", "/xem-thu-khac", "/giai-ngan/du-an/01PREVIEWPRJ0000000000001"]) {
      expect(proxy(request(path)).status, path).toBe(307);
    }
  });

  it("the path test is by segment", () => {
    expect(isDevPreviewPath("/xem-thu")).toBe(true);
    expect(isDevPreviewPath("/xem-thu/giai-ngan")).toBe(true);
    expect(isDevPreviewPath("/xem-thu-khac")).toBe(false);
    expect(isDevPreviewPath("/giai-ngan")).toBe(false);
  });
});

describe("fixture answering machine", () => {
  const at = (path: string, method = "GET") => answer(method, new URL(`https://xa.example.test${path}`));

  it("answers the Giải ngân reads from fixtures, for the year asked", async () => {
    const list = await at("/api/v1/investment-projects?year=2031").json();
    expect(list.year).toBe(2031);
    expect(list.items).toHaveLength(6);
    expect(list.items.filter((p: { is_delayed: boolean }) => p.is_delayed)).toHaveLength(1);
    const sources = await at("/api/v1/funding-sources?year=2031").json();
    expect(sources.items).toHaveLength(2);
    const vouchers = await at("/api/v1/investment-projects/01PREVIEWPRJ0000000000001/disbursements").json();
    expect(vouchers.items.map((v: { status: string }) => v.status).sort()).toEqual(["da-khoa", "da-xac-nhan", "ke-toan-nhap"]);
  });

  it("DENIED: every write is refused with the server's error shape — never a faked success", async () => {
    for (const method of ["POST", "PATCH", "PUT", "DELETE"]) {
      const res = at("/api/v1/investment-projects", method);
      expect(res.status, method).toBe(409);
      expect((await res.json()).message, method).toBe(PREVIEW_WRITE_REFUSAL);
    }
  });

  it("an unknown route is a 404 answered locally, never passed to the network", () => {
    expect(at("/api/v1/tasks").status).toBe(404);
    expect(at("/api/v1/investment-projects/khong-co").status).toBe(404);
  });

  it("the fixture holds no real person: staff are 'Cán bộ …', no phone field", async () => {
    const staff = await at("/api/v1/staff-directory").json();
    for (const s of staff.items as { full_name: string }[]) expect(s.full_name).toMatch(/^Cán bộ [A-Z]$/);
    expect(JSON.stringify(staff)).not.toMatch(/phone|\b0\d{9}\b/);
  });
});
