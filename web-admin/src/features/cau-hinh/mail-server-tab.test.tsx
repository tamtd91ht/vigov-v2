// @vitest-environment jsdom
//
// jsdom: the tab is gated on the session, loads once allowed, and the security boxes, the save and the
// test send are behaviour — clicked, not read from markup.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import type { PhienDaDoc } from "@/features/phien/phien-hien-tai";
import type { comms_mailSettingsOut } from "@/lib/api/schema.gen";

const H = vi.hoisted(() => ({ phien: null as PhienDaDoc }));
vi.mock("@/features/phien/phien-hien-tai", () => ({ usePhien: () => H.phien }));
vi.mock("@/components/cau-hinh-xa", () => ({
  useCauHinhXa: () => ({ displayName: "UBND xã Tân Phú", parentAuthority: "", logoUrl: "", webAdminBannerUrl: "" }),
}));
const T = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));
vi.mock("sonner", () => ({ toast: T }));

const { MailServerTab } = await import("./mail-server-tab");

const SAVED: comms_mailSettingsOut = {
  configured: true,
  host: "smtp.xa.gov.vn",
  port: 587,
  security: "starttls",
  username: "ubnd@xa.gov.vn",
  from_address: "ubnd@xa.gov.vn",
  from_name: "UBND xã",
  is_enabled: true,
  password_set: true,
  encryption_configured: true,
  last_test: null,
};

function session(permissions: string[]): PhienDaDoc {
  return { ok: true, duLieu: { staff: { full_name: "A", position: "B" }, role: null, permissions } } as unknown as PhienDaDoc;
}

type Reply = { status: number; body: unknown } | "network";
let calls: { method: string; url: string; body?: string }[] = [];
let putReply: Reply = { status: 200, body: SAVED };
let testReply: Reply = { status: 200, body: { sent: true } };
/** What GET answers; a test send may change it (the server stores the result). */
let getReply: { status: number; body: unknown } = { status: 200, body: SAVED };

const json = (r: { status: number; body: unknown }) =>
  new Response(JSON.stringify(r.body), { status: r.status, headers: { "Content-Type": "application/json" } });

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

