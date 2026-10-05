import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  createPairingCode,
  getCurrentZaloLink,
  getZaloChannelSettings,
  listZaloLinkedStaff,
  saveZaloChannelSettings,
  sendZaloTestMessage,
  unlinkCurrentZalo,
  ZALO_STATUS,
  type ZaloChannelSettings,
} from "./zalo";

/**
 * The Zalo client against a stubbed `fetch`: relative paths, no tenant, no staff identity in any
 * request, exact PUT keys, and the server's sentence on refusal.
 */

type Captured = { url: string; init: RequestInit };
let calls: Captured[] = [];
let answer: () => Response = () => json(200, {});

function json(status: number, body: unknown): Response {
  return new Response(status === 204 ? null : JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
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

const SETTINGS: ZaloChannelSettings = {
  is_enabled: true,
  kinds: ["sap-den-han", "qua-han"],
  quiet_start: "21:00",
  quiet_end: "06:00",
  overdue_start_after_days: 1,
  overdue_repeat_every_days: 2,
  updated_at: "2026-10-05T03:00:00Z",
  updated_by: "CB-00123",
};

describe("own link — everything from the session, nothing in the request", () => {
  it("GET current, POST pairing-codes, POST test-messages, DELETE current: relative paths, no body", async () => {
    answer = () => json(200, { linked: false, channel_enabled: true });
    await getCurrentZaloLink();
    answer = () => json(ZALO_STATUS.pairingCreated, { code: "ABCD2345", expires_at: "2026-10-05T03:10:00Z", chat_url: "https://example.test/c" });
    const code = await createPairingCode();
    expect(code).toEqual({ ok: true, duLieu: { code: "ABCD2345", expires_at: "2026-10-05T03:10:00Z", chat_url: "https://example.test/c" } });
    answer = () => json(ZALO_STATUS.testSent, {});
    expect(await sendZaloTestMessage()).toEqual({ ok: true, duLieu: null });
    answer = () => json(ZALO_STATUS.unlinked, null);
    expect(await unlinkCurrentZalo()).toEqual({ ok: true, duLieu: null });

    expect(calls.map((c) => [String(c.init.method), c.url])).toEqual([
      ["GET", "/api/v1/zalo-links/current"],
      ["POST", "/api/v1/zalo-links/current/pairing-codes"],
      ["POST", "/api/v1/zalo-links/current/test-messages"],
      ["DELETE", "/api/v1/zalo-links/current"],
    ]);
    for (const c of calls) {
      expect(c.init.body).toBeUndefined();
      expect(c.init.credentials).toBe("same-origin");
      expect(c.url).not.toMatch(/tenant|staff|CB-|chat/i);
    }
  });

  it("a refusal surfaces the server's sentence, not a code", async () => {
    answer = () => json(409, { code: "kenh-zalo-dang-tat", message: "Xã chưa bật kênh Zalo.", trace_id: "01JTRACE" });
    const r = await createPairingCode();
    expect(r).toEqual({ ok: false, thongBao: "Xã chưa bật kênh Zalo." });
  });
});

describe("commune settings and linked list", () => {
  it("PUT sends exactly the six editable keys — never updated_at / updated_by", async () => {
    answer = () => json(ZALO_STATUS.settingsSaved, SETTINGS);
    // The whole read shape passed in, `updated_*` included: they must not travel back up.
    await saveZaloChannelSettings(SETTINGS);
    const c = calls.at(-1)!;
    expect(c.url).toBe("/api/v1/zalo-channel-settings");
    expect(c.init.method).toBe("PUT");
    expect(JSON.parse(String(c.init.body))).toEqual({
      is_enabled: true,
      kinds: ["sap-den-han", "qua-han"],
      quiet_start: "21:00",
      quiet_end: "06:00",
      overdue_start_after_days: 1,
      overdue_repeat_every_days: 2,
    });
  });

  it("GET settings and GET zalo-links (items unwrapped)", async () => {
    answer = () => json(200, SETTINGS);
    expect(await getZaloChannelSettings()).toEqual({ ok: true, duLieu: SETTINGS });
    answer = () => json(200, { items: [{ staff_code: "CB-00123", staff_name: "Nguyễn Văn A", linked_at: "2026-10-05T03:00:00Z" }] });
    const r = await listZaloLinkedStaff();
    expect(r.ok && r.duLieu).toHaveLength(1);
    expect(calls.map((c) => c.url)).toEqual(["/api/v1/zalo-channel-settings", "/api/v1/zalo-links"]);
  });

  it("403 on the admin routes is a sentence, never an empty list", async () => {
    answer = () => json(403, { code: "forbidden", message: "Bạn không có quyền thực hiện thao tác này.", trace_id: "" });
    expect(await listZaloLinkedStaff()).toEqual({ ok: false, thongBao: "Bạn không có quyền thực hiện thao tác này." });
  });
});
