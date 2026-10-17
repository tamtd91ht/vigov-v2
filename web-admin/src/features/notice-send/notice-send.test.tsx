// @vitest-environment jsdom
//
// jsdom: the screen is driven for real — the session is faked, the network is a router over `fetch`,
// and what is checked is what the screen CALLS and what it SHOWS, state by state.

import type { AnchorHTMLAttributes } from "react";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { toast } from "sonner";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import type { PhienDaDoc } from "@/features/phien/phien-hien-tai";

import {
  allowedAttributeChoice,
  attributeMappingBody,
  countStaff,
  COUNT_OVERLAP,
  formatMoney,
  HISTORY_EMPTY,
  NO_CITIZEN_READ,
  NO_RECIPIENTS,
  NO_SEND_PERMISSION,
  NO_TEMPLATES,
  NOTICE_SEND_TITLE,
  OA_NOT_LINKED,
  OA_REAUTH_REASON,
  OA_REAUTH_TAG,
  PICK_TEMPLATE_FIRST,
  PREVIEW_EMPTY,
  PREVIEW_STALE,
  recipientSource,
  SEND_NOTE,
  SYNC_FORBIDDEN,
  ZNS_CONTENT,
} from "./notice-send-form";
import { NoticeSendScreen } from "./notice-send-screen";

const H = vi.hoisted(() => ({ session: null as PhienDaDoc }));

vi.mock("@/features/phien/phien-hien-tai", () => ({ usePhien: () => H.session }));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
// A plain anchor: the App Router's Link needs a router this test does not mount.
vi.mock("next/link", () => ({
  default: ({ href, ...rest }: AnchorHTMLAttributes<HTMLAnchorElement> & { href: string }) => <a href={href} {...rest} />,
}));

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  if (!("ResizeObserver" in globalThis)) {
    (globalThis as unknown as { ResizeObserver: unknown }).ResizeObserver = class {
      observe() {}
      unobserve() {}
      disconnect() {}
    };
  }
});

function session(permissions: string[]): PhienDaDoc {
  return { ok: true, duLieu: { staff: { full_name: "Nguyễn Văn Hùng", position: "Chuyên viên" }, role: null, permissions } } as unknown as PhienDaDoc;
}

const ALL = ["notice.send", "citizen.read", "admin.zalo"];

// ---- fixtures --------------------------------------------------------------------------------------

const OA_ROW = {
  id: "01JOA1",
  oa_id: "1234567890",
  oa_name: "UBND xã Tân Phú",
  authorized_at: "2026-10-16T01:00:00Z",
  authorized_by: "CB-00001",
  authorized_by_name: "Trần Văn Quản",
  access_expires_at: "2026-10-17T01:00:00Z",
  token_refreshed_at: "2026-10-16T01:00:00Z",
  needs_reauthorization: null as null | { since: string; reason: string },
  last_check: null,
  template_count: 2,
  attached_count: 0,
};
const OA_ROW_2 = { ...OA_ROW, id: "01JOA2", oa_id: "2222222222", oa_name: "Đoàn thanh niên xã Tân Phú" };
const REAUTH = { since: "2026-10-16T01:00:00Z", reason: "refresh-token-het-han" };

type OAAnswer = { configured: boolean; items: (typeof OA_ROW)[] };

const zns = (oaId: string, oaName: string, ref: string, name: string, params: string[], reauth = false) => ({
  channel: "zalo-zns",
  oa_id: oaId,
  oa_name: oaName,
  oa_needs_reauthorization: reauth,
  template_ref: ref,
  template_name: name,
  tag: "TRANSACTION",
  params: params.map((n) => ({ field: n, name: n, max_length: 30 })),
  unit_price: "200.00",
});
const sms = (accountId: string, label: string, ref: string, content: string, slots: number[]) => ({
  channel: "sms",
  sms_account_id: accountId,
  account_label: label,
  brandname: "UBND XA",
  template_ref: ref,
  content,
  params: slots.map((max, i) => ({ field: String(i + 1), name: "P1", max_length: max })),
  unit_price: null,
});

const ZNS_ONE = zns("01JOA1", "UBND xã Tân Phú", "zt-1", "Thông báo lịch tiêm", ["ngay_tiem", "dia_diem"]);
const ZNS_REAUTH = zns("01JOA2", "Đoàn thanh niên xã Tân Phú", "zt-3", "Mời họp đoàn", ["ngay"], true);
const SMS_A = sms("01JSMS1", "Tài khoản chính", "st-1", "UBND xa thong bao lich tiem ngay {P1,20}", [20]);
const SMS_B = sms("01JSMS2", "Tài khoản dự phòng", "st-2", "Hop dan {P1,10} luc {P1,5}", [10, 5]);

type Templates = { zns: unknown[]; sms: unknown[] };

const campaign = (id: string, status: string, extra: Record<string, unknown> = {}) => ({
  id,
  title: `Đợt ${id}`,
  channel: "zalo-zns",
  oa_id: "01JOA1",
  sms_account_id: null,
  source_name: "UBND xã Tân Phú",
  template_ref: "zt-1",
  template_name: "Thông báo lịch tiêm",
  audience_kind: "nguoi-dan-theo-nhom",
  audience_ids: ["01JG1"],
  recipient_count: 10,
  skipped_count: 0,
  sent_count: 0,
  failed_count: 0,
  unit_price: "200.00",
  estimated_cost: "2000.00",
  status,
  created_at: "2026-10-17T01:00:00Z",
  created_by: "CB-00123",
  created_by_name: "Lê Thị Gửi",
  finished_at: null,
  ...extra,
});

type Call = { url: string; init: RequestInit | undefined };
const calls: Call[] = [];
const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });

let oaAnswer: OAAnswer = { configured: true, items: [OA_ROW] };
let templatesAnswer: Templates = { zns: [ZNS_ONE], sms: [SMS_A] };
type PreviewBody = { channel: string; params: Record<string, string> };
const defaultPreview = (b: PreviewBody): Response => {
  const missing = Object.entries(b.params)
    .filter(([, v]) => v.trim() === "")
    .map(([k]) => k);
  return json({
    content: b.channel === "sms" ? "UBND xa thong bao lich tiem ngay 20/10" : null,
    recipient_count: 11,
    skipped_count: 0,
    unit_price: b.channel === "sms" ? null : "200.00",
    estimated_cost: b.channel === "sms" ? null : "2200.00",
    missing_params: missing,
  });
};
let previewReply: (b: PreviewBody) => Response = defaultPreview;
let sendReplies: Response[] = [];
let historyPages: Record<string, { items: unknown[]; next_cursor: string }> = { "": { items: [], next_cursor: "" } };
let campaignById: Record<string, unknown> = {};
let recipientPages: Record<string, { items: unknown[]; next_cursor: string }> = {};
const CITIZEN_ATTRS = [
  { code: "full_name", label: "Họ và tên" },
  { code: "address", label: "Địa chỉ" },
  { code: "hongheo", label: "Phân loại hộ" },
];
const defaultAttributes = (audience: string): Response =>
  json({ items: audience === "can-bo" ? [{ code: "full_name", label: "Họ và tên" }] : CITIZEN_ATTRS });
let attributesReply: (audience: string) => Response = defaultAttributes;
const SYNC_RESULT = { loaded: 3, enabled: 2, matching: 1 };
let syncReply: (oaId: string) => Response | Promise<Response> = () => json(SYNC_RESULT);

function serve() {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      calls.push({ url, init });
      if (url === "/api/v1/citizen-groups") {
        return json({
          items: [
            { id: "01JG1", name: "Tổ trưởng", description: "", sort_order: 0, member_count: 12, created_at: "", updated_at: "" },
            { id: "01JG2", name: "Hộ kinh doanh", description: "", sort_order: 1, member_count: 30, created_at: "", updated_at: "" },
          ],
        });
      }
      if (url === "/api/v1/citizen-contact-tags") return json({ items: [{ tag: "cựu chiến binh", count: 7 }, { tag: "hội phụ nữ", count: 9 }] });
      if (url === "/api/v1/citizen-contact-count-queries") {
        const body = JSON.parse(String(init?.body)) as { group_id: string };
        return json({ count: body.group_id === "01JG1" ? 11 : 250 });
      }
      if (url === "/api/v1/org-units") return json({ items: [{ id: "01JU1", code: "vp", name: "Văn phòng", parent_id: "", order: 0, staff_count: 2 }] });
      if (url === "/api/v1/staff-directory") {
        return json({
          items: [
            { code: "CB-1", full_name: "A", position: "", department_id: "01JU1", email_masked: null },
            { code: "CB-2", full_name: "B", position: "", department_id: "01JU1", email_masked: null },
            { code: "CB-3", full_name: "C", position: "", department_id: "", email_masked: null },
          ],
        });
      }
      if (url === "/api/v1/zalo-oas") return json(oaAnswer);
      if (url === "/api/v1/zalo-oas/authorization-links") {
        return json({ url: "https://oauth.zaloapp.com/v4/oa/permission?state=SECRET-STATE", expires_at: "2026-10-17T01:10:00Z" }, 201);
      }
      if (url === "/api/v1/notice-templates") return json(templatesAnswer);
      const sync = /^\/api\/v1\/zalo-oas\/([^/]+)\/templates\/sync$/.exec(url);
      if (sync && init?.method === "POST") return syncReply(sync[1]!);
      const attrs = /^\/api\/v1\/notice-attributes\?audience=(.*)$/.exec(url);
      if (attrs) return attributesReply(attrs[1]!);
      if (url === "/api/v1/notice-campaign-previews") return previewReply(JSON.parse(String(init?.body)) as PreviewBody);
      if (url === "/api/v1/notice-campaigns" && init?.method === "POST") return sendReplies.shift() ?? json(campaign("01JNEW", "dang-gui", { recipient_count: 11 }), 201);
      const list = /^\/api\/v1\/notice-campaigns(?:\?cursor=(.*))?$/.exec(url);
      if (list) return json(historyPages[decodeURIComponent(list[1] ?? "")] ?? { items: [], next_cursor: "" });
      const recips = /^\/api\/v1\/notice-campaigns\/([^/?]+)\/recipients(?:\?cursor=(.*))?$/.exec(url);
      if (recips) return json(recipientPages[decodeURIComponent(recips[2] ?? "")] ?? { items: [], next_cursor: "" });
      const one = /^\/api\/v1\/notice-campaigns\/([^/?]+)$/.exec(url);
      if (one && campaignById[one[1]!]) return json(campaignById[one[1]!]);
      return json({ code: "not_found", message: "Không có tuyến này.", trace_id: "" }, 404);
    }),
  );
}

