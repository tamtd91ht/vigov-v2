import { renderToString } from "react-dom/server";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  usePathname: () => "/zalo-bot",
  useRouter: () => ({ replace: vi.fn(), push: vi.fn(), refresh: vi.fn() }),
}));

import { NAV_ITEMS, SidebarView, visibleNavItems, ZALO_BOT_ITEM } from "@/components/sidebar";
import {
  ALL_OPS_KEYS,
  ApiError,
  checkSharedZaloBot,
  getSharedZaloBot,
  getSharedZaloBotWebhook,
  listZaloBotCommunes,
  setSharedZaloBot,
  setSharedZaloBotWebhook,
  type SharedZaloBot,
  type ZaloBotWebhook,
} from "@/lib/api";
import { canManageZaloBot, canReadConsole } from "@/lib/permissions";

import {
  buildBotChange,
  endedLinksText,
  outcomeText,
  RELINK_COUNT_MISSING,
  relinkQuestion,
  relinkStep,
  safeHttpsHref,
  UNKNOWN_OUTCOME,
  webhookSetText,
  zaloBotError,
} from "./zalo-bot-model";
import {
  CommuneUptakeView,
  DENIED_TEXT,
  NO_COMMUNE_ROWS,
  NOT_CONFIGURED_TITLE,
  RelinkConfirmBody,
  SharedBotView,
  TokenFormFields,
  WEBHOOK_ELSEWHERE,
  WEBHOOK_NEEDS_TOKEN,
  WebhookView,
  ZaloBotGate,
} from "./zalo-bot-screen";

const ZALO = "ops.zalo_bot.manage";
const OTHERS = ALL_OPS_KEYS.filter((k) => k !== ZALO);
const noop = () => {};

/** A fake token: printable, no `/ ? # %`, nothing real. */
const TOKEN_FAKE = "1234567890:zalo-bot-token-FAKE";

const CONFIGURED: SharedZaloBot = {
  configured: true,
  bot: {
    set_at: "2026-10-05T03:00:00Z",
    set_by: "VH-00001",
    has_token: true,
    bot_name: "Nhắc việc ViGov",
    chat_url: "https://zalo.me/chat-with-bot-fake",
  },
  last_check: { checked_at: "2026-10-05T04:00:00Z", outcome: "token-bi-tu-choi" },
};

// --- fetch stub -----------------------------------------------------------------------------------

type Captured = { url: string; init: RequestInit };
let calls: Captured[] = [];
let answer: () => Response = () => json(200, {});

