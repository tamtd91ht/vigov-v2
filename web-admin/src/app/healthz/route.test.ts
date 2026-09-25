import { afterEach, describe, expect, it, vi } from "vitest";

import { config } from "@/proxy";

import { GET } from "./route";

/**
 * The k8s probe connects with the POD IP as Host — no commune. If this route ever consulted
 * the commune, or the middleware ever redirected it to `/dang-nhap`, every healthy pod would
 * fail its probe the moment the gateway works.
 */

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * Next compiles `matcher` with path-to-regexp; for this pattern (one capture group, no named
 * params) that compiles to the same regular expression as the string itself, anchored.
 */
function khopMiddleware(duong: string): boolean {
  return config.matcher.some((m) => new RegExp(`^${m}$`).test(duong));
}

describe("/healthz", () => {
  it("trả 200 'ok' mà KHÔNG gọi ra ngoài", async () => {
    const goiRa = vi.fn();
    vi.stubGlobal("fetch", goiRa);

    const phanHoi = GET();

    expect(phanHoi.status).toBe(200);
    expect(await phanHoi.text()).toBe("ok");
    expect(goiRa).not.toHaveBeenCalled();
  });

  it("nằm ngoài middleware — không bị chuyển về /dang-nhap vì thiếu cookie", () => {
    expect(khopMiddleware("/healthz")).toBe(false);
  });
});

describe("matcher của middleware", () => {
  it("/api/ nằm ngoài middleware — Go tự xác thực, và middleware sẽ cắt thân ở 10MB", () => {
    expect(khopMiddleware("/api/v1/staff")).toBe(false);
    expect(khopMiddleware("/api/v1/incoming-documents/x/attachments")).toBe(false);
  });

  it("các trang vẫn đi qua middleware — kể cả trang có tên BẮT ĐẦU bằng healthz hay api", () => {
    expect(khopMiddleware("/")).toBe(true);
    expect(khopMiddleware("/van-ban")).toBe(true);
    expect(khopMiddleware("/healthz-gia")).toBe(true);
    expect(khopMiddleware("/apixyz")).toBe(true);
  });
});
