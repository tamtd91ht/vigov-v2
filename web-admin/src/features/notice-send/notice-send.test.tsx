// @vitest-environment jsdom
//
// jsdom: the screen is driven for real — the session is faked, the network is a router over `fetch`,
// and what is checked is what the screen CALLS and what it SHOWS, state by state.

import type { AnchorHTMLAttributes } from "react";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import type { PhienDaDoc } from "@/features/phien/phien-hien-tai";

import {
  countStaff,
  COUNT_OVERLAP,
  HISTORY_EMPTY,
  missingParams,
  NO_CITIZEN_READ,
  NO_SEND_PERMISSION,
  NO_TEMPLATE_PERMISSION,
  NO_TEMPLATES,
  NOTICE_SEND_TITLE,
  OA_NOT_LINKED,
  PREVIEW_EMPTY,
  recipientSource,
  SEND_NOTE,
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

const OA_ROW = {
  oa_id: "1234567890",
  oa_name: "UBND xã Tân Phú",
  authorized_at: "2026-10-16T01:00:00Z",
  authorized_by: "CB-00001",
  authorized_by_name: "Trần Văn Quản",
  access_expires_at: "2026-10-17T01:00:00Z",
  token_refreshed_at: "2026-10-16T01:00:00Z",
  needs_reauthorization: null as null | { since: string; reason: string },
  last_check: null,
};

type OAAnswer = { configured: boolean; has_oa: boolean; oa: typeof OA_ROW | null };

const LINKED: OAAnswer = { configured: true, has_oa: true, oa: OA_ROW };
const NOT_LINKED: OAAnswer = { configured: true, has_oa: false, oa: null };

const template = (id: string, name: string, status: string, params: string[]) => ({
  id,
  template_id: `zt-${id}`,
  template_name: name,
  zalo_status: status,
  zalo_quality: "HIGH",
  zalo_tag: "TRANSACTION",
  params: params.map((n) => ({ name: n, required: true, type: "STRING", max_length: 30 })),
  params_match: true,
  missing_params: [],
  milestone: null,
  milestone_set_at: null,
  milestone_set_by: null,
  milestone_set_by_name: null,
});

const TEMPLATES = [
  template("01JT1", "Thông báo lịch tiêm", "ENABLE", ["ngay_tiem", "dia_diem"]),
  template("01JT2", "Mẫu bị từ chối", "REJECT", ["x"]),
];

type Call = { url: string; init: RequestInit | undefined };
const calls: Call[] = [];
const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });

let oaAnswer: OAAnswer = LINKED;
let templateItems: unknown[] = TEMPLATES;

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
      if (url === "/api/v1/zalo-oas/current") return json({ ...oaAnswer, template_count: 2, attached_count: 0 });
      if (url === "/api/v1/zalo-oas/current/templates") return json({ items: templateItems, synced_at: "2026-10-16T01:00:00Z" });
      if (url === "/api/v1/zalo-oas/current/authorization-links") {
        return json({ url: "https://oauth.zaloapp.com/v4/oa/permission?state=SECRET-STATE", expires_at: "2026-10-17T01:10:00Z" }, 201);
      }
      return json({ code: "not_found", message: "Không có tuyến này.", trace_id: "" }, 404);
    }),
  );
}

let root: Root | null = null;
let host: HTMLDivElement | null = null;

beforeEach(() => {
  calls.length = 0;
  oaAnswer = LINKED;
  templateItems = TEMPLATES;
  serve();
});

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  vi.unstubAllGlobals();
  vi.clearAllMocks();
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

async function toTemplateStep(el: HTMLElement) {
  await click(el, "Tiếp tục");
}
async function pickFirstTemplate(el: HTMLElement) {
  await act(async () => el.querySelector<HTMLButtonElement>('[data-template-id="01JT1"]')!.click());
  await settle();
}

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

  it("ALLOWED with notice.send only: the screen, and each part needing another key says so — no certain 403 is called", async () => {
    const el = await mount(session(["notice.send"]));
    expect(el.querySelector("h1")?.textContent).toBe(NOTICE_SEND_TITLE);
    expect([...el.querySelectorAll('[role="tab"]')].map((t) => t.textContent)).toEqual(["Soạn tin", "Đã gửi"]);
    // OA card: the way to Cấu hình, nothing red.
    const card = el.querySelector('[data-oa-card="no-permission"]')!;
    expect(card.querySelector("a")?.getAttribute("href")).toBe("/cau-hinh?tab=kenh-zalo");
    expect(card.querySelector('[role="alert"]')).toBeNull();
    // Groups need citizen.read.
    await click(el, "Người dân theo nhóm");
    expect(el.textContent).toContain(NO_CITIZEN_READ);
    // Prototype: "Tiếp tục" is never locked.
    expect(button(el, "Tiếp tục")!.disabled).toBe(false);
    await click(el, "Toàn bộ người dân");
    await toTemplateStep(el);
    expect(step(el, 2).textContent).toContain(NO_TEMPLATE_PERMISSION);
    expect(calls.filter((c) => c.url.includes("zalo-oas") || c.url.includes("citizen"))).toHaveLength(0);
  });
});

