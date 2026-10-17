// @vitest-environment jsdom
//
// Cấu hình → Kênh SMS (ADR 0099 #2 #3): the `admin.sms` gate (denied case first), the account cards, the
// add / edit dialog with write-only keys, "Đặt mặc định", "Kiểm tra kết nối", "Ngừng dùng" with a reason,
// and the "Mẫu tin SMS" panel.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import type { PhienDaDoc } from "@/features/phien/phien-hien-tai";
import type { comms_smsAccountOut } from "@/lib/api/sms-accounts";

const H = vi.hoisted(() => ({
  phien: null as PhienDaDoc,
  toast: { success: vi.fn(), error: vi.fn() },
}));
vi.mock("@/features/phien/phien-hien-tai", () => ({ usePhien: () => H.phien }));
vi.mock("sonner", () => ({ toast: H.toast }));

const form = await import("./sms-channel-form");
const { SmsChannelTab } = await import("./sms-channel-tab");

const DEFAULT_ACCOUNT: comms_smsAccountOut = {
  id: "01JSMSA",
  provider: "esms",
  label: "Tài khoản chính",
  brandname: "UBND XA",
  is_default: true,
  set_at: "2026-10-17T01:00:00Z",
  set_by: "CB-00123",
  last_check: { at: "2026-10-17T01:30:00Z", result: "thanh-cong", balance: "36693.00" },
  created_at: "2026-10-17T01:00:00Z",
};
const OTHER_ACCOUNT: comms_smsAccountOut = {
  ...DEFAULT_ACCOUNT,
  id: "01JSMSB",
  label: "Tài khoản dự phòng",
  brandname: "XA THU",
  is_default: false,
  last_check: { at: "2026-10-17T01:40:00Z", result: "sai-khoa", balance: null },
};

const TEMPLATE = {
  content: "UBND xa thong bao: {P1,20} den {P2,30}",
  params: [
    { name: "P1", max_length: 20 },
    { name: "P2", max_length: 30 },
  ],
  networks: [
    { network_id: 1, template_id: "T-100" },
    { network_id: 3, template_id: "T-300" },
  ],
  synced_at: "2026-10-17T02:00:00Z",
};

/** Fake keys: never a real credential (rule 8). */
const FAKE_API_KEY = "fake-api-key-0000";
const FAKE_SECRET_KEY = "fake-secret-key-0000";

function session(permissions: string[]): PhienDaDoc {
  return { ok: true, duLieu: { staff: { full_name: "A", position: "B" }, role: null, permissions } } as unknown as PhienDaDoc;
}

type Call = { method: string; url: string; body?: string; headers: Record<string, string> };
let calls: Call[] = [];
let listNow: { status: number; body: unknown } = { status: 200, body: { items: [] } };
let answers: Record<string, { status: number; body: unknown }[]> = {};

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

