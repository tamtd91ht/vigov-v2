import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

/**
 * TASK-07 (ADR 0091) — the commune-app screens that needed new server data:
 *
 *   1. the disaster-alert screen and 2. its home banner   (`GET /api/v1/commune-disaster-alerts?host=`, public)
 *   3. the "Nhận thông báo" switch                          (`GET`/`PUT /api/v1/my-notification-preferences`, session)
 *   4. the step times on the petition timeline              (`classified_at` … `closed_at`)
 *   5. the per-field description example                    (`placeholder` on the field list)
 *   6. the video length badge                               (`video_duration_seconds`)
 *
 * Screens are rendered with `renderToStaticMarkup` (no DOM in this repository); the switch's behaviour is its PURE
 * controller, stepped with fakes. The session is faked for THIS file only, through `session.current`, as
 * `news-rate-limit.test.tsx` does — product code has no slot to put one in.
 */
const session = vi.hoisted(() => ({
  current: null as { token: string; ten_xa: string; phone_verified: boolean } | null,
}));
vi.mock("../api/phien-vigov", () => ({ layPhienViGov: () => session.current })); // vi-name-ok: existing module

import {
  communeDisasterAlerts,
  myNotificationPreference,
  type NotificationPreferenceResult,
  setMyNotificationPreference,
} from "../api/goi-vigov"; // vi-name-ok: existing module
// vi-name-ok: existing contract exports (`docTrangTinXa`, `TinXaTomTat`)
import { type DisasterAlert, docTrangTinXa, readDisasterAlerts, type TinXaTomTat } from "../api/hop-dong-cong-khai";
import { docPhieu, type PhieuCuaToi, readCitizenFields } from "../api/hop-dong-phan-anh"; // vi-name-ok: existing exports
import { notificationPreferenceBody, readNotificationPreference } from "../api/notification-preference";

import {
  DisasterAlertBody,
  type DisasterAlertsState,
  HomeAlertBanner,
  loadDisasterAlerts,
  zoneWords,
} from "./disaster-alerts";
import { VideoTile } from "./media-screens";
import { XA_PA, XA_TN } from "./noi-dung"; // vi-name-ok: existing string tables
import { createNotificationPreference, type NotificationPreferenceState } from "./notification-preference";
import { describeExample, DongThoiGian, stepTime } from "./PhanAnhAppXa"; // vi-name-ok: existing timeline component
import { CaNhanXa, NotificationSwitchRow } from "./TienIchAppXa"; // vi-name-ok: existing screen

const DOMAIN = "xa-thu.vigov.example";
const COMMUNE = "Xã Thử Nghiệm";
const html = (e: Parameters<typeof renderToStaticMarkup>[0]) => renderToStaticMarkup(e);
const noop = () => {};

type FakeAnswer = { status: number; ok: boolean; json: () => Promise<unknown> };
const answer = (status: number, body: unknown): FakeAnswer => ({ status, ok: status < 300, json: async () => body });

/** Stubs `fetch` with fixed answers (the last repeats); returns every call's URL and init. */
function stubFetch(...answers: FakeAnswer[]): Array<{ url: string; init: RequestInit }> {
  const calls: Array<{ url: string; init: RequestInit }> = [];
  vi.stubGlobal("fetch", (url: string, init: RequestInit) => {
    calls.push({ url, init });
    return Promise.resolve(answers[Math.min(calls.length - 1, answers.length - 1)]!);
  });
  return calls;
}

afterEach(() => {
  vi.unstubAllGlobals();
  session.current = null;
});

/* ═══════════════════════════════ 1 · 2 — DISASTER ALERTS ═══════════════════════════════ */

/** The server's public shape (`disaster_alerts.go` `publicDisasterAlertOut`). */
const WIRE_ALERT = {
  id: "al-01",
  kind: "mua-lon",
  title: "Mưa lớn diện rộng, ngập cục bộ",
  headline: "Cảnh báo mưa lớn, ngập cục bộ — chạm để xem chi tiết",
  level: "Cấp độ rủi ro thiên tai: Cấp 2",
  detail: "Bà con hạn chế ra đường.",
  effective_from: "2026-10-11T01:00:00Z",
  effective_to: "2026-10-12T11:00:00Z",
  danger_zone: [
    { x: 18, y: 26 },
    { x: 46, y: 18 },
    { x: 72, y: 34 },
  ],
  evacuation_points: [{ name: "Trường Tiểu học Thử", address: "Thôn Một", phone: "0900000000", capacity: 200 }],
  emergency_contacts: [{ role: "Trực ban PCTT xã", name: "Cán bộ Thử", phone: "0900 000 000" }],
};

