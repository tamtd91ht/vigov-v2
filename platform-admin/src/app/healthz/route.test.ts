import { afterEach, describe, expect, it, vi } from "vitest";

import { config } from "@/proxy";

import { GET } from "./route";

/**
 * The k8s probe connects with the POD IP as Host and no cookie. If this route ever called upstream
 * or needed the console's variables, or the proxy ever redirected it to `/dang-nhap`, every healthy
 * pod would fail its probe.
 */

afterEach(() => {
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
});

/**
 * Next compiles `matcher` with path-to-regexp; for this pattern (one capture group, no named
 * params) that compiles to the same regular expression as the string itself, anchored.
 */
function proxyMatches(path: string): boolean {
  return config.matcher.some((m) => new RegExp(`^${m}$`).test(path));
}

describe("/healthz", () => {
  it("answers 200 'ok' without any outbound call and without the console's variables", async () => {
    vi.stubEnv("PLATFORM_HTTP_ADDR", "");
    vi.stubEnv("OPERATOR_HOST", "");
    const outbound = vi.fn();
    vi.stubGlobal("fetch", outbound);

    const res = GET();

    expect(res.status).toBe(200);
    expect(await res.text()).toBe("ok");
    expect(res.headers.get("cache-control")).toBe("no-store");
    expect(outbound).not.toHaveBeenCalled();
  });

  it("is outside the proxy — never redirected to /dang-nhap for lacking a cookie", () => {
    expect(proxyMatches("/healthz")).toBe(false);
  });
});

describe("proxy matcher", () => {
  it("/api/ stays outside the proxy", () => {
    expect(proxyMatches("/api/v1/operator/communes")).toBe(false);
  });

  it("every page still goes through it — including names that only START with healthz or api", () => {
    expect(proxyMatches("/")).toBe(true);
    expect(proxyMatches("/xa")).toBe(true);
    expect(proxyMatches("/dang-nhap")).toBe(true);
    expect(proxyMatches("/healthz-fake")).toBe(true);
    expect(proxyMatches("/healthz/x")).toBe(true);
    expect(proxyMatches("/apixyz")).toBe(true);
  });
});
