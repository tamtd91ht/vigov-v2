import { NextRequest } from "next/server";
import { describe, expect, it } from "vitest";

import { SESSION_COOKIE } from "@/lib/session";

import proxy, { config } from "./proxy";

/**
 * `/basemap/*` is NOT a public route: it rides on the session gate of `proxy.ts` (cookie present or
 * redirect), and that is why `lib/may-chu/basemap.ts` declares no rate limit of its own (rule 13
 * invariant 7 applies to unauthenticated routes). Adding `basemap/` to the matcher's exclusions — the
 * way `maplibre/` is excluded — would silently make it one; these cases go red first.
 */
const matcher = new RegExp(`^${config.matcher[0]!.replace("/((?!", "/(?!").replace(").*)", ").*")}$`);

describe("proxy — /basemap/* stays behind the session gate", () => {
  it("the matcher covers /basemap paths", () => {
    expect(matcher.test("/basemap/vn-mainland.pmtiles")).toBe(true);
    expect(matcher.test("/basemap/fonts/Noto%20Sans%20Regular/0-255.pbf")).toBe(true);
    // sanity: the regex reading is right — an excluded path is excluded
    expect(matcher.test("/maplibre/maplibre-gl-worker.mjs")).toBe(false);
  });

  it("DENIED: no session cookie → redirected to /dang-nhap, never served", () => {
    const res = proxy(new NextRequest("https://xa-thu.example.test/basemap/vn-mainland.pmtiles"));
    expect(res.status).toBe(307);
    expect(new URL(res.headers.get("location")!).pathname).toBe("/dang-nhap");
  });

  it("ALLOWED: with the cookie the request passes to the route", () => {
    const res = proxy(
      new NextRequest("https://xa-thu.example.test/basemap/vn-mainland.pmtiles", {
        headers: { cookie: `${SESSION_COOKIE}=x` },
      }),
    );
    expect(res.headers.get("x-middleware-next")).toBe("1");
  });
});
