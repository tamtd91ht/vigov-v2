import { describe, expect, it } from "vitest";

import nextConfig, { SECURITY_HEADERS } from "../next.config";

import { buildDocumentCsp, createNonce } from "./lib/csp";

/** Rule 13 inv. 5 — TCVN 14423 §5.17.2.3. */

function header(name: string): string | undefined {
  return SECURITY_HEADERS.find((h) => h.key === name)?.value;
}

describe("next.config — static headers on every route", () => {
  it("applies to every path", async () => {
    const rules = await nextConfig.headers!();
    expect(rules).toEqual([{ source: "/:path*", headers: SECURITY_HEADERS }]);
  });

  it("sends CSP, HSTS, nosniff, Referrer-Policy and refuses framing both ways", () => {
    expect(header("Content-Security-Policy")).toContain("frame-ancestors 'none'");
    expect(header("Content-Security-Policy")).toContain("object-src 'none'");
    expect(header("Strict-Transport-Security")).toMatch(/max-age=(\d+)/);
    expect(Number(/max-age=(\d+)/.exec(header("Strict-Transport-Security") ?? "")?.[1])).toBeGreaterThanOrEqual(31536000);
    expect(header("X-Content-Type-Options")).toBe("nosniff");
    expect(header("Referrer-Policy")).toBe("no-referrer");
    expect(header("X-Frame-Options")).toBe("DENY");
  });

  it("bakes no public variable into the bundle", () => {
    expect(nextConfig.env).toBeUndefined();
    expect(nextConfig.poweredByHeader).toBe(false);
  });
});

describe("buildDocumentCsp — per-request policy of every page", () => {
  const prod = buildDocumentCsp("TESTNONCE", false);
  const directive = (csp: string, name: string) =>
    csp.split("; ").find((d) => d.startsWith(name + " ")) ?? "";

  it("production allows scripts only by nonce: no unsafe-inline, no unsafe-eval", () => {
    const script = directive(prod, "script-src");
    expect(script).toContain("'nonce-TESTNONCE'");
    expect(script).not.toContain("'unsafe-inline'");
    expect(script).not.toContain("'unsafe-eval'");
    expect(directive(prod, "style-src")).not.toContain("'unsafe-inline'");
  });

  it("refuses framing, plugins and base rewriting", () => {
    expect(prod).toContain("frame-ancestors 'none'");
    expect(prod).toContain("object-src 'none'");
    expect(prod).toContain("base-uri 'none'");
    expect(directive(prod, "connect-src")).toBe("connect-src 'self'");
  });

  it("development loosens only eval and inline styles", () => {
    const dev = buildDocumentCsp("N", true);
    expect(directive(dev, "script-src")).toContain("'unsafe-eval'");
    expect(directive(dev, "script-src")).not.toContain("'unsafe-inline'");
  });

  it("nonces are 128-bit and never repeat", () => {
    const seen = new Set(Array.from({ length: 200 }, () => createNonce()));
    expect(seen.size).toBe(200);
    for (const n of seen) expect(atob(n)).toHaveLength(16);
  });
});