function json(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
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

afterEach(() => vi.unstubAllGlobals());

function last() {
  const c = calls.at(-1)!;
  return {
    url: c.url,
    method: String(c.init.method),
    body: typeof c.init.body === "string" ? (JSON.parse(c.init.body) as unknown) : undefined,
  };
}

// --- permissions: denied case first ---------------------------------------------------------------

describe("ops.zalo_bot.manage gates the whole section, reads included (ADR 0074 #3)", () => {
  it("DENIED: no key, or any of the seven other keys, gives no menu entry and no page", () => {
    expect(canManageZaloBot([])).toBe(false);
    for (const k of OTHERS) expect(canManageZaloBot([k])).toBe(false);
    expect(canManageZaloBot(["ops.zalo_bot", "OPS.ZALO_BOT.MANAGE", "ops.*"])).toBe(false);
    expect(visibleNavItems(OTHERS)).not.toContain(ZALO_BOT_ITEM);
    const html = renderToString(<SidebarView permissionKeys={OTHERS} />);
    expect(html).not.toContain('href="/zalo-bot"');
  });

  it("DENIED: the page says why and renders no workspace (so nothing is requested)", () => {
    const html = renderToString(<ZaloBotGate status="ready" keys={OTHERS} />);
    expect(html).toContain(DENIED_TEXT);
    expect(html).not.toContain("Bot dùng chung");
    expect(calls).toHaveLength(0);
  });

  it("keys not loaded yet: no workspace either — the key must be seen first", () => {
    const html = renderToString(<ZaloBotGate status="loading" keys={[]} />);
    expect(html).not.toContain("Bot dùng chung");
    expect(html).not.toContain(DENIED_TEXT);
  });

  it("ALLOWED: the eighth key is in the closed set, reads the console, and lists Zalo Bot before the log", () => {
    expect(ALL_OPS_KEYS).toContain(ZALO);
    expect(canReadConsole([ZALO])).toBe(true);
    expect(canManageZaloBot([ZALO])).toBe(true);
    const items = visibleNavItems([ZALO]).map((i) => i.href);
    expect(items).toContain("/zalo-bot");
    expect(items.indexOf("/zalo-bot")).toBe(items.indexOf("/nhat-ky-van-hanh") - 1);
    expect(NAV_ITEMS).not.toContain(ZALO_BOT_ITEM);
    expect(renderToString(<SidebarView permissionKeys={[ZALO]} />)).toContain('href="/zalo-bot"');
  });
});

// --- client --------------------------------------------------------------------------------------

describe("client — paths, methods, exact bodies", () => {
  it("GET shared, communes, webhook; POST check and PUT webhook carry no body", async () => {
    answer = () => json(200, { configured: false });
    await getSharedZaloBot();
    expect(last()).toEqual({ url: "/api/v1/zalo-bots/shared", method: "GET", body: undefined });

    answer = () => json(200, { items: [{ tenant_id: "01J0000000000000000000000A", channel_enabled: true, linked_staff_count: 3 }] });
    expect(await listZaloBotCommunes()).toHaveLength(1);
    expect(last().url).toBe("/api/v1/zalo-bots/shared/communes");

    answer = () => json(200, { outcome: "thanh-cong", url: "", url_matches: false });
    await getSharedZaloBotWebhook();
    expect(last()).toEqual({ url: "/api/v1/zalo-bots/shared/webhook", method: "GET", body: undefined });

    answer = () => json(200, { outcome: "thanh-cong", account_name: "Bot" });
    await checkSharedZaloBot();
    expect(last()).toEqual({ url: "/api/v1/zalo-bots/shared/check", method: "POST", body: undefined });

    answer = () => json(200, { outcome: "thanh-cong", url: "https://example.test/x" });
    await setSharedZaloBotWebhook();
    expect(last()).toEqual({ url: "/api/v1/zalo-bots/shared/webhook", method: "PUT", body: undefined });
  });

  it("PUT shared: no expected_relink_count until confirmed, then exactly it", async () => {
    answer = () => json(200, CONFIGURED);
    await setSharedZaloBot({ token: TOKEN_FAKE, bot_name: "B", chat_url: "https://example.test/c" });
    expect(last()).toEqual({
      url: "/api/v1/zalo-bots/shared",
      method: "PUT",
      body: { token: TOKEN_FAKE, bot_name: "B", chat_url: "https://example.test/c" },
    });
    await setSharedZaloBot({ token: TOKEN_FAKE, bot_name: "B", chat_url: "https://example.test/c", expected_relink_count: 0 });
    expect(last().body).toEqual({ token: TOKEN_FAKE, bot_name: "B", chat_url: "https://example.test/c", expected_relink_count: 0 });
  });

  it("409 can-xac-nhan-ghep-lai carries live_link_count into ApiError → the confirm step", async () => {
    answer = () => json(409, { code: "can-xac-nhan-ghep-lai", live_link_count: 12, message: "x", trace_id: "t" });
    const err = await setSharedZaloBot({ token: TOKEN_FAKE, bot_name: "B", chat_url: "https://example.test/c" }).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).relinkCount).toBe(12);
    expect(relinkStep(err as ApiError)).toEqual({ kind: "confirm", count: 12 });
    expect(relinkQuestion(12)).toBe("Đổi sang tài khoản bot khác: 12 cán bộ sẽ phải ghép nối lại. Tiếp tục?");
  });

  it("409 can-xac-nhan-ghep-lai without a usable count → `missing`, never a box with a made-up number", async () => {
    for (const bad of [undefined, "12", -1, 1.5]) {
      answer = () => json(409, { code: "can-xac-nhan-ghep-lai", live_link_count: bad });
      const err = (await setSharedZaloBot({ token: TOKEN_FAKE, bot_name: "B", chat_url: "https://example.test/c" }).catch(
        (e: unknown) => e,
      )) as ApiError;
      expect(relinkStep(err)).toEqual({ kind: "missing" });
    }
    expect(RELINK_COUNT_MISSING).toContain("Chưa có gì được thay đổi");
    expect(relinkStep(new ApiError(409, "other", "", ""))).toEqual({ kind: "none" });
    expect(relinkStep(new ApiError(422, "can-xac-nhan-ghep-lai", "", ""))).toEqual({ kind: "none" });
  });

  it("422 token-khong-dung-duoc: nothing saved, the outcome's sentence under the token box", async () => {
    answer = () => json(422, { code: "token-khong-dung-duoc", outcome: "token-bi-tu-choi" });
    const err = (await setSharedZaloBot({ token: TOKEN_FAKE, bot_name: "B", chat_url: "https://example.test/c" }).catch(
      (e: unknown) => e,
    )) as ApiError;
    expect(err.zaloOutcome).toBe("token-bi-tu-choi");
    expect(zaloBotError(err)).toEqual({ field: "token", text: `Chưa lưu token. ${outcomeText("token-bi-tu-choi")}` });
  });

  it("success with ended links says how many; none says nothing", () => {
    expect(endedLinksText(3)).toBe("Đã gỡ 3 liên kết của cán bộ với tài khoản bot cũ.");
    expect(endedLinksText(0)).toBeNull();
    expect(endedLinksText(undefined)).toBeNull();
  });

  it("a 403 is surfaced, never treated as an empty bot", async () => {
    answer = () => json(403, { code: "forbidden", message: "", trace_id: "" });
    await expect(getSharedZaloBot()).rejects.toMatchObject({ status: 403, code: "forbidden" });
  });
});

