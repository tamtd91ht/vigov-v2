// @vitest-environment jsdom
//
// jsdom: the tab is gated on the session, loads five routes once allowed, and saves on every change;
// §3 (the commune's own bot) writes only on its own buttons.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { renderToStaticMarkup as renderToString } from "react-dom/server";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import type { PhienDaDoc } from "@/features/phien/phien-hien-tai";
import type { comms_communeZaloBotCurrentOut } from "@/lib/api/schema.gen";
import type { ZaloChannelSettings } from "@/lib/api/zalo";

const H = vi.hoisted(() => ({
  phien: null as PhienDaDoc,
  toast: { success: vi.fn(), error: vi.fn() },
}));
vi.mock("@/features/phien/phien-hien-tai", () => ({ usePhien: () => H.phien }));
vi.mock("sonner", () => ({ toast: H.toast }));

const {
  buildBotChange,
  buildChange,
  checkRetireReason,
  dayOptions,
  draftFromSettings,
  hourOptions,
  joinStaffLinks,
  NO_LINKED_STAFF,
  relinkSentence,
  rowKinds,
  toggleKind,
  webhookResultText,
  ZALO_EVENT_GROUPS,
} = await import("./zalo-channel-form");
const { peopleState, ZaloChannelTab, ZaloChannelView } = await import("./zalo-channel-tab");

/** comms `domain.ZaloReminderKinds` — what `supported_events` carries today (zalo_links_test.go:338). */
const SUPPORTED = [
  "nhiem-vu.sap-den-han",
  "nhiem-vu.qua-han",
  "nhiem-vu.chua-cu-nguoi",
  "nhiem-vu.leo-thang",
  "van-ban.sap-den-han",
  "van-ban.qua-han",
  "van-ban.chua-cu-nguoi",
  "van-ban.leo-thang",
  "phan-anh.sap-den-han",
  "phan-anh.qua-han",
  "phan-anh.chua-cu-nguoi",
  "phan-anh.leo-thang",
  "ban-tin-tuan",
];

const SETTINGS: ZaloChannelSettings = {
  is_enabled: true,
  kinds: ["nhiem-vu.sap-den-han", "nhiem-vu.qua-han"],
  quiet_start: "21:00",
  quiet_end: "06:00",
  overdue_start_after_days: 1,
  overdue_repeat_every_days: 2,
  updated_at: "2026-10-05T03:00:00Z",
  updated_by: "CB-00123",
  supported_events: SUPPORTED,
  platform_ready: true,
};

const OFF: ZaloChannelSettings = {
  is_enabled: false,
  kinds: [],
  quiet_start: "21:00",
  quiet_end: "06:00",
  overdue_start_after_days: null,
  overdue_repeat_every_days: null,
  supported_events: SUPPORTED,
  platform_ready: true,
};

const SHARED_BOT: comms_communeZaloBotCurrentOut = { has_own_bot: false, bot: null, live_link_count: 2 };
const OWN_BOT: comms_communeZaloBotCurrentOut = {
  has_own_bot: true,
  live_link_count: 5,
  bot: {
    bot_account_id: "000000001",
    bot_name: "Bot Xã Thử",
    chat_url: "https://zalo.me/000000001",
    set_at: "2026-10-08T01:00:00Z",
    set_by: "CB-00123",
    webhook_set_at: null,
    webhook_set_by: null,
    webhook_pending: false,
    last_check: { at: "2026-10-08T02:00:00Z", result: "thanh-cong" },
  },
};
const TOKEN_FAKE = "000000000:fake-token-for-tests";
const SECRET_FAKE = "fake-webhook-secret-0000";

const LINKED = [{ staff_code: "CB-00123", staff_name: "Nguyễn Văn A", linked_at: "2026-10-05T03:00:00Z" }];
const DIRECTORY = [
  { code: "CB-00123", full_name: "Nguyễn Văn A", position: "Chủ tịch", department_id: "01UNITVP" },
  { code: "CB-00200", full_name: "Trần Thị B", position: "Văn thư", department_id: "01UNITVP" },
];
const UNITS = [{ id: "01UNITVP", code: "vp", name: "Văn phòng", parent_id: "", order: 1, staff_count: 2 }];

function session(permissions: string[]): PhienDaDoc {
  return { ok: true, duLieu: { staff: { full_name: "A", position: "B" }, role: null, permissions } } as unknown as PhienDaDoc;
}

let calls: { method: string; url: string; body?: string }[] = [];
let stored: ZaloChannelSettings = SETTINGS;
/** What the next PUT answers; null = echo the body back as saved. */
let putAnswer: { status: number; body: unknown } | null = null;
/** What GET /api/v1/staff-directory answers. */
let directoryAnswer: { status: number; body: unknown } = { status: 200, body: { items: DIRECTORY } };
/** What GET /api/v1/zalo-bots/current answers; null = a 500. */
let botNow: comms_communeZaloBotCurrentOut | null = SHARED_BOT;
/** What the bot writes answer, by "METHOD url". */
let botAnswers: Record<string, { status: number; body: unknown }> = {};

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