const ALERT = readDisasterAlerts({ items: [WIRE_ALERT] })![0]!;
const ready = (items: readonly DisasterAlert[]): DisasterAlertsState => ({ kind: "ready", items });

describe("disaster alerts — the contract", () => {
  it("reads the public shape; headline falls back to the title; level and end are optional", () => {
    expect(ALERT).toEqual({
      id: "al-01",
      kind: "mua-lon",
      title: WIRE_ALERT.title,
      headline: WIRE_ALERT.headline,
      level: WIRE_ALERT.level,
      detail: WIRE_ALERT.detail,
      effectiveFrom: WIRE_ALERT.effective_from,
      effectiveTo: WIRE_ALERT.effective_to,
      dangerZone: WIRE_ALERT.danger_zone,
      evacuationPoints: [{ name: "Trường Tiểu học Thử", address: "Thôn Một", phone: "0900000000", capacity: 200 }],
      emergencyContacts: [{ role: "Trực ban PCTT xã", name: "Cán bộ Thử", phone: "0900 000 000" }],
    });
    const bare = readDisasterAlerts({
      items: [
        {
          ...WIRE_ALERT,
          headline: undefined,
          level: undefined,
          effective_to: undefined,
          evacuation_points: [{ name: "Nhà văn hoá" }],
        },
      ],
    })![0]!;
    expect(bare.headline).toBe(WIRE_ALERT.title);
    expect(bare.level).toBe("");
    expect("effectiveTo" in bare).toBe(false);
    expect(bare.evacuationPoints).toEqual([{ name: "Nhà văn hoá", address: "", phone: "", capacity: null }]);
  });

  it("a zone of fewer than three points is no zone; a wrong type anywhere is a malformed answer", () => {
    expect(readDisasterAlerts({ items: [{ ...WIRE_ALERT, danger_zone: [{ x: 1, y: 1 }] }] })![0]!.dangerZone).toEqual([]);
    expect(readDisasterAlerts({ items: [] })).toEqual([]);
    for (const broken of [
      { ...WIRE_ALERT, title: 3 },
      { ...WIRE_ALERT, effective_from: undefined },
      { ...WIRE_ALERT, danger_zone: [{ x: 120, y: 1 }, { x: 1, y: 1 }, { x: 2, y: 2 }] },
      { ...WIRE_ALERT, evacuation_points: [{ name: "A", capacity: -1 }] },
      { ...WIRE_ALERT, emergency_contacts: [{ role: "A", name: "B" }] },
    ]) {
      expect(readDisasterAlerts({ items: [broken] })).toBeNull();
    }
    expect(readDisasterAlerts({ items: [WIRE_ALERT, WIRE_ALERT] }), "a repeated id").toBeNull();
    expect(readDisasterAlerts({})).toBeNull();
  });

  it("the read: comms host, `?host=` only, NO bearer even with a session; an unknown host's [] is 'no alert'", async () => {
    session.current = { token: "tok-thu", ten_xa: COMMUNE, phone_verified: true };
    const calls = stubFetch(answer(200, { items: [] }));
    expect(await communeDisasterAlerts(DOMAIN)).toEqual({ kieu: "xong", gia_tri: [] });
    const url = new URL(calls[0]!.url);
    expect(url.origin + url.pathname).toBe("https://comms.api.vigov.vn/api/v1/commune-disaster-alerts");
    expect([...url.searchParams.keys()]).toEqual(["host"]);
    expect(url.searchParams.get("host")).toBe(DOMAIN);
    expect(calls[0]!.init.headers as Record<string, string>).not.toHaveProperty("Authorization");
    // A malformed domain never leaves the phone.
    expect((await communeDisasterAlerts("not a domain")).kieu).toBe("khong-hop-le");
    expect(calls).toHaveLength(1);
  });

  it("429: ONE retry after the server's wait (capped), then the answer — never a loop", async () => {
    const waits: number[] = [];
    const wait = async (ms: number) => void waits.push(ms);
    const twice = vi
      .fn()
      .mockResolvedValueOnce({ kieu: "rate-limited", retryAfterSeconds: 7 })
      .mockResolvedValueOnce({ kieu: "xong", gia_tri: [ALERT] });
    expect(await loadDisasterAlerts(twice, wait)).toEqual(ready([ALERT]));
    expect(waits).toEqual([7000]);
    const always = vi.fn().mockResolvedValue({ kieu: "rate-limited", retryAfterSeconds: 900 });
    expect(await loadDisasterAlerts(always, wait)).toEqual({ kind: "failed", error: "rate-limited", retryAfterSeconds: 900 });
    expect(always).toHaveBeenCalledTimes(2);
    expect(waits[1]).toBe(60_000);
    expect(await loadDisasterAlerts(async () => ({ kieu: "loi-mang" }), wait)).toEqual({
      kind: "failed",
      error: "loi-mang",
      retryAfterSeconds: null,
    });
  });
});