let root: Root | null = null;
let host: HTMLDivElement | null = null;

beforeEach(() => {
  calls.length = 0;
  oaAnswer = { configured: true, items: [OA_ROW] };
  templatesAnswer = { zns: [ZNS_ONE], sms: [SMS_A] };
  previewReply = defaultPreview;
  sendReplies = [];
  historyPages = { "": { items: [], next_cursor: "" } };
  campaignById = {};
  recipientPages = {};
  attributesReply = defaultAttributes;
  syncReply = () => json(SYNC_RESULT);
  serve();
});

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  vi.unstubAllGlobals();
  vi.clearAllMocks();
  vi.useRealTimers();
});

const settle = () => act(async () => {});

async function mount(s: PhienDaDoc): Promise<HTMLDivElement> {
  H.session = s;
  host = document.createElement("div");
  document.body.append(host);
  root = createRoot(host);
  await act(async () => {
    root!.render(<NoticeSendScreen />);
  });
  await settle();
  await settle();
  return host;
}

const button = (el: HTMLElement, text: string) => [...el.querySelectorAll("button")].find((b) => b.textContent === text) ?? null;
async function click(el: HTMLElement, text: string) {
  const b = button(el, text);
  expect(b, text).not.toBeNull();
  await act(async () => b!.click());
  await settle();
}
const step = (el: HTMLElement, n: number) => el.querySelector<HTMLElement>(`[data-step="${n}"]`)!;
const stepOpen = (el: HTMLElement, n: number) => step(el, n).querySelector("button")!.getAttribute("aria-expanded") === "true";
const recipientsRow = (el: HTMLElement) => el.querySelector<HTMLElement>('[data-row="recipients"]')!;
const aside = (el: HTMLElement) => el.querySelector<HTMLElement>("[data-preview]")!;
const row = (el: HTMLElement, name: string) => el.querySelector<HTMLElement>(`[data-row="${name}"] dd`)?.textContent ?? null;
const sendButton = (el: HTMLElement) => el.querySelector<HTMLButtonElement>("button[data-send]");
const posts = (url: string) => calls.filter((c) => c.url === url && c.init?.method === "POST");
const bodyOf = (c: Call | undefined) => JSON.parse(String(c?.init?.body)) as Record<string, unknown>;
const tabs = (el: HTMLElement) => [...el.querySelectorAll<HTMLButtonElement>('[role="tab"]')];

const ZNS_KEY = "zns:01JOA1:zt-1";

async function toTemplateStep(el: HTMLElement) {
  await click(el, "Tiếp tục");
}
async function pickTemplate(el: HTMLElement, key = ZNS_KEY) {
  await act(async () => el.querySelector<HTMLButtonElement>(`[data-template-id="${key}"]`)!.click());
  await settle();
}
async function type(el: HTMLElement, id: string, value: string) {
  const input = el.querySelector<HTMLInputElement>(`#${id}`)!;
  const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!;
  await act(async () => {
    setter.call(input, value);
    input.dispatchEvent(new Event("input", { bubbles: true }));
  });
}
async function fillAndPreview(el: HTMLElement) {
  await type(el, "notice-param-0", "20/10");
  await type(el, "notice-param-1", "Trạm y tế");
  await click(el, "Xem trước");
}

// ---- the gate -------------------------------------------------------------------------------------

describe("the gate (notice.send)", () => {
  it("DENIED — no notice.send: the title and the prototype's sentence, NO route called", async () => {
    const el = await mount(session(["citizen.read", "admin.zalo"]));
    expect(el.textContent).toContain(NOTICE_SEND_TITLE);
    expect(el.textContent).toContain(NO_SEND_PERMISSION);
    expect(el.querySelector("[data-step]")).toBeNull();
    expect(calls).toHaveLength(0);
  });

  it("session not read yet: the checking sentence, nothing fetched", async () => {
    const el = await mount(null);
    expect(el.querySelector('[role="status"]')?.textContent).toContain("Đang kiểm tra");
    expect(calls).toHaveLength(0);
  });

  it("ALLOWED with notice.send only: templates come from notice-templates in ONE call; no zalo-oas, no citizen route", async () => {
    const el = await mount(session(["notice.send"]));
    expect(el.querySelector("h1")?.textContent).toBe(NOTICE_SEND_TITLE);
    expect(tabs(el).map((t) => t.textContent)).toEqual(["Soạn tin", "Đã gửi (0)"]);
    const card = el.querySelector('[data-oa-card="no-permission"]')!;
    expect(card.querySelector("a")?.getAttribute("href")).toBe("/cau-hinh?tab=kenh-zalo");
    await click(el, "Người dân theo nhóm");
    expect(el.textContent).toContain(NO_CITIZEN_READ);
    expect(button(el, "Tiếp tục")!.disabled).toBe(false);
    await click(el, "Toàn bộ người dân");
    await toTemplateStep(el);
    expect(step(el, 2).querySelector(`[data-template-id="${ZNS_KEY}"]`)).not.toBeNull();
    expect(step(el, 2).textContent).not.toContain("chưa có quyền xem mẫu");
    expect(calls.filter((c) => c.url === "/api/v1/notice-templates")).toHaveLength(1);
    expect(calls.filter((c) => c.url.includes("zalo-oas") || c.url.includes("citizen"))).toHaveLength(0);
  });
});

// ---- the steps and the preview ----------------------------------------------------------------------

describe("steps and the real preview", () => {
  it("1 → 2 → pick a ZNS template → 3: one field per parameter, the preview runs at once with the ADR 0011 kind and params keyed by field", async () => {
    const el = await mount(session(ALL));
    expect(aside(el).textContent).toContain(PREVIEW_EMPTY);
    await toTemplateStep(el);
    expect(stepOpen(el, 2)).toBe(true);
    const card = step(el, 2).querySelector(`[data-template-id="${ZNS_KEY}"]`)!;
    expect(card.textContent).toContain("Tham số: ngay_tiem, dia_diem");
    expect(card.textContent).toContain("Zalo ZNS · 200 đ một tin");
    expect(calls.find((c) => /zalo-oas\/[^/]+\/templates/.test(c.url))).toBeUndefined();

    await pickTemplate(el);
    expect(stepOpen(el, 3)).toBe(true);
    expect(el.querySelector<HTMLInputElement>("#notice-title")!.value).toBe("Thông báo lịch tiêm");
    expect([...step(el, 3).querySelectorAll("label")].map((l) => l.textContent)).toEqual(["Tên đợt gửi", "ngay_tiem", "dia_diem"]);
    expect(el.querySelector<HTMLInputElement>("#notice-param-0")!.maxLength).toBe(30);

    const p = posts("/api/v1/notice-campaign-previews");
    expect(p).toHaveLength(1);
    expect(bodyOf(p[0])).toEqual({
      channel: "zalo-zns",
      oa_id: "01JOA1",
      template_ref: "zt-1",
      audience: { kind: "nguoi-dan-tat-ca", ids: [], tags: [] },
      params: { ngay_tiem: "", dia_diem: "" },
    });
    expect(el.querySelector("[data-preview-content]")!.textContent).toBe(ZNS_CONTENT);
    expect(row(el, "preview-recipients")).toBe("11 người");
    expect(el.querySelector("[data-missing-params]")!.textContent).toBe("Còn chỗ trống chưa điền: ngay_tiem, dia_diem.");
    expect(sendButton(el)!.disabled).toBe(true);
    expect(el.textContent).toContain(SEND_NOTE);
  });

  it("filled parameters + Xem trước: the typed values go up keyed by field; group and tag audiences send their lists", async () => {
    const el = await mount(session(ALL));
    await click(el, "Người dân theo nhóm");
    await click(el, "Tổ trưởng (12)");
    await toTemplateStep(el);
    await pickTemplate(el);
    await fillAndPreview(el);
    expect(bodyOf(posts("/api/v1/notice-campaign-previews").at(-1))).toMatchObject({
      audience: { kind: "nguoi-dan-theo-nhom", ids: ["01JG1"], tags: [] },
      params: { ngay_tiem: "20/10", dia_diem: "Trạm y tế" },
    });

    await act(async () => step(el, 1).querySelector("button")!.click());
    await click(el, "Người dân theo nhãn");
    await click(el, "hội phụ nữ (9)");
    await toTemplateStep(el);
    await pickTemplate(el);
    expect(bodyOf(posts("/api/v1/notice-campaign-previews").at(-1))).toMatchObject({
      audience: { kind: "nguoi-dan-theo-nhan", ids: [], tags: ["hội phụ nữ"] },
    });

    await act(async () => step(el, 1).querySelector("button")!.click());
    await click(el, "Cán bộ theo bộ phận");
    await click(el, "Văn phòng");
    await toTemplateStep(el);
    await pickTemplate(el);
    expect(bodyOf(posts("/api/v1/notice-campaign-previews").at(-1))).toMatchObject({
      audience: { kind: "can-bo-theo-bo-phan", ids: ["01JU1"], tags: [] },
    });
  });

  it("the aside: recipients, skipped (warn), unit price and cost as vi-VN text; null cost → 'Chưa tính được'; the send unlocks when nothing is missing", async () => {
    previewReply = () => json({ content: null, recipient_count: 1250, skipped_count: 3, unit_price: "200.00", estimated_cost: "250000.00", missing_params: [] });
    const el = await mount(session(ALL));
    await toTemplateStep(el);
    await pickTemplate(el);
    expect(row(el, "preview-recipients")).toBe("1.250 người");
    expect(row(el, "preview-skipped")).toBe("3 người");
    expect(el.querySelector('[data-row="preview-skipped"] dt')!.textContent).toBe("Bỏ qua (ngừng liên hệ / thiếu số)");
    expect(row(el, "preview-unit-price")).toBe("200 đ");
    expect(row(el, "preview-cost")).toBe("250.000 đ");
    expect(sendButton(el)!.disabled).toBe(false);

    previewReply = () => json({ content: null, recipient_count: 0, skipped_count: 0, unit_price: "200.00", estimated_cost: null, missing_params: [] });
    await click(el, "Xem trước");
    expect(row(el, "preview-cost")).toBe("Chưa tính được");
    expect(el.querySelector('[data-row="preview-skipped"]')).toBeNull();
    expect(sendButton(el)!.disabled).toBe(true);
  });

  it("a changed parameter after the preview: the send locks with the reason until Xem trước runs again", async () => {
    const el = await mount(session(ALL));
    await toTemplateStep(el);
    await pickTemplate(el);
    await fillAndPreview(el);
    expect(sendButton(el)!.disabled).toBe(false);
    await type(el, "notice-param-0", "21/10");
    expect(el.querySelector("[data-preview-stale]")!.textContent).toBe(PREVIEW_STALE);
    expect(sendButton(el)!.disabled).toBe(true);
    await click(el, "Xem trước");
    expect(el.querySelector("[data-preview-stale]")).toBeNull();
    expect(sendButton(el)!.disabled).toBe(false);
  });

  it("a refused preview shows the server's sentence, no send button", async () => {
    previewReply = () => json({ code: "identity_unavailable", message: "Chưa tải được danh sách người nhận. Vui lòng thử lại sau ít phút.", trace_id: "" }, 503);
    const el = await mount(session(ALL));
    await toTemplateStep(el);
    await pickTemplate(el);
    expect(el.querySelector("[data-preview-refused]")!.textContent).toBe("Chưa tải được danh sách người nhận. Vui lòng thử lại sau ít phút.");
    expect(sendButton(el)).toBeNull();
  });
});

