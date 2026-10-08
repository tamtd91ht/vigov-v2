import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  checkCommuneZaloBot,
  createPairingCode,
  getCommuneZaloBot,
  getCurrentZaloLink,
  getZaloChannelSettings,
  listZaloLinkedStaff,
  registerCommuneZaloWebhook,
  retireCommuneZaloBot,
  saveCommuneZaloBot,
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
  kinds: ["nhiem-vu.sap-den-han", "nhiem-vu.qua-han"],
  quiet_start: "21:00",
  quiet_end: "06:00",
  overdue_start_after_days: 1,
  overdue_repeat_every_days: 2,
  updated_at: "2026-10-05T03:00:00Z",
  updated_by: "CB-00123",
  supported_events: ["nhiem-vu.sap-den-han", "nhiem-vu.qua-han", "ban-tin-tuan"],
  platform_ready: true,
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
  it("PUT sends exactly the six editable keys — never updated_*, supported_events, platform_ready", async () => {
    answer = () => json(ZALO_STATUS.settingsSaved, SETTINGS);
    // The whole read shape passed in, `updated_*` included: they must not travel back up.
    await saveZaloChannelSettings(SETTINGS);
    const c = calls.at(-1)!;
    expect(c.url).toBe("/api/v1/zalo-channel-settings");
    expect(c.init.method).toBe("PUT");
    expect(JSON.parse(String(c.init.body))).toEqual({
      is_enabled: true,
      kinds: ["nhiem-vu.sap-den-han", "nhiem-vu.qua-han"],
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

describe("the commune's own bot — zalo-bots/current", () => {
  const TOKEN_FAKE = "000000000:fake-token-for-tests";

  it("PUT: token sent VERBATIM only when typed; no other key goes up", async () => {
    answer = () => json(200, { bot: {}, adopted: true, ended_link_count: 3 });
    await saveCommuneZaloBot({ bot_token: ` ${TOKEN_FAKE}`, bot_name: "Bot Xã", chat_url: "https://zalo.me/1" });
    await saveCommuneZaloBot({ bot_token: "", bot_name: "Bot Xã", chat_url: "https://zalo.me/1" });
    expect(calls.map((c) => [String(c.init.method), c.url])).toEqual([
      ["PUT", "/api/v1/zalo-bots/current"],
      ["PUT", "/api/v1/zalo-bots/current"],
    ]);
    expect(JSON.parse(String(calls[0]!.init.body))).toEqual({
      bot_token: ` ${TOKEN_FAKE}`,
      bot_name: "Bot Xã",
      chat_url: "https://zalo.me/1",
    });
    // Empty = keep the live token: the key is ABSENT, not "".
    expect(JSON.parse(String(calls[1]!.init.body))).toEqual({ bot_name: "Bot Xã", chat_url: "https://zalo.me/1" });
    for (const c of calls) expect(c.url).not.toContain(TOKEN_FAKE);
  });

  it("GET current, POST check, POST webhook: relative paths, no body; webhook secret handed back as is", async () => {
    answer = () => json(200, { has_own_bot: false, bot: null, live_link_count: 2 });
    expect(await getCommuneZaloBot()).toEqual({ ok: true, duLieu: { has_own_bot: false, bot: null, live_link_count: 2 } });
    answer = () => json(200, { result: "thanh-cong", checked_at: "2026-10-08T03:00:00Z" });
    expect((await checkCommuneZaloBot()).ok).toBe(true);
    answer = () => json(200, { result: "thanh-cong", url: "https://xa.example.test/api/v1/zalo-bot-updates", secret: "s3cr3t-fake" });
    const w = await registerCommuneZaloWebhook();
    expect(w.ok && w.duLieu.secret).toBe("s3cr3t-fake");
    expect(calls.map((c) => [String(c.init.method), c.url])).toEqual([
      ["GET", "/api/v1/zalo-bots/current"],
      ["POST", "/api/v1/zalo-bots/current/check"],
      ["POST", "/api/v1/zalo-bots/current/webhook"],
    ]);
    for (const c of calls) {
      expect(c.init.body).toBeUndefined();
      expect(c.init.credentials).toBe("same-origin");
    }
  });

  it("DELETE carries the reason in the BODY, never in the URL", async () => {
    answer = () => json(200, { retired: true, ended_link_count: 1, revoke_notice: "Thu hồi mã cũ." });
    const r = await retireCommuneZaloBot("Xã thôi dùng bot riêng");
    expect(r).toEqual({ ok: true, duLieu: { retired: true, ended_link_count: 1, revoke_notice: "Thu hồi mã cũ." } });
    const c = calls.at(-1)!;
    expect([c.init.method, c.url]).toEqual(["DELETE", "/api/v1/zalo-bots/current"]);
    expect(JSON.parse(String(c.init.body))).toEqual({ reason: "Xã thôi dùng bot riêng" });
  });

  it("a refusal is the server's sentence (409 zalo_bot_in_use)", async () => {
    answer = () =>
      json(409, { code: "zalo_bot_in_use", message: "Con bot này đang được dùng ở nơi khác trên nền tảng nên xã không dùng được.", trace_id: "" });
    expect(await saveCommuneZaloBot({ bot_token: TOKEN_FAKE, bot_name: "Bot X", chat_url: "https://zalo.me/1" })).toEqual({
      ok: false,
      thongBao: "Con bot này đang được dùng ở nơi khác trên nền tảng nên xã không dùng được.",
    });
  });
});