describe("the three steps open one after another (prototype StepCard)", () => {
  it("1 → Tiếp tục → 2 → pick a template → 3, with the template's name as the campaign title and one field per parameter", async () => {
    const el = await mount(session(ALL));
    expect(stepOpen(el, 1)).toBe(true);
    expect(stepOpen(el, 2)).toBe(false);
    expect(stepOpen(el, 3)).toBe(false);
    expect(el.querySelector("aside")!.textContent).toContain(PREVIEW_EMPTY);

    await toTemplateStep(el);
    expect(stepOpen(el, 1)).toBe(false);
    expect(step(el, 1).textContent).toContain("Toàn bộ người dân");
    expect(stepOpen(el, 2)).toBe(true);
    expect(step(el, 2).textContent).toContain("Zalo ZNS · Đang bật");
    expect(step(el, 2).textContent).toContain("Tham số: ngay_tiem, dia_diem");

    await pickFirstTemplate(el);
    expect(stepOpen(el, 2)).toBe(false);
    expect(stepOpen(el, 3)).toBe(true);
    expect(el.querySelector<HTMLInputElement>("#notice-title")!.value).toBe("Thông báo lịch tiêm");
    expect([...step(el, 3).querySelectorAll("label")].map((l) => l.textContent)).toEqual(["Tên đợt gửi", "ngay_tiem", "dia_diem"]);
    // The aside: the parameters still empty, computed here.
    expect(el.querySelector("[data-missing-params]")!.textContent).toBe("Còn chỗ trống chưa điền: ngay_tiem, dia_diem.");
  });

  it("a template that is not ENABLE is drawn disabled with its status, and pressing it picks nothing", async () => {
    const el = await mount(session(ALL));
    await toTemplateStep(el);
    const rejected = el.querySelector<HTMLButtonElement>('[data-template-id="01JT2"]')!;
    expect(rejected.disabled).toBe(true);
    expect(rejected.textContent).toContain("Bị từ chối");
    await act(async () => rejected.click());
    expect(stepOpen(el, 3)).toBe(false);
    expect(el.querySelector("aside")!.textContent).toContain(PREVIEW_EMPTY);
  });

  it("no template on the OA: the prototype's sentence and the way to Cấu hình → Kênh Zalo", async () => {
    templateItems = [];
    const el = await mount(session(ALL));
    await toTemplateStep(el);
    expect(step(el, 2).textContent).toContain(NO_TEMPLATES);
    expect(step(el, 2).querySelector("a")?.getAttribute("href")).toBe("/cau-hinh?tab=kenh-zalo");
  });

  it("SMS: no list, the '?' block (eSMS next round), no route called for it", async () => {
    const el = await mount(session(ALL));
    await toTemplateStep(el);
    const before = calls.length;
    await click(el, "Tin nhắn SMS");
    expect(step(el, 2).querySelector("[data-sms-pending] [data-pending-marker]")).not.toBeNull();
    expect(step(el, 2).querySelector("[data-template-id]")).toBeNull();
    expect(calls.length).toBe(before);
  });
});

describe("the recipient count — real where the server can count it, '?' where it cannot", () => {
  it("ONE group: the count route with that group, the real figure; TWO groups: '?' with the overlap reason", async () => {
    const el = await mount(session(ALL));
    await click(el, "Người dân theo nhóm");
    expect(button(el, "Tiếp tục")!.disabled).toBe(false);
    await click(el, "Tổ trưởng (12)");
    await toTemplateStep(el);
    await pickFirstTemplate(el);
    const count = calls.filter((c) => c.url === "/api/v1/citizen-contact-count-queries").at(-1)!;
    expect(JSON.parse(String(count.init?.body))).toMatchObject({ group_id: "01JG1" });
    expect(recipientsRow(el).textContent).toContain("11 người");

    // Back to step 1, add a second group.
    await act(async () => step(el, 1).querySelector("button")!.click());
    await click(el, "Hộ kinh doanh (30)");
    const marker = recipientsRow(el).querySelector("[data-pending-marker]");
    expect(marker).not.toBeNull();
    expect(recipientsRow(el).textContent).not.toContain("người");
    expect(recipientSource("citizen_group", ["a", "b"], [], [], true)).toEqual({ type: "unknown", reason: COUNT_OVERLAP });
  });

  it("ONE tag: the tag strip's own count; staff by unit: the directory filtered on the ONE unit each person has", async () => {
    const el = await mount(session(ALL));
    await click(el, "Người dân theo nhãn");
    await click(el, "cựu chiến binh (7)");
    await toTemplateStep(el);
    await pickFirstTemplate(el);
    expect(recipientsRow(el).textContent).toContain("7 người");

    await act(async () => step(el, 1).querySelector("button")!.click());
    await click(el, "Cán bộ theo bộ phận");
    await click(el, "Văn phòng");
    await toTemplateStep(el);
    await pickFirstTemplate(el);
    expect(recipientsRow(el).textContent).toContain("2 người");
    expect(countStaff([{ department_id: "u1" }, { department_id: "" }], null)).toBe(2);
  });

  it("all citizens without citizen.read: '?' with the reason, the count route NOT called", () => {
    const s = recipientSource("citizen_all", [], [], [], false);
    expect(s.type).toBe("unknown");
    expect(missingParams(["a", "b"], { a: " x ", b: "  " })).toEqual(["b"]);
  });
});