// ---- the send -------------------------------------------------------------------------------------

describe("the send (Idempotency-Key, toast, history re-read)", () => {
  it("POST notice-campaigns with an Idempotency-Key, the title and the previewed body; the prototype toast; history read again; tab unchanged", async () => {
    const el = await mount(session(ALL));
    await toTemplateStep(el);
    await pickTemplate(el);
    await fillAndPreview(el);
    const historyReadsBefore = calls.filter((c) => c.url === "/api/v1/notice-campaigns" && c.init?.method === "GET").length;
    await act(async () => sendButton(el)!.click());
    await settle();
    const sent = posts("/api/v1/notice-campaigns");
    expect(sent).toHaveLength(1);
    expect((sent[0]!.init!.headers as Record<string, string>)["Idempotency-Key"]).toMatch(/^[0-9a-f-]{36}$/);
    expect(bodyOf(sent[0])).toEqual({
      title: "Thông báo lịch tiêm",
      channel: "zalo-zns",
      oa_id: "01JOA1",
      template_ref: "zt-1",
      audience: { kind: "nguoi-dan-tat-ca", ids: [], tags: [] },
      params: { ngay_tiem: "20/10", dia_diem: "Trạm y tế" },
    });
    expect(toast.success).toHaveBeenCalledWith("Đã xếp hàng gửi cho 11 người.");
    expect(calls.filter((c) => c.url === "/api/v1/notice-campaigns" && c.init?.method === "GET").length).toBe(historyReadsBefore + 1);
    expect(tabs(el)[0]!.getAttribute("aria-selected")).toBe("true");
  });

  it("a retry after a refusal reuses the SAME key; a changed title gets a new one", async () => {
    sendReplies = [json({ code: "identity_unavailable", message: "Chưa tải được danh sách người nhận.", trace_id: "" }, 503)];
    const el = await mount(session(ALL));
    await toTemplateStep(el);
    await pickTemplate(el);
    await fillAndPreview(el);
    await act(async () => sendButton(el)!.click());
    await settle();
    expect(toast.error).toHaveBeenCalledWith("Chưa tải được danh sách người nhận.");
    await act(async () => sendButton(el)!.click());
    await settle();
    await type(el, "notice-title", "Lịch tiêm đợt 2");
    await act(async () => sendButton(el)!.click());
    await settle();
    const keys = posts("/api/v1/notice-campaigns").map((c) => (c.init!.headers as Record<string, string>)["Idempotency-Key"]);
    expect(keys).toHaveLength(3);
    expect(keys[1]).toBe(keys[0]);
    expect(keys[2]).not.toBe(keys[0]);
  });

  it("409 no_recipients: the prototype's sentence, not the server's", async () => {
    sendReplies = [json({ code: "no_recipients", message: "Đối tượng đã chọn không có ai nhận được tin. Chưa có gì được gửi.", trace_id: "" }, 409)];
    const el = await mount(session(ALL));
    await toTemplateStep(el);
    await pickTemplate(el);
    await fillAndPreview(el);
    await act(async () => sendButton(el)!.click());
    await settle();
    expect(toast.error).toHaveBeenCalledWith(NO_RECIPIENTS);
    expect(toast.success).not.toHaveBeenCalled();
  });
});

// ---- step 2: OAs and SMS accounts -----------------------------------------------------------------

describe("step 2 — several OAs, OA needing re-authorisation, SMS accounts", () => {
  it("an OA needing re-authorisation: its templates drawn disabled with the tag and the reason; ≥2 OAs name the OA on each card", async () => {
    templatesAnswer = { zns: [ZNS_ONE, ZNS_REAUTH], sms: [] };
    const el = await mount(session(ALL));
    await toTemplateStep(el);
    const blocked = el.querySelector<HTMLButtonElement>('[data-template-id="zns:01JOA2:zt-3"]')!;
    expect(blocked.disabled).toBe(true);
    expect(blocked.textContent).toContain(OA_REAUTH_TAG);
    expect(blocked.textContent).toContain(OA_REAUTH_REASON);
    expect(el.querySelector(`[data-template-id="${ZNS_KEY}"]`)!.textContent).toContain("Zalo ZNS · UBND xã Tân Phú · 200 đ một tin");
    expect(el.querySelectorAll("[data-oa-group]")).toHaveLength(2);
    await act(async () => blocked.click());
    expect(stepOpen(el, 3)).toBe(false);
    expect(posts("/api/v1/notice-campaign-previews")).toHaveLength(0);
  });

  it("no ZNS template: the prototype's sentence and the way to Cấu hình → Kênh Zalo", async () => {
    templatesAnswer = { zns: [], sms: [] };
    const el = await mount(session(ALL));
    await toTemplateStep(el);
    expect(step(el, 2).textContent).toContain(NO_TEMPLATES);
    expect(step(el, 2).querySelector("a")?.getAttribute("href")).toBe("/cau-hinh?tab=kenh-zalo");
  });

  it("SMS: the account select (first by default), the template text on the card, slot labels 'Ô n (≤m ký tự)', sms_account_id and positional params", async () => {
    templatesAnswer = { zns: [ZNS_ONE], sms: [SMS_A, SMS_B] };
    const el = await mount(session(ALL));
    await toTemplateStep(el);
    await click(el, "Tin nhắn SMS");
    const select = el.querySelector<HTMLSelectElement>("#notice-sms-account")!;
    expect([...select.options].map((o) => o.textContent)).toEqual(["Tài khoản chính · UBND XA", "Tài khoản dự phòng · UBND XA"]);
    expect(select.value).toBe("01JSMS1");
    const cardA = el.querySelector('[data-template-id="sms:01JSMS1:st-1"]')!;
    expect(cardA.textContent).toContain(SMS_A.content);
    expect(cardA.textContent).toContain("Tin nhắn SMS · Tài khoản chính · UBND XA · Đơn giá tính khi xem trước");
    expect(el.querySelector('[data-template-id="sms:01JSMS2:st-2"]')).toBeNull();

    await act(async () => {
      select.value = "01JSMS2";
      select.dispatchEvent(new Event("change", { bubbles: true }));
    });
    await pickTemplate(el, "sms:01JSMS2:st-2");
    expect([...step(el, 3).querySelectorAll("label")].map((l) => l.textContent)).toEqual(["Tên đợt gửi", "Ô 1 (≤10 ký tự)", "Ô 2 (≤5 ký tự)"]);
    expect(el.querySelector<HTMLInputElement>("#notice-param-1")!.maxLength).toBe(5);
    const body = bodyOf(posts("/api/v1/notice-campaign-previews").at(-1));
    expect(body).toMatchObject({ channel: "sms", sms_account_id: "01JSMS2", template_ref: "st-2", params: { "1": "", "2": "" } });
    expect("oa_id" in body).toBe(false);
    expect(el.querySelector("[data-preview-content]")!.textContent).toBe("UBND xa thong bao lich tiem ngay 20/10");
    expect(row(el, "preview-unit-price")).toBe("Chưa tính được");
    expect(el.querySelector("[data-missing-params]")!.textContent).toBe("Còn chỗ trống chưa điền: Ô 1, Ô 2.");
  });
});

