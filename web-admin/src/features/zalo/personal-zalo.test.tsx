// @vitest-environment jsdom
//
// jsdom: the card is a flow — a code appears, a timer polls, the poll ends on `linked`.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import { PersonalZaloCard } from "./personal-zalo-card";
import {
  CHANNEL_OFF,
  CODE_EXPIRED,
  formatCountdown,
  LINKED_NOW,
  NOT_LINKED,
  POLL_INTERVAL_MS,
  safeHttpsHref,
  secondsLeft,
  shouldPoll,
  TEST_SENT,
  UNLINK_QUESTION,
  UNLINKED,
} from "./personal-zalo-model";

/**
 * `/ca-nhan` → "Nhận nhắc việc qua Zalo" against a stubbed `fetch`. The server state is a small
 * object the stub reads, so "the bot received the code" is one assignment between two polls.
 */

const T0 = Date.parse("2026-10-05T03:00:00Z");

const server = {
  linked: false,
  channelEnabled: true,
  codeCalls: 0,
};

let calls: { method: string; url: string }[] = [];

function json(status: number, body: unknown): Response {
  return new Response(status === 204 ? null : JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

function route(url: string, method: string): Response {
  if (url === "/api/v1/zalo-links/current" && method === "GET") {
    return json(
      200,
      server.linked
        ? { linked: true, linked_at: "2026-10-05T03:01:00Z", bot_name: "Nhắc việc", channel_enabled: server.channelEnabled }
        : { linked: false, channel_enabled: server.channelEnabled },
    );
  }
  if (url === "/api/v1/zalo-links/current/pairing-codes" && method === "POST") {
    server.codeCalls += 1;
    return json(201, {
      code: "K7M2Q9XA",
      expires_at: new Date(Date.now() + 10 * 60_000).toISOString(),
      chat_url: "https://example.test/chat-bot",
    });
  }
  if (url === "/api/v1/zalo-links/current/test-messages" && method === "POST") return json(200, {});
  if (url === "/api/v1/zalo-links/current" && method === "DELETE") {
    server.linked = false;
    return json(204, null);
  }
  return json(404, { code: "not_found", message: "Không tìm thấy.", trace_id: "" });
}

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

let root: Root | null = null;
let host: HTMLDivElement | null = null;

beforeEach(() => {
  vi.useFakeTimers({ now: T0 });
  server.linked = false;
  server.channelEnabled = true;
  server.codeCalls = 0;
  calls = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init: RequestInit) => {
      const method = String(init.method ?? "GET");
      calls.push({ method, url });
      return route(url, method);
    }),
  );
});

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

/** Let pending fetch promises and the state they set settle. */
async function flush() {
  for (let i = 0; i < 5; i++) await act(async () => {});
}

async function mount(): Promise<HTMLDivElement> {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() => r.render(<PersonalZaloCard />));
  await flush();
  return host;
}

function button(el: HTMLElement, label: string): HTMLButtonElement {
  const b = [...el.querySelectorAll("button")].find((x) => x.textContent?.includes(label));
  if (!b) throw new Error(`no button "${label}" in: ${el.textContent}`);
  return b;
}

async function advance(ms: number) {
  await act(async () => {
    vi.advanceTimersByTime(ms);
  });
  await flush();
}

const polls = () => calls.filter((c) => c.method === "GET" && c.url === "/api/v1/zalo-links/current").length;

describe("model", () => {
  it("countdown, expiry, and when to poll", () => {
    const exp = new Date(T0 + 10 * 60_000).toISOString();
    expect(secondsLeft(exp, T0)).toBe(600);
    expect(formatCountdown(600)).toBe("10:00");
    expect(formatCountdown(65)).toBe("01:05");
    expect(secondsLeft(exp, T0 + 11 * 60_000)).toBe(0);
    expect(secondsLeft("not a date", T0)).toBe(0);
    expect(shouldPoll({ expires_at: exp }, false, T0)).toBe(true);
    expect(shouldPoll({ expires_at: exp }, true, T0)).toBe(false);
    expect(shouldPoll({ expires_at: exp }, false, T0 + 11 * 60_000)).toBe(false);
    expect(shouldPoll(null, false, T0)).toBe(false);
    expect(safeHttpsHref("javascript:alert(1)")).toBeNull();
    expect(safeHttpsHref("http://example.test")).toBeNull();
    expect(safeHttpsHref("https://example.test/x")).toBe("https://example.test/x");
  });
});

