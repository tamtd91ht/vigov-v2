// @vitest-environment jsdom
//
// jsdom: the tab is gated on the session and loads two routes once allowed.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { renderToStaticMarkup as renderToString } from "react-dom/server";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import type { PhienDaDoc } from "@/features/phien/phien-hien-tai";
import type { ZaloChannelSettings } from "@/lib/api/zalo";

const H = vi.hoisted(() => ({ phien: null as PhienDaDoc }));
vi.mock("@/features/phien/phien-hien-tai", () => ({ usePhien: () => H.phien }));

const { buildChange, draftFromSettings, NO_LINKED_STAFF, toggleKind } = await import("./zalo-channel-form");
const { ZaloChannelTab, ZaloChannelView } = await import("./zalo-channel-tab");

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

function session(permissions: string[]): PhienDaDoc {
  return { ok: true, duLieu: { staff: { full_name: "A", position: "B" }, role: null, permissions } } as unknown as PhienDaDoc;
}

let calls: { method: string; url: string; body?: string }[] = [];

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

beforeEach(() => {
  calls = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init: RequestInit) => {
      calls.push({ method: String(init.method ?? "GET"), url, body: typeof init.body === "string" ? init.body : undefined });
      const body =
        url === "/api/v1/zalo-links"
          ? { items: [{ staff_code: "CB-00123", staff_name: "Nguyễn Văn A", linked_at: "2026-10-05T03:00:00Z" }] }
          : SETTINGS;
      return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } });
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

async function mount(): Promise<HTMLDivElement> {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() => r.render(<ZaloChannelTab />));
  for (let i = 0; i < 5; i++) await act(async () => {});
  return host;
}

describe("form check (a hint; comms re-checks)", () => {
  it("turning on needs at least one kind", () => {
    expect(buildChange({ ...draftFromSettings(OFF), isEnabled: true })).toMatchObject({ ok: false, field: "kinds" });
    // Off with no kind is a valid state: the commune with no row is exactly that.
    expect(buildChange(draftFromSettings(OFF))).toMatchObject({ ok: true, change: { is_enabled: false, kinds: [] } });
  });

  it("qua-han chosen → both cadences required, whole days ≥ 1", () => {
    const d = toggleKind({ ...draftFromSettings(OFF), isEnabled: true }, "qua-han", true);
    expect(buildChange(d)).toMatchObject({ ok: false, field: "lateStartDays" });
    expect(buildChange({ ...d, lateStartDays: "0", lateRepeatDays: "2" })).toMatchObject({ ok: false, field: "lateStartDays" });
    expect(buildChange({ ...d, lateStartDays: "1.5", lateRepeatDays: "2" })).toMatchObject({ ok: false, field: "lateStartDays" });
    expect(buildChange({ ...d, lateStartDays: "1", lateRepeatDays: "" })).toMatchObject({ ok: false, field: "lateRepeatDays" });
    expect(buildChange({ ...d, lateStartDays: "1", lateRepeatDays: "3" })).toEqual({
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

  it("qua-han un-ticked → both cadences sent as null, whatever was typed", () => {
    const d = toggleKind(draftFromSettings(SETTINGS), "qua-han", false);
    expect(buildChange(d)).toMatchObject({ ok: true, change: { kinds: ["sap-den-han"], overdue_start_after_days: null, overdue_repeat_every_days: null } });
  });

  it("kinds keep the fixed order whatever the click order; quiet hours must be HH:MM", () => {
    let d = draftFromSettings(OFF);
    d = toggleKind(d, "ban-tin-tuan", true);
    d = toggleKind(d, "sap-den-han", true);
    expect(d.kinds).toEqual(["sap-den-han", "ban-tin-tuan"]);
    expect(buildChange({ ...d, quietStart: "25:00" })).toMatchObject({ ok: false, field: "quiet" });
  });
});

describe("view", () => {
  it("cadence boxes only with qua-han; who saved and when; never a chat id", () => {
    const html = renderToString(
      <ZaloChannelView
        saved={SETTINGS}
        draft={draftFromSettings(SETTINGS)}
        setDraft={() => {}}
        saving={false}
        saveMessage={null}
        onSave={() => {}}
        staff={{ ok: true, duLieu: [{ staff_code: "CB-00123", staff_name: "Nguyễn Văn A", linked_at: "2026-10-05T03:00:00Z" }] }}
      />,
    );
    expect(html).toContain('name="overdue_start_after_days"');
    expect(html).toContain("CB-00123");
    expect(html).toContain("Nguyễn Văn A");
    expect(html).not.toMatch(/chat_id/i);

    const off = renderToString(
      <ZaloChannelView
        saved={OFF}
        draft={draftFromSettings(OFF)}
        setDraft={() => {}}
        saving={false}
        saveMessage={null}
        onSave={() => {}}
        staff={{ ok: true, duLieu: [] }}
      />,
    );
    expect(off).not.toContain('name="overdue_start_after_days"');
    expect(off).toContain(NO_LINKED_STAFF);
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

  it("ALLOWED: loads settings and the linked list, then saves exactly the form", async () => {
    H.phien = session(["admin.lookup"]);
    const el = await mount();
    expect(calls.map((c) => c.url).sort()).toEqual(["/api/v1/zalo-channel-settings", "/api/v1/zalo-links"]);
    expect(el.textContent).toContain("Nguyễn Văn A");
    const form = el.querySelector<HTMLFormElement>('form[aria-label="Cấu hình kênh Zalo"]')!;
    act(() => form.requestSubmit());
    for (let i = 0; i < 5; i++) await act(async () => {});
    const put = calls.find((c) => c.method === "PUT")!;
    expect(JSON.parse(put.body!)).toEqual({
      is_enabled: true,
      kinds: ["sap-den-han", "qua-han"],
      quiet_start: "21:00",
      quiet_end: "06:00",
      overdue_start_after_days: 1,
      overdue_repeat_every_days: 2,
    });
    expect(el.textContent).toContain("Đã lưu cấu hình kênh Zalo.");
  });
});