beforeEach(() => {
  calls = [];
  stored = SETTINGS;
  putAnswer = null;
  directoryAnswer = { status: 200, body: { items: DIRECTORY } };
  botNow = SHARED_BOT;
  botAnswers = {};
  H.toast.success.mockReset();
  H.toast.error.mockReset();
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init: RequestInit) => {
      const method = String(init.method ?? "GET");
      const body = typeof init.body === "string" ? init.body : undefined;
      calls.push({ method, url, body });
      const json = (status: number, b: unknown) =>
        new Response(JSON.stringify(b), { status, headers: { "Content-Type": "application/json" } });
      if (url === "/api/v1/zalo-links") return json(200, { items: LINKED });
      if (url === "/api/v1/staff-directory") return json(directoryAnswer.status, directoryAnswer.body);
      if (url === "/api/v1/org-units") return json(200, { items: UNITS });
      if (url.startsWith("/api/v1/zalo-bots/")) {
        if (method === "GET") {
          return botNow === null ? json(500, { code: "internal", message: "Máy chủ bận" }) : json(200, botNow);
        }
        const a = botAnswers[`${method} ${url}`];
        return a ? json(a.status, a.body) : json(500, { code: "internal", message: "no stub" });
      }
      // The settings PUT reply carries supported_events but NOT platform_ready (GET only, zalo_links.go:387).
      if (method === "PUT") {
        return putAnswer
          ? json(putAnswer.status, putAnswer.body)
          : json(200, { ...JSON.parse(body!), supported_events: SUPPORTED });
      }
      return json(200, stored);
    }),
  );
});

let root: Root | null = null;
let host: HTMLDivElement | null = null;

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  H.phien = null;
  vi.unstubAllGlobals();
});

async function settle() {
  for (let i = 0; i < 8; i++) await act(async () => {});
}

async function mount(): Promise<HTMLDivElement> {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() => r.render(<ZaloChannelTab />));
  await settle();
  return host;
}

function box(el: HTMLElement, code: string): HTMLInputElement {
  return el.querySelector<HTMLInputElement>(`input[data-event="${code}"]`)!;
}

function view(
  settings: ZaloChannelSettings,
  links = LINKED,
  directory: typeof DIRECTORY | null = DIRECTORY,
  platformReady: boolean | null = true,
) {
  const dir =
    directory === null ? { ok: false as const, thongBao: "Máy chủ bận" } : { ok: true as const, duLieu: { items: directory } };
  return renderToString(
    <ZaloChannelView
      draft={draftFromSettings(settings)}
      supported={settings.supported_events}
      platformReady={platformReady}
      onChange={() => {}}
      saving={false}
      held={null}
      people={peopleState({ ok: true, duLieu: links }, dir, { ok: true, duLieu: { items: UNITS } })}
      botSection={<section aria-label="bot slot" />}
    />,
  );
}

function html(markup: string): HTMLDivElement {
  host = document.createElement("div");
  host.innerHTML = markup;
  return host;
}

/** Types into a controlled input the way React listens for it (native setter + bubbling `input`). */
function typeInto(el: HTMLInputElement, value: string) {
  const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!;
  act(() => {
    setter.call(el, value);
    el.dispatchEvent(new Event("input", { bubbles: true }));
  });
}

function button(el: HTMLElement, text: string): HTMLButtonElement {
  const found = Array.from(el.querySelectorAll<HTMLButtonElement>("button")).filter((b) => b.textContent?.trim() === text);
  expect(found.length, `button "${text}"`).toBeGreaterThan(0);
  return found.at(-1)!;
}

function botSection(el: HTMLElement): HTMLElement {
  return el.querySelector<HTMLElement>('section[aria-labelledby="zalo-bot-title"]')!;
}

describe("form check (mirrors comms' refusals; comms re-checks)", () => {
  it("turning on needs at least one kind — refused, not held", () => {
    expect(buildChange({ ...draftFromSettings(OFF), isEnabled: true })).toMatchObject({ ok: false, held: false });
    // Off with no kind is a valid state: the commune with no row is exactly that.
    expect(buildChange(draftFromSettings(OFF))).toMatchObject({ ok: true, change: { is_enabled: false, kinds: [] } });
  });

  it("REGRESSION: a quiet window that starts and ends at the same hour is refused before it is sent", () => {
    // The old check let it through to a 422; with one select per hour it is one click away.
    expect(buildChange({ ...draftFromSettings(SETTINGS), quietEnd: "21:00" })).toMatchObject({ ok: false, held: false });
  });

  it("REGRESSION: 0 days = 'Ngay hôm quá hạn' is a valid start (server: 0…365)", () => {
    expect(buildChange({ ...draftFromSettings(SETTINGS), lateStartDays: 0 })).toMatchObject({
      ok: true,
      change: { overdue_start_after_days: 0, overdue_repeat_every_days: 2 },
    });
  });

  it("one cadence number picked and the other unset → HELD (half-way), never sent", () => {
    expect(buildChange({ ...draftFromSettings(OFF), lateStartDays: 3 })).toMatchObject({ ok: false, held: true });
  });

  it("an overdue kind chosen → both cadences required (any domain); unassigned/escalation need none", () => {
    expect(
      buildChange(toggleKind({ ...draftFromSettings(OFF), isEnabled: true }, "phan-anh.qua-han", true, SUPPORTED)),
    ).toMatchObject({ ok: false, held: false });
    expect(
      buildChange(toggleKind({ ...draftFromSettings(OFF), isEnabled: true }, "van-ban.chua-cu-nguoi", true, SUPPORTED)),
    ).toMatchObject({ ok: true });
    const d = toggleKind({ ...draftFromSettings(OFF), isEnabled: true }, "nhiem-vu.qua-han", true, SUPPORTED);
    expect(buildChange(d)).toMatchObject({ ok: false, held: false });
    expect(buildChange({ ...d, lateStartDays: 1, lateRepeatDays: 3 })).toEqual({
      ok: true,
      change: {
        is_enabled: true,
        kinds: ["nhiem-vu.qua-han"],
        quiet_start: "21:00",
        quiet_end: "06:00",
        overdue_start_after_days: 1,
        overdue_repeat_every_days: 3,
      },
    });
  });

  it("overdue un-ticked keeps the cadence shown on screen (both set is valid without it)", () => {
    const d = toggleKind(draftFromSettings(SETTINGS), "nhiem-vu.qua-han", false, SUPPORTED);
    expect(buildChange(d)).toMatchObject({
      ok: true,
      change: { kinds: ["nhiem-vu.sap-den-han"], overdue_start_after_days: 1, overdue_repeat_every_days: 2 },
    });
  });

  it("kinds keep the server's order whatever the click order; a kind with no row is kept, never dropped", () => {
    let d = draftFromSettings({ ...OFF, kinds: ["kind-from-a-newer-server"] });
    d = toggleKind(d, "ban-tin-tuan", true, SUPPORTED);
    d = toggleKind(d, "nhiem-vu.sap-den-han", true, SUPPORTED);
    expect(d.kinds).toEqual(["nhiem-vu.sap-den-han", "ban-tin-tuan", "kind-from-a-newer-server"]);
  });

  it("selects: whole hours, days min…14 — a stored value outside them stays visible", () => {
    expect(hourOptions("21:00")).toHaveLength(24);
    expect(hourOptions("21:30")).toContain("21:30");
    expect(dayOptions(0, 1)).toEqual([0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14]);
    expect(dayOptions(1, 30)).toContain(30);
  });
});

