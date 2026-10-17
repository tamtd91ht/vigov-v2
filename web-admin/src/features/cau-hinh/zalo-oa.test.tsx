// @vitest-environment jsdom
//
// "OA Zalo của xã" and "Mẫu tin ZBS" (ADR 0098 khối A; several OAs per commune since ADR 0099 #1), mounted
// inside the real `ZaloChannelTab` so the admin.zalo gate, the slot order and the one-time `?oa=` handling
// are all exercised.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import type { PhienDaDoc } from "@/features/phien/phien-hien-tai";
import type { comms_communeZaloOAListOut, comms_communeZaloOAOut, comms_zaloOATemplateOut } from "@/lib/api/zalo-oa";

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

const OA_A: comms_communeZaloOAOut = {
  id: "01OAA",
  oa_id: "1234567890",
  oa_name: "UBND Xã Thử",
  authorized_at: "2026-10-16T01:00:00Z",
  authorized_by: "CB-00123",
  authorized_by_name: "Nguyễn Văn A",
  access_expires_at: "2026-10-17T02:00:00Z",
  token_refreshed_at: "2026-10-16T01:00:00Z",
  needs_reauthorization: null,
  last_check: { at: "2026-10-16T02:00:00Z", result: "thanh-cong" },
  template_count: 2,
  attached_count: 1,
};
const OA_B: comms_communeZaloOAOut = {
  ...OA_A,
  id: "01OAB",
  oa_id: "2222222222",
  oa_name: "Văn hoá Xã Thử",
  authorized_by_name: "",
  last_check: null,
  template_count: 1,
  attached_count: 0,
};
const NO_OA: comms_communeZaloOAListOut = { configured: true, items: [] };
const LIVE_OA: comms_communeZaloOAListOut = { configured: true, items: [OA_A] };
const TWO_OAS: comms_communeZaloOAListOut = { configured: true, items: [OA_A, OA_B] };
const REAUTH_OA: comms_communeZaloOAListOut = {
  configured: true,
  items: [{ ...OA_A, needs_reauthorization: { since: "2026-10-15T00:00:00Z", reason: "refresh-token-het-han" } }],
};