describe("disaster alerts — the home banner", () => {
  it("hidden while loading, after a failure, and when nothing is in force", () => {
    const states: DisasterAlertsState[] = [
      { kind: "loading" },
      { kind: "failed", error: "loi-mang", retryAfterSeconds: null },
      ready([]),
    ];
    for (const state of states) expect(html(createElement(HomeAlertBanner, { state, onOpen: noop }))).toBe("");
  });

  it("one tappable band with the NEWEST alert's headline, named as a warning in words", () => {
    const older = { ...ALERT, id: "al-00", headline: "Cảnh báo cũ" };
    const band = html(createElement(HomeAlertBanner, { state: ready([ALERT, older]), onOpen: noop }));
    expect(band).toContain(`<button type="button" class="xa-alert-banner" aria-label="${XA_TN.alert_banner_label(ALERT.headline)}"`);
    expect(band).toContain(`<span class="xa-alert-banner__text">${ALERT.headline}</span>`);
    expect(band).not.toContain("Cảnh báo cũ");
    expect(XA_TN.alert_banner_label("X")).toBe("Cảnh báo thiên tai: X");
  });
});

describe("disaster alerts — the screen", () => {
  const body = (state: DisasterAlertsState) =>
    html(createElement(DisasterAlertBody, { state, communeName: COMMUNE, onRetry: noop }));

  it("empty: the prototype's sentence verbatim, and the hint names the commune — no Zalo promise (no ZNS yet)", () => {
    const page = body(ready([]));
    expect(page).toContain("Hiện không có cảnh báo nào");
    expect(page).toContain(`Khi có bão, lũ, ngập lụt hay sạt lở, ${COMMUNE} sẽ gửi cảnh báo tới đây.`);
    expect(page).not.toContain("báo qua Zalo");
  });

  it("loading says so in words; a failure says what to do and offers 'Thử lại'", () => {
    expect(body({ kind: "loading" })).toContain(XA_TN.alert_loading);
    const failed = body({ kind: "failed", error: "loi-mang", retryAfterSeconds: null });
    expect(failed).toContain('role="alert"');
    expect(failed).toContain("Thử lại");
  });

  it("an alert: red frame with title, level, both times, the detail; zone, evacuation and calls", () => {
    const page = body(ready([ALERT]));
    expect(page).toContain(`<h2 class="xa-alert__title" id="xa-alert-al-01">${ALERT.title}</h2>`);
    expect(page).toContain(`<p class="xa-alert__level">${ALERT.level}</p>`);
    expect(page).toContain("<dt>Có hiệu lực từ:</dt><dd>08:00 ngày 11/10/2026</dd>");
    expect(page).toContain("<dt>Đến:</dt><dd>18:00 ngày 12/10/2026</dd>");
    expect(page).toContain(ALERT.detail);
    // The prototype's heading and caption for a flood-type alert, verbatim.
    expect(page).toContain("Vùng có nguy cơ ngập");
    expect(page).toContain("Vùng tô đỏ là khu vực có nguy cơ ngập cao. Bản đồ chỉ mang tính minh hoạ vị trí tương đối.");
    expect(page).toContain('points="18,26 46,18 72,34"');
    expect(page).toContain(`aria-label="Bản đồ minh hoạ vùng có nguy cơ ngập trong địa bàn ${COMMUNE}"`);
    // Evacuation: name, address, capacity — and NO distance (ADR 0091 §2).
    expect(page).toContain("Điểm sơ tán gần bà con");
    expect(page).toContain("Sức chứa khoảng 200 người");
    expect(page).not.toMatch(/Cách \d|km\b/);
    // The calls: tel: links in words, the dialled number cleaned.
    expect(page).toContain("Số điện thoại khẩn cấp");
    expect(page).toContain('<a class="xa-call-button" href="tel:0900000000" aria-label="Gọi Trường Tiểu học Thử, số 0900000000">');
    expect(page).toContain('href="tel:0900000000" aria-label="Gọi Trực ban PCTT xã, số 0900 000 000"');
    expect(page).toContain("GỌI 0900 000 000");
  });

  it("no end → no 'Đến:'; no zone → no map; no points → no empty heading; a phone with no digits → no button", () => {
    const { effectiveTo: _end, ...rest } = ALERT;
    const bare: DisasterAlert = {
      ...rest,
      dangerZone: [],
      evacuationPoints: [],
      emergencyContacts: [{ role: "Trực ban", name: "", phone: "—" }],
    };
    const page = body(ready([bare]));
    expect(page).not.toContain("Đến:");
    expect(page).not.toContain("<svg viewBox=\"0 0 100 100\"");
    expect(page).not.toContain("Điểm sơ tán");
    expect(page).not.toContain("xa-call-button");
  });

  it("a fire or an epidemic is never called a flood zone", () => {
    expect(zoneWords("chay", COMMUNE).title).toBe("Vùng nguy hiểm");
    expect(zoneWords("dich-benh", COMMUNE).caption).not.toContain("ngập");
    expect(zoneWords("bao", COMMUNE).title).toBe("Vùng có nguy cơ ngập");
  });

  it("several alerts in force: all of them, in the server's order (newest first)", () => {
    const second = { ...ALERT, id: "al-02", title: "Sạt lở đồi Thử", kind: "sat-lo" };
    const page = body(ready([ALERT, second]));
    expect(page.match(/<article class="xa-alert"/g)).toHaveLength(2);
    expect(page.indexOf(ALERT.title)).toBeLessThan(page.indexOf("Sạt lở đồi Thử"));
  });
});

