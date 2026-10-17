import { afterEach, describe, expect, it, vi } from "vitest";

import {
  checkSmsAccount,
  createSmsAccount,
  listSmsAccounts,
  listSmsTemplates,
  retireSmsAccount,
  syncSmsTemplates,
  updateSmsAccount,
} from "./sms-accounts";

/**
 * The seven `sms-accounts` routes (ADR 0099 #2). The failures that matter are silent: a POST without
 * `Idempotency-Key` (comms refuses — idem.Required), an empty key sent on PATCH, a key echoed somewhere,
 * an id not encoded into the path, and a refusal whose `code` the tab needs but never receives.
 */

function stubFetch(reply: Response) {
  const fake = vi.fn(async (_path: string, _init?: RequestInit) => reply);
  vi.stubGlobal("fetch", fake);
  return fake;
}

function json(body: unknown, status: number): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

function sentBody(fake: ReturnType<typeof stubFetch>): Record<string, unknown> | undefined {
  const raw = fake.mock.calls[0]?.[1]?.body;
  return typeof raw === "string" ? (JSON.parse(raw) as Record<string, unknown>) : undefined;
}

function sentHeaders(fake: ReturnType<typeof stubFetch>): Record<string, string> {
  return (fake.mock.calls[0]?.[1]?.headers ?? {}) as Record<string, string>;
}

const ACCOUNT = {
  id: "01JSMS",
  provider: "esms",
  label: "Tài khoản chính",
  brandname: "UBND XA",
  is_default: true,
  set_at: "2026-10-17T01:00:00Z",
  set_by: "CB-00123",
  last_check: { at: "2026-10-17T01:00:00Z", result: "thanh-cong", balance: "36693.00" },
  created_at: "2026-10-17T01:00:00Z",
};

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("GET routes", () => {
  it("list: GET /api/v1/sms-accounts, unwraps `items`, names no commune", async () => {
    const fake = stubFetch(json({ items: [ACCOUNT] }, 200));
    const r = await listSmsAccounts();
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/sms-accounts");
    expect(fake.mock.calls[0]?.[1]?.method).toBe("GET");
    expect(r).toEqual({ ok: true, duLieu: [ACCOUNT] });
  });

  it("templates: the id is encoded into the path", async () => {
    const fake = stubFetch(json({ items: [] }, 200));
    expect(await listSmsTemplates("a/b")).toEqual({ ok: true, duLieu: [] });
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/sms-accounts/a%2Fb/templates");
  });
});

describe("POST /api/v1/sms-accounts", () => {
  it("body built key by key, keys VERBATIM, Idempotency-Key from the caller, 201", async () => {
    const fake = stubFetch(json(ACCOUNT, 201));
    const body = { label: "A", api_key: " k1 ", secret_key: "s1", brandname: "UBND XA", is_default: true, extra: "x" };
    const r = await createSmsAccount(body, "idem-1");
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/sms-accounts");
    expect(fake.mock.calls[0]?.[1]?.method).toBe("POST");
    expect(sentHeaders(fake)["Idempotency-Key"]).toBe("idem-1");
    expect(sentBody(fake)).toEqual({ label: "A", api_key: " k1 ", secret_key: "s1", brandname: "UBND XA", is_default: true });
    expect(r.ok).toBe(true);
  });

  it("422 sms_key_rejected: the server's sentence verbatim AND its code (the tab places it under the keys)", async () => {
    const sentence = "eSMS không nhận ApiKey / SecretKey này. Hãy chép lại khoá trong trang quản trị eSMS. Chưa có gì được lưu.";
    stubFetch(json({ code: "sms_key_rejected", message: sentence, trace_id: "t" }, 422));
    expect(await createSmsAccount({ label: "A", api_key: "k", secret_key: "s", brandname: "B", is_default: false }, "k")).toEqual({
      ok: false,
      thongBao: sentence,
      code: "sms_key_rejected",
    });
  });
});

describe("PATCH /api/v1/sms-accounts/{id}", () => {
  it("only the keys set travel; an EMPTY key is not sent; no Idempotency-Key", async () => {
    const fake = stubFetch(json(ACCOUNT, 200));
    await updateSmsAccount("01J/S", { label: "Mới", api_key: "", secret_key: "" });
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/sms-accounts/01J%2FS");
    expect(fake.mock.calls[0]?.[1]?.method).toBe("PATCH");
    expect(sentBody(fake)).toEqual({ label: "Mới" });
    expect(sentHeaders(fake)).not.toHaveProperty("Idempotency-Key");
  });

  it("is_default false IS sent (un-setting the default is a choice)", async () => {
    const fake = stubFetch(json(ACCOUNT, 200));
    await updateSmsAccount("x", { is_default: false });
    expect(sentBody(fake)).toEqual({ is_default: false });
  });
});

describe("DELETE · check · sync", () => {
  it("retire: DELETE with {reason}, 200 {retired}", async () => {
    const fake = stubFetch(json({ retired: true }, 200));
    expect(await retireSmsAccount("x", "Hết hợp đồng")).toEqual({ ok: true, duLieu: { retired: true } });
    expect(fake.mock.calls[0]?.[1]?.method).toBe("DELETE");
    expect(sentBody(fake)).toEqual({ reason: "Hết hợp đồng" });
  });

  it("check: POST with NO body; a failing check is still a 200 carrying its class", async () => {
    const fake = stubFetch(json({ result: "sai-khoa", checked_at: "2026-10-17T02:00:00Z", balance: null }, 200));
    const r = await checkSmsAccount("x");
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/sms-accounts/x/check");
    expect(fake.mock.calls[0]?.[1]?.body).toBeUndefined();
    expect(r).toEqual({ ok: true, duLieu: { result: "sai-khoa", checked_at: "2026-10-17T02:00:00Z", balance: null } });
  });

  it("sync: POST …/templates/sync with no body; 503 encryption_not_configured keeps its code", async () => {
    const fake = stubFetch(json({ code: "encryption_not_configured", message: "Nền tảng chưa cấu hình khoá." }, 503));
    const r = await syncSmsTemplates("x");
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/sms-accounts/x/templates/sync");
    expect(fake.mock.calls[0]?.[1]?.body).toBeUndefined();
    expect(r).toEqual({ ok: false, thongBao: "Nền tảng chưa cấu hình khoá.", code: "encryption_not_configured" });
  });
});
