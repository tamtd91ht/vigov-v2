import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  addDomain,
  ApiError,
  attachMiniApp,
  beginEnrollment,
  changePassword,
  completeEnrollment,
  correctName,
  createCommune,
  getCommune,
  getCurrentOperator,
  listCommunes,
  listProvinces,
  parseRetryAfter,
  regenerateRecoveryCodes,
  setActivation,
  setPrimaryDomain,
  signIn,
  signOut,
} from "./api";

/**
 * The client against a stubbed `fetch`: what leaves the browser (path, method, EXACT body keys)
 * and how every refusal is mapped. Body keys are compared exactly because service-platform decodes
 * with `DisallowUnknownFields` — one extra key is a 400 on every call.
 */

type Captured = { url: string; init: RequestInit };
let calls: Captured[] = [];
let answer: () => Response = () => json(200, {});

function json(status: number, body: unknown, headers: Record<string, string> = {}): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json", ...headers },
  });
}

beforeEach(() => {
  calls = [];
  answer = () => json(200, {});
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init: RequestInit) => {
      calls.push({ url, init });
      return answer();
    }),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
});

function last(): { url: string; method: string; body: unknown } {
  const c = calls.at(-1);
  if (!c) throw new Error("no call made");
  return {
    url: c.url,
    method: String(c.init.method),
    body: typeof c.init.body === "string" ? JSON.parse(c.init.body) : undefined,
  };
}

const PASSWORD = "mat-khau-thu-nghiem";

describe("requests — path, method and exactly the documented body", () => {
  it.each([
    [
      "sign in with TOTP",
      () => signIn({ email: "op@example.vn", password: PASSWORD, secondFactor: { kind: "totp", code: "123456" } }),
      "POST",
      "/api/v1/operator-sessions",
      { email: "op@example.vn", password: PASSWORD, totp_code: "123456" },
    ],
    [
      "sign in with a recovery code sends ONLY recovery_code",
      () => signIn({ email: "op@example.vn", password: PASSWORD, secondFactor: { kind: "recovery", code: "abcd-efgh" } }),
      "POST",
      "/api/v1/operator-sessions",
      { email: "op@example.vn", password: PASSWORD, recovery_code: "abcd-efgh" },
    ],
    [
      "begin enrolment",
      () => beginEnrollment({ email: "op@example.vn", temporaryPassword: PASSWORD }),
      "POST",
      "/api/v1/operator-enrollments",
      { email: "op@example.vn", temporary_password: PASSWORD },
    ],
    [
      "complete enrolment",
      () => completeEnrollment({ email: "op@example.vn", temporaryPassword: PASSWORD, newPassword: "moi", totpCode: "654321" }),
      "POST",
      "/api/v1/operator-enrollments/completion",
      { email: "op@example.vn", temporary_password: PASSWORD, new_password: "moi", totp_code: "654321" },
    ],
    [
      "change password",
      () => changePassword({ currentPassword: PASSWORD, newPassword: "moi", totpCode: "111111" }),
      "PUT",
      "/api/v1/operators/current/password",
      { current_password: PASSWORD, new_password: "moi", totp_code: "111111" },
    ],
    [
      "regenerate recovery codes",
      () => regenerateRecoveryCodes({ totpCode: "222222" }),
      "POST",
      "/api/v1/operators/current/recovery-codes",
      { totp_code: "222222" },
    ],
    [
      "create a commune",
      () => createCommune({ name: "Xã A", provinceId: "01", primaryDomain: "xa-a.example.vn" }),
      "POST",
      "/api/v1/communes",
      { name: "Xã A", province_id: "01", primary_domain: "xa-a.example.vn" },
    ],
    [
      "add a domain",
      () => addDomain("01HZZZ", "khac.example.vn"),
      "POST",
      "/api/v1/communes/01HZZZ/domains",
      { domain: "khac.example.vn" },
    ],
    [
      "set the primary domain",
      () => setPrimaryDomain("01HZZZ", "khac.example.vn"),
      "PUT",
      "/api/v1/communes/01HZZZ/primary-domain",
      { domain: "khac.example.vn" },
    ],
    [
      "correct the name",
      () => correctName("01HZZZ", { name: "Xã Á", reason: "thiếu dấu" }),
      "PUT",
      "/api/v1/communes/01HZZZ/name",
      { name: "Xã Á", reason: "thiếu dấu" },
    ],
    [
      "deactivate — `active` is always sent, never left out",
      () => setActivation("01HZZZ", { active: false, reason: "tạm dừng" }),
      "PUT",
      "/api/v1/communes/01HZZZ/activation",
      { active: false, reason: "tạm dừng" },
    ],
    [
      "attach a Mini App without a note omits `note`",
      () => attachMiniApp("01HZZZ", { appId: "123456789" }),
      "POST",
      "/api/v1/communes/01HZZZ/mini-apps",
      { app_id: "123456789" },
    ],
    [
      "attach a Mini App with a note",
      () => attachMiniApp("01HZZZ", { appId: "123456789", note: "ghi chú" }),
      "POST",
      "/api/v1/communes/01HZZZ/mini-apps",
      { app_id: "123456789", note: "ghi chú" },
    ],
  ])("%s", async (_name, run, method, url, body) => {
    await run();
    expect(calls).toHaveLength(1);
    expect(last()).toEqual({ url, method, body });
    expect(calls[0]?.init.credentials).toBe("same-origin");
  });

  it("reads carry no body", async () => {
    answer = () => json(200, { items: [] });
    await getCurrentOperator();
    await getCommune("01HZZZ");
    await listProvinces();
    for (const c of calls) {
      expect(c.init.method).toBe("GET");
      expect(c.init.body).toBeUndefined();
    }
    expect(calls.map((c) => c.url)).toEqual([
      "/api/v1/operator-sessions/current",
      "/api/v1/communes/01HZZZ",
      "/api/v1/provinces",
    ]);
  });

  it("sign-out is DELETE on the current session, with no id from anywhere", async () => {
    answer = () => new Response(null, { status: 204 });
    await expect(signOut()).resolves.toBeUndefined();
    expect(last()).toEqual({ url: "/api/v1/operator-sessions/current", method: "DELETE", body: undefined });
  });

  it("list pages by limit and cursor only", async () => {
    answer = () => json(200, { items: [], next_cursor: "", has_more: false });
    await listCommunes({ limit: 20 });
    await listCommunes({ limit: 20, cursor: "abc+/=" });
    expect(calls[0]?.url).toBe("/api/v1/communes?limit=20");
    expect(calls[1]?.url).toBe("/api/v1/communes?limit=20&cursor=abc%2B%2F%3D");
  });

  it("a commune id is escaped into the path, never spliced raw", async () => {
    await getCommune("../operator-sessions/current");
    expect(last().url).toBe("/api/v1/communes/..%2Foperator-sessions%2Fcurrent");
  });

  it("no credential ever reaches a URL", async () => {
    await signIn({ email: "op@example.vn", password: PASSWORD, secondFactor: { kind: "totp", code: "123456" } });
    await beginEnrollment({ email: "op@example.vn", temporaryPassword: PASSWORD });
    await completeEnrollment({ email: "op@example.vn", temporaryPassword: PASSWORD, newPassword: PASSWORD + "2", totpCode: "1" });
    await changePassword({ currentPassword: PASSWORD, newPassword: PASSWORD + "2", totpCode: "1" });
    for (const c of calls) {
      expect(c.url).not.toContain("?");
      expect(c.url).not.toContain(PASSWORD);
      expect(c.url).not.toContain("op@example.vn");
    }
  });
});