/* ═══════════════════════════════ 3 — "NHẬN THÔNG BÁO" ═══════════════════════════════ */

describe("notification switch — the contract and the calls", () => {
  it("body and reader: one boolean key, never a guessed default", () => {
    expect(notificationPreferenceBody(false)).toBe('{"zalo_enabled":false}');
    expect(readNotificationPreference({ zalo_enabled: true })).toBe(true);
    expect(readNotificationPreference({})).toBeNull();
    expect(readNotificationPreference({ zalo_enabled: "false" })).toBeNull();
  });

  it("no session → nothing leaves the phone", async () => {
    const calls = stubFetch(answer(200, { zalo_enabled: true }));
    expect(await myNotificationPreference()).toEqual({ kieu: "chua-co-phien" });
    expect(await setMyNotificationPreference(false)).toEqual({ kieu: "chua-co-phien" });
    expect(calls).toEqual([]);
  });

  it("GET and PUT on comms with the session's bearer; no identity, no commune in the URL or the body", async () => {
    session.current = { token: "tok-thu", ten_xa: COMMUNE, phone_verified: true };
    const calls = stubFetch(answer(200, { zalo_enabled: true }), answer(200, { zalo_enabled: false }));
    expect(await myNotificationPreference()).toEqual({ kieu: "xong", zaloEnabled: true });
    expect(await setMyNotificationPreference(false)).toEqual({ kieu: "xong", zaloEnabled: false });
    for (const c of calls) {
      expect(c.url).toBe("https://comms.api.vigov.vn/api/v1/my-notification-preferences");
      expect((c.init.headers as Record<string, string>)["Authorization"]).toBe("Bearer tok-thu");
    }
    expect(calls[0]!.init.method).toBe("GET");
    expect(calls[1]!.init.method).toBe("PUT");
    expect(calls[1]!.init.body).toBe('{"zalo_enabled":false}');
  });

  it("401 → het-phien, 403 chua_xac_thuc_so → can-xac-thuc-so, a stray 404 → loi-may-chu", async () => {
    session.current = { token: "tok-thu", ten_xa: COMMUNE, phone_verified: true };
    stubFetch(answer(401, {}));
    expect(await myNotificationPreference()).toEqual({ kieu: "het-phien" });
    stubFetch(answer(403, { code: "chua_xac_thuc_so" }));
    expect(await setMyNotificationPreference(true)).toEqual({ kieu: "can-xac-thuc-so" });
    stubFetch(answer(404, {}));
    expect(await myNotificationPreference()).toEqual({ kieu: "loi-may-chu" });
  });
});

