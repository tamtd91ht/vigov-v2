// @vitest-environment jsdom
//
// "OA Zalo của xã" and "Mẫu tin ZBS" (ADR 0098 khối A), mounted inside the real `ZaloChannelTab` so the
// admin.zalo gate, the slot order and the one-time `?oa=` handling are all exercised.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import type { PhienDaDoc } from "@/features/phien/phien-hien-tai";
import type { comms_communeZaloOACurrentOut, comms_zaloOATemplateOut } from "@/lib/api/zalo-oa";

const H = vi.hoisted(() => ({
  phien: null as PhienDaDoc,
  toast: { success: vi.fn(), error: vi.fn() },
}));
vi.mock("@/features/phien/phien-hien-tai", () => ({ usePhien: () => H.phien }));
vi.mock("sonner", () => ({ toast: H.toast }));
vi.mock("@/components/cau-hinh-xa", () => ({
  useCauHinhXa: () => ({ displayName: "Xã Thử Nghiệm", parentAuthority: "", logoUrl: "", webAdminBannerUrl: "" }),
}));

const form = await import("./zalo-oa-form");
const { ZaloChannelTab } = await import("./zalo-channel-tab");

const SETTINGS = {
  is_enabled: true,
  kinds: ["nhiem-vu.sap-den-han"],
  quiet_start: "21:00",
  quiet_end: "06:00",
  overdue_start_after_days: null,
  overdue_repeat_every_days: null,
  due_soon_days: null,
  supported_events: ["nhiem-vu.sap-den-han"],
  platform_ready: true,
};

const NO_OA: comms_communeZaloOACurrentOut = { configured: true, has_oa: false, oa: null, template_count: 0, attached_count: 0 };
const LIVE_OA: comms_communeZaloOACurrentOut = {
  configured: true,
  has_oa: true,
  template_count: 2,
  attached_count: 1,
  oa: {
    oa_id: "1234567890",
    oa_name: "UBND Xã Thử",
    authorized_at: "2026-10-16T01:00:00Z",
    authorized_by: "CB-00123",
    authorized_by_name: "Nguyễn Văn A",
    access_expires_at: "2026-10-17T02:00:00Z",
    token_refreshed_at: "2026-10-16T01:00:00Z",
    needs_reauthorization: null,
    last_check: { at: "2026-10-16T02:00:00Z", result: "thanh-cong" },
  },
};
const REAUTH_OA: comms_communeZaloOACurrentOut = {
  ...LIVE_OA,
  oa: { ...LIVE_OA.oa!, needs_reauthorization: { since: "2026-10-15T00:00:00Z", reason: "refresh-token-het-han" } },
};

const PARAMS_OK = form.REQUIRED_PARAMS.map((name) => ({ name, required: true, type: "STRING", max_length: 100 }));
const T_ATTACHED: comms_zaloOATemplateOut = {
  id: "01TPLA",
  template_id: "300001",
  template_name: "Tiếp nhận phản ánh",
  zalo_status: "ENABLE",
  zalo_quality: "HIGH",
  zalo_tag: "TRANSACTION",
  params: PARAMS_OK,
  params_match: true,
  missing_params: [],
  milestone: "da-tiep-nhan",
  milestone_set_at: "2026-10-16T02:30:00Z",
  milestone_set_by: "CB-00123",
  milestone_set_by_name: "Nguyễn Văn A",
};
const T_FREE: comms_zaloOATemplateOut = {
  ...T_ATTACHED,
  id: "01TPLB",
  template_id: "300002",
  template_name: "Đóng phản ánh",
  milestone: null,
  milestone_set_at: null,
  milestone_set_by: null,
  milestone_set_by_name: null,
};
const T_PENDING: comms_zaloOATemplateOut = { ...T_FREE, id: "01TPLC", template_id: "300003", template_name: "Chờ duyệt", zalo_status: "PENDING_REVIEW" };
const T_MISSING: comms_zaloOATemplateOut = {
  ...T_FREE,
  id: "01TPLD",
  template_id: "300004",
  template_name: "Thiếu tham số",
  params: PARAMS_OK.slice(0, 1),
  params_match: false,
  missing_params: ["moc_nhan", "viec_tiep_theo"],
};
/** Attached earlier, then disabled on Zalo: stale. */
const T_STALE: comms_zaloOATemplateOut = { ...T_ATTACHED, id: "01TPLE", template_id: "300005", zalo_status: "DISABLE", milestone: "da-dong" };