describe("errors — the server's shape, mapped once", () => {
  it("401 carries code, message and trace id", async () => {
    answer = () => json(401, { code: "sign_in_refused", message: "x", trace_id: "t-1" });
    const err = await signIn({ email: "a", password: "b", secondFactor: { kind: "totp", code: "1" } }).catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err).toMatchObject({ status: 401, code: "sign_in_refused", traceId: "t-1", retryAfterSeconds: null });
  });

  it("429 reads Retry-After in whole seconds", async () => {
    answer = () => json(429, { code: "rate_limited", message: "", trace_id: "" }, { "Retry-After": "60" });
    const err = await signIn({ email: "a", password: "b", secondFactor: { kind: "totp", code: "1" } }).catch((e) => e);
    expect(err).toMatchObject({ status: 429, code: "rate_limited", retryAfterSeconds: 60 });
  });

  it("422 new_password_rejected keeps the rule and the server's bounds", async () => {
    answer = () =>
      json(422, { code: "new_password_rejected", message: "", trace_id: "", problem: "too_short", min_length: 14, max_length: 128 });
    const err = await changePassword({ currentPassword: "a", newPassword: "b", totpCode: "1" }).catch((e) => e);
    expect(err).toMatchObject({ status: 422, passwordRejection: { problem: "too_short", minLength: 14, maxLength: 128 } });
  });

  it("a body that is not the error shape keeps the status and names no code", async () => {
    answer = () => new Response("<html>bad gateway</html>", { status: 502 });
    const err = await getCommune("01HZZZ").catch((e) => e);
    expect(err).toMatchObject({ status: 502, code: "unexpected_response" });
  });

  it("no answer at all is status 0 — never mistaken for 401", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => {
        throw new TypeError("Failed to fetch");
      }),
    );
    const err = await getCommune("01HZZZ").catch((e) => e);
    expect(err).toMatchObject({ status: 0, code: "network_error" });
  });

  it("a failed write is NOT retried", async () => {
    answer = () => json(503, { code: "operator_auth_unavailable", message: "", trace_id: "" });
    await createCommune({ name: "Xã A", provinceId: "01", primaryDomain: "a.example.vn" }).catch(() => {});
    await setActivation("01HZZZ", { active: false, reason: "r" }).catch(() => {});
    expect(calls).toHaveLength(2);
  });

  it("parseRetryAfter accepts whole seconds only", () => {
    expect(parseRetryAfter("60")).toBe(60);
    expect(parseRetryAfter(" 5 ")).toBe(5);
    expect(parseRetryAfter("0")).toBeNull();
    expect(parseRetryAfter("Wed, 21 Oct 2026 07:28:00 GMT")).toBeNull();
    expect(parseRetryAfter(null)).toBeNull();
  });
});