describe("notification switch — never optimistic, reverts on failure", () => {
  function harness(
    read: NotificationPreferenceResult,
    write: NotificationPreferenceResult | (() => Promise<NotificationPreferenceResult>),
  ) {
    let state: NotificationPreferenceState = { kind: "unknown" };
    const seen: NotificationPreferenceState[] = [];
    const lost: Array<() => void> = [];
    const control = createNotificationPreference({
      read: async () => read,
      write: typeof write === "function" ? write : async () => write,
      onSessionLost: (retry) => void lost.push(retry),
      getState: () => state,
      setState: (s) => {
        state = s;
        seen.push(s);
      },
    });
    return { control, seen, lost, now: () => state };
  }

  it("read → the server's value; a tap shows the OLD value while saving, then the server's answer", async () => {
    const h = harness({ kieu: "xong", zaloEnabled: true }, { kieu: "xong", zaloEnabled: false });
    await h.control.load();
    expect(h.now()).toEqual({ kind: "ready", enabled: true, saving: false, error: null });
    await h.control.toggle();
    expect(h.seen).toContainEqual({ kind: "ready", enabled: true, saving: true, error: null });
    expect(h.seen.some((s) => s.kind === "ready" && s.saving && !s.enabled), "flipped before the server answered").toBe(false);
    expect(h.now()).toEqual({ kind: "ready", enabled: false, saving: false, error: null });
  });

  it("a failed save keeps the value and says what to do — network and server have their own sentence", async () => {
    const net = harness({ kieu: "xong", zaloEnabled: true }, { kieu: "loi-mang" });
    await net.control.load();
    await net.control.toggle();
    expect(net.now()).toEqual({ kind: "ready", enabled: true, saving: false, error: XA_TN.notify_save_network });
    const server = harness({ kieu: "xong", zaloEnabled: false }, { kieu: "loi-may-chu" });
    await server.control.load();
    await server.control.toggle();
    expect(server.now()).toEqual({ kind: "ready", enabled: false, saving: false, error: XA_TN.notify_save_failed });
    expect(XA_TN.notify_save_failed).not.toMatch(/\d{3}|code|mã lỗi/i);
  });

  it("session lost on save: value kept, no sentence, the gate gets a retry that saves again", async () => {
    const answers: NotificationPreferenceResult[] = [{ kieu: "het-phien" }, { kieu: "xong", zaloEnabled: false }];
    const h = harness({ kieu: "xong", zaloEnabled: true }, async () => answers.shift()!);
    await h.control.load();
    await h.control.toggle();
    expect(h.now()).toEqual({ kind: "ready", enabled: true, saving: false, error: null });
    expect(h.lost).toHaveLength(1);
    h.lost[0]!();
    await vi.waitFor(() => expect(h.now()).toEqual({ kind: "ready", enabled: false, saving: false, error: null }));
  });

  it("no session on read → back to 'unknown' and the gate; a failed read is a sentence; not ready → no toggle", async () => {
    const h = harness({ kieu: "chua-co-phien" }, { kieu: "loi-mang" });
    await h.control.load();
    expect(h.now()).toEqual({ kind: "unknown" });
    expect(h.lost).toHaveLength(1);
    const failed = harness({ kieu: "loi-mang" }, { kieu: "loi-mang" });
    await failed.control.load();
    expect(failed.now()).toEqual({ kind: "read-failed", message: XA_TN.notify_read_network });
    await failed.control.toggle();
    expect(failed.now().kind).toBe("read-failed");
  });
});