// ---- the history -----------------------------------------------------------------------------------

describe("the Đã gửi tab", () => {
  it("five prototype columns, the four statuses in words, cost as text; the count follows the rows loaded; Tải thêm reads the next cursor", async () => {
    historyPages = {
      "": {
        items: [
          campaign("01JC1", "dang-gui"),
          campaign("01JC2", "da-gui", { audience_kind: "can-bo-tat-ca", created_by_name: "" }),
          campaign("01JC3", "gui-mot-phan", { sent_count: 7 }),
          campaign("01JC4", "that-bai", { estimated_cost: null }),
        ],
        next_cursor: "c2",
      },
      c2: { items: [campaign("01JC5", "da-gui")], next_cursor: "" },
    };
    const el = await mount(session(ALL));
    expect(tabs(el)[1]!.textContent).toBe("Đã gửi (4)");
    await act(async () => tabs(el)[1]!.click());
    const table = el.querySelector("[data-history] table")!;
    expect([...table.querySelectorAll("th")].map((t) => t.textContent)).toEqual(["Đợt gửi", "Đối tượng", "Người nhận", "Chi phí", "Trạng thái"]);
    const cells = (id: string) => [...el.querySelectorAll(`[data-campaign-id="${id}"] td`)].map((td) => td.textContent);
    expect(cells("01JC1")).toEqual(["Đợt 01JC1Thông báo lịch tiêm · Lê Thị Gửi", "Người dân theo nhóm", "10", "2.000 đ", "Đang gửi"]);
    expect(cells("01JC2")[0]).toContain("Thông báo lịch tiêm · CB-00123");
    expect(cells("01JC2")[1]).toBe("Toàn bộ cán bộ");
    expect(cells("01JC2")[4]).toBe("Đã gửi");
    expect(cells("01JC3")[4]).toBe("Gửi một phần · 7/10");
    expect(cells("01JC4")[3]).toBe("Chưa tính được");
    expect(cells("01JC4")[4]).toBe("Hỏng");

    await click(el, "Tải thêm");
    expect(calls.map((c) => c.url)).toContain("/api/v1/notice-campaigns?cursor=c2");
    expect(el.querySelector('[data-campaign-id="01JC5"]')).not.toBeNull();
    expect(tabs(el)[1]!.textContent).toBe("Đã gửi (5)");
    expect(button(el, "Tải thêm")).toBeNull();
  });

  it("empty: the prototype's sentence", async () => {
    const el = await mount(session(ALL));
    await act(async () => tabs(el)[1]!.click());
    expect(el.textContent).toContain(HISTORY_EMPTY);
  });

  it("a campaign still sending is re-read every 10 s while the tab is open — and not while it is closed", async () => {
    vi.useFakeTimers({ toFake: ["setInterval", "clearInterval"] });
    historyPages = { "": { items: [campaign("01JC1", "dang-gui")], next_cursor: "" } };
    campaignById = { "01JC1": campaign("01JC1", "da-gui", { sent_count: 10 }) };
    const el = await mount(session(ALL));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(10_000);
    });
    expect(calls.find((c) => c.url === "/api/v1/notice-campaigns/01JC1")).toBeUndefined();

    await act(async () => tabs(el)[1]!.click());
    await act(async () => {
      await vi.advanceTimersByTimeAsync(10_000);
    });
    await settle();
    expect(calls.filter((c) => c.url === "/api/v1/notice-campaigns/01JC1")).toHaveLength(1);
    expect(el.querySelector('[data-campaign-id="01JC1"] [data-status]')!.textContent).toBe("Đã gửi");
    await act(async () => {
      await vi.advanceTimersByTimeAsync(20_000);
    });
    expect(calls.filter((c) => c.url === "/api/v1/notice-campaigns/01JC1")).toHaveLength(1);
  });
});

// ---- the step-1 count ------------------------------------------------------------------------------

describe("the step-1 recipient count — real where the client can count it, '?' where it cannot", () => {
  it("ONE group: the count route with that group; TWO groups: '?' with the overlap reason", async () => {
    const el = await mount(session(ALL));
    await click(el, "Người dân theo nhóm");
    await click(el, "Tổ trưởng (12)");
    const count = calls.filter((c) => c.url === "/api/v1/citizen-contact-count-queries").at(-1)!;
    expect(JSON.parse(String(count.init?.body))).toMatchObject({ group_id: "01JG1" });
    expect(recipientsRow(el).textContent).toContain("11 người");
    await click(el, "Hộ kinh doanh (30)");
    expect(recipientsRow(el).querySelector("[data-pending-marker]")).not.toBeNull();
    expect(recipientsRow(el).textContent).not.toContain("người");
    expect(recipientSource("citizen_group", ["a", "b"], [], [], true)).toEqual({ type: "unknown", reason: COUNT_OVERLAP });
  });

  it("ONE tag: the tag strip's own count; staff by unit: the directory filtered on the ONE unit each person has", async () => {
    const el = await mount(session(ALL));
    await click(el, "Người dân theo nhãn");
    await click(el, "cựu chiến binh (7)");
    expect(recipientsRow(el).textContent).toContain("7 người");
    await click(el, "Cán bộ theo bộ phận");
    await click(el, "Văn phòng");
    expect(recipientsRow(el).textContent).toContain("2 người");
    expect(countStaff([{ department_id: "u1" }, { department_id: "" }], null)).toBe(2);
  });
});

// ---- rule 3 ----------------------------------------------------------------------------------------

describe("rule 3 — nothing personal on this screen", () => {
  it("no phone number, no recipient name, no reveal route — through preview, send and history", async () => {
    historyPages = { "": { items: [campaign("01JC1", "da-gui")], next_cursor: "" } };
    const el = await mount(session(ALL));
    await click(el, "Cán bộ theo bộ phận");
    await click(el, "Văn phòng");
    await toTemplateStep(el);
    await pickTemplate(el);
    await fillAndPreview(el);
    await act(async () => sendButton(el)!.click());
    await settle();
    const compose = el.textContent ?? "";
    await act(async () => tabs(el)[1]!.click());
    for (const text of [compose, el.textContent ?? ""]) {
      expect(text).not.toMatch(/0\d{9}|\d{2,4}\*{3,}\d{3,4}/);
      expect(text).not.toContain("Ví dụ người nhận");
      for (const code of ["CB-1", "CB-2"]) expect(text).not.toContain(`${code} `);
    }
    expect(calls.find((c) => c.url.endsWith("/phone") || c.url.endsWith("/email"))).toBeUndefined();
  });
});

// ---- pure ------------------------------------------------------------------------------------------

describe("money is formatted from the decimal TEXT, never a float", () => {
  it("groups thousands with '.', keeps a non-zero fraction with ',', drops trailing zeros", () => {
    expect(formatMoney("3500.00")).toBe("3.500 đ");
    expect(formatMoney("200.00")).toBe("200 đ");
    expect(formatMoney("1234567.50")).toBe("1.234.567,5 đ");
    expect(formatMoney("0.00")).toBe("0 đ");
    expect(formatMoney("90071992547409930.10")).toBe("90.071.992.547.409.930,1 đ");
  });
});

// ---- the OA card -------------------------------------------------------------------------------------

describe("the OA card (ADR 0098, several OAs since ADR 0099 #1)", () => {
  it("linked: which OA sends, and the way to manage it", async () => {
    const el = await mount(session(ALL));
    const card = el.querySelector('[data-oa-card="linked"]')!;
    expect(card.querySelector("[data-oa-sender]")!.textContent).toBe("Gửi từ OA: UBND xã Tân Phú");
    expect(card.querySelector("a")?.textContent).toBe("Quản lý OA");
  });

  it("two OAs, one needing re-authorisation: the card lists the one that can send and warns naming the other", async () => {
    oaAnswer = { configured: true, items: [OA_ROW, { ...OA_ROW_2, needs_reauthorization: REAUTH }] };
    const el = await mount(session(ALL));
    const card = el.querySelector('[data-oa-card="linked"]')!;
    expect(card.querySelector("[data-oa-sender]")!.textContent).toBe("Gửi từ OA: UBND xã Tân Phú");
    const warning = card.querySelector('[role="alert"]')!.textContent;
    expect(warning).toContain("Đoàn thanh niên xã Tân Phú:");
    expect(warning).toContain("quá 3 tháng");
    expect(card.querySelector("button")!.textContent).toBe("Uỷ quyền lại");
  });

  it("not linked: Kết nối OA opens the link in a new tab and never puts it in the DOM", async () => {
    oaAnswer = { configured: true, items: [] };
    const open = vi.fn(() => null);
    vi.stubGlobal("open", open);
    const el = await mount(session(ALL));
    expect(el.querySelector('[data-oa-card="not-linked"]')!.textContent).toContain(OA_NOT_LINKED);
    const connect = el.querySelector<HTMLButtonElement>('[data-oa-card="not-linked"] button')!;
    expect(connect.textContent).toBe("Kết nối OA");
    await act(async () => connect.click());
    await settle();
    expect(calls.find((c) => c.url === "/api/v1/zalo-oas/authorization-links")?.init?.method).toBe("POST");
    expect(open).toHaveBeenCalledWith(expect.stringContaining("oauth.zaloapp.com"), "_blank", "noopener");
    expect(el.innerHTML).not.toContain("SECRET-STATE");
  });

  it("platform app not declared (configured=false): the platform sentence, no connect button", async () => {
    oaAnswer = { configured: false, items: [] };
    const el = await mount(session(ALL));
    const card = el.querySelector('[data-oa-card="not-configured"]')!;
    expect(card.textContent).toContain("Nền tảng chưa khai ứng dụng Zalo dùng chung");
    expect(card.querySelector("button")).toBeNull();
  });
});