describe("bot form check (mirrors comms' communeBotRefusals; comms re-checks)", () => {
  const ok = { token: "", name: "Bot Xã", chatUrl: "https://zalo.me/1" };

  it("shared bot needs a token; own bot keeps its token when the box is empty", () => {
    expect(buildBotChange(ok, false)).toEqual({ ok: false, text: "Xã chưa có bot riêng nên phải nhập mã bot." });
    expect(buildBotChange(ok, true)).toEqual({ ok: true, kind: "edit", body: { bot_name: "Bot Xã", chat_url: "https://zalo.me/1" } });
  });

  it("token typed: adopt from shared, possible replace on own; token VERBATIM, name and link trimmed", () => {
    const d = { token: ` ${TOKEN_FAKE}`, name: "  Bot Xã ", chatUrl: " https://zalo.me/1 " };
    expect(buildBotChange(d, false)).toEqual({
      ok: true,
      kind: "adopt",
      body: { bot_token: ` ${TOKEN_FAKE}`, bot_name: "Bot Xã", chat_url: "https://zalo.me/1" },
    });
    expect(buildBotChange(d, true)).toMatchObject({ ok: true, kind: "replace" });
  });

  it("name must start with 'Bot'; link must be https", () => {
    expect(buildBotChange({ ...ok, name: "Nhắc việc" }, true)).toMatchObject({ ok: false });
    expect(buildBotChange({ ...ok, chatUrl: "http://zalo.me/1" }, true)).toMatchObject({ ok: false });
    expect(buildBotChange({ ...ok, chatUrl: "javascript:alert(1)" }, true)).toMatchObject({ ok: false });
  });

  it("retire reason: required, trimmed, at most 200 characters", () => {
    expect(checkRetireReason("   ")).toMatchObject({ ok: false });
    expect(checkRetireReason("x".repeat(201))).toMatchObject({ ok: false });
    expect(checkRetireReason("  Lý do  ")).toEqual({ ok: true, reason: "Lý do" });
  });

  it("relink sentence names the server's count; zero says nobody", () => {
    expect(relinkSentence(4)).toBe("4 cán bộ đang ghép nối sẽ phải ghép nối lại.");
    expect(relinkSentence(0)).toBe("Hiện chưa cán bộ nào ghép nối, nên không ai phải ghép nối lại.");
  });

  it("webhook: ambiguous outcomes say 'press again', refusals say 'Chưa đăng ký được.', unknown is never a success", () => {
    expect(webhookResultText("khong-kha-dung")).toContain("bấm “Đăng ký webhook” lần nữa");
    expect(webhookResultText("token-bi-tu-choi")).toMatch(/^Chưa đăng ký được\. /);
    expect(webhookResultText("toString")).toMatch(/^Chưa đăng ký được\. Không rõ kết quả/);
  });
});