describe("notification switch — the row on Cá nhân", () => {
  const row = (state: NotificationPreferenceState) => html(createElement(NotificationSwitchRow, { state, onPress: noop }));

  it("unknown: the prototype's label and hint, NO switch state (none is known), and what a tap does", () => {
    const page = row({ kind: "unknown" });
    expect(page).toContain("Nhận thông báo");
    expect(page).toContain("Báo qua Zalo khi phiếu phản ánh hoặc hồ sơ có kết quả");
    expect(page).not.toContain('role="switch"');
    expect(page).toContain(XA_TN.notify_unknown);
  });

  it("ready: a real switch; OFF adds the line about the two messages that still go", () => {
    const on = row({ kind: "ready", enabled: true, saving: false, error: null });
    expect(on).toContain('role="switch" aria-checked="true"');
    expect(on).not.toContain(XA_TN.notify_off_note);
    const off = row({ kind: "ready", enabled: false, saving: false, error: null });
    expect(off).toContain('role="switch" aria-checked="false"');
    expect(off).toContain(XA_TN.notify_off_note);
    expect(XA_TN.notify_off_note).toContain("không được tiếp nhận");
    expect(XA_TN.notify_off_note).toContain("chuyển lên cấp trên");
  });

  it("saving: disabled and said in words; a failed save: the sentence as an alert", () => {
    const saving = row({ kind: "ready", enabled: true, saving: true, error: null });
    expect(saving).toMatch(/role="switch" aria-checked="true" aria-busy="true"[^>]*disabled=""/);
    expect(saving).toContain(XA_TN.notify_saving);
    const failed = row({ kind: "ready", enabled: true, saving: false, error: XA_TN.notify_save_network });
    expect(failed).toContain(`role="alert">${XA_TN.notify_save_network}</p>`);
  });

  it("Cá nhân shows the row only when given one; 'Cỡ chữ lớn' stays the first switch", () => {
    const base = {
      ho_ten: null, // vi-name-ok: existing prop of CaNhanXa
      ten_xa: COMMUNE, // vi-name-ok: existing prop of CaNhanXa
      tinh: "",
      largeText: false,
      onLargeText: noop,
      onMoPhanAnh: noop,
      onMoTraCuu: noop,
      onOpenNotifications: noop,
    };
    expect(html(createElement(CaNhanXa, base))).not.toContain("Nhận thông báo");
    const page = html(
      createElement(CaNhanXa, {
        ...base,
        notification: { state: { kind: "ready", enabled: false, saving: false, error: null }, onPress: noop },
      }),
    );
    expect(page.match(/role="switch"/g)).toHaveLength(2);
    expect(page.indexOf(XA_TN.co_chu_lon)).toBeLessThan(page.indexOf("Nhận thông báo"));
    expect(page.indexOf("Nhận thông báo")).toBeLessThan(page.indexOf(XA_TN.language));
  });
});

/* ═══════════════════════════════ 4 — STEP TIMES ═══════════════════════════════ */

/** The citizen read's body (`phieu_cua_toi.go`), status `da-chuyen-xu-ly`. */
const WIRE_PETITION = {
  code: "PA7K2QX9M4TD",
  channel: "zalo-mini-app",
  status: "da-chuyen-xu-ly",
  field: "giao-thong",
  field_label: "Hạ tầng giao thông",
  content: "Ổ gà lớn trước cổng chợ.",
  address: "Thôn Một",
  reporter_name: "Nguyễn V. A.",
  reporter_phone: "09****0000",
  anonymous: false,
  clock_from: "2026-09-28T01:00:00Z",
  acknowledge_due: "2026-09-28T03:00:00Z",
  resolve_due: null,
  result: "",
  classified_at: "2026-09-28T02:30:00Z",
  forwarded_for_processing_at: "2026-09-29T07:30:00Z",
  // An earlier cycle's time on a step NOT reached now (the petition was reopened): never shown.
  resolved_at: "2026-09-20T04:00:00Z",
};