// ---- the imported sources (ADR 0099 §Sửa đổi S1–S4) -------------------------------------------------

async function typeArea(el: HTMLElement, id: string, value: string) {
  const area = el.querySelector<HTMLTextAreaElement | HTMLInputElement>(`#${id}`)!;
  const proto = area instanceof HTMLInputElement ? HTMLInputElement.prototype : HTMLTextAreaElement.prototype;
  const setter = Object.getOwnPropertyDescriptor(proto, "value")!.set!;
  await act(async () => {
    setter.call(area, value);
    area.dispatchEvent(new Event("input", { bubbles: true }));
  });
}
async function choose(el: HTMLElement, id: string, value: string) {
  const select = el.querySelector<HTMLSelectElement>(`#${id}`)!;
  await act(async () => {
    select.value = value;
    select.dispatchEvent(new Event("change", { bubbles: true }));
  });
}
async function upload(el: HTMLElement, file: File) {
  const input = el.querySelector<HTMLInputElement>("#notice-file")!;
  Object.defineProperty(input, "files", { value: [file], configurable: true });
  await act(async () => {
    input.dispatchEvent(new Event("change", { bubbles: true }));
  });
  for (let i = 0; i < 5; i++) await settle();
}

const CSV = "﻿Họ tên,Số điện thoại,Ngày\r\nBà A,0900000000,20/10\r\nÔng B,0900000001,21/10\r\nBà C,0900000000,22/10\r\n";
const importedPreview = (extra: Record<string, unknown> = {}) =>
  json({ content: "Mời Bà A họp", recipient_count: 2, skipped_count: 0, invalid_count: 1, duplicate_count: 1, unit_price: null, estimated_cost: null, missing_params: [], ...extra });

describe("imported source — typed list", () => {
  it("SMS: live count, account select, shared content with a counter; the body carries danh-sach-so, rows, the content, template_ref empty (required by the contract), NO mapping", async () => {
    previewReply = () => importedPreview();
    const el = await mount(session(["notice.send"]));
    await click(el, "Nhập số điện thoại");
    await typeArea(el, "notice-typed-phones", "0900000000, 0900000001, 0900000002");
    expect(el.querySelector("[data-typed-count]")!.textContent).toBe("3 số đã nhập");
    expect(recipientsRow(el).textContent).toContain("3 số đã nhập");
    await toTemplateStep(el);
    await click(el, "Tin nhắn SMS");
    expect(el.querySelector("[data-template-id]")).toBeNull();
    expect(el.querySelector<HTMLSelectElement>("#notice-sms-account")!.value).toBe("01JSMS1");
    await click(el, "Tiếp tục");
    expect(stepOpen(el, 3)).toBe(true);
    // A typed list has no columns: no mode switch, no "?" for variables.
    expect(el.querySelector('input[name="notice-sms-mode"]')).toBeNull();
    expect(el.querySelector("[data-variables-help]")).toBeNull();
    await typeArea(el, "notice-sms-content", "UBND xa thong bao lich tiem");
    expect(el.querySelector("[data-char-counter]")!.textContent).toBe("27/1.000 ký tự");
    await click(el, "Xem trước");
    const body = bodyOf(posts("/api/v1/notice-campaign-previews").at(-1));
    expect(body).toEqual({
      channel: "sms",
      sms_account_id: "01JSMS1",
      template_ref: "",
      audience: { kind: "danh-sach-so", ids: [], tags: [] },
      params: {},
      recipients: [{ phone: "0900000000" }, { phone: "0900000001" }, { phone: "0900000002" }],
      sms_content_mode: "noi-dung-chung",
      content: "UBND xa thong bao lich tiem",
    });
    expect(row(el, "preview-invalid")).toBe("1");
    expect(row(el, "preview-duplicate")).toBe("1");
    expect(el.querySelector('[data-row="preview-invalid"] dt')!.textContent).toBe("Số không hợp lệ");
    expect(el.querySelector('[data-row="preview-duplicate"] dt')!.textContent).toBe("Trùng, đã bỏ");
    expect(sendButton(el)!.disabled).toBe(false);

    await act(async () => sendButton(el)!.click());
    await settle();
    expect(bodyOf(posts("/api/v1/notice-campaigns")[0])).toMatchObject({ title: "UBND xa thong bao lich tiem", audience: { kind: "danh-sach-so" } });
  });

  it("refusals: variables in a typed list, over 1.000 numbers — Xem trước locked, nothing posted", async () => {
    const el = await mount(session(["notice.send"]));
    await click(el, "Nhập số điện thoại");
    await typeArea(el, "notice-typed-phones", Array.from({ length: 1001 }, () => "0900000000").join(","));
    expect(el.querySelector('[data-typed-phones] [role="alert"]')!.textContent).toContain("Tối đa 1.000 số");
    await typeArea(el, "notice-typed-phones", "0900000000");
    await toTemplateStep(el);
    await click(el, "Tin nhắn SMS");
    await click(el, "Tiếp tục");
    await typeArea(el, "notice-sms-content", "Chào {{variable_1}}");
    expect(el.querySelector("[data-import-errors]")!.textContent).toContain("Danh sách nhập tay không có cột để thay biến");
    expect(button(el, "Xem trước")!.disabled).toBe(true);
    expect(posts("/api/v1/notice-campaign-previews")).toHaveLength(0);
  });
});

describe("imported source — file read in the browser", () => {
  it("SMS shared text with {{variable_N}}: summary, guessed phone column, MASKED preview, one select per variable, the '?' help, rows + mapping in the body", async () => {
    const el = await mount(session(["notice.send"]));
    await click(el, "Nhập tệp");
    await upload(el, new File([CSV], "ds.csv"));
    expect(el.querySelector("[data-file-summary]")!.textContent).toBe("ds.csv · 3 dòng");
    expect(el.querySelector<HTMLSelectElement>("#notice-phone-column")!.value).toBe("1");
    const preview = el.querySelector("[data-file-preview]")!;
    expect(preview.querySelectorAll("tbody tr")).toHaveLength(3);
    expect(preview.textContent).toContain("09****0000");
    expect(el.textContent).not.toMatch(/0900000000|0900000001/);
    expect(calls.some((c) => c.url.includes("0900"))).toBe(false);

    await toTemplateStep(el);
    await click(el, "Tin nhắn SMS");
    await click(el, "Tiếp tục");
    const shared = el.querySelector<HTMLInputElement>('input[name="notice-sms-mode"][value="noi-dung-chung"]')!;
    await act(async () => shared.click());
    await typeArea(el, "notice-sms-content", "Mời {{variable_1}} họp ngày {{variable_2}}");
    expect([...el.querySelectorAll("[data-variables] label")].map((l) => l.textContent)).toEqual(["{{variable_1}}", "{{variable_2}}"]);

    const help = el.querySelector<HTMLButtonElement>("[data-variables-help]")!;
    expect(help.getAttribute("aria-label")).toBe("Cách dùng biến trong nội dung");
    await act(async () => help.click());
    expect(el.querySelector("[data-variables-help-text]")!.textContent).toContain("{{variable_1}}, {{variable_2}}");

    await choose(el, "notice-variable-1", "0");
    await click(el, "Xem trước");
    // variable_2 unmapped: the request goes, comms names it, the aside shows it as written.
    previewReply = () => importedPreview({ missing_params: ["variable_2"] });
    await click(el, "Xem trước");
    expect(el.querySelector("[data-missing-params]")!.textContent).toBe("Còn chỗ trống chưa điền: {{variable_2}}.");
    expect(sendButton(el)!.disabled).toBe(true);

    await choose(el, "notice-variable-2", "2");
    previewReply = () => importedPreview();
    await click(el, "Xem trước");
    expect(bodyOf(posts("/api/v1/notice-campaign-previews").at(-1))).toEqual({
      channel: "sms",
      sms_account_id: "01JSMS1",
      template_ref: "",
      audience: { kind: "tep-nhap", ids: [], tags: [] },
      params: {},
      recipients: [
        { phone: "0900000000", values: { variable_1: "Bà A", variable_2: "20/10" } },
        { phone: "0900000001", values: { variable_1: "Ông B", variable_2: "21/10" } },
        { phone: "0900000000", values: { variable_1: "Bà C", variable_2: "22/10" } },
      ],
      sms_content_mode: "noi-dung-chung",
      content: "Mời {{variable_1}} họp ngày {{variable_2}}",
      mapping: { phone_column: "Số điện thoại", variables: { variable_1: "Họ tên", variable_2: "Ngày" } },
    });
    expect(sendButton(el)!.disabled).toBe(false);
  });

  it("SMS from a content column: {phone, content} rows, content_column in the mapping; no column = a sentence", async () => {
    const el = await mount(session(["notice.send"]));
    await click(el, "Nhập tệp");
    await upload(el, new File(["SĐT,Nội dung\n0900000000,Mời họp\n"], "a.csv"));
    await toTemplateStep(el);
    await click(el, "Tin nhắn SMS");
    await click(el, "Tiếp tục");
    expect(el.querySelector("[data-import-errors]")!.textContent).toBe("Chọn cột nội dung.");
    await choose(el, "notice-content-column", "1");
    await click(el, "Xem trước");
    expect(bodyOf(posts("/api/v1/notice-campaign-previews").at(-1))).toMatchObject({
      recipients: [{ phone: "0900000000", content: "Mời họp" }],
      sms_content_mode: "cot-trong-tep",
      mapping: { phone_column: "SĐT", content_column: "Nội dung" },
    });
    expect("content" in bodyOf(posts("/api/v1/notice-campaign-previews").at(-1))).toBe(false);
  });

  it("ZNS: each parameter 'Giá trị chung | Lấy từ cột …'; a column-fed parameter goes in each row's values and the mapping", async () => {
    const el = await mount(session(["notice.send"]));
    await click(el, "Nhập tệp");
    await upload(el, new File([CSV], "ds.csv"));
    await toTemplateStep(el);
    await pickTemplate(el);
    expect([...el.querySelectorAll<HTMLOptionElement>("#notice-param-src-0 option")].map((o) => o.textContent)).toEqual([
      "Giá trị chung",
      "Lấy từ Họ tên (cột A)",
      "Lấy từ Số điện thoại (cột B)",
      "Lấy từ Ngày (cột C)",
    ]);
    await choose(el, "notice-param-src-0", "2");
    expect(el.querySelector("#notice-param-0")).toBeNull();
    await type(el, "notice-param-1", "Trạm y tế");
    await click(el, "Xem trước");
    expect(bodyOf(posts("/api/v1/notice-campaign-previews").at(-1))).toEqual({
      channel: "zalo-zns",
      oa_id: "01JOA1",
      template_ref: "zt-1",
      audience: { kind: "tep-nhap", ids: [], tags: [] },
      params: { ngay_tiem: "", dia_diem: "Trạm y tế" },
      recipients: [
        { phone: "0900000000", values: { ngay_tiem: "20/10" } },
        { phone: "0900000001", values: { ngay_tiem: "21/10" } },
        { phone: "0900000000", values: { ngay_tiem: "22/10" } },
      ],
      mapping: { phone_column: "Số điện thoại", params: { ngay_tiem: "Ngày" } },
    });
  });

  it("a wrong file: the sentence, no table, nothing posted", async () => {
    const el = await mount(session(["notice.send"]));
    await click(el, "Nhập tệp");
    await upload(el, new File(["x"], "ds.xls"));
    expect(el.querySelector('[data-file-import] [role="alert"]')!.textContent).toContain("Chỉ nhận tệp .xlsx hoặc .csv");
    expect(el.querySelector("[data-file-summary]")).toBeNull();
    expect(posts("/api/v1/notice-campaign-previews")).toHaveLength(0);
  });
});