const PARAMS_OK = form.REQUIRED_PARAMS.map((name) => ({ name, required: true, type: "STRING", max_length: 100 }));
const T_ATTACHED: comms_zaloOATemplateOut = {
  id: "01TPLA",
  oa_row_id: "01OAA",
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
/** On the SECOND OA, attached to "Đã chuyển xử lý": that milestone is covered for the whole commune. */
const T_B_ATTACHED: comms_zaloOATemplateOut = {
  ...T_ATTACHED,
  id: "01TPLF",
  oa_row_id: "01OAB",
  template_id: "400001",
  template_name: "Chuyển xử lý",
  milestone: "da-chuyen-xu-ly",
};
/** On the SECOND OA, attached then rejected: stale, and named with its OA. */
const T_B_STALE: comms_zaloOATemplateOut = {
  ...T_B_ATTACHED,
  id: "01TPLG",
  template_id: "400002",
  template_name: "Chờ dân xác nhận",
  zalo_status: "REJECT",
  milestone: "cho-dan-xac-nhan",
};

const LINK_FAKE = "https://oauth.example.test/v4/oa/permission?app_id=1&state=fake-state-0000";

function session(permissions: string[]): PhienDaDoc {
  return { ok: true, duLieu: { staff: { full_name: "A", position: "B" }, role: null, permissions } } as unknown as PhienDaDoc;
}

let calls: { method: string; url: string; body?: string }[] = [];
let oaNow: { status: number; body: unknown } = { status: 200, body: NO_OA };
/** GET zalo-oas/{id}/templates, per OA row id; an id absent here answers an empty list. */
let templatesBy: Record<string, { status: number; body: unknown }> = {};
let answers: Record<string, { status: number; body: unknown }> = {};
let openSpy: ReturnType<typeof vi.fn>;

const OAS_URL = "/api/v1/zalo-oas";
const templatesUrl = (id: string) => `/api/v1/zalo-oas/${id}/templates`;

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

beforeEach(() => {
  calls = [];
  oaNow = { status: 200, body: NO_OA };
  templatesBy = {};
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
      if (url.startsWith(OAS_URL) || url.startsWith("/api/v1/zalo-oa-templates/")) {
        if (method === "GET" && url === OAS_URL) return json(oaNow.status, oaNow.body);
        const read = /^\/api\/v1\/zalo-oas\/([^/]+)\/templates$/.exec(url);
        if (method === "GET" && read) {
          const t = templatesBy[decodeURIComponent(read[1]!)] ?? { status: 200, body: { items: [], synced_at: null } };
          return json(t.status, t.body);
        }
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
function oaCard(el: HTMLElement, id: string): HTMLElement {
  return oaSection(el).querySelector<HTMLElement>(`[data-oa-row-id="${id}"]`)!;
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

  it("unattached milestones and stale attachments — across the templates of every OA", () => {
    const items = [T_ATTACHED, T_FREE, T_STALE, T_B_ATTACHED];
    expect(form.unattachedMilestones(items)).toEqual([
      "cho-dan-xac-nhan",
      "khong-tiep-nhan",
      "chuyen-cap-tren",
      "gop-phieu",
      "tach-phieu",
    ]);
    expect(form.staleAttachments(items).map((t) => t.id)).toEqual(["01TPLE"]);
    expect(form.staleText("da-dong", "UBND Xã Thử")).toBe(
      "Mẫu đã gắn cho mốc “Đã đóng” (OA “UBND Xã Thử”) không còn dùng được — tin cho mốc này sẽ không gửi được.",
    );
  });

  it("pills: none / how many; one OA's pill says whether it must be authorised again", () => {
    expect(form.oaListPill([])).toBe("Chưa nối");
    expect(form.oaListPill([OA_A, OA_B])).toBe("Đã nối 2 OA");
    expect(form.oaPill(OA_A)).toBe("Đã nối");
    expect(form.oaPill(REAUTH_OA.items[0]!)).toBe("Cần uỷ quyền lại");
    expect(form.templateCountText(OA_A)).toBe("2 mẫu đã nạp · 1 mốc đã gắn");
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
    expect(calls.some((c) => c.url.startsWith("/api/v1/zalo-oa"))).toBe(false);
    expect(H.toast.success).not.toHaveBeenCalled();
  });
});

describe("OA Zalo của xã — a list, one card per OA", () => {
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
    for (const label of ["Kết nối thêm OA", "Kiểm tra kết nối", "Ngắt OA"]) expect(button(sec, label).disabled).toBe(true);
    expect(sec.querySelectorAll("[data-pending-marker]")).toHaveLength(3);
    expect(calls.some((c) => c.method !== "GET")).toBe(false);
  });

  it("no OA: 'Kết nối OA' opens Zalo's link in a new tab (noopener) and never renders it; no templates section", async () => {
    H.phien = session(["admin.zalo"]);
    answers["POST /api/v1/zalo-oas/authorization-links"] = {
      status: 201,
      body: { url: LINK_FAKE, expires_at: "2026-10-16T03:10:00Z" },
    };
    const el = await mount();
    const sec = oaSection(el);
    expect(sec.querySelector("[data-oa-pill]")!.textContent).toBe("Chưa nối");
    expect(sec.querySelector("[data-oa-row-id]")).toBeNull();
    expect(sec.textContent).toContain(form.AUTHORIZE_INSTRUCTION);
    expect(sec.textContent).not.toContain("Kết nối thêm OA");
    expect(templatesSection(el)).toBeNull();
    act(() => button(sec, "Kết nối OA").click());
    await settle();
    const post = calls.find((c) => c.method === "POST")!;
    expect(post.url).toBe("/api/v1/zalo-oas/authorization-links");
    expect(post.body).toBeUndefined();
    expect(openSpy).toHaveBeenCalledWith(LINK_FAKE, "_blank", "noopener");
    expect(H.toast.success).toHaveBeenCalledWith(form.LINK_OPENED_TOAST);
    // The link carries a one-time state: never in the page, an attribute or the URL bar.
    expect(document.body.innerHTML).not.toContain("fake-state-0000");
    expect(window.location.href).not.toContain("fake-state-0000");
  });

  it("503 zalo_oa_not_configured: the server's sentence under the button, which stays disabled", async () => {
    H.phien = session(["admin.zalo"]);
    answers["POST /api/v1/zalo-oas/authorization-links"] = {
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
    answers["POST /api/v1/zalo-oas/authorization-links"] = {
      status: 503,
      body: { code: "encryption_not_configured", message: "Máy chủ chưa sẵn sàng cất bí mật của xã." },
    };
    const el = await mount();
    act(() => button(oaSection(el), "Kết nối OA").click());
    await settle();
    expect(H.toast.error).toHaveBeenCalledWith("Máy chủ chưa sẵn sàng cất bí mật của xã.");
    expect(button(oaSection(el), "Kết nối OA").disabled).toBe(false);
  });

  it("one OA: name, id, counts, 'nối lúc … bởi …', refresh time, last check; 'Kết nối thêm OA' at the head; check toasts", async () => {
    H.phien = session(["admin.zalo"]);
    oaNow = { status: 200, body: LIVE_OA };
    answers["POST /api/v1/zalo-oas/01OAA/check"] = {
      status: 200,
      body: { result: "token-bi-tu-choi", checked_at: "2026-10-16T03:00:00Z" },
    };
    const el = await mount();
    const sec = oaSection(el);
    expect(sec.querySelector("[data-oa-pill]")!.textContent).toBe("Đã nối 1 OA");
    const card = oaCard(el, "01OAA");
    expect(card.querySelector("[data-oa-name]")!.textContent).toBe("UBND Xã Thử");
    expect(card.querySelector("[data-oa-card-pill]")!.textContent).toBe("Đã nối");
    expect(card.querySelector("[data-oa-id]")!.textContent).toBe("1234567890");
    expect(card.querySelector("[data-oa-template-count]")!.textContent).toBe("2 mẫu đã nạp · 1 mốc đã gắn");
    expect(card.querySelector("[data-oa-authorized]")!.textContent).toMatch(/^Nối lúc .+ · bởi Nguyễn Văn A \(CB-00123\)$/);
    expect(card.querySelector("[data-oa-refreshed]")!.textContent).toMatch(/^Token làm mới lúc /);
    expect(card.querySelector("[data-oa-last-check]")!.textContent).toMatch(/· kết nối tốt$/);
    expect(button(sec, "Kết nối thêm OA").disabled).toBe(false);
    expect(card.textContent).not.toContain("Uỷ quyền lại");
    act(() => button(card, "Kiểm tra kết nối").click());
    await settle();
    expect(calls.find((c) => c.method === "POST")!.url).toBe("/api/v1/zalo-oas/01OAA/check");
    expect(H.toast.error).toHaveBeenCalledWith(expect.stringContaining("Zalo không nhận uỷ quyền của xã"));
    expect(templatesSection(el)).not.toBeNull();
  });

  it("two OAs: two cards in the server's order; each act names ITS OA; a name missing shows the code", async () => {
    H.phien = session(["admin.zalo"]);
    oaNow = { status: 200, body: TWO_OAS };
    answers["POST /api/v1/zalo-oas/01OAB/check"] = {
      status: 200,
      body: { result: "thanh-cong", checked_at: "2026-10-16T03:00:00Z", oa_name: "Văn hoá Xã Thử" },
    };
    const el = await mount();
    const sec = oaSection(el);
    expect(sec.querySelector("[data-oa-pill]")!.textContent).toBe("Đã nối 2 OA");
    expect(Array.from(sec.querySelectorAll("[data-oa-row-id]")).map((c) => c.getAttribute("data-oa-row-id"))).toEqual(["01OAA", "01OAB"]);
    const b = oaCard(el, "01OAB");
    expect(b.querySelector("[data-oa-authorized]")!.textContent).toMatch(/· bởi CB-00123$/);
    expect(b.querySelector("[data-oa-last-check]")).toBeNull();
    act(() => button(b, "Kiểm tra kết nối").click());
    await settle();
    expect(calls.filter((c) => c.method === "POST").map((c) => c.url)).toEqual(["/api/v1/zalo-oas/01OAB/check"]);
    expect(H.toast.success).toHaveBeenCalledWith("Kết nối tốt: Zalo nhận uỷ quyền OA của xã (Văn hoá Xã Thử).");
  });

  it("needs re-authorisation: warning badge on the card, the reason, 'Uỷ quyền lại' asks for a link", async () => {
    H.phien = session(["admin.zalo"]);
    oaNow = { status: 200, body: REAUTH_OA };
    answers["POST /api/v1/zalo-oas/authorization-links"] = {
      status: 201,
      body: { url: LINK_FAKE, expires_at: "2026-10-16T03:10:00Z" },
    };
    const el = await mount();
    const card = oaCard(el, "01OAA");
    expect(card.querySelector("[data-oa-card-pill]")!.textContent).toBe("Cần uỷ quyền lại");
    expect(card.querySelector("[data-oa-reauth]")!.textContent).toContain("quá 3 tháng");
    expect(oaSection(el).textContent).toContain(form.AUTHORIZE_INSTRUCTION);
    act(() => button(card, "Uỷ quyền lại").click());
    await settle();
    expect(calls.find((c) => c.method === "POST")!.url).toBe("/api/v1/zalo-oas/authorization-links");
    expect(openSpy).toHaveBeenCalledWith(LINK_FAKE, "_blank", "noopener");
  });

  it("Ngắt OA: consequence named, reason REQUIRED, DELETE that OA's id with {reason}, revoke notice, the other OA stays", async () => {
    H.phien = session(["admin.zalo"]);
    oaNow = { status: 200, body: TWO_OAS };
    answers["DELETE /api/v1/zalo-oas/01OAB"] = {
      status: 200,
      body: { disconnected: true, detached_template_count: 1, revoke_notice: "Hãy thu hồi quyền của ứng dụng trong OA Manager." },
    };
    const el = await mount();
    act(() => button(oaCard(el, "01OAB"), "Ngắt OA").click());
    await settle();
    const dialog = el.querySelector<HTMLElement>("dialog")!;
    expect(dialog.textContent).toContain("Ngắt OA “Văn hoá Xã Thử” khỏi xã?");
    expect(dialog.querySelector("[data-oa-consequence]")!.textContent).toBe(form.DISCONNECT_CONSEQUENCE);
    act(() => button(dialog, "Ngắt OA").click());
    await settle();
    expect(calls.some((c) => c.method === "DELETE")).toBe(false);
    expect(dialog.textContent).toContain("Cần nhập lý do ngắt OA");

    typeInto(dialog.querySelector<HTMLInputElement>("#zalo-oa-disconnect-reason")!, "  Xã đổi OA  ");
    oaNow = { status: 200, body: LIVE_OA };
    act(() => button(dialog, "Ngắt OA").click());
    await settle();
    const del = calls.find((c) => c.method === "DELETE")!;
    expect(del.url).toBe("/api/v1/zalo-oas/01OAB");
    expect(JSON.parse(del.body!)).toEqual({ reason: "Xã đổi OA" });
    expect(H.toast.success).toHaveBeenCalledWith("Đã ngắt OA. Đã gỡ 1 mốc đã gắn mẫu.");
    expect(el.querySelector("[data-oa-revoke-notice]")!.textContent).toBe("Hãy thu hồi quyền của ứng dụng trong OA Manager.");
    expect(oaSection(el).querySelector('[data-oa-row-id="01OAB"]')).toBeNull();
    expect(oaCard(el, "01OAA")).not.toBeNull();
    expect(templatesSection(el)).not.toBeNull();
  });

  it("Ngắt OA on the last OA: the templates section goes", async () => {
    H.phien = session(["admin.zalo"]);
    oaNow = { status: 200, body: LIVE_OA };
    answers["DELETE /api/v1/zalo-oas/01OAA"] = { status: 200, body: { disconnected: true, detached_template_count: 0, revoke_notice: "" } };
    const el = await mount();
    act(() => button(oaCard(el, "01OAA"), "Ngắt OA").click());
    await settle();
    const dialog = el.querySelector<HTMLElement>("dialog")!;
    typeInto(dialog.querySelector<HTMLInputElement>("#zalo-oa-disconnect-reason")!, "Thôi dùng");
    oaNow = { status: 200, body: NO_OA };
    act(() => button(dialog, "Ngắt OA").click());
    await settle();
    expect(H.toast.success).toHaveBeenCalledWith("Đã ngắt OA.");
    expect(el.querySelector("[data-oa-revoke-notice]")).toBeNull();
    expect(templatesSection(el)).toBeNull();
  });
});

describe("the callback's ?oa= — read once, then removed from the URL", () => {
  it("da-noi: success toast, `oa` gone, `tab` kept, the list read", async () => {
    H.phien = session(["admin.zalo"]);
    window.history.replaceState(null, "", "/cau-hinh?tab=kenh-zalo&oa=da-noi");
    oaNow = { status: 200, body: LIVE_OA };
    const el = await mount();
    expect(H.toast.success).toHaveBeenCalledWith(form.callbackOutcomeText("da-noi").text);
    expect(window.location.search).toBe("?tab=kenh-zalo");
    expect(oaSection(el).querySelector("[data-oa-pill]")!.textContent).toBe("Đã nối 1 OA");
    expect(H.toast.success).toHaveBeenCalledTimes(1);
  });

  it("oa-dang-o-xa-khac: the error toast names no commune", async () => {
    H.phien = session(["admin.zalo"]);
    window.history.replaceState(null, "", "/cau-hinh?tab=kenh-zalo&oa=oa-dang-o-xa-khac");
    await mount();
    expect(H.toast.error).toHaveBeenCalledWith("OA này đang được một đơn vị khác dùng. Hãy chọn OA của chính xã.");
    expect(window.location.search).toBe("?tab=kenh-zalo");
  });
});

describe("Mẫu tin ZBS — one OA", () => {
  beforeEach(() => {
    oaNow = { status: 200, body: LIVE_OA };
    templatesBy["01OAA"] = {
      status: 200,
      body: { items: [T_ATTACHED, T_FREE, T_PENDING, T_MISSING, T_STALE], synced_at: "2026-10-16T02:00:00Z" },
    };
  });

  it("rows: Vietnamese status, quality, params; only ENABLE + matching selects open; stale keeps a detach-only select; no OA picker", async () => {
    H.phien = session(["admin.zalo"]);
    const el = await mount();
    const sec = templatesSection(el)!;
    expect(sec.querySelector("#zalo-oa-templates-oa")).toBeNull();
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
      "Mẫu đã gắn cho mốc “Đã đóng” (OA “UBND Xã Thử”) không còn dùng được — tin cho mốc này sẽ không gửi được.",
    );
  });

  it("changing a select PATCHes zalo-oa-templates/{id} at once; the reply replaces the row; 'Đã lưu.'; the OA counts re-read", async () => {
    H.phien = session(["admin.zalo"]);
    answers["PATCH /api/v1/zalo-oa-templates/01TPLB"] = {
      status: 200,
      body: { ...T_FREE, milestone: "da-chuyen-xu-ly", milestone_set_at: "2026-10-16T03:00:00Z", milestone_set_by: "CB-00123", milestone_set_by_name: "Nguyễn Văn A" },
    };
    const el = await mount();
    const listReads = () => calls.filter((c) => c.method === "GET" && c.url === OAS_URL).length;
    expect(listReads()).toBe(1);
    pick(milestoneSelect(templatesSection(el)!, "01TPLB"), "da-chuyen-xu-ly");
    await settle();
    const patch = calls.find((c) => c.method === "PATCH")!;
    expect(patch.url).toBe("/api/v1/zalo-oa-templates/01TPLB");
    expect(JSON.parse(patch.body!)).toEqual({ milestone: "da-chuyen-xu-ly" });
    expect(H.toast.success).toHaveBeenCalledWith("Đã lưu.");
    expect(milestoneSelect(templatesSection(el)!, "01TPLB").value).toBe("da-chuyen-xu-ly");
    expect(templatesSection(el)!.querySelector('[data-template-id="01TPLB"] [data-attached-by]')).not.toBeNull();
    expect(listReads()).toBe(2);
  });

  it("409 milestone_taken: the select goes back, the server's sentence is toasted", async () => {
    H.phien = session(["admin.zalo"]);
    answers["PATCH /api/v1/zalo-oa-templates/01TPLB"] = {
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
    answers["PATCH /api/v1/zalo-oa-templates/01TPLE"] = { status: 200, body: { ...T_STALE, milestone: null } };
    const el = await mount();
    pick(milestoneSelect(templatesSection(el)!, "01TPLE"), "");
    await settle();
    expect(JSON.parse(calls.find((c) => c.method === "PATCH")!.body!)).toEqual({ milestone: null });
    expect(templatesSection(el)!.querySelector("[data-stale]")).toBeNull();
  });

  it("'Nạp mẫu từ OA': POST that OA's sync, the server's three counts, its list read again", async () => {
    H.phien = session(["admin.zalo"]);
    answers["POST /api/v1/zalo-oas/01OAA/templates/sync"] = {
      status: 200,
      body: { loaded: 5, enabled: 3, matching: 2, changed: 1, synced_at: "2026-10-16T03:00:00Z" },
    };
    const el = await mount();
    const reads = () => calls.filter((c) => c.method === "GET" && c.url === templatesUrl("01OAA")).length;
    expect(reads()).toBe(1);
    act(() => button(templatesSection(el)!, "Nạp mẫu từ OA").click());
    await settle();
    expect(H.toast.success).toHaveBeenCalledWith("Đã nạp 5 mẫu, 3 đang bật, 2 khớp tham số.");
    expect(reads()).toBe(2);
  });

  it("sync 422 zalo_token_rejected: the server's sentence", async () => {
    H.phien = session(["admin.zalo"]);
    answers["POST /api/v1/zalo-oas/01OAA/templates/sync"] = {
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
    templatesBy["01OAA"] = { status: 200, body: { items: [], synced_at: null } };
    const el = await mount();
    const sec = templatesSection(el)!;
    expect(sec.querySelector("[data-templates-empty]")!.textContent).toBe(form.NO_TEMPLATES);
    expect(sec.querySelector("table")).toBeNull();
    expect(sec.querySelector("[data-synced-at]")).toBeNull();
    expect(sec.querySelector("[data-unattached]")!.textContent).toBe(form.unattachedText(form.MILESTONES.map((m) => m.code)));
  });
});

describe("Mẫu tin ZBS — several OAs", () => {
  beforeEach(() => {
    oaNow = { status: 200, body: TWO_OAS };
    templatesBy["01OAA"] = { status: 200, body: { items: [T_ATTACHED, T_FREE], synced_at: "2026-10-16T02:00:00Z" } };
    templatesBy["01OAB"] = { status: 200, body: { items: [T_B_ATTACHED, T_B_STALE], synced_at: null } };
  });

  it("every OA's templates are read; the picker lists the OAs; the first OA's table is shown", async () => {
    H.phien = session(["admin.zalo"]);
    const el = await mount();
    expect(calls.filter((c) => c.method === "GET" && c.url.endsWith("/templates")).map((c) => c.url).sort()).toEqual([
      templatesUrl("01OAA"),
      templatesUrl("01OAB"),
    ]);
    const sec = templatesSection(el)!;
    const picker = sec.querySelector<HTMLSelectElement>("#zalo-oa-templates-oa")!;
    expect(Array.from(picker.options).map((o) => o.textContent)).toEqual(["UBND Xã Thử", "Văn hoá Xã Thử"]);
    expect(picker.value).toBe("01OAA");
    expect(sec.querySelector('[data-template-id="01TPLA"]')).not.toBeNull();
    expect(sec.querySelector('[data-template-id="01TPLF"]')).toBeNull();
    expect(sec.querySelector("[data-synced-at]")).not.toBeNull();
  });

  it("'Chưa có mẫu cho' counts EVERY OA; a broken attached template is named with ITS OA even when another OA is shown", async () => {
    H.phien = session(["admin.zalo"]);
    const el = await mount();
    const sec = templatesSection(el)!;
    // da-tiep-nhan on OA A, da-chuyen-xu-ly on OA B; cho-dan-xac-nhan is attached on B but its template is rejected.
    expect(sec.querySelector("[data-unattached]")!.textContent).toBe(
      "Chưa có mẫu cho: Đã đóng, Không tiếp nhận, Chuyển cấp trên, Ghép với phản ánh cùng vụ việc, Xử lý riêng.",
    );
    expect(Array.from(sec.querySelectorAll("[data-stale]")).map((p) => p.textContent)).toEqual([
      "Mẫu đã gắn cho mốc “Chờ dân xác nhận” (OA “Văn hoá Xã Thử”) không còn dùng được — tin cho mốc này sẽ không gửi được.",
    ]);
  });

  it("picking the second OA shows its table; 'Nạp mẫu từ OA' then syncs THAT OA", async () => {
    H.phien = session(["admin.zalo"]);
    answers["POST /api/v1/zalo-oas/01OAB/templates/sync"] = {
      status: 200,
      body: { loaded: 2, enabled: 1, matching: 1, changed: 0, synced_at: "2026-10-16T03:00:00Z" },
    };
    const el = await mount();
    pick(templatesSection(el)!.querySelector<HTMLSelectElement>("#zalo-oa-templates-oa")!, "01OAB");
    await settle();
    const sec = templatesSection(el)!;
    expect(sec.querySelector('[data-template-id="01TPLF"]')).not.toBeNull();
    expect(sec.querySelector('[data-template-id="01TPLA"]')).toBeNull();
    expect(sec.querySelector("[data-synced-at]")).toBeNull();
    act(() => button(sec, "Nạp mẫu từ OA").click());
    await settle();
    expect(calls.filter((c) => c.method === "POST").map((c) => c.url)).toEqual(["/api/v1/zalo-oas/01OAB/templates/sync"]);
  });

  it("one OA's templates refused: its sentence when shown, and NO 'Chưa có mẫu cho' drawn on half the facts", async () => {
    H.phien = session(["admin.zalo"]);
    templatesBy["01OAB"] = { status: 404, body: { code: "not_found", message: "Không tìm thấy OA này trong các OA đang nối của xã." } };
    const el = await mount();
    expect(templatesSection(el)!.querySelector("[data-unattached]")).toBeNull();
    pick(templatesSection(el)!.querySelector<HTMLSelectElement>("#zalo-oa-templates-oa")!, "01OAB");
    await settle();
    expect(templatesSection(el)!.querySelector('[role="alert"]')!.textContent).toBe("Không tìm thấy OA này trong các OA đang nối của xã.");
  });
});