const LINK_FAKE = "https://oauth.example.test/v4/oa/permission?app_id=1&state=fake-state-0000";

function session(permissions: string[]): PhienDaDoc {
  return { ok: true, duLieu: { staff: { full_name: "A", position: "B" }, role: null, permissions } } as unknown as PhienDaDoc;
}

let calls: { method: string; url: string; body?: string }[] = [];
let oaNow: { status: number; body: unknown } = { status: 200, body: NO_OA };
let templatesNow: { status: number; body: unknown } = { status: 200, body: { items: [], synced_at: null } };
let answers: Record<string, { status: number; body: unknown }> = {};
let openSpy: ReturnType<typeof vi.fn>;

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

beforeEach(() => {
  calls = [];
  oaNow = { status: 200, body: NO_OA };
  templatesNow = { status: 200, body: { items: [], synced_at: null } };
  answers = {};
  H.toast.success.mockReset();
  H.toast.error.mockReset();
  window.history.replaceState(null, "", "/cau-hinh");
  openSpy = vi.fn(() => null);
  vi.stubGlobal("open", openSpy);
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init: RequestInit) => {
      const method = String(init.method ?? "GET");
      const body = typeof init.body === "string" ? init.body : undefined;
      calls.push({ method, url, body });
      const json = (status: number, b: unknown) =>
        new Response(JSON.stringify(b), { status, headers: { "Content-Type": "application/json" } });
      if (url.startsWith("/api/v1/zalo-oas/")) {
        if (method === "GET" && url === "/api/v1/zalo-oas/current") return json(oaNow.status, oaNow.body);
        if (method === "GET" && url === "/api/v1/zalo-oas/current/templates") return json(templatesNow.status, templatesNow.body);
        const a = answers[`${method} ${url}`];
        return a ? json(a.status, a.body) : json(500, { code: "internal", message: "no stub" });
      }
      if (url === "/api/v1/zalo-links") return json(200, { items: [] });
      if (url === "/api/v1/staff-directory") return json(200, { items: [] });
      if (url === "/api/v1/org-units") return json(200, { items: [] });
      if (url === "/api/v1/commune-launch-link") return json(200, { url: "https://zalo.me/s/1000/?src=qr", source: "rieng" });
      if (url === "/api/v1/zalo-groups") return json(200, { items: [], supported_kinds: [] });
      if (url === "/api/v1/zalo-bots/current") return json(200, { has_own_bot: false, bot: null, live_link_count: 0, live_group_count: 0 });
      return json(200, SETTINGS);
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

function oaSection(el: HTMLElement): HTMLElement {
  return el.querySelector<HTMLElement>('section[aria-labelledby="zalo-oa-title"]')!;
}
function templatesSection(el: HTMLElement): HTMLElement | null {
  return el.querySelector<HTMLElement>('section[aria-labelledby="zalo-oa-templates-title"]');
}
function button(el: HTMLElement, text: string): HTMLButtonElement {
  const found = Array.from(el.querySelectorAll<HTMLButtonElement>("button")).filter((b) => b.textContent?.trim() === text);
  expect(found.length, `button "${text}"`).toBeGreaterThan(0);
  return found.at(-1)!;
}
function typeInto(el: HTMLInputElement, value: string) {
  const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!;
  act(() => {
    setter.call(el, value);
    el.dispatchEvent(new Event("input", { bubbles: true }));
  });
}
function pick(select: HTMLSelectElement, value: string) {
  act(() => {
    select.value = value;
    select.dispatchEvent(new Event("change", { bubbles: true }));
  });
}
function milestoneSelect(el: HTMLElement, id: string): HTMLSelectElement {
  return el.querySelector<HTMLSelectElement>(`#zalo-oa-template-${id}-milestone`)!;
}

describe("pure rules (comms re-checks every one)", () => {
  it("eight milestones, ADR 0041 order then merge/split, with the agreed labels", () => {
    expect(form.MILESTONES.map((m) => m.code)).toEqual([
      "da-tiep-nhan",
      "da-chuyen-xu-ly",
      "cho-dan-xac-nhan",
      "da-dong",
      "khong-tiep-nhan",
      "chuyen-cap-tren",
      "gop-phieu",
      "tach-phieu",
    ]);
    expect(form.milestoneLabel("gop-phieu")).toBe("Ghép với phản ánh cùng vụ việc");
    expect(form.milestoneLabel("tach-phieu")).toBe("Xử lý riêng");
  });

  it("canAttach: ENABLE AND parameters matching — nothing else", () => {
    expect(form.canAttach(T_FREE)).toBe(true);
    expect(form.canAttach(T_PENDING)).toBe(false);
    expect(form.canAttach(T_MISSING)).toBe(false);
    expect(form.missingText(T_MISSING)).toBe("Thiếu: moc_nhan, viec_tiep_theo");
    expect(form.missingText(T_FREE)).toBeNull();
  });

  it("unattached milestones and stale attachments", () => {
    const items = [T_ATTACHED, T_FREE, T_STALE];
    expect(form.unattachedMilestones(items)).toEqual([
      "da-chuyen-xu-ly",
      "cho-dan-xac-nhan",
      "khong-tiep-nhan",
      "chuyen-cap-tren",
      "gop-phieu",
      "tach-phieu",
    ]);
    expect(form.staleAttachments(items).map((t) => t.id)).toEqual(["01TPLE"]);
  });

  it("callback outcomes: da-noi is the only success; another commune is never named; unknown is a failure", () => {
    expect(form.callbackOutcomeText("da-noi").ok).toBe(true);
    expect(form.callbackOutcomeText("oa-dang-o-xa-khac")).toEqual({
      ok: false,
      text: "OA này đang được một đơn vị khác dùng. Hãy chọn OA của chính xã.",
    });
    for (const o of ["zalo-tu-choi", "khong-kha-dung", "phan-hoi-sai-dang", "het-han", "sai-khuon", "toString", "x"]) {
      expect(form.callbackOutcomeText(o).ok, o).toBe(false);
    }
    expect(form.searchWithoutOutcome("?tab=kenh-zalo&oa=da-noi")).toBe("?tab=kenh-zalo");
    expect(form.searchWithoutOutcome("?oa=het-han")).toBe("");
    expect(form.outcomeFromSearch("?tab=kenh-zalo")).toBeNull();
  });

  it("an unknown check class is never a success; toString is not a sentence", () => {
    expect(form.oaCheckResultText("toString")).toMatch(/^Không rõ kết quả/);
    expect(form.oaCheckResultText("thanh-cong", "UBND Xã Thử")).toBe("Kết nối tốt: Zalo nhận uỷ quyền OA của xã (UBND Xã Thử).");
  });
});

describe("gate — admin.zalo, denied case first", () => {
  it("DENIED: without admin.zalo nothing under zalo-oas is requested, and `?oa=` is not toasted", async () => {
    H.phien = session(["admin.lookup", "admin.sla"]);
    window.history.replaceState(null, "", "/cau-hinh?tab=kenh-zalo&oa=da-noi");
    const el = await mount();
    expect(el.textContent).toContain("không có quyền cấu hình kênh Zalo");
    expect(calls.some((c) => c.url.startsWith("/api/v1/zalo-oas"))).toBe(false);
    expect(H.toast.success).not.toHaveBeenCalled();
  });
});

describe("OA Zalo của xã", () => {
  it("sits right after 'Con bot của xã' and before 'Nhóm Zalo'", async () => {
    H.phien = session(["admin.zalo"]);
    oaNow = { status: 200, body: LIVE_OA };
    const el = await mount();
    const titles = Array.from(el.querySelectorAll("section > div h3, section > h3")).map((h) => h.textContent);
    const bot = titles.indexOf("Con bot của xã");
    expect(titles.slice(bot, bot + 4)).toEqual(["Con bot của xã", "OA Zalo của xã", "Mẫu tin ZBS", "Nhóm Zalo"]);
  });

  it("platform not configured: the warning, every button disabled with '?', nothing written", async () => {
    H.phien = session(["admin.zalo"]);
    oaNow = { status: 200, body: { ...LIVE_OA, configured: false } };
    const el = await mount();
    const sec = oaSection(el);
    expect(sec.querySelector("[data-oa-not-configured]")!.textContent).toBe(form.OA_NOT_CONFIGURED);
    for (const label of ["Kiểm tra kết nối", "Ngắt OA"]) expect(button(sec, label).disabled).toBe(true);
    expect(sec.querySelectorAll("[data-pending-marker]")).toHaveLength(2);
    expect(calls.some((c) => c.method !== "GET")).toBe(false);
  });

  it("no OA: 'Kết nối OA' opens Zalo's link in a new tab (noopener) and never renders it; no templates section", async () => {
    H.phien = session(["admin.zalo"]);
    answers["POST /api/v1/zalo-oas/current/authorization-links"] = {
      status: 201,
      body: { url: LINK_FAKE, expires_at: "2026-10-16T03:10:00Z" },
    };
    const el = await mount();
    const sec = oaSection(el);
    expect(sec.querySelector("[data-oa-pill]")!.textContent).toBe("Chưa nối");
    expect(sec.textContent).toContain(form.AUTHORIZE_INSTRUCTION);
    expect(templatesSection(el)).toBeNull();
    act(() => button(sec, "Kết nối OA").click());
    await settle();
    const post = calls.find((c) => c.method === "POST")!;
    expect(post.url).toBe("/api/v1/zalo-oas/current/authorization-links");
    expect(post.body).toBeUndefined();
    expect(openSpy).toHaveBeenCalledWith(LINK_FAKE, "_blank", "noopener");
    expect(H.toast.success).toHaveBeenCalledWith(form.LINK_OPENED_TOAST);
    // The link carries a one-time state: never in the page, an attribute or the URL bar.
    expect(document.body.innerHTML).not.toContain("fake-state-0000");
    expect(window.location.href).not.toContain("fake-state-0000");
  });

  it("503 zalo_oa_not_configured: the server's sentence under the button, which stays disabled", async () => {
    H.phien = session(["admin.zalo"]);
    answers["POST /api/v1/zalo-oas/current/authorization-links"] = {
      status: 503,
      body: { code: "zalo_oa_not_configured", message: "Nền tảng chưa khai ứng dụng Zalo dùng chung." },
    };
    const el = await mount();
    act(() => button(oaSection(el), "Kết nối OA").click());
    await settle();
    expect(button(oaSection(el), "Kết nối OA").disabled).toBe(true);
    expect(oaSection(el).querySelector("#zalo-oa-authorize-hint")!.textContent).toBe("Nền tảng chưa khai ứng dụng Zalo dùng chung.");
    expect(openSpy).not.toHaveBeenCalled();
  });

  it("503 encryption_not_configured: the server's sentence as a toast", async () => {
    H.phien = session(["admin.zalo"]);
    answers["POST /api/v1/zalo-oas/current/authorization-links"] = {
      status: 503,
      body: { code: "encryption_not_configured", message: "Máy chủ chưa sẵn sàng cất bí mật của xã." },
    };
    const el = await mount();
    act(() => button(oaSection(el), "Kết nối OA").click());
    await settle();
    expect(H.toast.error).toHaveBeenCalledWith("Máy chủ chưa sẵn sàng cất bí mật của xã.");
    expect(button(oaSection(el), "Kết nối OA").disabled).toBe(false);
  });

  it("live OA: name, id, 'nối lúc … bởi …', refresh time, last check; no authorise button; check toasts", async () => {
    H.phien = session(["admin.zalo"]);
    oaNow = { status: 200, body: LIVE_OA };
    answers["POST /api/v1/zalo-oas/current/check"] = {
      status: 200,
      body: { result: "token-bi-tu-choi", checked_at: "2026-10-16T03:00:00Z" },
    };
    const el = await mount();
    const sec = oaSection(el);
    expect(sec.querySelector("[data-oa-pill]")!.textContent).toBe("Đã nối · UBND Xã Thử");
    expect(sec.querySelector("[data-oa-id]")!.textContent).toBe("1234567890");
    expect(sec.querySelector("[data-oa-authorized]")!.textContent).toMatch(/^Nối lúc .+ · bởi Nguyễn Văn A \(CB-00123\)$/);
    expect(sec.querySelector("[data-oa-refreshed]")!.textContent).toMatch(/^Token làm mới lúc /);
    expect(sec.querySelector("[data-oa-last-check]")!.textContent).toMatch(/· kết nối tốt$/);
    expect(sec.textContent).not.toContain("Kết nối OA");
    act(() => button(sec, "Kiểm tra kết nối").click());
    await settle();
    expect(H.toast.error).toHaveBeenCalledWith(expect.stringContaining("Zalo không nhận uỷ quyền của xã"));
    expect(templatesSection(el)).not.toBeNull();
  });

  it("needs re-authorisation: warning badge, the reason, 'Uỷ quyền lại'", async () => {
    H.phien = session(["admin.zalo"]);
    oaNow = { status: 200, body: REAUTH_OA };
    const el = await mount();
    const sec = oaSection(el);
    expect(sec.querySelector("[data-oa-pill]")!.textContent).toBe("Cần uỷ quyền lại · UBND Xã Thử");
    expect(sec.querySelector("[data-oa-reauth]")!.textContent).toContain("quá 3 tháng");
    expect(button(sec, "Uỷ quyền lại").disabled).toBe(false);
    expect(sec.textContent).toContain(form.AUTHORIZE_INSTRUCTION);
  });

  it("Ngắt OA: consequence named, reason REQUIRED, DELETE {reason}, revoke notice, templates section gone", async () => {
    H.phien = session(["admin.zalo"]);
    oaNow = { status: 200, body: LIVE_OA };
    answers["DELETE /api/v1/zalo-oas/current"] = {
      status: 200,
      body: { disconnected: true, detached_template_count: 1, revoke_notice: "Hãy thu hồi quyền của ứng dụng trong OA Manager." },
    };
    const el = await mount();
    act(() => button(oaSection(el), "Ngắt OA").click());
    await settle();
    const dialog = el.querySelector<HTMLElement>("dialog")!;
    expect(dialog.querySelector("[data-oa-consequence]")!.textContent).toBe(form.DISCONNECT_CONSEQUENCE);
    act(() => button(dialog, "Ngắt OA").click());
    await settle();
    expect(calls.some((c) => c.method === "DELETE")).toBe(false);
    expect(dialog.textContent).toContain("Cần nhập lý do ngắt OA");

    typeInto(dialog.querySelector<HTMLInputElement>("#zalo-oa-disconnect-reason")!, "  Xã đổi OA  ");
    oaNow = { status: 200, body: NO_OA };
    act(() => button(dialog, "Ngắt OA").click());
    await settle();
    const del = calls.find((c) => c.method === "DELETE")!;
    expect(del.url).toBe("/api/v1/zalo-oas/current");
    expect(JSON.parse(del.body!)).toEqual({ reason: "Xã đổi OA" });
    expect(H.toast.success).toHaveBeenCalledWith("Đã ngắt OA. Đã gỡ 1 mốc đã gắn mẫu.");
    expect(el.querySelector("[data-oa-revoke-notice]")!.textContent).toBe("Hãy thu hồi quyền của ứng dụng trong OA Manager.");
    expect(templatesSection(el)).toBeNull();
  });
});

describe("the callback's ?oa= — read once, then removed from the URL", () => {
  it("da-noi: success toast, `oa` gone, `tab` kept, the OA read", async () => {
    H.phien = session(["admin.zalo"]);
    window.history.replaceState(null, "", "/cau-hinh?tab=kenh-zalo&oa=da-noi");
    oaNow = { status: 200, body: LIVE_OA };
    const el = await mount();
    expect(H.toast.success).toHaveBeenCalledWith(form.callbackOutcomeText("da-noi").text);
    expect(window.location.search).toBe("?tab=kenh-zalo");
    expect(oaSection(el).querySelector("[data-oa-pill]")!.textContent).toBe("Đã nối · UBND Xã Thử");
  });

  it("oa-dang-o-xa-khac: the error toast names no commune", async () => {
    H.phien = session(["admin.zalo"]);
    window.history.replaceState(null, "", "/cau-hinh?tab=kenh-zalo&oa=oa-dang-o-xa-khac");
    await mount();
    expect(H.toast.error).toHaveBeenCalledWith("OA này đang được một đơn vị khác dùng. Hãy chọn OA của chính xã.");
    expect(window.location.search).toBe("?tab=kenh-zalo");
  });
});

describe("Mẫu tin ZBS", () => {
  beforeEach(() => {
    oaNow = { status: 200, body: LIVE_OA };
    templatesNow = {
      status: 200,
      body: { items: [T_ATTACHED, T_FREE, T_PENDING, T_MISSING, T_STALE], synced_at: "2026-10-16T02:00:00Z" },
    };
  });

  it("rows: Vietnamese status, quality, params; only ENABLE + matching selects open; stale keeps a detach-only select", async () => {
    H.phien = session(["admin.zalo"]);
    const el = await mount();
    const sec = templatesSection(el)!;
    expect(sec.querySelector("[data-params-rule]")!.textContent).toBe(
      "Mẫu đăng ký với Zalo phải có đúng ba tham số: ma_tra_cuu, moc_nhan, viec_tiep_theo.",
    );
    const free = milestoneSelect(sec, "01TPLB");
    expect(free.disabled).toBe(false);
    expect(Array.from(free.options).map((o) => o.textContent)).toEqual([
      "— Không gắn —",
      ...form.MILESTONES.map((m) => m.label),
    ]);
    expect(milestoneSelect(sec, "01TPLA").value).toBe("da-tiep-nhan");
    expect(sec.querySelector('[data-template-id="01TPLA"] [data-attached-by]')!.textContent).toMatch(/^Gắn lúc .+ · bởi Nguyễn Văn A \(CB-00123\)$/);

    const pending = milestoneSelect(sec, "01TPLC");
    expect(pending.disabled).toBe(true);
    expect(sec.querySelector('[data-template-id="01TPLC"]')!.textContent).toContain("Chờ duyệt");
    expect(sec.querySelector('[data-template-id="01TPLC"] [data-attach-blocked]')!.textContent).toContain("Chờ duyệt");

    const missing = milestoneSelect(sec, "01TPLD");
    expect(missing.disabled).toBe(true);
    expect(sec.querySelector('[data-template-id="01TPLD"] [data-missing]')!.textContent).toBe("Thiếu: moc_nhan, viec_tiep_theo");

    const stale = milestoneSelect(sec, "01TPLE");
    expect(stale.disabled).toBe(false);
    expect(Array.from(stale.options).map((o) => o.value)).toEqual(["", "da-dong"]);

    expect(sec.querySelector("[data-unattached]")!.textContent).toBe(
      "Chưa có mẫu cho: Đã chuyển xử lý, Chờ dân xác nhận, Không tiếp nhận, Chuyển cấp trên, Ghép với phản ánh cùng vụ việc, Xử lý riêng.",
    );
    expect(sec.querySelector("[data-stale]")!.textContent).toBe(
      "Mẫu đã gắn cho mốc “Đã đóng” không còn dùng được — tin cho mốc này sẽ không gửi được.",
    );
  });

  it("changing a select PATCHes at once; the reply replaces the row; 'Đã lưu.'", async () => {
    H.phien = session(["admin.zalo"]);
    answers["PATCH /api/v1/zalo-oas/current/templates/01TPLB"] = {
      status: 200,
      body: { ...T_FREE, milestone: "da-chuyen-xu-ly", milestone_set_at: "2026-10-16T03:00:00Z", milestone_set_by: "CB-00123", milestone_set_by_name: "Nguyễn Văn A" },
    };
    const el = await mount();
    pick(milestoneSelect(templatesSection(el)!, "01TPLB"), "da-chuyen-xu-ly");
    await settle();
    const patch = calls.find((c) => c.method === "PATCH")!;
    expect(JSON.parse(patch.body!)).toEqual({ milestone: "da-chuyen-xu-ly" });
    expect(H.toast.success).toHaveBeenCalledWith("Đã lưu.");
    expect(milestoneSelect(templatesSection(el)!, "01TPLB").value).toBe("da-chuyen-xu-ly");
    expect(templatesSection(el)!.querySelector('[data-template-id="01TPLB"] [data-attached-by]')).not.toBeNull();
  });

  it("409 milestone_taken: the select goes back, the server's sentence is toasted", async () => {
    H.phien = session(["admin.zalo"]);
    answers["PATCH /api/v1/zalo-oas/current/templates/01TPLB"] = {
      status: 409,
      body: { code: "milestone_taken", message: "Mốc “Đã tiếp nhận” đã có mẫu khác gắn." },
    };
    const el = await mount();
    pick(milestoneSelect(templatesSection(el)!, "01TPLB"), "da-tiep-nhan");
    await settle();
    expect(H.toast.error).toHaveBeenCalledWith("Mốc “Đã tiếp nhận” đã có mẫu khác gắn.");
    expect(milestoneSelect(templatesSection(el)!, "01TPLB").value).toBe("");
  });

  it("detaching a stale template sends milestone: null", async () => {
    H.phien = session(["admin.zalo"]);
    answers["PATCH /api/v1/zalo-oas/current/templates/01TPLE"] = { status: 200, body: { ...T_STALE, milestone: null } };
    const el = await mount();
    pick(milestoneSelect(templatesSection(el)!, "01TPLE"), "");
    await settle();
    expect(JSON.parse(calls.find((c) => c.method === "PATCH")!.body!)).toEqual({ milestone: null });
    expect(templatesSection(el)!.querySelector("[data-stale]")).toBeNull();
  });

  it("'Nạp mẫu từ OA': POST sync, the server's three counts, list read again", async () => {
    H.phien = session(["admin.zalo"]);
    answers["POST /api/v1/zalo-oas/current/templates/sync"] = {
      status: 200,
      body: { loaded: 5, enabled: 3, matching: 2, changed: 1, synced_at: "2026-10-16T03:00:00Z" },
    };
    const el = await mount();
    const reads = () => calls.filter((c) => c.method === "GET" && c.url === "/api/v1/zalo-oas/current/templates").length;
    expect(reads()).toBe(1);
    act(() => button(templatesSection(el)!, "Nạp mẫu từ OA").click());
    await settle();
    expect(H.toast.success).toHaveBeenCalledWith("Đã nạp 5 mẫu, 3 đang bật, 2 khớp tham số.");
    expect(reads()).toBe(2);
  });

  it("sync 422 zalo_token_rejected: the server's sentence", async () => {
    H.phien = session(["admin.zalo"]);
    answers["POST /api/v1/zalo-oas/current/templates/sync"] = {
      status: 422,
      body: { code: "zalo_token_rejected", message: "Zalo không nhận uỷ quyền của xã. Hãy uỷ quyền lại." },
    };
    const el = await mount();
    act(() => button(templatesSection(el)!, "Nạp mẫu từ OA").click());
    await settle();
    expect(H.toast.error).toHaveBeenCalledWith("Zalo không nhận uỷ quyền của xã. Hãy uỷ quyền lại.");
  });

  it("empty list: the sentence, no table; every milestone listed as missing", async () => {
    H.phien = session(["admin.zalo"]);
    templatesNow = { status: 200, body: { items: [], synced_at: null } };
    const el = await mount();
    const sec = templatesSection(el)!;
    expect(sec.querySelector("[data-templates-empty]")!.textContent).toBe(form.NO_TEMPLATES);
    expect(sec.querySelector("table")).toBeNull();
    expect(sec.querySelector("[data-synced-at]")).toBeNull();
  });
});