describe("history — one campaign's recipients and its mapping (S5)", () => {
  const recipient = (id: string, status: string, extra: Record<string, unknown> = {}) => ({
    id,
    phone_masked: "09****0000",
    status,
    error_message: "",
    sent_at: null,
    created_at: "2026-10-17T01:00:00Z",
    ...extra,
  });

  it("the row shows the invalid count; a click opens the masked recipients, the five results in words, the provider error, Tải thêm, the mapping block", async () => {
    const imported = campaign("01JC9", "gui-mot-phan", {
      channel: "sms",
      template_name: "",
      audience_kind: "tep-nhap",
      invalid_count: 2,
      sent_count: 1,
      recipient_mapping: { phone_column: "Số điện thoại", variables: { variable_1: "Họ tên" } },
      content: "Mời {{variable_1}} họp",
      sms_content_mode: "noi-dung-chung",
    });
    historyPages = { "": { items: [imported], next_cursor: "" } };
    recipientPages = {
      "": {
        items: [
          recipient("r1", "da-gui", { sent_at: "2026-10-17T02:05:00Z" }),
          recipient("r2", "that-bai", { error_message: "Brandname chưa kích hoạt (104)" }),
          recipient("r3", "khong-hop-le", { phone_masked: "12" }),
          recipient("r4", "cho-gui"),
          recipient("r5", "bo-qua"),
        ],
        next_cursor: "p2",
      },
      p2: { items: [recipient("r6", "da-gui")], next_cursor: "" },
    };
    const el = await mount(session(ALL));
    await act(async () => tabs(el)[1]!.click());
    const cells = [...el.querySelectorAll('[data-campaign-id="01JC9"] td')].map((td) => td.textContent);
    expect(cells[0]).toBe("Đợt 01JC9Lê Thị Gửi");
    expect(cells[1]).toBe("Danh sách từ tệp");
    expect(el.querySelector('[data-campaign-id="01JC9"] [data-invalid-count]')!.textContent).toBe("2 số không hợp lệ");

    await act(async () => el.querySelector<HTMLButtonElement>('[data-open-campaign="01JC9"]')!.click());
    await settle();
    expect(calls.map((c) => c.url)).toContain("/api/v1/notice-campaigns/01JC9/recipients");
    const statuses = [...el.querySelectorAll("[data-recipient-status]")].map((b) => b.textContent);
    expect(statuses).toEqual(["Đã gửi", "Gửi thất bại", "Số không hợp lệ", "Đang chờ gửi", "Bỏ qua"]);
    const r2 = [...el.querySelectorAll('[data-recipient-id="r2"] td')].map((td) => td.textContent);
    expect(r2[0]).toBe("09****0000");
    expect(r2[2]).toBe("Brandname chưa kích hoạt (104)");
    expect([...el.querySelectorAll('[data-recipient-id="r1"] td')][3]!.textContent).toBe("17/10/2026 09:05");

    const mapping = el.querySelector("[data-mapping]")!;
    expect(mapping.querySelector("h3")!.textContent).toBe("Thông tin map biến");
    expect(mapping.querySelector('[data-mapping-row="phone"]')!.textContent).toBe("Cột số điện thoại:Số điện thoại");
    expect(mapping.querySelector('[data-mapping-row="shared-content"]')!.textContent).toBe("Nội dung chung:Mời {{variable_1}} họp");
    expect([...mapping.querySelectorAll('[data-mapping-table="variables"] td')].map((td) => td.textContent)).toEqual(["{{variable_1}}", "Họ tên"]);

    await click(el, "Tải thêm");
    expect(calls.map((c) => c.url)).toContain("/api/v1/notice-campaigns/01JC9/recipients?cursor=p2");
    expect(el.querySelector('[data-recipient-id="r6"]')).not.toBeNull();
    expect(button(el, "Tải thêm")).toBeNull();

    await click(el, "Quay lại danh sách đợt");
    expect(el.querySelector('[data-campaign-id="01JC9"]')).not.toBeNull();
  });

  it("ZNS mapping: Tham số → Cột, the rest shared; a directory campaign has no mapping block", async () => {
    historyPages = {
      "": {
        items: [
          campaign("01JZ", "da-gui", { audience_kind: "tep-nhap", recipient_mapping: { phone_column: "SĐT", params: { ngay_tiem: "Ngày" } }, content: null, sms_content_mode: null }),
          campaign("01JD", "da-gui", { recipient_mapping: null, content: null, sms_content_mode: null }),
        ],
        next_cursor: "",
      },
    };
    const el = await mount(session(ALL));
    await act(async () => tabs(el)[1]!.click());
    await act(async () => el.querySelector<HTMLButtonElement>('[data-open-campaign="01JZ"]')!.click());
    await settle();
    const mapping = el.querySelector("[data-mapping]")!;
    expect([...mapping.querySelectorAll('[data-mapping-table="params"] td')].map((td) => td.textContent)).toEqual(["ngay_tiem", "Ngày"]);
    expect(mapping.textContent).toContain("Tham số không có trong bảng dùng giá trị chung.");
    await click(el, "Quay lại danh sách đợt");
    await act(async () => el.querySelector<HTMLButtonElement>('[data-open-campaign="01JD"]')!.click());
    await settle();
    expect(el.querySelector("[data-mapping]")).toBeNull();
    expect(el.textContent).toContain("Đợt này chưa có dòng người nhận nào.");
  });
});

// ---- ADR 0101 #12: a parameter fed by each recipient's attribute -------------------------------------