// --- model ---------------------------------------------------------------------------------------

describe("outcome sentences — Zalo never quoted, unknown never success", () => {
  it("each known class has its own sentence; an unknown one is a failure of unknown cause", () => {
    const known = [
      "thanh-cong",
      "chua-cau-hinh",
      "token-bi-tu-choi",
      "gioi-han-tan-suat",
      "khong-kha-dung",
      "bi-tu-choi",
      "phan-hoi-sai-dang",
    ];
    const texts = known.map(outcomeText);
    expect(new Set(texts).size).toBe(known.length);
    for (const t of texts) expect(t).not.toBe(UNKNOWN_OUTCOME);
    expect(outcomeText("unspecified")).toBe(UNKNOWN_OUTCOME);
    expect(outcomeText("ok")).toBe(UNKNOWN_OUTCOME); // the English name is not the stored value (ADR 0011)
    expect(outcomeText("toString")).toBe(UNKNOWN_OUTCOME);
  });

  it("webhook set: ambiguous outcomes tell the operator to look, not that it failed", () => {
    expect(webhookSetText("khong-kha-dung")).toContain("Chưa rõ Zalo đã nhận địa chỉ mới");
    expect(webhookSetText("phan-hoi-sai-dang")).toContain("Chưa rõ Zalo đã nhận địa chỉ mới");
    expect(webhookSetText("token-bi-tu-choi")).not.toContain("Chưa rõ");
  });

  it("token form check: token verbatim, name and https link trimmed", () => {
    expect(buildBotChange({ token: "", botName: "B", chatUrl: "https://a.test" })).toMatchObject({ ok: false, field: "token" });
    for (const t of ["a b", "a/b", "a?b", "a#b", "a%b", "x".repeat(257)]) {
      expect(buildBotChange({ token: t, botName: "B", chatUrl: "https://a.test" })).toMatchObject({ ok: false, field: "token" });
    }
    expect(buildBotChange({ token: TOKEN_FAKE, botName: "  ", chatUrl: "https://a.test" })).toMatchObject({ field: "botName" });
    for (const u of ["http://a.test", "javascript:alert(1)", "zalo.me/x", "https://u:p@a.test"]) {
      expect(buildBotChange({ token: TOKEN_FAKE, botName: "B", chatUrl: u })).toMatchObject({ field: "chatUrl" });
    }
    expect(buildBotChange({ token: TOKEN_FAKE, botName: " Bot ", chatUrl: " https://a.test/c " })).toEqual({
      ok: true,
      change: { token: TOKEN_FAKE, bot_name: "Bot", chat_url: "https://a.test/c" },
    });
    expect(safeHttpsHref("javascript:alert(1)")).toBeNull();
  });
});

// --- views ---------------------------------------------------------------------------------------

