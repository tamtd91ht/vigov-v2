import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  attachZaloOATemplate,
  checkCommuneZaloOA,
  createZaloOAAuthorizationLink,
  disconnectCommuneZaloOA,
  listCommuneZaloOAs,
  listZaloOATemplates,
  syncZaloOATemplates,
  ZALO_OA_NOT_CONFIGURED,
  ZALO_OA_STATUS,
} from "./zalo-oa";

/**
 * The commune-OA client (ADR 0098, several OAs per commune since ADR 0099 #1) against a stubbed `fetch`:
 * relative paths, the OA named by its row id in the PATH, no tenant and no staff identity in any request,
 * exact bodies, and the server's sentence on refusal.
 */

type Captured = { url: string; init: RequestInit };
let calls: Captured[] = [];
let answer: () => Response = () => json(200, {});

function json(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

beforeEach(() => {
  calls = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init: RequestInit) => {
      calls.push({ url, init });
      return answer();
    }),
  );
});

afterEach(() => vi.unstubAllGlobals());

const LINK_FAKE = "https://oauth.example.test/v4/oa/permission?app_id=1&state=fake-state-0000";

describe("OA routes — commune from Host, officer from the session, the OA by row id", () => {
  it("GET list, POST authorization-links, POST {id}/check: relative paths, no body, same-origin", async () => {
    answer = () => json(200, { configured: true, items: [] });
    expect(await listCommuneZaloOAs()).toEqual({ ok: true, duLieu: { configured: true, items: [] } });
    answer = () => json(ZALO_OA_STATUS.linkCreated, { url: LINK_FAKE, expires_at: "2026-10-16T03:10:00Z" });
    const link = await createZaloOAAuthorizationLink();
    // Handed back as is, to be opened by the caller.
    expect(link).toEqual({ ok: true, duLieu: { url: LINK_FAKE, expires_at: "2026-10-16T03:10:00Z" } });
    answer = () => json(ZALO_OA_STATUS.checked, { result: "thanh-cong", checked_at: "2026-10-16T03:00:00Z", oa_name: "UBND Xã Thử" });
    expect(await checkCommuneZaloOA("01OA/1")).toMatchObject({ ok: true, duLieu: { result: "thanh-cong" } });

    expect(calls.map((c) => [String(c.init.method), c.url])).toEqual([
      ["GET", "/api/v1/zalo-oas"],
      ["POST", "/api/v1/zalo-oas/authorization-links"],
      ["POST", "/api/v1/zalo-oas/01OA%2F1/check"],
    ]);
    for (const c of calls) {
      expect(c.init.body).toBeUndefined();
      expect(c.init.credentials).toBe("same-origin");
      expect(c.url).not.toMatch(/tenant|staff|CB-|current/i);
    }
  });

  it("authorization-links 503 zalo_oa_not_configured: the server's sentence AND its code (the button keys on it)", async () => {
    answer = () =>
      json(503, { code: ZALO_OA_NOT_CONFIGURED, message: "Nền tảng chưa khai ứng dụng Zalo dùng chung.", trace_id: "" });
    expect(await createZaloOAAuthorizationLink()).toEqual({
      ok: false,
      thongBao: "Nền tảng chưa khai ứng dụng Zalo dùng chung.",
      code: ZALO_OA_NOT_CONFIGURED,
    });
  });

  it("DELETE {id} carries the reason in the BODY, never in the URL", async () => {
    answer = () => json(200, { disconnected: true, detached_template_count: 2, revoke_notice: "Thu hồi quyền trong OA Manager." });
    const r = await disconnectCommuneZaloOA("01OAA", "Xã đổi OA");
    expect(r).toMatchObject({ ok: true, duLieu: { disconnected: true, detached_template_count: 2 } });
    const c = calls.at(-1)!;
    expect([c.init.method, c.url]).toEqual(["DELETE", "/api/v1/zalo-oas/01OAA"]);
    expect(JSON.parse(String(c.init.body))).toEqual({ reason: "Xã đổi OA" });
  });

  it("an id that is not a live OA of the commune: 404 not_found, the server's one sentence", async () => {
    answer = () => json(404, { code: "not_found", message: "Không tìm thấy OA này trong các OA đang nối của xã.", trace_id: "" });
    expect(await disconnectCommuneZaloOA("01OAX", "Lần hai")).toEqual({
      ok: false,
      thongBao: "Không tìm thấy OA này trong các OA đang nối của xã.",
    });
  });

  it("check 409 zalo_oa_needs_reauthorization: the server's sentence, verbatim", async () => {
    answer = () => json(409, { code: "zalo_oa_needs_reauthorization", message: "OA của xã cần uỷ quyền lại.", trace_id: "" });
    expect(await checkCommuneZaloOA("01OAA")).toEqual({ ok: false, thongBao: "OA của xã cần uỷ quyền lại." });
  });
});

describe("templates", () => {
  it("GET {id}/templates, POST {id}/templates/sync (no body)", async () => {
    answer = () => json(200, { items: [], synced_at: null });
    expect(await listZaloOATemplates("01OAA")).toEqual({ ok: true, duLieu: { items: [], synced_at: null } });
    answer = () => json(200, { loaded: 3, enabled: 2, matching: 1, changed: 3, synced_at: "2026-10-16T03:00:00Z" });
    expect(await syncZaloOATemplates("01OAA")).toMatchObject({ ok: true, duLieu: { loaded: 3 } });
    expect(calls.map((c) => [String(c.init.method), c.url, c.init.body])).toEqual([
      ["GET", "/api/v1/zalo-oas/01OAA/templates", undefined],
      ["POST", "/api/v1/zalo-oas/01OAA/templates/sync", undefined],
    ]);
  });

  it("PATCH zalo-oa-templates/{id}: the id is encoded into the path; `milestone` is ALWAYS a key, null = detach", async () => {
    answer = () => json(200, { id: "01T/1" });
    await attachZaloOATemplate("01T/1", "da-tiep-nhan");
    await attachZaloOATemplate("01T/1", null);
    expect(calls.map((c) => [String(c.init.method), c.url])).toEqual([
      ["PATCH", "/api/v1/zalo-oa-templates/01T%2F1"],
      ["PATCH", "/api/v1/zalo-oa-templates/01T%2F1"],
    ]);
    expect(JSON.parse(String(calls[0]!.init.body))).toEqual({ milestone: "da-tiep-nhan" });
    expect(JSON.parse(String(calls[1]!.init.body))).toEqual({ milestone: null });
  });

  it("PATCH 409 milestone_taken: the server's sentence", async () => {
    answer = () => json(409, { code: "milestone_taken", message: "Mốc này đã có mẫu khác gắn.", trace_id: "" });
    expect(await attachZaloOATemplate("01T", "da-dong")).toEqual({ ok: false, thongBao: "Mốc này đã có mẫu khác gắn." });
  });
});