describe("directory — parameters mapped to an attribute (ADR 0101 #12)", () => {
  async function rowSource(el: HTMLElement, field: string, label: "Giá trị chung" | "Theo thuộc tính") {
    const b = [...el.querySelectorAll<HTMLButtonElement>(`[data-param="${field}"] button`)].find((x) => x.textContent === label);
    expect(b, `${field} ${label}`).toBeDefined();
    await act(async () => b!.click());
    await settle();
  }
  const optionTexts = (el: HTMLElement, id: string) => [...el.querySelector<HTMLSelectElement>(`#${id}`)!.options].map((o) => o.textContent);

  it("ZNS: 'Theo thuộc tính' picks from the citizen list; the mapped key goes up '' in params (not the typed value) and in attribute_mapping — preview and send", async () => {
    const el = await mount(session(ALL));
    await toTemplateStep(el);
    await pickTemplate(el);
    expect(calls.map((c) => c.url)).toContain("/api/v1/notice-attributes?audience=nguoi-dan");
    expect(bodyOf(posts("/api/v1/notice-campaign-previews")[0])).not.toHaveProperty("attribute_mapping");

    await type(el, "notice-param-0", "giá trị gõ trước");
    await type(el, "notice-param-1", "Trạm y tế");
    await rowSource(el, "ngay_tiem", "Theo thuộc tính");
    expect(el.querySelector("#notice-param-0")).toBeNull();
    expect(optionTexts(el, "notice-param-attr-0")).toEqual(["Chọn thuộc tính", "Họ và tên", "Địa chỉ", "Phân loại hộ"]);
    await choose(el, "notice-param-attr-0", "hongheo");
    await click(el, "Xem trước");
    const body = bodyOf(posts("/api/v1/notice-campaign-previews").at(-1));
    expect(body.params).toEqual({ ngay_tiem: "", dia_diem: "Trạm y tế" });
    expect(body.attribute_mapping).toEqual({ ngay_tiem: "hongheo" });

    previewReply = () => json({ content: null, recipient_count: 3, skipped_count: 0, unit_price: "200.00", estimated_cost: "600.00", missing_params: [] });
    await click(el, "Xem trước");
    await act(async () => sendButton(el)!.click());
    await settle();
    const sent = bodyOf(posts("/api/v1/notice-campaigns")[0]);
    expect(sent.params).toEqual({ ngay_tiem: "", dia_diem: "Trạm y tế" });
    expect(sent.attribute_mapping).toEqual({ ngay_tiem: "hongheo" });

    // Back to "Giá trị chung": the mapping leaves the body.
    await rowSource(el, "ngay_tiem", "Giá trị chung");
    await type(el, "notice-param-0", "20/10");
    await click(el, "Xem trước");
    const back = bodyOf(posts("/api/v1/notice-campaign-previews").at(-1));
    expect(back).not.toHaveProperty("attribute_mapping");
    expect(back.params).toEqual({ ngay_tiem: "20/10", dia_diem: "Trạm y tế" });
  });

  it("'Theo thuộc tính' with nothing picked: no mapping, the key empty — the server reports it missing", async () => {
    const el = await mount(session(ALL));
    await toTemplateStep(el);
    await pickTemplate(el);
    await rowSource(el, "ngay_tiem", "Theo thuộc tính");
    await click(el, "Xem trước");
    const body = bodyOf(posts("/api/v1/notice-campaign-previews").at(-1));
    expect(body).not.toHaveProperty("attribute_mapping");
    expect(el.querySelector("[data-missing-params]")!.textContent).toContain("ngay_tiem");
  });

  it("SMS: slot keys '1','2' map; the preview shows the server's sample with [Nhãn] as returned", async () => {
    templatesAnswer = { zns: [ZNS_ONE], sms: [SMS_B] };
    previewReply = () => json({ content: "Hop dan [Họ và tên] luc 8h", recipient_count: 2, skipped_count: 0, unit_price: null, estimated_cost: null, missing_params: [] });
    const el = await mount(session(ALL));
    await toTemplateStep(el);
    await click(el, "Tin nhắn SMS");
    await pickTemplate(el, "sms:01JSMS2:st-2");
    await rowSource(el, "1", "Theo thuộc tính");
    await choose(el, "notice-param-attr-0", "full_name");
    await type(el, "notice-param-1", "8h");
    await click(el, "Xem trước");
    const body = bodyOf(posts("/api/v1/notice-campaign-previews").at(-1));
    expect(body).toMatchObject({ channel: "sms", params: { "1": "", "2": "8h" }, attribute_mapping: { "1": "full_name" } });
    expect(el.querySelector("[data-preview-content]")!.textContent).toBe("Hop dan [Họ và tên] luc 8h");
  });

  it("staff audience: the can-bo list (Họ và tên only); switching citizen → staff drops the mapping back to 'Giá trị chung'", async () => {
    const el = await mount(session(ALL));
    await toTemplateStep(el);
    await pickTemplate(el);
    await rowSource(el, "ngay_tiem", "Theo thuộc tính");
    await choose(el, "notice-param-attr-0", "address");

    await act(async () => step(el, 1).querySelector("button")!.click());
    await click(el, "Toàn bộ cán bộ");
    await toTemplateStep(el);
    await pickTemplate(el);
    expect(calls.map((c) => c.url)).toContain("/api/v1/notice-attributes?audience=can-bo");
    expect(el.querySelector('[data-param="ngay_tiem"] button[aria-pressed="true"]')!.textContent).toBe("Giá trị chung");
    expect(el.querySelector("#notice-param-attr-0")).toBeNull();
    const first = bodyOf(posts("/api/v1/notice-campaign-previews").at(-1));
    expect(first).toMatchObject({ audience: { kind: "can-bo-tat-ca" } });
    expect(first).not.toHaveProperty("attribute_mapping");

    await rowSource(el, "ngay_tiem", "Theo thuộc tính");
    expect(optionTexts(el, "notice-param-attr-0")).toEqual(["Chọn thuộc tính", "Họ và tên"]);
  });

  it("the attribute list refused: the server's sentence, the select disabled", async () => {
    attributesReply = () => json({ code: "identity_unavailable", message: "Danh bạ tạm thời không trả lời.", trace_id: "" }, 503);
    const el = await mount(session(ALL));
    await toTemplateStep(el);
    await pickTemplate(el);
    await rowSource(el, "ngay_tiem", "Theo thuộc tính");
    expect(el.querySelector<HTMLSelectElement>("#notice-param-attr-0")!.disabled).toBe(true);
    expect(el.querySelector("[data-attributes-refused]")!.textContent).toBe("Danh bạ tạm thời không trả lời.");
  });

  it("the '?' opens the explanation on press; imported sources show no attribute toggle", async () => {
    const el = await mount(session(ALL));
    await toTemplateStep(el);
    await pickTemplate(el);
    await act(async () => el.querySelector<HTMLButtonElement>("[data-attribute-help]")!.click());
    const help = el.querySelector("[data-attribute-help-text]")!.textContent!;
    expect(help).toContain("giá trị thuộc tính đó của chính mình");
    expect(help).toContain("ô trống");
    expect(help).toContain("cắt bớt");

    await act(async () => step(el, 1).querySelector("button")!.click());
    await click(el, "Nhập số điện thoại");
    await toTemplateStep(el);
    await pickTemplate(el);
    expect(el.querySelector("[data-attribute-help]")).toBeNull();
    expect([...el.querySelectorAll('[data-param="ngay_tiem"] button')].map((b) => b.textContent)).not.toContain("Theo thuộc tính");
  });

  it("history: Tham số → Thuộc tính with the label from the list, the code when the list lacks it; SMS slots as 'Ô n'", async () => {
    historyPages = {
      "": {
        items: [
          campaign("01JA", "da-gui", { recipient_mapping: null, content: null, sms_content_mode: null, attribute_mapping: { ngay_tiem: "hongheo", dia_diem: "da_xoa" } }),
          campaign("01JS", "da-gui", {
            channel: "sms",
            audience_kind: "can-bo-tat-ca",
            recipient_mapping: null,
            content: null,
            sms_content_mode: null,
            attribute_mapping: { "1": "full_name" },
          }),
        ],
        next_cursor: "",
      },
    };
    const el = await mount(session(ALL));
    await act(async () => tabs(el)[1]!.click());
    await act(async () => el.querySelector<HTMLButtonElement>('[data-open-campaign="01JA"]')!.click());
    await settle();
    expect(calls.map((c) => c.url)).toContain("/api/v1/notice-attributes?audience=nguoi-dan");
    const mapping = el.querySelector("[data-mapping]")!;
    expect(mapping.querySelector("h3")!.textContent).toBe("Thông tin map biến");
    expect([...mapping.querySelectorAll('[data-mapping-table="attributes"] th')].map((th) => th.textContent)).toEqual(["Tham số", "Thuộc tính"]);
    expect([...mapping.querySelectorAll('[data-mapping-table="attributes"] td')].map((td) => td.textContent)).toEqual([
      "ngay_tiem",
      "Phân loại hộ",
      "dia_diem",
      "da_xoa",
    ]);
    expect(mapping.querySelector('[data-mapping-row="phone"]')).toBeNull();

    await click(el, "Quay lại danh sách đợt");
    await act(async () => el.querySelector<HTMLButtonElement>('[data-open-campaign="01JS"]')!.click());
    await settle();
    expect(calls.map((c) => c.url)).toContain("/api/v1/notice-attributes?audience=can-bo");
    expect([...el.querySelectorAll('[data-mapping-table="attributes"] td')].map((td) => td.textContent)).toEqual(["Ô 1", "Họ và tên"]);
  });

  it("history: attribute_mapping null → no block, no attribute read", async () => {
    historyPages = { "": { items: [campaign("01JN", "da-gui", { recipient_mapping: null, content: null, sms_content_mode: null, attribute_mapping: null })], next_cursor: "" } };
    const el = await mount(session(ALL));
    await act(async () => tabs(el)[1]!.click());
    await act(async () => el.querySelector<HTMLButtonElement>('[data-open-campaign="01JN"]')!.click());
    await settle();
    expect(el.querySelector("[data-mapping]")).toBeNull();
    expect(calls.some((c) => c.url.startsWith("/api/v1/notice-attributes"))).toBe(false);
  });
});

describe("attributeMappingBody / allowedAttributeChoice (pure)", () => {
  it("only the template's fields, only a picked code; a staff audience keeps full_name only", () => {
    const choice = { a: "address", b: "full_name", c: "", stray: "address" };
    expect(attributeMappingBody(["a", "b", "c"], choice, "citizen_all")).toEqual({ a: "address", b: "full_name" });
    expect(attributeMappingBody(["a", "b", "c"], choice, "staff_unit")).toEqual({ b: "full_name" });
    expect(allowedAttributeChoice(choice, "staff_all")).toEqual({ b: "full_name", c: "" });
    expect(allowedAttributeChoice(choice, "citizen_tag")).toEqual(choice);
  });
});