describe("views", () => {
  it("not configured: the sentence and Đặt token; no check button", () => {
    const html = renderToString(
      <SharedBotView state={{ status: "ready", data: { configured: false } }} onEdit={noop} onCheck={noop} checking={false} checkResult={null} />,
    );
    expect(html).toContain(NOT_CONFIGURED_TITLE);
    expect(html).toContain("Đặt token");
    expect(html).not.toContain("Kiểm tra kết nối");
  });

  it("configured: who set the token and when — never the token — and the last check's sentence", () => {
    const html = renderToString(
      <SharedBotView state={{ status: "ready", data: CONFIGURED }} onEdit={noop} onCheck={noop} checking={false} checkResult={null} />,
    );
    expect(html).toContain("Đã đặt lúc");
    expect(html).toContain("bởi VH-00001");
    expect(html).toContain("Thay token");
    expect(html).toContain(outcomeText("token-bi-tu-choi"));
    expect(html).not.toContain(TOKEN_FAKE);
    expect(html).not.toMatch(/chat_id/i);
  });

  it("the token box is empty, a password field, even when the form's draft is reopened", () => {
    const html = renderToString(
      <TokenFormFields draft={{ token: "", botName: "Nhắc việc", chatUrl: "https://a.test/c" }} onDraft={noop} busy={false} error={null} />,
    );
    expect(html).toMatch(/type="password"[^>]*name="zalo_bot_token"[^>]*value=""|name="zalo_bot_token"[^>]*type="password"[^>]*value=""/);
    expect(html).toContain('value="Nhắc việc"');
  });

  it("relink confirmation shows the server's count", () => {
    expect(renderToString(<RelinkConfirmBody count={7} />)).toContain("7 cán bộ sẽ phải ghép nối lại");
  });

  it("webhook: needs a token first; a mismatched URL is warned; a match is said", () => {
    const ok: ZaloBotWebhook = {
      outcome: "thanh-cong",
      url: "https://elsewhere.test/hook",
      url_matches: false,
      secret_set_at: "2026-10-05T03:00:00Z",
      secret_set_by: "VH-00002",
    };
    const none = renderToString(
      <WebhookView configured={false} state={{ status: "loading" }} onRefresh={noop} onPoint={noop} pointing={false} pointResult={null} />,
    );
    expect(none).toContain(WEBHOOK_NEEDS_TOKEN);
    expect(none).not.toContain("Trỏ webhook về hệ thống");

    const wrong = renderToString(
      <WebhookView configured state={{ status: "ready", data: ok }} onRefresh={noop} onPoint={noop} pointing={false} pointResult={null} />,
    );
    expect(wrong).toContain(WEBHOOK_ELSEWHERE);
    expect(wrong).toContain("bởi VH-00002");

    const right = renderToString(
      <WebhookView
        configured
        state={{ status: "ready", data: { ...ok, url_matches: true } }}
        onRefresh={noop}
        onPoint={noop}
        pointing={false}
        pointResult={null}
      />,
    );
    expect(right).toContain("Đúng địa chỉ của hệ thống");
    expect(right).not.toContain(WEBHOOK_ELSEWHERE);

    const failed = renderToString(
      <WebhookView
        configured
        state={{ status: "ready", data: { outcome: "gioi-han-tan-suat", url: "", url_matches: false } }}
        onRefresh={noop}
        onPoint={noop}
        pointing={false}
        pointResult={null}
      />,
    );
    expect(failed).toContain(outcomeText("gioi-han-tan-suat"));
  });

  it("communes: counts only, name or ULID, totals; empty list is a sentence", () => {
    const html = renderToString(
      <CommuneUptakeView
        state={{
          status: "ready",
          data: [
            { tenant_id: "01J0000000000000000000000A", commune_name: "Xã Kiểm Thử", channel_enabled: true, linked_staff_count: 4 },
            { tenant_id: "01J0000000000000000000000B", channel_enabled: false, linked_staff_count: 0 },
          ],
        }}
      />,
    );
    expect(html).toContain("Xã Kiểm Thử");
    expect(html).toContain("01J0000000000000000000000B");
    expect(html).toContain("1 xã bật kênh · 4 cán bộ đã ghép nối");
    expect(renderToString(<CommuneUptakeView state={{ status: "ready", data: [] }} />)).toContain(NO_COMMUNE_ROWS);
  });
});