beforeEach(() => {
  calls = [];
  listNow = { status: 200, body: { items: [] } };
  answers = {};
  H.toast.success.mockReset();
  H.toast.error.mockReset();
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init: RequestInit) => {
      const method = String(init.method ?? "GET");
      const body = typeof init.body === "string" ? init.body : undefined;
      calls.push({ method, url, body, headers: (init.headers ?? {}) as Record<string, string> });
      const json = (status: number, b: unknown) =>
        new Response(JSON.stringify(b), { status, headers: { "Content-Type": "application/json" } });
      if (method === "GET" && url === "/api/v1/sms-accounts") return json(listNow.status, listNow.body);
      const queue = answers[`${method} ${url}`];
      const a = queue?.length ? (queue.length > 1 ? queue.shift()! : queue[0]!) : undefined;
      return a ? json(a.status, a.body) : json(500, { code: "internal", message: "no stub" });
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
  act(() => r.render(<SmsChannelTab />));
  await settle();
  return host;
}

function button(el: HTMLElement, text: string): HTMLButtonElement {
  const found = Array.from(el.querySelectorAll<HTMLButtonElement>("button")).filter((b) => b.textContent?.trim() === text);
  expect(found.length, `button "${text}"`).toBeGreaterThan(0);
  return found.at(-1)!;
}
function card(el: HTMLElement, id: string): HTMLElement {
  return el.querySelector<HTMLElement>(`[data-sms-account="${id}"]`)!;
}
function dialog(): HTMLElement {
  return document.querySelector<HTMLElement>("form[aria-label$='tài khoản eSMS']")!;
}
function typeInto(el: HTMLInputElement, value: string) {
  const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!;
  act(() => {
    setter.call(el, value);
    el.dispatchEvent(new Event("input", { bubbles: true }));
  });
}
function input(root: HTMLElement, label: string): HTMLInputElement {
  const l = Array.from(root.querySelectorAll("label")).find((x) => x.textContent?.trim() === label)!;
  return root.querySelector<HTMLInputElement>(`#${CSS.escape(l.htmlFor)}`)!;
}
async function submit(f: HTMLElement) {
  act(() => button(f, "Lưu").click());
  await settle();
}
const writes = () => calls.filter((c) => c.method !== "GET");

describe("gate — admin.sms, denied case first", () => {
  it("DENIED: with admin.zalo / admin.mail but not admin.sms, the tab says so and requests NOTHING", async () => {
    H.phien = session(["admin.zalo", "admin.mail", "admin.lookup", "admin.smss", "ADMIN.SMS"]);
    const el = await mount();
    expect(el.textContent).toContain("không có quyền cấu hình kênh SMS");
    expect(calls.some((c) => c.url.startsWith("/api/v1/sms-accounts"))).toBe(false);
  });

  it("session unreadable: the server's sentence, nothing requested", async () => {
    H.phien = { ok: false, thongBao: "Phiên làm việc đã hết hạn." } as PhienDaDoc;
    const el = await mount();
    expect(el.textContent).toContain("Phiên làm việc đã hết hạn.");
    expect(calls).toHaveLength(0);
  });

  it("ALLOWED: admin.sms reads the list", async () => {
    H.phien = session(["admin.sms"]);
    await mount();
    expect(calls.map((c) => `${c.method} ${c.url}`)).toEqual(["GET /api/v1/sms-accounts"]);
  });
});

describe("account list", () => {
  it("empty: the description, the sentence and one 'Thêm tài khoản'", async () => {
    H.phien = session(["admin.sms"]);
    const el = await mount();
    expect(el.textContent).toContain(form.SMS_DESCRIPTION);
    expect(el.querySelector("[data-sms-empty]")!.textContent).toContain(form.NO_ACCOUNTS);
    expect(Array.from(el.querySelectorAll("button")).filter((b) => b.textContent?.trim() === "Thêm tài khoản")).toHaveLength(1);
  });

  it("cards: name, Brandname, 'Mặc định' only on the default, who set the keys, last check with the vi-VN balance", async () => {
    H.phien = session(["admin.sms"]);
    listNow = { status: 200, body: { items: [DEFAULT_ACCOUNT, OTHER_ACCOUNT] } };
    const el = await mount();
    const a = card(el, DEFAULT_ACCOUNT.id);
    const b = card(el, OTHER_ACCOUNT.id);
    expect(a.querySelector("h3")!.textContent).toBe("Tài khoản chính");
    expect(a.querySelector("[data-brandname]")!.textContent).toBe("UBND XA");
    expect(a.querySelector("[data-default-badge]")!.textContent).toBe("Mặc định");
    expect(b.querySelector("[data-default-badge]")).toBeNull();
    expect(a.querySelector("[data-set-by]")!.textContent).toMatch(/^Khoá đặt lúc .+ · bởi CB-00123$/);
    expect(a.querySelector("[data-last-check]")!.textContent).toMatch(/· Kết nối tốt · Số dư 36\.693 đ$/);
    expect(b.querySelector("[data-last-check]")!.textContent).toMatch(/· Sai khoá$/);
    // "Đặt mặc định" only where it changes something.
    expect(a.textContent).not.toContain("Đặt mặc định");
    expect(button(b, "Đặt mặc định")).toBeTruthy();
    for (const label of ["Kiểm tra kết nối", "Sửa", "Ngừng dùng"]) expect(button(a, label)).toBeTruthy();
  });

  it("a failed read: the server's sentence", async () => {
    H.phien = session(["admin.sms"]);
    listNow = { status: 500, body: { code: "internal", message: "Đã xảy ra lỗi. Vui lòng thử lại." } };
    const el = await mount();
    expect(el.textContent).toContain("Chưa tải được tài khoản SMS của xã");
    expect(el.textContent).toContain("Đã xảy ra lỗi. Vui lòng thử lại.");
  });
});

describe("add / edit dialog — write-only keys", () => {
  it("key inputs are password, carry no `name`, autocomplete off; client checks stop an empty form with no request", async () => {
    H.phien = session(["admin.sms"]);
    const el = await mount();
    act(() => button(el, "Thêm tài khoản").click());
    const f = dialog();
    for (const label of ["ApiKey", "SecretKey"]) {
      const i = input(f, label);
      expect(i.type).toBe("password");
      expect(i.getAttribute("name")).toBeNull();
      expect(i.getAttribute("autocomplete")).toBe("off");
      expect(i.value).toBe("");
    }
    // The first account of a commune is offered as the default.
    expect(f.querySelector<HTMLInputElement>("input[type=checkbox]")!.checked).toBe(true);
    await submit(f);
    expect(writes()).toHaveLength(0);
    expect(f.textContent).toContain("Cần nhập tên gợi nhớ.");
    expect(f.textContent).toContain("Tài khoản mới cần nhập cả ApiKey và SecretKey.");
  });

  it("add: POST with the body and an Idempotency-Key; the keys leave the form at once; list re-read", async () => {
    H.phien = session(["admin.sms"]);
    answers["POST /api/v1/sms-accounts"] = [{ status: 201, body: DEFAULT_ACCOUNT }];
    const el = await mount();
    act(() => button(el, "Thêm tài khoản").click());
    const f = dialog();
    typeInto(input(f, "Tên gợi nhớ"), " Tài khoản chính ");
    typeInto(input(f, "ApiKey"), FAKE_API_KEY);
    typeInto(input(f, "SecretKey"), FAKE_SECRET_KEY);
    typeInto(input(f, "Brandname"), "UBND XA");
    listNow = { status: 200, body: { items: [DEFAULT_ACCOUNT] } };
    await submit(f);
    const post = writes()[0]!;
    expect(post.url).toBe("/api/v1/sms-accounts");
    expect(JSON.parse(post.body!)).toEqual({
      label: "Tài khoản chính",
      api_key: FAKE_API_KEY,
      secret_key: FAKE_SECRET_KEY,
      brandname: "UBND XA",
      is_default: true,
    });
    expect(post.headers["Idempotency-Key"]).toMatch(/^[0-9a-f-]{36}$/);
    expect(H.toast.success).toHaveBeenCalledWith(form.SAVED_TOAST);
    expect(dialog()).toBeNull();
    expect(card(el, DEFAULT_ACCOUNT.id)).not.toBeNull();
    // The keys are nowhere in the page.
    expect(document.body.innerHTML).not.toContain(FAKE_API_KEY);
    expect(document.body.innerHTML).not.toContain(FAKE_SECRET_KEY);
  });

  it("422 sms_key_rejected: the sentence under the keys, the boxes emptied, the retry keeps the SAME Idempotency-Key", async () => {
    H.phien = session(["admin.sms"]);
    const sentence = "eSMS không nhận ApiKey / SecretKey này. Hãy chép lại khoá trong trang quản trị eSMS. Chưa có gì được lưu.";
    answers["POST /api/v1/sms-accounts"] = [{ status: 422, body: { code: "sms_key_rejected", message: sentence } }];
    const el = await mount();
    act(() => button(el, "Thêm tài khoản").click());
    const f = dialog();
    typeInto(input(f, "Tên gợi nhớ"), "A");
    typeInto(input(f, "ApiKey"), FAKE_API_KEY);
    typeInto(input(f, "SecretKey"), FAKE_SECRET_KEY);
    typeInto(input(f, "Brandname"), "UBND XA");
    await submit(f);
    const keyField = input(f, "ApiKey").parentElement!;
    expect(keyField.querySelector("[data-field-error]")!.textContent).toBe(sentence);
    expect(input(f, "ApiKey").value).toBe("");
    expect(input(f, "SecretKey").value).toBe("");
    expect(f.querySelector("[data-key-hint]")!.textContent).toContain(form.KEY_RETYPE_HINT);
    expect(input(f, "Tên gợi nhớ").value).toBe("A");
    typeInto(input(f, "ApiKey"), FAKE_API_KEY);
    typeInto(input(f, "SecretKey"), FAKE_SECRET_KEY);
    await submit(f);
    const posts = writes();
    expect(posts).toHaveLength(2);
    expect(posts[1]!.headers["Idempotency-Key"]).toBe(posts[0]!.headers["Idempotency-Key"]);
  });

  it("409 sms_label_taken: under 'Tên gợi nhớ'", async () => {
    H.phien = session(["admin.sms"]);
    const sentence = "Xã đã có một tài khoản SMS đang dùng mang tên này. Hãy chọn tên khác.";
    answers["POST /api/v1/sms-accounts"] = [{ status: 409, body: { code: "sms_label_taken", message: sentence } }];
    const el = await mount();
    act(() => button(el, "Thêm tài khoản").click());
    const f = dialog();
    typeInto(input(f, "Tên gợi nhớ"), "A");
    typeInto(input(f, "ApiKey"), FAKE_API_KEY);
    typeInto(input(f, "SecretKey"), FAKE_SECRET_KEY);
    typeInto(input(f, "Brandname"), "B");
    await submit(f);
    expect(input(f, "Tên gợi nhớ").parentElement!.querySelector("[data-field-error]")!.textContent).toBe(sentence);
  });

  it("503 encryption_not_configured: the server's sentence, 'Lưu' disabled", async () => {
    H.phien = session(["admin.sms"]);
    const sentence = "Nền tảng chưa cấu hình khoá mã hoá bí mật (SECRET_ENCRYPTION_KEYS). Chưa có gì được ghi.";
    answers["POST /api/v1/sms-accounts"] = [{ status: 503, body: { code: "encryption_not_configured", message: sentence } }];
    const el = await mount();
    act(() => button(el, "Thêm tài khoản").click());
    const f = dialog();
    typeInto(input(f, "Tên gợi nhớ"), "A");
    typeInto(input(f, "ApiKey"), FAKE_API_KEY);
    typeInto(input(f, "SecretKey"), FAKE_SECRET_KEY);
    typeInto(input(f, "Brandname"), "B");
    await submit(f);
    expect(f.querySelector("[data-encryption-missing]")!.textContent).toBe(sentence);
    expect(button(f, "Lưu").disabled).toBe(true);
    expect(el.querySelector("[data-encryption-missing]")!.textContent).toBe(sentence);
  });

  it("edit: keys start empty; empty keys are kept (not sent); only what changed travels", async () => {
    H.phien = session(["admin.sms"]);
    listNow = { status: 200, body: { items: [DEFAULT_ACCOUNT] } };
    answers[`PATCH /api/v1/sms-accounts/${DEFAULT_ACCOUNT.id}`] = [{ status: 200, body: DEFAULT_ACCOUNT }];
    const el = await mount();
    act(() => button(card(el, DEFAULT_ACCOUNT.id), "Sửa").click());
    const f = dialog();
    expect(input(f, "Tên gợi nhớ").value).toBe("Tài khoản chính");
    expect(input(f, "ApiKey").value).toBe("");
    expect(f.querySelector("[data-key-hint]")!.textContent).toBe(form.KEY_KEPT_HINT);
    typeInto(input(f, "Brandname"), "UBND MOI");
    await submit(f);
    const patch = writes()[0]!;
    expect(patch.method).toBe("PATCH");
    expect(JSON.parse(patch.body!)).toEqual({ brandname: "UBND MOI" });
    expect(patch.headers["Idempotency-Key"]).toBeUndefined();
  });

  it("edit: one key without the other is stopped here, no request", async () => {
    H.phien = session(["admin.sms"]);
    listNow = { status: 200, body: { items: [DEFAULT_ACCOUNT] } };
    const el = await mount();
    act(() => button(card(el, DEFAULT_ACCOUNT.id), "Sửa").click());
    const f = dialog();
    typeInto(input(f, "ApiKey"), FAKE_API_KEY);
    await submit(f);
    expect(writes()).toHaveLength(0);
    expect(f.textContent).toContain("Đổi khoá thì nhập cả ApiKey và SecretKey.");
  });
});

describe("card actions", () => {
  it("'Đặt mặc định': PATCH {is_default:true}, toast, list re-read", async () => {
    H.phien = session(["admin.sms"]);
    listNow = { status: 200, body: { items: [DEFAULT_ACCOUNT, OTHER_ACCOUNT] } };
    answers[`PATCH /api/v1/sms-accounts/${OTHER_ACCOUNT.id}`] = [{ status: 200, body: { ...OTHER_ACCOUNT, is_default: true } }];
    const el = await mount();
    act(() => button(card(el, OTHER_ACCOUNT.id), "Đặt mặc định").click());
    await settle();
    expect(JSON.parse(writes()[0]!.body!)).toEqual({ is_default: true });
    expect(H.toast.success).toHaveBeenCalledWith(form.DEFAULT_SET_TOAST);
    expect(calls.filter((c) => c.method === "GET" && c.url === "/api/v1/sms-accounts")).toHaveLength(2);
  });

  it("'Kiểm tra kết nối': POST check, success toast with the balance; a failing class is an error toast", async () => {
    H.phien = session(["admin.sms"]);
    listNow = { status: 200, body: { items: [DEFAULT_ACCOUNT, OTHER_ACCOUNT] } };
    answers[`POST /api/v1/sms-accounts/${DEFAULT_ACCOUNT.id}/check`] = [
      { status: 200, body: { result: "thanh-cong", checked_at: "2026-10-17T03:00:00Z", balance: "1200000.50" } },
    ];
    answers[`POST /api/v1/sms-accounts/${OTHER_ACCOUNT.id}/check`] = [
      { status: 200, body: { result: "brandname-chua-kich-hoat", checked_at: "2026-10-17T03:00:00Z", balance: null } },
    ];
    const el = await mount();
    act(() => button(card(el, DEFAULT_ACCOUNT.id), "Kiểm tra kết nối").click());
    await settle();
    expect(H.toast.success).toHaveBeenCalledWith(expect.stringContaining("Số dư: 1.200.000,5 đ."));
    act(() => button(card(el, OTHER_ACCOUNT.id), "Kiểm tra kết nối").click());
    await settle();
    expect(H.toast.error).toHaveBeenCalledWith(expect.stringContaining("Brandname chưa được kích hoạt"));
  });

  it("'Ngừng dùng': no reason → stopped here; with a reason → DELETE {reason}", async () => {
    H.phien = session(["admin.sms"]);
    listNow = { status: 200, body: { items: [DEFAULT_ACCOUNT] } };
    answers[`DELETE /api/v1/sms-accounts/${DEFAULT_ACCOUNT.id}`] = [{ status: 200, body: { retired: true } }];
    const el = await mount();
    act(() => button(card(el, DEFAULT_ACCOUNT.id), "Ngừng dùng").click());
    const f = document.querySelector<HTMLElement>("form[aria-label='Ngừng dùng tài khoản SMS']")!;
    expect(f.querySelector("[data-retire-consequence]")!.textContent).toBe(form.RETIRE_CONSEQUENCE);
    act(() => button(f, "Ngừng dùng").click());
    await settle();
    expect(writes()).toHaveLength(0);
    expect(f.textContent).toContain("Cần nhập lý do ngừng dùng");
    typeInto(input(f, "Lý do ngừng dùng"), "  Hết hợp đồng với eSMS  ");
    act(() => button(f, "Ngừng dùng").click());
    await settle();
    expect(writes()[0]!.method).toBe("DELETE");
    expect(JSON.parse(writes()[0]!.body!)).toEqual({ reason: "Hết hợp đồng với eSMS" });
    expect(H.toast.success).toHaveBeenCalledWith(form.RETIRED_TOAST);
  });
});

describe("Mẫu tin SMS", () => {
  it("collapsed: nothing read; opened: the table with 'P1 ≤20' and network names; sync toasts the four counts", async () => {
    H.phien = session(["admin.sms"]);
    listNow = { status: 200, body: { items: [DEFAULT_ACCOUNT] } };
    answers[`GET /api/v1/sms-accounts/${DEFAULT_ACCOUNT.id}/templates`] = [{ status: 200, body: { items: [] } }];
    answers[`POST /api/v1/sms-accounts/${DEFAULT_ACCOUNT.id}/templates/sync`] = [
      { status: 200, body: { added: 1, updated: 0, vanished: 0, skipped: 2, items: [TEMPLATE] } },
    ];
    const el = await mount();
    expect(calls.some((c) => c.url.endsWith("/templates"))).toBe(false);
    const c = card(el, DEFAULT_ACCOUNT.id);
    const toggle = button(c, "Mẫu tin SMS");
    expect(toggle.getAttribute("aria-expanded")).toBe("false");
    act(() => toggle.click());
    await settle();
    expect(toggle.getAttribute("aria-expanded")).toBe("true");
    expect(c.querySelector("[data-templates-empty]")!.textContent).toBe(form.NO_TEMPLATES);
    act(() => button(c, "Nạp mẫu từ eSMS").click());
    await settle();
    expect(H.toast.success).toHaveBeenCalledWith("Đã nạp mẫu: thêm 1, cập nhật 0, không còn trên eSMS 0, bỏ qua 2 mẫu không lưu được.");
    const row = c.querySelector<HTMLElement>("[data-sms-template]")!;
    expect(row.textContent).toContain("UBND xa thong bao: {P1,20} den {P2,30}");
    expect(row.querySelector("[data-params]")!.textContent).toBe("P1 ≤20, P2 ≤30");
    expect(row.querySelector("[data-networks]")!.textContent).toBe("Viettel · T-100Vinaphone · T-300");
    expect(c.querySelector("[data-synced-at]")!.textContent).toMatch(/^Nạp lúc /);
  });
});

describe("pure helpers", () => {
  it("formatBalance: vi-VN grouping on the digits, never a float", () => {
    expect(form.formatBalance("36693.00")).toBe("36.693 đ");
    expect(form.formatBalance("0")).toBe("0 đ");
    expect(form.formatBalance("1200000.50")).toBe("1.200.000,5 đ");
    expect(form.formatBalance("123456789012345678.10")).toBe("123.456.789.012.345.678,1 đ");
    expect(form.formatBalance("abc")).toBe("abc đ");
  });

  it("an unknown class is never a success; toString is not a sentence; an unknown network is named by id", () => {
    expect(form.checkResultLabel("toString")).toBe("Không rõ kết quả");
    expect(form.isCheckOk("toString")).toBe(false);
    expect(form.networkLabel(7)).toBe("Reddi");
    expect(form.networkLabel(9)).toBe("Nhà mạng 9");
  });

  it("refusalField goes by CODE; sms_account_invalid stays above the buttons", () => {
    expect(form.refusalField("sms_label_taken")).toBe("label");
    expect(form.refusalField("sms_key_rejected")).toBe("keys");
    expect(form.refusalField("sms_brandname_inactive")).toBe("brandname");
    expect(form.refusalField("sms_account_invalid")).toBe("form");
    expect(form.refusalField(undefined)).toBe("form");
  });
});