describe("view", () => {
  it("event table: spec's 18 rows only (owner 'theo prototype'); EVERY supported kind on exactly one box", () => {
    const rows = ZALO_EVENT_GROUPS.flatMap((g) => g.events);
    expect(rows).toHaveLength(18);
    expect(rows.map((r) => r.label)).not.toContain("Văn bản, đơn thư bị đôn đốc lên cấp trên");
    expect(rows.map((r) => r.label)).not.toContain("Phản ánh chưa cử người xử lý");
    // Fails if comms offers a kind no box switches — selected with no control anywhere.
    for (const kind of SUPPORTED) expect(rows.filter((r) => rowKinds(r).includes(kind)), kind).toHaveLength(1);
    const mapped = Object.fromEntries(rows.filter((e) => e.kind !== null).map((e) => [e.code, rowKinds(e)]));
    expect(mapped).toEqual({
      "task.due_soon": ["nhiem-vu.sap-den-han"],
      "task.overdue": ["nhiem-vu.qua-han"],
      "task.unassigned_too_long": ["nhiem-vu.chua-cu-nguoi"],
      "task.escalated": ["nhiem-vu.leo-thang"],
      "document.due_soon": ["van-ban.sap-den-han"],
      "document.overdue": ["van-ban.qua-han", "van-ban.chua-cu-nguoi", "van-ban.leo-thang"],
      "feedback.due_soon": ["phan-anh.sap-den-han"],
      "feedback.overdue": ["phan-anh.qua-han", "phan-anh.chua-cu-nguoi", "phan-anh.leo-thang"],
      "digest.weekly": ["ban-tin-tuan"],
    });

    // Checked = the row's qua-han kind selected; a half-state without it reads unchecked.
    const el = html(view({ ...SETTINGS, kinds: ["nhiem-vu.qua-han", "van-ban.chua-cu-nguoi", "phan-anh.qua-han"] }));
    expect(box(el, "task.overdue").checked).toBe(true);
    expect(box(el, "document.overdue").checked).toBe(false);
    expect(box(el, "feedback.overdue").checked).toBe(true);
    expect(box(el, "task.unassigned_too_long").checked).toBe(false);
    expect(box(el, "digest.weekly").disabled).toBe(false);
    expect(box(el, "task.assigned").disabled).toBe(true);
    expect(el.querySelectorAll('[aria-label^="Thêm loại việc nhắn qua Zalo"]')).toHaveLength(9);
  });

  it("a kind the server does NOT list in supported_events is drawn disabled with '?', never tickable", () => {
    const el = html(view({ ...SETTINGS, supported_events: SUPPORTED.filter((k) => k !== "ban-tin-tuan") }));
    expect(box(el, "digest.weekly").disabled).toBe(true);
    expect(el.querySelectorAll('[aria-label^="Thêm loại việc nhắn qua Zalo"]')).toHaveLength(10);
    // A grouped row needs ALL its kinds offered.
    const el2 = html(view({ ...SETTINGS, supported_events: SUPPORTED.filter((k) => k !== "van-ban.leo-thang") }));
    expect(box(el2, "document.overdue").disabled).toBe(true);
  });

  it("§0: the tangerine warning only when platform_ready is FALSE — not when true, not when unknown", () => {
    expect(view(SETTINGS, LINKED, DIRECTORY, false)).toContain("Chưa có con bot nào phục vụ xã");
    expect(view(SETTINGS, LINKED, DIRECTORY, true)).not.toContain("Chưa có con bot nào phục vụ xã");
    expect(view(SETTINGS, LINKED, DIRECTORY, null)).not.toContain("Chưa có con bot nào phục vụ xã");
  });

  it("no chat id; no 'ViGov', no 'và thư'", () => {
    const markup = view(SETTINGS);
    expect(markup).toContain("Nguyễn Văn A");
    expect(markup).toContain("Hồ sơ cá nhân");
    expect(markup).not.toMatch(/chat_id/i);
    expect(markup).not.toContain("ViGov");
    expect(markup).not.toContain("và thư");
    expect(markup).not.toContain("Lưu cấu hình");
  });

  it("REGRESSION: §2 description no longer claims the threshold is changed here (ADR 0079 Q6)", () => {
    const markup = view(SETTINGS);
    expect(markup).not.toContain("đổi ở đây là đổi cho cả hai");
    expect(markup).toContain(
      "Ngưỡng “sắp đến hạn” dùng chung với cái chuông và danh sách “Sắp đến hạn”, nên con số trong tin nhắn luôn bằng con số trên màn hình.",
    );
  });

  it("empty list keeps its sentence", () => {
    expect(view(OFF, [], [])).toContain(NO_LINKED_STAFF);
  });

  it("REGRESSION Z1/Z8: no child of a space-y-* stack carries its own margin reset (it would beat :where())", () => {
    const el = html(view(SETTINGS));
    for (const stack of el.querySelectorAll<HTMLElement>('[class*="space-y-"]')) {
      for (const child of Array.from(stack.children).slice(0, -1)) {
        expect(child.className).not.toMatch(/(^|\s)(m-0|my-0|mb-0)(\s|$)/);
      }
    }
    // The page stack is a flex gap, independent of child margins.
    expect(el.firstElementChild!.className).toMatch(/(^|\s)flex-col(\s|$)/);
    expect(el.firstElementChild!.className).toMatch(/(^|\s)gap-5(\s|$)/);
  });

  it("REGRESSION Z23: a one-sided hairline is border-0 + that side (preflight off → 'medium' 3px elsewhere)", () => {
    const el = html(view(SETTINGS));
    const oneSided = Array.from(el.querySelectorAll<HTMLElement>("*")).filter(
      (n) =>
        /(^|\s)border-solid(\s|$)/.test(n.className) &&
        /(^|\s)border-[bt](\s|$)/.test(n.className) &&
        !/(^|\s)border(\s|$)/.test(n.className),
    );
    expect(oneSided.length).toBeGreaterThan(2);
    for (const n of oneSided) expect(n.className).toMatch(/(^|\s)border-0(\s|$)/);
  });

  it("REGRESSION Z12: every field's label row has the same fixed height, marker or not", () => {
    const el = html(view(SETTINGS));
    const rows = el.querySelectorAll<HTMLElement>("[data-field-label-row]");
    expect(rows.length).toBeGreaterThan(3);
    for (const r of rows) expect(r.className).toMatch(/(^|\s)h-5(\s|$)/);
  });
});

