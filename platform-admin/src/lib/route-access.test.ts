import { NextRequest } from "next/server";
import { describe, expect, it } from "vitest";

import proxy from "../proxy";

import { needsSignIn } from "./route-access";

const none = () => false;
const all = () => true;

describe("needsSignIn — denied cases", () => {
  it("a console route without the cookie is sent to sign-in", () => {
    expect(needsSignIn("/xa", "s", none)).toBe(true);
    expect(needsSignIn("/", "s", none)).toBe(true);
  });

  it("while the cookie name is undecided, EVERY console route is denied, cookies or not", () => {
    expect(needsSignIn("/xa", null, all)).toBe(true);
  });

  it("an unknown path is protected too — the list is an allow-list", () => {
    expect(needsSignIn("/khong-co", "s", none)).toBe(true);
  });
});

describe("needsSignIn — allowed cases", () => {
  it("the sign-in page is public", () => {
    expect(needsSignIn("/dang-nhap", null, none)).toBe(false);
  });

  it("a console route with the cookie present passes (the server still validates it)", () => {
    expect(needsSignIn("/xa", "s", (n) => n === "s")).toBe(false);
  });
});

describe("proxy", () => {
  it("redirects /xa to /dang-nhap today, with the CSP on the redirect", () => {
    const res = proxy(new NextRequest("https://admin.example.gov.vn/xa?x=1"));
    expect(res.status).toBe(307);
    expect(new URL(res.headers.get("location") ?? "").pathname).toBe("/dang-nhap");
    expect(new URL(res.headers.get("location") ?? "").search).toBe("");
    expect(res.headers.get("content-security-policy")).toContain("frame-ancestors 'none'");
  });

  it("lets /dang-nhap through with a fresh nonce policy", () => {
    const a = proxy(new NextRequest("https://admin.example.gov.vn/dang-nhap"));
    const b = proxy(new NextRequest("https://admin.example.gov.vn/dang-nhap"));
    expect(a.headers.get("location")).toBeNull();
    expect(a.headers.get("content-security-policy")).toMatch(/'nonce-[A-Za-z0-9+/=]+'/);
    expect(a.headers.get("content-security-policy")).not.toBe(b.headers.get("content-security-policy"));
  });
});