// ---- "Nạp mẫu từ OA" in step 2 (TASK D-8) ------------------------------------------------------------

describe("Nạp mẫu từ OA — step 2, per OA, admin.zalo only", () => {
  const ZNS_NEW = zns("01JOA1", "UBND xã Tân Phú", "zt-9", "Mời họp dân", ["ngay"]);
  const ZNS_OA2 = zns("01JOA2", "Đoàn thanh niên xã Tân Phú", "zt-2", "Thông báo đoàn", ["ngay"]);
  const syncButton = (el: HTMLElement, oaId: string) => el.querySelector<HTMLButtonElement>(`[data-sync-oa="${oaId}"]`);
  const syncPosts = () => calls.filter((c) => /\/templates\/sync$/.test(c.url) && c.init?.method === "POST");
  const templateReads = () => calls.filter((c) => c.url === "/api/v1/notice-templates");
  async function press(b: HTMLButtonElement | null) {
    expect(b).not.toBeNull();
    await act(async () => b!.click());
    await settle();
    await settle();
  }
  async function openStep(el: HTMLElement, n: number) {
    await act(async () => step(el, n).querySelector("button")!.click());
    await settle();
  }

  it("ALLOWED — admin.zalo: one outline button on the single OA's list header", async () => {
    const el = await mount(session(ALL));
    await toTemplateStep(el);
    const b = syncButton(el, "01JOA1");
    expect(b?.textContent).toBe("Nạp mẫu từ OA");
    expect(step(el, 2).querySelectorAll("[data-sync-oa]")).toHaveLength(1);
  });

  it("DENIED — no admin.zalo: no button anywhere, no zalo-oas route called", async () => {
    const el = await mount(session(["notice.send", "citizen.read"]));
    await toTemplateStep(el);
    expect(el.querySelector("[data-sync-oa]")).toBeNull();
    expect(calls.some((c) => c.url.includes("zalo-oas"))).toBe(false);
  });

  it("click: POST sync for that OA, the reused counts toast, notice-templates read again and the new template drawn", async () => {
    const el = await mount(session(ALL));
    await toTemplateStep(el);
    expect(templateReads()).toHaveLength(1);
    syncReply = () => {
      templatesAnswer = { zns: [ZNS_ONE, ZNS_NEW], sms: [SMS_A] };
      return json(SYNC_RESULT);
    };
    await press(syncButton(el, "01JOA1"));
    expect(syncPosts().map((c) => c.url)).toEqual(["/api/v1/zalo-oas/01JOA1/templates/sync"]);
    expect(toast.success).toHaveBeenCalledWith("Đã nạp 3 mẫu, 2 đang bật, 1 khớp tham số.");
    expect(templateReads()).toHaveLength(2);
    const syncAt = calls.findIndex((c) => c.url.endsWith("/templates/sync"));
    expect(calls.findLastIndex((c) => c.url === "/api/v1/notice-templates")).toBeGreaterThan(syncAt);
    expect(el.querySelector('[data-template-id="zns:01JOA1:zt-9"]')).not.toBeNull();
  });

  it("busy on that button only while the sync is in flight", async () => {
    oaAnswer = { configured: true, items: [OA_ROW, OA_ROW_2] };
    templatesAnswer = { zns: [ZNS_ONE, ZNS_OA2], sms: [] };
    let release: (r: Response) => void = () => {};
    syncReply = () => new Promise<Response>((res) => (release = res));
    const el = await mount(session(ALL));
    await toTemplateStep(el);
    await press(syncButton(el, "01JOA1"));
    expect(syncButton(el, "01JOA1")!.disabled).toBe(true);
    expect(syncButton(el, "01JOA1")!.getAttribute("aria-busy")).toBe("true");
    expect(syncButton(el, "01JOA2")!.disabled).toBe(false);
    await act(async () => release(json(SYNC_RESULT)));
    await settle();
    await settle();
    expect(syncButton(el, "01JOA1")!.disabled).toBe(false);
    expect(syncButton(el, "01JOA1")!.getAttribute("aria-busy")).toBe("false");
  });

  it("the picked template still listed: kept, its typed parameter too", async () => {
    const el = await mount(session(ALL));
    await toTemplateStep(el);
    await pickTemplate(el);
    await type(el, "notice-param-0", "20/10");
    await openStep(el, 2);
    syncReply = () => {
      templatesAnswer = { zns: [ZNS_ONE, ZNS_NEW], sms: [SMS_A] };
      return json(SYNC_RESULT);
    };
    await press(syncButton(el, "01JOA1"));
    expect(el.querySelector(`[data-template-id="${ZNS_KEY}"]`)!.getAttribute("aria-pressed")).toBe("true");
    await openStep(el, 3);
    expect(el.querySelector<HTMLInputElement>("#notice-param-0")!.value).toBe("20/10");
  });

  it("the picked template gone after the sync: the pick, its parameters and the preview cleared", async () => {
    const el = await mount(session(ALL));
    await toTemplateStep(el);
    await pickTemplate(el);
    await type(el, "notice-param-0", "20/10");
    await openStep(el, 2);
    syncReply = () => {
      templatesAnswer = { zns: [ZNS_NEW], sms: [SMS_A] };
      return json(SYNC_RESULT);
    };
    await press(syncButton(el, "01JOA1"));
    expect(el.querySelector(`[data-template-id="${ZNS_KEY}"]`)).toBeNull();
    expect(el.querySelector('[aria-pressed="true"][data-template-id]')).toBeNull();
    expect(aside(el).textContent).toContain(PREVIEW_EMPTY);
    await openStep(el, 3);
    expect(step(el, 3).textContent).toContain(PICK_TEMPLATE_FIRST);
    expect(el.querySelector("#notice-param-0")).toBeNull();
  });

  it("a refusal: the server's sentence toasted, the list not read again", async () => {
    syncReply = () => json({ code: "zalo_oa_needs_reauthorization", message: "OA cần được uỷ quyền lại.", trace_id: "" }, 409);
    const el = await mount(session(ALL));
    await toTemplateStep(el);
    await press(syncButton(el, "01JOA1"));
    expect(toast.error).toHaveBeenCalledWith("OA cần được uỷ quyền lại.");
    expect(toast.success).not.toHaveBeenCalled();
    expect(templateReads()).toHaveLength(1);
    expect(syncButton(el, "01JOA1")!.disabled).toBe(false);
  });

  it("403 forbidden: the screen's own sentence naming the missing right", async () => {
    syncReply = () => json({ code: "forbidden", message: "Tài khoản của bạn không có quyền thực hiện thao tác này.", trace_id: "" }, 403);
    const el = await mount(session(ALL));
    await toTemplateStep(el);
    await press(syncButton(el, "01JOA1"));
    expect(toast.error).toHaveBeenCalledWith(SYNC_FORBIDDEN);
    expect(SYNC_FORBIDDEN).toBe("Bạn không có quyền nạp mẫu (cần quyền Cấu hình kênh Zalo)");
  });

  it("several OAs: each group's button posts for its own OA", async () => {
    oaAnswer = { configured: true, items: [OA_ROW, OA_ROW_2] };
    templatesAnswer = { zns: [ZNS_ONE, ZNS_OA2], sms: [] };
    const el = await mount(session(ALL));
    await toTemplateStep(el);
    const group2 = el.querySelector<HTMLElement>('[data-oa-group="01JOA2"]')!;
    expect(group2.textContent).toContain("Đoàn thanh niên xã Tân Phú");
    await press(group2.querySelector<HTMLButtonElement>("[data-sync-oa]"));
    await press(el.querySelector<HTMLElement>('[data-oa-group="01JOA1"]')!.querySelector<HTMLButtonElement>("[data-sync-oa]"));
    expect(syncPosts().map((c) => c.url)).toEqual(["/api/v1/zalo-oas/01JOA2/templates/sync", "/api/v1/zalo-oas/01JOA1/templates/sync"]);
  });

  it("an OA needing re-authorisation: no button on its group; the other OA keeps its own", async () => {
    oaAnswer = { configured: true, items: [OA_ROW, { ...OA_ROW_2, needs_reauthorization: REAUTH }] };
    templatesAnswer = { zns: [ZNS_ONE, ZNS_REAUTH], sms: [] };
    const el = await mount(session(ALL));
    await toTemplateStep(el);
    expect(syncButton(el, "01JOA2")).toBeNull();
    expect(syncButton(el, "01JOA1")).not.toBeNull();
  });

  it("a linked OA with no template yet: the empty sentence and link stay, its button under them", async () => {
    templatesAnswer = { zns: [], sms: [] };
    const el = await mount(session(ALL));
    await toTemplateStep(el);
    expect(step(el, 2).textContent).toContain(NO_TEMPLATES);
    expect(step(el, 2).querySelector("a")?.getAttribute("href")).toBe("/cau-hinh?tab=kenh-zalo");
    syncReply = () => {
      templatesAnswer = { zns: [ZNS_ONE], sms: [] };
      return json(SYNC_RESULT);
    };
    await press(syncButton(el, "01JOA1"));
    expect(el.querySelector(`[data-template-id="${ZNS_KEY}"]`)).not.toBeNull();
    expect(step(el, 2).textContent).not.toContain(NO_TEMPLATES);
  });

  it("SMS channel: no sync button", async () => {
    const el = await mount(session(ALL));
    await toTemplateStep(el);
    await click(el, "Tin nhắn SMS");
    expect(el.querySelector("[data-sync-oa]")).toBeNull();
  });
});