describe("Ai đã ghép nối — directory joined with links by staff code", () => {
  it("every staff member, linked or not; title · unit; locked-but-linked appended and counted", () => {
    const j = joinStaffLinks(DIRECTORY, UNITS, [
      ...LINKED,
      { staff_code: "CB-00999", staff_name: "Lê Văn C", linked_at: "2026-10-01T00:00:00Z" },
    ]);
    expect(j.rows).toEqual([
      { code: "CB-00123", name: "Nguyễn Văn A", detail: "Chủ tịch · Văn phòng", linked: true },
      { code: "CB-00200", name: "Trần Thị B", detail: "Văn thư · Văn phòng", linked: false },
      { code: "CB-00999", name: "Lê Văn C", detail: "CB-00999", linked: true },
    ]);
    expect([j.linked, j.total]).toEqual([2, 3]);
  });

  it("rows show 'Đã ghép nối' / 'Chưa'; the count reads linked/total; the two '?' are gone", () => {
    const el = html(view(SETTINGS));
    expect(el.querySelector('[data-staff-code="CB-00123"]')!.textContent).toContain("Đã ghép nối");
    expect(el.querySelector('[data-staff-code="CB-00200"]')!.textContent).toContain("Chưa");
    expect(el.textContent).toContain("Chủ tịch · Văn phòng");
    expect(el.textContent).toContain("1/2 cán bộ");
    expect(el.querySelector<HTMLInputElement>('input[name="only_missing"]')!.disabled).toBe(false);
    expect(el.querySelector('[aria-label^="Số cán bộ đã ghép nối"]')).toBeNull();
    expect(el.querySelector('[aria-label^="Cán bộ chưa ghép nối"]')).toBeNull();
  });

  it("FAIL CLOSED: directory unreadable → linked-only list, says so, no total, filter disabled", () => {
    const el = html(view(SETTINGS, LINKED, null));
    expect(el.textContent).toContain("Chưa đọc được danh bạ cán bộ");
    expect(el.textContent).toContain("1 cán bộ");
    expect(el.textContent).not.toContain("1/");
    expect(el.querySelector('[data-staff-code="CB-00200"]')).toBeNull();
    expect(el.querySelector<HTMLInputElement>('input[name="only_missing"]')!.disabled).toBe(true);
  });
});

describe("ZaloChannelTab — admin.lookup, denied case first", () => {
  it("DENIED: without admin.lookup the tab says so and requests NOTHING — the bot route included", async () => {
    H.phien = session(["admin.user", "admin.sla", "admin.lookups"]);
    const el = await mount();
    expect(el.textContent).toContain("không có quyền cấu hình kênh Zalo");
    expect(calls).toHaveLength(0);
  });

  it("DENIED: a session that could not be read is not a yes", async () => {
    H.phien = { ok: false, thongBao: "Phiên đã hết hạn" } as unknown as PhienDaDoc;
    const el = await mount();
    expect(el.textContent).toContain("Phiên đã hết hạn");
    expect(calls).toHaveLength(0);
  });

  it("ALLOWED: one tick saves at once — the WHOLE row is PUT, then 'Đã lưu.'", async () => {
    H.phien = session(["admin.lookup"]);
    const el = await mount();
    expect(calls.map((c) => c.url).sort()).toEqual([
      "/api/v1/org-units",
      "/api/v1/staff-directory",
      "/api/v1/zalo-bots/current",
      "/api/v1/zalo-channel-settings",
      "/api/v1/zalo-links",
    ]);
    expect(el.textContent).toContain("Nguyễn Văn A");
    act(() => box(el, "digest.weekly").click());
    await settle();
    const put = calls.find((c) => c.method === "PUT")!;
    expect(JSON.parse(put.body!)).toEqual({
      is_enabled: true,
      kinds: ["nhiem-vu.sap-den-han", "nhiem-vu.qua-han", "ban-tin-tuan"],
      quiet_start: "21:00",
      quiet_end: "06:00",
      overdue_start_after_days: 1,
      overdue_repeat_every_days: 2,
    });
    expect(H.toast.success).toHaveBeenCalledWith("Đã lưu.");
    expect(box(el, "digest.weekly").checked).toBe(true);
  });

  it("'Văn bản, đơn thư quá hạn' switches qua-han + chua-cu-nguoi + leo-thang TOGETHER, on and off", async () => {
    H.phien = session(["admin.lookup"]);
    // Half-state stored: chua-cu-nguoi alone → unchecked; one tick sets all three.
    stored = { ...SETTINGS, kinds: ["nhiem-vu.sap-den-han", "nhiem-vu.qua-han", "van-ban.chua-cu-nguoi"] };
    const el = await mount();
    expect(box(el, "document.overdue").checked).toBe(false);
    act(() => box(el, "document.overdue").click());
    await settle();
    const on = calls.filter((c) => c.method === "PUT" && c.url === "/api/v1/zalo-channel-settings").at(-1)!;
    expect(JSON.parse(on.body!).kinds).toEqual([
      "nhiem-vu.sap-den-han",
      "nhiem-vu.qua-han",
      "van-ban.qua-han",
      "van-ban.chua-cu-nguoi",
      "van-ban.leo-thang",
    ]);
    expect(box(el, "document.overdue").checked).toBe(true);

    act(() => box(el, "document.overdue").click());
    await settle();
    const off = calls.filter((c) => c.method === "PUT" && c.url === "/api/v1/zalo-channel-settings").at(-1)!;
    expect(JSON.parse(off.body!).kinds).toEqual(["nhiem-vu.sap-den-han", "nhiem-vu.qua-han"]);
  });

  it("a refusal from the server is shown in its words and the screen goes back to what is stored", async () => {
    H.phien = session(["admin.lookup"]);
    putAnswer = { status: 422, body: { code: "overdue_cadence_required", message: "Đã chọn nhắc việc quá hạn thì phải đặt nhịp nhắc." } };
    const el = await mount();
    act(() => box(el, "digest.weekly").click());
    await settle();
    expect(H.toast.error).toHaveBeenCalledWith("Đã chọn nhắc việc quá hạn thì phải đặt nhịp nhắc.");
    expect(box(el, "digest.weekly").checked).toBe(false);
  });

  it("'Chỉ người chưa ghép nối' filters to the unlinked, and saves nothing", async () => {
    H.phien = session(["admin.lookup"]);
    const el = await mount();
    expect(el.querySelector('[data-staff-code="CB-00123"]')).not.toBeNull();
    act(() => el.querySelector<HTMLInputElement>('input[name="only_missing"]')!.click());
    await settle();
    expect(el.querySelector('[data-staff-code="CB-00123"]')).toBeNull();
    expect(el.querySelector('[data-staff-code="CB-00200"]')).not.toBeNull();
    expect(calls.some((c) => c.method === "PUT")).toBe(false);
  });

  it("REGRESSION: filter on and nobody unlinked → the empty list, no invented sentence", async () => {
    H.phien = session(["admin.lookup"]);
    directoryAnswer = { status: 200, body: { items: [DIRECTORY[0]] } };
    const el = await mount();
    act(() => el.querySelector<HTMLInputElement>('input[name="only_missing"]')!.click());
    await settle();
    const section = el.querySelector('section[aria-labelledby="zalo-linked-title"]')!;
    expect(section.querySelectorAll("tbody tr")).toHaveLength(0);
    expect(section.querySelector("table")).not.toBeNull();
    expect(section.textContent).not.toContain("Mọi cán bộ");
    expect(section.textContent).not.toContain(NO_LINKED_STAFF);
  });

  it("directory refused (500) → the tab still works on the linked list and says why", async () => {
    H.phien = session(["admin.lookup"]);
    directoryAnswer = { status: 500, body: { code: "internal", message: "Máy chủ bận" } };
    const el = await mount();
    expect(el.textContent).toContain("Chưa đọc được danh bạ cán bộ");
    expect(el.textContent).toContain("Nguyễn Văn A");
  });

  it("switching ON with no kind is refused on the spot: nothing is sent, the switch stays off", async () => {
    H.phien = session(["admin.lookup"]);
    stored = OFF;
    const el = await mount();
    const sw = el.querySelector<HTMLButtonElement>('button[role="switch"]')!;
    act(() => sw.click());
    await settle();
    expect(calls.some((c) => c.method === "PUT")).toBe(false);
    expect(H.toast.error).toHaveBeenCalledWith("Bật kênh Zalo thì phải chọn ít nhất một loại nhắc việc.");
    expect(sw.getAttribute("aria-checked")).toBe("false");
  });

  it("REGRESSION: an autosave does not erase the §0 warning (the PUT reply has no platform_ready)", async () => {
    H.phien = session(["admin.lookup"]);
    stored = { ...SETTINGS, platform_ready: false };
    const el = await mount();
    expect(el.textContent).toContain("Chưa có con bot nào phục vụ xã");
    act(() => box(el, "digest.weekly").click());
    await settle();
    expect(H.toast.success).toHaveBeenCalledWith("Đã lưu.");
    expect(el.textContent).toContain("Chưa có con bot nào phục vụ xã");
  });
});