describe("petition timeline — the time beside each REACHED step, never invented", () => {
  const petition = docPhieu(WIRE_PETITION)!;
  const timeLines = (s: string) => s.match(/<span class="xa-phu">\d{2}:\d{2} ngày/g) ?? [];

  it("reads the five optional times; null/absent/unreadable → no time; a wrong type → malformed", () => {
    expect(petition.step_times).toEqual({
      classified_at: WIRE_PETITION.classified_at,
      forwarded_for_processing_at: WIRE_PETITION.forwarded_for_processing_at,
      resolved_at: WIRE_PETITION.resolved_at,
    });
    const none = docPhieu({
      ...WIRE_PETITION,
      classified_at: "không phải giờ",
      forwarded_for_processing_at: null,
      resolved_at: undefined,
    })!;
    expect(none.step_times).toBeUndefined();
    expect(docPhieu({ ...WIRE_PETITION, closed_at: 12 })).toBeNull();
  });

  it("each reached step shows its recorded time; an earlier cycle's later step is not shown", () => {
    const tl = html(createElement(DongThoiGian, { petition }));
    expect(tl).toContain("08:00 ngày 28/9/2026"); // Đã tiếp nhận — clock_from
    expect(tl).toContain("09:30 ngày 28/9/2026"); // Đang phân loại — classified_at
    expect(tl).toContain("14:30 ngày 29/9/2026"); // Đã chuyển xử lý — forwarded_for_processing_at
    // `resolved_at` is on the row, but `da-xu-ly` is not reached: its time is not shown.
    expect(tl).not.toContain("11:00 ngày 20/9/2026");
    expect(timeLines(tl)).toHaveLength(3);
  });

  it("stepTime: dang-xu-ly has no column, terminal branches expose none, an absent time is null", () => {
    const p: PhieuCuaToi = { ...petition, trang_thai: "dang-xu-ly" }; // vi-name-ok: existing contract field
    expect(stepTime(p, "dang-xu-ly")).toBeNull();
    expect(stepTime(p, "khong-tiep-nhan")).toBeNull();
    expect(stepTime(p, "da-dong")).toBeNull();
    expect(stepTime(p, "da-tiep-nhan")).toBe(WIRE_PETITION.clock_from);
    const noTimes = docPhieu({
      ...WIRE_PETITION,
      classified_at: undefined,
      forwarded_for_processing_at: undefined,
      resolved_at: undefined,
    })!;
    expect(timeLines(html(createElement(DongThoiGian, { petition: noTimes })))).toHaveLength(1);
  });
});

/* ═══════════════════════════════ 5 — DESCRIPTION EXAMPLE ═══════════════════════════════ */

describe("send form — the picked field's example, else the prototype's default", () => {
  it("the field list carries `placeholder` when sent; blank or absent is no example; a wrong type is malformed", () => {
    const fields = readCitizenFields({
      items: [
        { code: "dien", label: "Điện", icon: null, tone: null, placeholder: "Ví dụ: cột điện nghiêng ở đầu thôn" },
        { code: "nuoc", label: "Nước", icon: null, tone: null, placeholder: "   " },
        { code: "rac", label: "Rác", icon: null, tone: null },
      ],
    })!;
    expect(fields[0]!.placeholder).toBe("Ví dụ: cột điện nghiêng ở đầu thôn");
    expect("placeholder" in fields[1]!).toBe(false);
    expect("placeholder" in fields[2]!).toBe(false);
    expect(readCitizenFields({ items: [{ code: "x", label: "X", icon: null, tone: null, placeholder: 3 }] })).toBeNull();
    expect(describeExample(fields[0])).toBe("Ví dụ: cột điện nghiêng ở đầu thôn");
    expect(describeExample(fields[2])).toBe(XA_PA.goi_y_su_viec);
    expect(describeExample(undefined)).toBe("Bà con mô tả giúp sự việc, ở đâu, xảy ra khi nào.");
  });
});

/* ═══════════════════════════════ 6 — VIDEO LENGTH ═══════════════════════════════ */

describe("video grid — the typed length as a badge, only when present", () => {
  const firstItem = (extra: Record<string, unknown>): TinXaTomTat => // vi-name-ok: existing contract type
    docTrangTinXa({ // vi-name-ok: existing contract reader
      items: [
        { id: "v1", title: "Bản tin v1", summary: "", published_on: "2026-10-01", category_name: "", type: "video", ...extra },
      ],
      next_cursor: "",
      has_more: false,
    })!.muc[0]!;

  it("read only on a video, only a whole number > 0", () => {
    expect(firstItem({ video_duration_seconds: 312 }).videoDurationSeconds).toBe(312);
    for (const bad of [0, -5, 1.5, "312", null]) {
      expect("videoDurationSeconds" in firstItem({ video_duration_seconds: bad }), String(bad)).toBe(false);
    }
    expect("videoDurationSeconds" in firstItem({ type: "tin-tuc", video_duration_seconds: 312 })).toBe(false);
  });

  it("badge m:ss in the cover; the tile's name says the length in words; none without a length", () => {
    const tile = html(createElement(VideoTile, { tin: firstItem({ video_duration_seconds: 312 }), onMo: noop }));
    expect(tile).toContain('<span class="xa-video-tile__length">5:12</span>');
    expect(tile).toContain('aria-label="Xem video Bản tin v1, dài 5 phút 12 giây"');
    const without = html(createElement(VideoTile, { tin: firstItem({}), onMo: noop }));
    expect(without).not.toContain("xa-video-tile__length");
    expect(without).toContain('aria-label="Xem video Bản tin v1"');
  });
});
