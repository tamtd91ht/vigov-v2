// @vitest-environment jsdom
//
// jsdom: the tab is gated on the session, loads four routes once allowed, and saves on every change.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { renderToStaticMarkup as renderToString } from "react-dom/server";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import type { PhienDaDoc } from "@/features/phien/phien-hien-tai";
import type { ZaloChannelSettings } from "@/lib/api/zalo";

const H = vi.hoisted(() => ({
  phien: null as PhienDaDoc,
  toast: { success: vi.fn(), error: vi.fn() },
}));
vi.mock("@/features/phien/phien-hien-tai", () => ({ usePhien: () => H.phien }));
vi.mock("sonner", () => ({ toast: H.toast }));

const {
  buildChange,
  dayOptions,
  draftFromSettings,
  hourOptions,
  joinStaffLinks,
  NO_LINKED_STAFF,
  toggleKind,
  ZALO_EVENT_GROUPS,
} = await import("./zalo-channel-form");
const { peopleState, ZaloChannelTab, ZaloChannelView } = await import("./zalo-channel-tab");

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

const OFF: ZaloChannelSettings = {
  is_enabled: false,
  kinds: [],
  quiet_start: "21:00",
  quiet_end: "06:00",
  overdue_start_after_days: null,
  overdue_repeat_every_days: null,
};

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

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

beforeEach(() => {
  calls = [];
  stored = SETTINGS;
  putAnswer = null;
  directoryAnswer = { status: 200, body: { items: DIRECTORY } };
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
      if (method === "PUT") return putAnswer ? json(putAnswer.status, putAnswer.body) : json(200, JSON.parse(body!));
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
  for (let i = 0; i < 6; i++) await act(async () => {});
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

function view(settings: ZaloChannelSettings, links = LINKED, directory: typeof DIRECTORY | null = DIRECTORY) {
  const dir =
    directory === null ? { ok: false as const, thongBao: "Máy chủ bận" } : { ok: true as const, duLieu: { items: directory } };
  return renderToString(
    <ZaloChannelView
      draft={draftFromSettings(settings)}
      onChange={() => {}}
      saving={false}
      held={null}
      people={peopleState({ ok: true, duLieu: links }, dir, { ok: true, duLieu: { items: UNITS } })}
    />,
  );
}

function html(markup: string): HTMLDivElement {
  host = document.createElement("div");
  host.innerHTML = markup;
  return host;
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

  it("qua-han chosen → both cadences required", () => {
    const d = toggleKind({ ...draftFromSettings(OFF), isEnabled: true }, "qua-han", true);
    expect(buildChange(d)).toMatchObject({ ok: false, held: false });
    expect(buildChange({ ...d, lateStartDays: 1, lateRepeatDays: 3 })).toEqual({
      ok: true,
      change: {
        is_enabled: true,
        kinds: ["qua-han"],
        quiet_start: "21:00",
        quiet_end: "06:00",
        overdue_start_after_days: 1,
        overdue_repeat_every_days: 3,
      },
    });
  });

  it("qua-han un-ticked keeps the cadence shown on screen (both set is valid without it)", () => {
    const d = toggleKind(draftFromSettings(SETTINGS), "qua-han", false);
    expect(buildChange(d)).toMatchObject({
      ok: true,
      change: { kinds: ["sap-den-han"], overdue_start_after_days: 1, overdue_repeat_every_days: 2 },
    });
  });

  it("kinds keep the fixed order whatever the click order", () => {
    let d = draftFromSettings(OFF);
    d = toggleKind(d, "ban-tin-tuan", true);
    d = toggleKind(d, "sap-den-han", true);
    expect(d.kinds).toEqual(["sap-den-han", "ban-tin-tuan"]);
  });

  it("selects: whole hours, days min…14 — a stored value outside them stays visible", () => {
    expect(hourOptions("21:00")).toHaveLength(24);
    expect(hourOptions("21:30")).toContain("21:30");
    expect(dayOptions(0, 1)).toEqual([0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14]);
    expect(dayOptions(1, 30)).toContain(30);
  });
});

describe("view", () => {
  it("event table: every server kind on a real checkbox, every other spec event disabled with '?'", () => {
    const codes = ZALO_EVENT_GROUPS.flatMap((g) => g.events);
    expect(codes).toHaveLength(18); // spec 11 §2 table
    const mapped = Object.fromEntries(codes.filter((e) => e.kind !== null).map((e) => [e.code, e.kind]));
    expect(mapped).toEqual({
      "task.due_soon": "sap-den-han",
      "document.due_soon": "sap-den-han",
      "feedback.due_soon": "sap-den-han",
      "task.overdue": "qua-han",
      "task.unassigned_too_long": "qua-han",
      "document.overdue": "qua-han",
      "feedback.overdue": "qua-han",
      "task.escalated": "leo-thang",
      "digest.weekly": "ban-tin-tuan",
    });

    const el = html(view(SETTINGS));
    for (const code of ["task.overdue", "document.overdue", "feedback.overdue", "task.unassigned_too_long"]) {
      expect(box(el, code).checked).toBe(true);
    }
    expect(box(el, "digest.weekly").checked).toBe(false);
    expect(box(el, "task.assigned").disabled).toBe(true);
    expect(el.querySelectorAll('[aria-label^="Thêm loại việc nhắn qua Zalo"]')).toHaveLength(9);
  });

  it("bot section is its shape only: disabled, nameless, one '?'; no chat id; no 'ViGov', no 'và thư'", () => {
    const markup = view(SETTINGS);
    const el = html(markup);
    for (const id of ["zalo-bot-token", "zalo-bot-name", "zalo-bot-chat-url", "zalo-bot-secret"]) {
      const input = el.querySelector<HTMLInputElement>(`#${id}`)!;
      expect(input.disabled).toBe(true);
      expect(input.name).toBe("");
    }
    expect(el.querySelectorAll('[aria-label^="Con bot của xã — "]')).toHaveLength(1);
    // REGRESSION: a password input outside a <form> makes the browser warn on every load.
    const token = el.querySelector<HTMLInputElement>("#zalo-bot-token")!;
    expect(token.closest("form")).not.toBeNull();
    expect(token.closest("fieldset")!.disabled).toBe(true);
    expect(markup).toContain("Bot chung");
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
  it("DENIED: without admin.lookup the tab says so and requests NOTHING", async () => {
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
      "/api/v1/zalo-channel-settings",
      "/api/v1/zalo-links",
    ]);
    expect(el.textContent).toContain("Nguyễn Văn A");
    act(() => box(el, "digest.weekly").click());
    await settle();
    const put = calls.find((c) => c.method === "PUT")!;
    expect(JSON.parse(put.body!)).toEqual({
      is_enabled: true,
      kinds: ["sap-den-han", "qua-han", "ban-tin-tuan"],
      quiet_start: "21:00",
      quiet_end: "06:00",
      overdue_start_after_days: 1,
      overdue_repeat_every_days: 2,
    });
    expect(H.toast.success).toHaveBeenCalledWith("Đã lưu.");
    expect(box(el, "digest.weekly").checked).toBe(true);
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
});