describe("§3 Con bot của xã — zalo-bots/current (ADR 0079 Q1)", () => {
  it("shared bot: pill 'Bot chung', shared description, empty token; check / webhook / retire HIDDEN; no '?'", async () => {
    H.phien = session(["admin.lookup"]);
    const el = await mount();
    const sec = botSection(el);
    expect(sec.querySelector("[data-bot-pill]")!.textContent).toBe("Bot chung");
    expect(sec.textContent).toContain("Xã đang dùng bot chung của nền tảng.");
    const token = sec.querySelector<HTMLInputElement>("#zalo-bot-token")!;
    expect(token.type).toBe("password");
    expect(token.value).toBe("");
    expect(token.name).toBe("");
    expect(token.placeholder).toBe("123456789:abc-xyz");
    expect(token.disabled).toBe(false);
    // REGRESSION: a password input outside a <form> makes the browser warn on every load.
    expect(token.closest("form")).not.toBeNull();
    // Owner 08/10/2026: hidden, not disabled, while the shared bot serves the commune.
    expect(sec.textContent).not.toContain("Kiểm tra kết nối");
    expect(sec.textContent).not.toContain("Đăng ký webhook");
    expect(sec.textContent).not.toContain("Quay về bot chung");
    expect(sec.querySelector('[aria-label^="Con bot của xã — "]')).toBeNull();
    // Spec's 4th field is the one-time box only (ADR 0079 Q1 #3): no standing secret input.
    expect(sec.querySelector("#zalo-bot-secret")).toBeNull();
  });

  it("own bot: pill 'Bot riêng · {tên}', token stays EMPTY with '••••••••', last check and webhook lines", async () => {
    H.phien = session(["admin.lookup"]);
    botNow = OWN_BOT;
    const el = await mount();
    const sec = botSection(el);
    expect(sec.querySelector("[data-bot-pill]")!.textContent).toBe("Bot riêng · Bot Xã Thử");
    expect(sec.textContent).toContain("Xã đang dùng bot riêng.");
    const token = sec.querySelector<HTMLInputElement>("#zalo-bot-token")!;
    expect(token.value).toBe("");
    expect(token.placeholder).toBe("••••••••");
    expect(sec.querySelector<HTMLInputElement>("#zalo-bot-name")!.value).toBe("Bot Xã Thử");
    expect(sec.querySelector("[data-last-check]")!.textContent).toMatch(/^Lần kiểm gần nhất: .+ · kết nối tốt$/);
    expect(sec.querySelector("[data-webhook-state]")!.textContent).toBe("Chưa đăng ký webhook.");
    expect(button(sec, "Kiểm tra kết nối").disabled).toBe(false);
    button(sec, "Đăng ký webhook");
    button(sec, "Quay về bot chung");
  });

  it("the bot read failing is said inside §3 and does not take the rest of the tab down", async () => {
    H.phien = session(["admin.lookup"]);
    botNow = null;
    const el = await mount();
    expect(botSection(el).textContent).toContain("Máy chủ bận");
    expect(el.textContent).toContain("Nhắc việc qua Zalo");
    expect(box(el, "digest.weekly").disabled).toBe(false);
  });

  it("ADOPT: nothing is sent before the confirmation naming live_link_count; then the token goes up and is cleared", async () => {
    H.phien = session(["admin.lookup"]);
    botAnswers["PUT /api/v1/zalo-bots/current"] = {
      status: 200,
      body: { bot: OWN_BOT.bot, adopted: true, ended_link_count: 2 },
    };
    const el = await mount();
    const sec = botSection(el);
    typeInto(sec.querySelector<HTMLInputElement>("#zalo-bot-token")!, TOKEN_FAKE);
    typeInto(sec.querySelector<HTMLInputElement>("#zalo-bot-name")!, "Bot Xã Thử");
    typeInto(sec.querySelector<HTMLInputElement>("#zalo-bot-chat-url")!, "https://zalo.me/000000001");
    act(() => button(sec, "Lưu con bot").click());
    await settle();
    expect(calls.some((c) => c.method === "PUT" && c.url === "/api/v1/zalo-bots/current")).toBe(false);
    expect(el.querySelector("[data-relink]")!.textContent).toContain("2 cán bộ đang ghép nối sẽ phải ghép nối lại.");

    botNow = OWN_BOT;
    act(() => button(el.querySelector<HTMLElement>("dialog")!, "Lưu con bot").click());
    await settle();
    const put = calls.find((c) => c.method === "PUT" && c.url === "/api/v1/zalo-bots/current")!;
    expect(JSON.parse(put.body!)).toEqual({ bot_token: TOKEN_FAKE, bot_name: "Bot Xã Thử", chat_url: "https://zalo.me/000000001" });
    expect(H.toast.success).toHaveBeenCalledWith("Đã lưu con bot. 2 cán bộ cần ghép nối lại.");
    expect(el.querySelector("dialog")).toBeNull();
    expect(botSection(el).querySelector<HTMLInputElement>("#zalo-bot-token")!.value).toBe("");
    expect(botSection(el).querySelector("[data-bot-pill]")!.textContent).toBe("Bot riêng · Bot Xã Thử");
    // A switch re-reads platform_ready (a second GET of the settings).
    expect(calls.filter((c) => c.method === "GET" && c.url === "/api/v1/zalo-channel-settings")).toHaveLength(2);
    // The token never reaches a URL.
    for (const c of calls) expect(c.url).not.toContain(TOKEN_FAKE);
  });

  it("CANCEL the switch: nothing is sent", async () => {
    H.phien = session(["admin.lookup"]);
    const el = await mount();
    const sec = botSection(el);
    typeInto(sec.querySelector<HTMLInputElement>("#zalo-bot-token")!, TOKEN_FAKE);
    typeInto(sec.querySelector<HTMLInputElement>("#zalo-bot-name")!, "Bot Xã Thử");
    typeInto(sec.querySelector<HTMLInputElement>("#zalo-bot-chat-url")!, "https://zalo.me/1");
    act(() => button(sec, "Lưu con bot").click());
    await settle();
    act(() => button(el.querySelector<HTMLElement>("dialog")!, "Huỷ").click());
    await settle();
    expect(el.querySelector("dialog")).toBeNull();
    expect(calls.some((c) => c.url.startsWith("/api/v1/zalo-bots/") && c.method !== "GET")).toBe(false);
  });

  it("own bot, name/link only (no token): saved at once, no confirmation, no token key", async () => {
    H.phien = session(["admin.lookup"]);
    botNow = OWN_BOT;
    botAnswers["PUT /api/v1/zalo-bots/current"] = { status: 200, body: { bot: OWN_BOT.bot, adopted: false, ended_link_count: 0 } };
    const el = await mount();
    const sec = botSection(el);
    typeInto(sec.querySelector<HTMLInputElement>("#zalo-bot-name")!, "Bot Xã Mới");
    act(() => button(sec, "Lưu con bot").click());
    await settle();
    const put = calls.find((c) => c.method === "PUT" && c.url === "/api/v1/zalo-bots/current")!;
    expect(JSON.parse(put.body!)).toEqual({ bot_name: "Bot Xã Mới", chat_url: "https://zalo.me/000000001" });
    expect(H.toast.success).toHaveBeenCalledWith("Đã lưu con bot.");
  });

  it("own bot + NEW token = possible replace → confirmed with the count first", async () => {
    H.phien = session(["admin.lookup"]);
    botNow = OWN_BOT;
    const el = await mount();
    const sec = botSection(el);
    typeInto(sec.querySelector<HTMLInputElement>("#zalo-bot-token")!, TOKEN_FAKE);
    act(() => button(sec, "Lưu con bot").click());
    await settle();
    expect(el.querySelector("[data-relink]")!.textContent).toContain(
      "Nếu mã mới là của một con bot khác, 5 cán bộ đang ghép nối sẽ phải ghép nối lại.",
    );
    expect(calls.some((c) => c.method === "PUT")).toBe(false);
  });

  it("a name not starting with 'Bot' is refused on the spot; nothing is sent", async () => {
    H.phien = session(["admin.lookup"]);
    botNow = OWN_BOT;
    const el = await mount();
    const sec = botSection(el);
    typeInto(sec.querySelector<HTMLInputElement>("#zalo-bot-name")!, "Nhắc việc xã");
    act(() => button(sec, "Lưu con bot").click());
    await settle();
    expect(H.toast.error).toHaveBeenCalledWith("Tên bot phải bắt đầu bằng “Bot”.");
    expect(calls.some((c) => c.method === "PUT")).toBe(false);
  });

  it("server refusal on save is shown in its words (422 zalo_token_rejected)", async () => {
    H.phien = session(["admin.lookup"]);
    botNow = OWN_BOT;
    botAnswers["PUT /api/v1/zalo-bots/current"] = {
      status: 422,
      body: { code: "zalo_token_rejected", message: "Zalo không nhận mã bot này. Chưa có gì được lưu.", trace_id: "" },
    };
    const el = await mount();
    const sec = botSection(el);
    typeInto(sec.querySelector<HTMLInputElement>("#zalo-bot-name")!, "Bot Xã Thử 2");
    act(() => button(sec, "Lưu con bot").click());
    await settle();
    expect(H.toast.error).toHaveBeenCalledWith("Zalo không nhận mã bot này. Chưa có gì được lưu.");
  });

  it("Kiểm tra kết nối: POST check, the outcome's sentence as a toast", async () => {
    H.phien = session(["admin.lookup"]);
    botNow = OWN_BOT;
    botAnswers["POST /api/v1/zalo-bots/current/check"] = {
      status: 200,
      body: { result: "token-bi-tu-choi", checked_at: "2026-10-08T03:00:00Z" },
    };
    const el = await mount();
    act(() => button(botSection(el), "Kiểm tra kết nối").click());
    await settle();
    expect(calls.some((c) => c.method === "POST" && c.url === "/api/v1/zalo-bots/current/check")).toBe(true);
    expect(H.toast.error).toHaveBeenCalledWith(expect.stringContaining("Zalo không nhận mã bot"));
  });

  it("Đăng ký webhook: the secret is shown ONCE with its sentence, and gone for good after 'Tôi đã chép, đóng'", async () => {
    H.phien = session(["admin.lookup"]);
    botNow = OWN_BOT;
    botAnswers["POST /api/v1/zalo-bots/current/webhook"] = {
      status: 200,
      body: { result: "thanh-cong", url: "https://xa.example.test/api/v1/zalo-bot-updates", secret: SECRET_FAKE },
    };
    const el = await mount();
    act(() => button(botSection(el), "Đăng ký webhook").click());
    await settle();
    const boxEl = el.querySelector<HTMLElement>("[data-webhook-secret]")!;
    const input = boxEl.querySelector<HTMLInputElement>("#zalo-bot-secret")!;
    expect(input.value).toBe(SECRET_FAKE);
    expect(input.readOnly).toBe(true);
    expect(input.name).toBe("");
    expect(boxEl.textContent).toContain("Mã này chỉ hiện một lần");
    // Never in an attribute a screen reader or a tooltip reads out.
    for (const n of el.querySelectorAll("*")) {
      for (const attr of ["aria-label", "title", "placeholder"]) expect(n.getAttribute(attr) ?? "").not.toContain(SECRET_FAKE);
    }
    expect(H.toast.success).toHaveBeenCalledWith(expect.stringContaining("Đã đăng ký webhook"));

    act(() => button(el, "Tôi đã chép, đóng").click());
    await settle();
    expect(el.querySelector("[data-webhook-secret]")).toBeNull();
    expect(el.innerHTML).not.toContain(SECRET_FAKE);
  });

  it("webhook refused by Zalo: no secret box, 'Chưa đăng ký được.'", async () => {
    H.phien = session(["admin.lookup"]);
    botNow = OWN_BOT;
    botAnswers["POST /api/v1/zalo-bots/current/webhook"] = {
      status: 200,
      body: { result: "token-bi-tu-choi", url: "https://xa.example.test/api/v1/zalo-bot-updates" },
    };
    const el = await mount();
    act(() => button(botSection(el), "Đăng ký webhook").click());
    await settle();
    expect(el.querySelector("[data-webhook-secret]")).toBeNull();
    expect(H.toast.error).toHaveBeenCalledWith(expect.stringMatching(/^Chưa đăng ký được\. /));
  });

  it("Quay về bot chung: count shown, reason REQUIRED, DELETE {reason}, revoke_notice shown", async () => {
    H.phien = session(["admin.lookup"]);
    botNow = OWN_BOT;
    botAnswers["DELETE /api/v1/zalo-bots/current"] = {
      status: 200,
      body: { retired: true, ended_link_count: 5, revoke_notice: "Mã của con bot cũ vẫn còn hiệu lực ở phía Zalo." },
    };
    const el = await mount();
    act(() => button(botSection(el), "Quay về bot chung").click());
    await settle();
    const dialog = el.querySelector<HTMLElement>("dialog")!;
    expect(dialog.querySelector("[data-relink]")!.textContent).toContain("5 cán bộ đang ghép nối sẽ phải ghép nối lại.");

    act(() => button(dialog, "Quay về bot chung").click());
    await settle();
    expect(calls.some((c) => c.method === "DELETE")).toBe(false);
    expect(dialog.textContent).toContain("Cần nhập lý do quay về bot chung");

    typeInto(dialog.querySelector<HTMLInputElement>("#zalo-bot-retire-reason")!, "  Xã thôi dùng bot riêng  ");
    botNow = SHARED_BOT;
    act(() => button(dialog, "Quay về bot chung").click());
    await settle();
    const del = calls.find((c) => c.method === "DELETE")!;
    expect(del.url).toBe("/api/v1/zalo-bots/current");
    expect(JSON.parse(del.body!)).toEqual({ reason: "Xã thôi dùng bot riêng" });
    expect(H.toast.success).toHaveBeenCalledWith("Đã quay về bot chung. 5 cán bộ cần ghép nối lại.");
    expect(el.querySelector("[data-revoke-notice]")!.textContent).toBe("Mã của con bot cũ vẫn còn hiệu lực ở phía Zalo.");
    expect(botSection(el).querySelector("[data-bot-pill]")!.textContent).toBe("Bot chung");
  });
});