beforeEach(() => {
  calls = [];
  putReply = { status: 200, body: SAVED };
  testReply = { status: 200, body: { sent: true } };
  getReply = { status: 200, body: SAVED };
  T.success.mockReset();
  T.error.mockReset();
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init: RequestInit = {}) => {
      const method = String(init.method ?? "GET");
      calls.push({ method, url, body: typeof init.body === "string" ? init.body : undefined });
      const reply = url.endsWith("/test-messages") ? testReply : method === "PUT" ? putReply : getReply;
      if (reply === "network") throw new TypeError("Failed to fetch");
      return json(reply);
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
  act(() => r.render(<MailServerTab />));
  await settle();
  return host;
}

const box = (el: HTMLElement, name: string) => el.querySelector<HTMLInputElement>(`input[name="${name}"]`)!;

/** Types into a React-controlled input (the native setter, then the event React listens to). */
function type(input: HTMLInputElement, value: string) {
  const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!;
  act(() => {
    setter.call(input, value);
    input.dispatchEvent(new Event("input", { bubbles: true }));
  });
}

function click(el: HTMLElement) {
  act(() => el.click());
}

async function submitSave(el: HTMLElement) {
  click(el.querySelector<HTMLButtonElement>('button[form="form-may-chu-thu"]')!);
  await settle();
}

describe("denied case", () => {
  it("without admin.lookup: the tab's own sentence, and NOTHING is fetched", async () => {
    H.phien = session(["task.read"]);
    const el = await mount();
    expect(el.textContent).toContain("Tài khoản của bạn không có quyền cấu hình máy chủ thư");
    expect(el.querySelector("form")).toBeNull();
    expect(calls).toEqual([]);
  });
});

describe("security boxes — one choice, never neither (rule 13)", () => {
  it("ticking TLS unticks STARTTLS; un-ticking the ticked one is not possible; the PUT carries one value", async () => {
    H.phien = session(["admin.lookup"]);
    const el = await mount();
    expect(box(el, "security-starttls").checked).toBe(true);
    expect(box(el, "security-tls").checked).toBe(false);

    click(box(el, "security-tls"));
    expect(box(el, "security-tls").checked).toBe(true);
    expect(box(el, "security-starttls").checked).toBe(false);

    // Clicking the ticked box again leaves it ticked: there is no "neither" (plaintext) state.
    click(box(el, "security-tls"));
    expect(box(el, "security-tls").checked).toBe(true);
    expect(box(el, "security-starttls").checked).toBe(false);

    await submitSave(el);
    const put = calls.find((c) => c.method === "PUT")!;
    expect(JSON.parse(put.body!)).toMatchObject({ security: "tls" });
    expect(JSON.parse(put.body!)).not.toHaveProperty("password");
  });
});

describe("save", () => {
  it("success → toast 'Đã lưu cấu hình máy chủ thư.', nothing in place", async () => {
    H.phien = session(["admin.lookup"]);
    const el = await mount();
    await submitSave(el);
    expect(T.success).toHaveBeenCalledWith("Đã lưu cấu hình máy chủ thư.");
    expect(el.querySelector('[role="alert"]')).toBeNull();
  });

  it("400 password_required_for_new_host → §10's sentence IN PLACE, no toast", async () => {
    H.phien = session(["admin.lookup"]);
    putReply = {
      status: 400,
      body: { code: "password_required_for_new_host", message: "Đã đổi máy chủ… (câu của máy chủ)", trace_id: "t" },
    };
    const el = await mount();
    type(box(el, "host"), "smtp.khac.gov.vn");
    await submitSave(el);
    const alert = el.querySelector('[role="alert"]');
    expect(alert?.textContent).toBe(
      "Đổi máy chủ, cổng hoặc tài khoản thì phải gõ lại mật khẩu. Mật khẩu cũ không đi theo sang máy chủ mới.",
    );
    expect(T.success).not.toHaveBeenCalled();
    expect(T.error).not.toHaveBeenCalled();
  });

  it("any other refusal → the server's own sentence in place", async () => {
    H.phien = session(["admin.lookup"]);
    putReply = { status: 400, body: { code: "invalid_request", message: "Cổng phải là 587, 465, 25 hoặc 2525.", trace_id: "t" } };
    const el = await mount();
    await submitSave(el);
    expect(el.querySelector('[role="alert"]')?.textContent).toBe("Cổng phải là 587, 465, 25 hoặc 2525.");
  });
});

describe("test send", () => {
  async function sendTo(el: HTMLElement, to: string) {
    type(box(el, "recipient"), to);
    const button = [...el.querySelectorAll("button")].find((b) => b.textContent === "Gửi thử")!;
    expect(button.disabled).toBe(false);
    click(button);
    await settle();
  }

  it("ok → toast 'Đã gửi. Kiểm tra hộp thư.' and a leaf line", async () => {
    H.phien = session(["admin.lookup"]);
    const el = await mount();
    await sendTo(el, "a@b.vn");
    expect(T.success).toHaveBeenCalledWith("Đã gửi. Kiểm tra hộp thư.");
    expect(el.querySelector('[role="status"].text-leaf')).not.toBeNull();
  });

  it("refused by the commune's server → toast 'Không gửi được: {error}' and a danger line", async () => {
    H.phien = session(["admin.lookup"]);
    testReply = { status: 502, body: { code: "smtp_auth", message: "Máy chủ thư từ chối đăng nhập.", trace_id: "t" } };
    const el = await mount();
    await sendTo(el, "a@b.vn");
    expect(T.error).toHaveBeenCalledWith("Không gửi được: Máy chủ thư từ chối đăng nhập.");
    expect(el.querySelector('[role="alert"].text-danger')?.textContent).toBe("Máy chủ thư từ chối đăng nhập.");
  });

  it("after a send the settings are READ AGAIN: the stored last test replaces the session line", async () => {
    // Failed before: no GET followed the send, and the stored line was a "?".
    H.phien = session(["admin.lookup"]);
    const el = await mount();
    getReply = {
      status: 200,
      body: { ...SAVED, last_test: { at: "2026-10-08T02:05:00Z", to: "a***@b.vn", ok: false, error_class: "het-thoi-gian" } },
    };
    testReply = { status: 502, body: { code: "mail_timeout", message: "Máy chủ thư không trả lời kịp.", trace_id: "t" } };
    type(box(el, "host"), "smtp.dang-go.gov.vn");
    await sendTo(el, "a@b.vn");
    expect(calls.filter((c) => c.method === "GET")).toHaveLength(2);
    expect(el.textContent).toContain("Lần thử gần nhất 09:05 08/10/2026 tới a***@b.vn: máy chủ thư không trả lời kịp");
    // One line, not two saying the same thing.
    expect(el.querySelector('[role="alert"].text-danger')).toBeNull();
    // The re-read replaced the saved state only: what staff typed and did not save is still there.
    expect(box(el, "host").value).toBe("smtp.dang-go.gov.vn");
  });

  it("nothing saved yet → the box is locked and says 'Lưu cấu hình trước khi gửi thử.' (user 09/10/2026)", async () => {
    H.phien = session(["admin.lookup"]);
    getReply = { status: 200, body: { ...SAVED, configured: false } };
    const el = await mount();
    const recipient = box(el, "recipient");
    expect(recipient.disabled).toBe(true);
    const hint = el.querySelector("#goi-y-gui-thu");
    expect(hint?.textContent).toBe("Lưu cấu hình trước khi gửi thử.");
    expect(recipient.getAttribute("aria-describedby")).toBe("goi-y-gui-thu");
  });

  it("saved → the box is open and the hint is gone", async () => {
    H.phien = session(["admin.lookup"]);
    const el = await mount();
    expect(box(el, "recipient").disabled).toBe(false);
    expect(el.textContent).not.toContain("Lưu cấu hình trước khi gửi thử.");
  });

  it("network failure → 'Không gọi được máy chủ.'", async () => {
    H.phien = session(["admin.lookup"]);
    testReply = "network";
    const el = await mount();
    await sendTo(el, "a@b.vn");
    expect(T.error).toHaveBeenCalledWith("Không gọi được máy chủ.");
  });
});
