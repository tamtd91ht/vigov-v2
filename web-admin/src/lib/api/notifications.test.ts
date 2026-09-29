import { afterEach, describe, expect, it, vi } from "vitest";

import {
  listNotifications,
  markAllNotificationsRead,
  markNotificationRead,
  notificationsPath,
  unreadCount,
} from "./notifications";

/**
 * The four bell routes. Isolation between staff is the SERVER's (it filters by the session's staff
 * code); what the client must guarantee is that it never names a recipient anywhere — no query, no
 * body field, no header — and only ever sends `{ read: true }`.
 */

function reply(status: number, body?: unknown) {
  return body === undefined
    ? new Response(null, { status })
    : new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

function stubFetch(answer: () => Response) {
  const fake = vi.fn(async () => answer());
  vi.stubGlobal("fetch", fake);
  return fake;
}

function call(fake: ReturnType<typeof stubFetch>, i = 0) {
  const [path, init] = fake.mock.calls[i] as unknown as [string, RequestInit];
  return {
    path,
    method: init.method,
    credentials: init.credentials,
    headers: new Headers(init.headers),
    body: init.body === undefined ? undefined : (JSON.parse(String(init.body)) as unknown),
  };
}

const NOTICE = {
  id: "01JNOTICE",
  kind: "qua-han",
  title: "Nhiệm vụ quá hạn: Rà soát hồ sơ",
  body: "Quá hạn 4 giờ làm việc",
  link: "/nhiem-vu?id=01JTASK",
  read: false,
  read_at: null,
  created_at: "2026-09-29T02:00:00Z",
};

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("listNotifications — GET /api/v1/notifications", () => {
  it("first page: relative path, same-origin, NO recipient / staff / tenant parameter", async () => {
    const fake = stubFetch(() => reply(200, { items: [NOTICE], next_cursor: "c2", has_more: true }));
    const r = await listNotifications(null, 20);
    expect(r).toEqual({ ok: true, duLieu: { items: [NOTICE], next_cursor: "c2", has_more: true } });
    const c = call(fake);
    expect(c.path).toBe("/api/v1/notifications?limit=20");
    expect(c.method).toBe("GET");
    expect(c.credentials).toBe("same-origin");
    expect(c.path).not.toMatch(/staff|recipient|tenant|ma_can_bo|code=/i);
  });

  it("next page carries the cursor; an empty cursor is not sent (it would be a 400)", () => {
    expect(notificationsPath("abc", 20)).toBe("/api/v1/notifications?limit=20&cursor=abc");
    expect(notificationsPath("", undefined)).toBe("/api/v1/notifications");
    expect(notificationsPath(null)).toBe("/api/v1/notifications");
  });

  it("401 is the server's sentence", async () => {
    stubFetch(() => reply(401, { code: "unauthorized", message: "Phiên làm việc không hợp lệ hoặc đã kết thúc." }));
    expect(await listNotifications(null)).toEqual({
      ok: false,
      thongBao: "Phiên làm việc không hợp lệ hoặc đã kết thúc.",
    });
  });
});

describe("unreadCount — GET /api/v1/notifications/unread-count", () => {
  it("returns the number", async () => {
    const fake = stubFetch(() => reply(200, { unread: 7 }));
    expect(await unreadCount()).toEqual({ ok: true, duLieu: 7 });
    expect(call(fake).path).toBe("/api/v1/notifications/unread-count");
  });

  it("a body without a number is a failure — never a guessed 0", async () => {
    stubFetch(() => reply(200, {}));
    expect((await unreadCount()).ok).toBe(false);
  });
});

describe("mark read — PATCH, always { read: true }", () => {
  it("one notice: PATCH /api/v1/notifications/{id}, id encoded", async () => {
    const fake = stubFetch(() => reply(200, { ...NOTICE, read: true, read_at: "2026-09-29T03:00:00Z" }));
    const r = await markNotificationRead("01J/../x");
    expect(r.ok).toBe(true);
    const c = call(fake);
    expect(c.path).toBe("/api/v1/notifications/01J%2F..%2Fx");
    expect(c.method).toBe("PATCH");
    expect(c.body).toEqual({ read: true });
  });

  it("another person's notice answers 404 — the sentence, nothing else", async () => {
    stubFetch(() => reply(404, { code: "not_found", message: "Không tìm thấy thông báo này." }));
    expect(await markNotificationRead("01JOTHER")).toEqual({ ok: false, thongBao: "Không tìm thấy thông báo này." });
  });

  it("Đọc hết: PATCH /api/v1/notifications { read: true } → { marked }", async () => {
    const fake = stubFetch(() => reply(200, { marked: 3 }));
    expect(await markAllNotificationsRead()).toEqual({ ok: true, duLieu: { marked: 3 } });
    const c = call(fake);
    expect(c.path).toBe("/api/v1/notifications");
    expect(c.method).toBe("PATCH");
    expect(c.body).toEqual({ read: true });
  });
});