describe("rule 3 — nothing personal on this screen", () => {
  it("no phone number, no recipient name, no reveal route — at any step", async () => {
    const el = await mount(session(ALL));
    await click(el, "Cán bộ theo bộ phận");
    await click(el, "Văn phòng");
    await toTemplateStep(el);
    await pickFirstTemplate(el);
    const text = el.textContent ?? "";
    expect(text).not.toMatch(/0\d{9}|\d{2}\*{4}\d{4}/);
    expect(text).not.toContain("Ví dụ người nhận");
    for (const name of ["CB-1", "CB-2"]) expect(text).not.toContain(name);
    expect(calls.find((c) => c.url.endsWith("/phone") || c.url.endsWith("/email"))).toBeUndefined();
  });
});

describe("the send button is a '?' — no route exists", () => {
  it("disabled, the '?' marker beside it, the true sentence under it; pressing it sends nothing", async () => {
    const el = await mount(session(ALL));
    await toTemplateStep(el);
    await pickFirstTemplate(el);
    const send = el.querySelector<HTMLElement>("[data-send]")!;
    const b = send.querySelector<HTMLButtonElement>("button:not([data-pending-marker])")!;
    expect(b.textContent).toBe("Gửi thông báo");
    expect(b.disabled).toBe(true);
    expect(send.querySelector("[data-pending-marker]")).not.toBeNull();
    expect(el.textContent).toContain(SEND_NOTE);
    expect(el.textContent).not.toContain("Đường gửi sang eSMS chưa nối");
    const before = calls.length;
    await act(async () => b.click());
    expect(calls.length).toBe(before);
    expect(calls.filter((c) => c.init?.method === "POST").map((c) => c.url)).toEqual(
      calls.filter((c) => c.url === "/api/v1/citizen-contact-count-queries").map((c) => c.url),
    );
  });

  it("the Đã gửi tab: no invented count, the prototype's empty sentence", async () => {
    const el = await mount(session(ALL));
    await act(async () => [...el.querySelectorAll<HTMLButtonElement>('[role="tab"]')][1]!.click());
    expect(el.textContent).toContain(HISTORY_EMPTY);
    expect([...el.querySelectorAll('[role="tab"]')][1]!.textContent).toBe("Đã gửi");
  });
});

describe("the OA card (ADR 0098)", () => {
  it("linked: which OA sends, and the way to manage it", async () => {
    const el = await mount(session(ALL));
    const card = el.querySelector('[data-oa-card="linked"]')!;
    expect(card.textContent).toContain("Gửi từ OA: UBND xã Tân Phú · nối lúc");
    expect(card.querySelector("a")?.textContent).toBe("Quản lý OA");
  });

  it("not linked: the sentence, Kết nối OA opens the link in a new tab and never puts it in the DOM; step 2 shows the card, no templates read", async () => {
    oaAnswer = NOT_LINKED;
    const open = vi.fn(() => null);
    vi.stubGlobal("open", open);
    const el = await mount(session(ALL));
    expect(el.querySelector('[data-oa-card="not-linked"]')!.textContent).toContain(OA_NOT_LINKED);
    await toTemplateStep(el);
    expect(step(el, 2).querySelector('[data-oa-card="not-linked"]')).not.toBeNull();
    expect(calls.find((c) => c.url === "/api/v1/zalo-oas/current/templates")).toBeUndefined();

    const connect = el.querySelector<HTMLButtonElement>('[data-oa-card="not-linked"] button')!;
    expect(connect.textContent).toBe("Kết nối OA");
    await act(async () => connect.click());
    await settle();
    expect(calls.find((c) => c.url === "/api/v1/zalo-oas/current/authorization-links")?.init?.method).toBe("POST");
    expect(open).toHaveBeenCalledWith(expect.stringContaining("oauth.zaloapp.com"), "_blank", "noopener");
    expect(el.innerHTML).not.toContain("SECRET-STATE");
  });

  it("needs re-authorisation: the warning and Uỷ quyền lại", async () => {
    oaAnswer = { ...LINKED, oa: { ...OA_ROW, needs_reauthorization: { since: "2026-10-16T01:00:00Z", reason: "refresh-token-het-han" } } };
    const el = await mount(session(ALL));
    const card = el.querySelector('[data-oa-card="reauthorize"]')!;
    expect(card.querySelector('[role="alert"]')!.textContent).toContain("quá 3 tháng");
    expect(card.querySelector("button")!.textContent).toBe("Uỷ quyền lại");
  });

  it("platform app not declared (configured=false): the platform sentence, no connect button", async () => {
    oaAnswer = { configured: false, has_oa: false, oa: null };
    const el = await mount(session(ALL));
    const card = el.querySelector('[data-oa-card="not-configured"]')!;
    expect(card.textContent).toContain("Nền tảng chưa khai ứng dụng Zalo dùng chung");
    expect(card.querySelector("button")).toBeNull();
  });
});