describe("PersonalZaloCard", () => {
  it("not linked → code shown big with countdown and Mở Zalo; polls every 5 s; stops on linked", async () => {
    const el = await mount();
    expect(el.textContent).toContain(NOT_LINKED);
    expect(polls()).toBe(1);

    act(() => button(el, "Lấy mã ghép nối").click());
    await flush();
    expect(server.codeCalls).toBe(1);
    expect(el.querySelector("output")?.textContent).toBe("K7M2Q9XA");
    expect(el.textContent).toContain("10:00");
    expect(el.textContent).toContain("Gửi mã này cho bot");
    const open = [...el.querySelectorAll("a")].find((a) => a.textContent?.includes("Mở Zalo"));
    expect(open?.getAttribute("href")).toBe("https://example.test/chat-bot");
    expect(open?.getAttribute("rel")).toContain("noopener");

    await advance(POLL_INTERVAL_MS);
    expect(polls()).toBe(2);
    expect(el.textContent).toContain("09:55");

    server.linked = true;
    await advance(POLL_INTERVAL_MS);
    expect(polls()).toBe(3);
    expect(el.textContent).toContain(LINKED_NOW);
    expect(el.textContent).toContain("Đã ghép nối");
    expect(el.querySelector("output")).toBeNull();

    // Linked: no more polling.
    await advance(POLL_INTERVAL_MS * 3);
    expect(polls()).toBe(3);
  });

  it("a code left unused expires: polling stops, the card says so and offers a new code", async () => {
    const el = await mount();
    act(() => button(el, "Lấy mã ghép nối").click());
    await flush();
    await advance(10 * 60_000 + 1000);
    const after = polls();
    expect(el.textContent).toContain(CODE_EXPIRED);
    expect(button(el, "Lấy mã mới")).toBeTruthy();
    await advance(POLL_INTERVAL_MS * 2);
    expect(polls()).toBe(after);
  });

  it("DENIED by the commune: channel off → explained, pairing disabled, no code request possible", async () => {
    server.channelEnabled = false;
    const el = await mount();
    expect(el.textContent).toContain(CHANNEL_OFF);
    expect(button(el, "Lấy mã ghép nối").disabled).toBe(true);
    act(() => button(el, "Lấy mã ghép nối").click());
    await flush();
    expect(server.codeCalls).toBe(0);
  });

  it("linked: test message, and unlink only after the confirmation", async () => {
    server.linked = true;
    const el = await mount();
    act(() => button(el, "Gửi tin thử").click());
    await flush();
    expect(el.textContent).toContain(TEST_SENT);

    act(() => button(el, "Gỡ ghép nối").click());
    await flush();
    expect(el.textContent).toContain(UNLINK_QUESTION);
    expect(calls.some((c) => c.method === "DELETE")).toBe(false);

    act(() => button(el, "Huỷ").click());
    expect(el.textContent).not.toContain(UNLINK_QUESTION);

    act(() => button(el, "Gỡ ghép nối").click());
    act(() => [...el.querySelectorAll("button")].filter((b) => b.textContent?.includes("Gỡ ghép nối"))[0]!.click());
    await flush();
    expect(calls.filter((c) => c.method === "DELETE")).toHaveLength(1);
    expect(el.textContent).toContain(UNLINKED);
    expect(el.textContent).toContain(NOT_LINKED);
  });

  it("linked while the commune is off: test disabled, unlinking still allowed", async () => {
    server.linked = true;
    server.channelEnabled = false;
    const el = await mount();
    expect(button(el, "Gửi tin thử").disabled).toBe(true);
    expect(button(el, "Gỡ ghép nối").disabled).toBe(false);
  });

  it("no request carries an identity or a chat id", async () => {
    server.linked = true;
    await mount();
    for (const c of calls) expect(c.url).not.toMatch(/staff|tenant|chat_id|CB-/i);
  });
});
