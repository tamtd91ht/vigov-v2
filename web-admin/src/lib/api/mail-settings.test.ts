import { afterEach, describe, expect, it, vi } from "vitest";

import { LOI_KHONG_RO } from "./goi";
import { TEST_UNREACHABLE_FALLBACK, getMailSettings, saveMailSettings, sendTestMail } from "./mail-settings";

function reply(status: number, body: unknown) {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

function stubFetch(answer: () => Response) {
  const fake = vi.fn(async () => answer());
  vi.stubGlobal("fetch", fake);
  return fake;
}

function call(fake: ReturnType<typeof stubFetch>, i = 0) {
  const [path, init] = fake.mock.calls[i] as unknown as [string, RequestInit];
  return {
    path,
    method: init.method,
    headers: new Headers(init.headers),
    body: init.body === undefined ? undefined : (JSON.parse(String(init.body)) as Record<string, unknown>),
  };
}

const SAVED = {
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
};

const INPUT = {
  host: "smtp.xa.gov.vn",
  port: 587,
  security: "starttls",
  username: "ubnd@xa.gov.vn",
  from_address: "ubnd@xa.gov.vn",
  from_name: "UBND xã",
  is_enabled: true,
};

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("getMailSettings", () => {
  it("GET /api/v1/mail-settings", async () => {
    const fake = stubFetch(() => reply(200, SAVED));
    expect(await getMailSettings()).toEqual({ ok: true, duLieu: SAVED });
    expect(call(fake).path).toBe("/api/v1/mail-settings");
    expect(call(fake).method).toBe("GET");
  });
});

describe("saveMailSettings — PUT", () => {
  it("a blank password is NOT sent — absent means keep the stored one", async () => {
    const fake = stubFetch(() => reply(200, SAVED));
    await saveMailSettings({ ...INPUT, password: "" });
    const c = call(fake);
    expect(c.method).toBe("PUT");
    expect(c.path).toBe("/api/v1/mail-settings");
    expect(c.body).toEqual(INPUT);
    expect(c.body).not.toHaveProperty("password");
  });

  it("a typed password is sent", async () => {
    const fake = stubFetch(() => reply(200, SAVED));
    await saveMailSettings({ ...INPUT, password: "mat-khau-thu" });
    expect(call(fake).body).toEqual({ ...INPUT, password: "mat-khau-thu" });
  });

  it("no stray field goes up (built field by field)", async () => {
    const fake = stubFetch(() => reply(200, SAVED));
    await saveMailSettings({ ...INPUT, password_set: true } as unknown as Parameters<typeof saveMailSettings>[0]);
    expect(call(fake).body).not.toHaveProperty("password_set");
  });

  it("400 password_required_for_new_host and 503 encryption are the server's sentences", async () => {
    const s1 = "Đã đổi máy chủ, cổng hoặc tài khoản thì phải nhập lại mật khẩu.";
    stubFetch(() => reply(400, { code: "password_required_for_new_host", message: s1 }));
    expect(await saveMailSettings(INPUT)).toEqual({ ok: false, thongBao: s1 });
    const s2 = "Nền tảng chưa cấu hình khoá mã hoá bí mật…";
    stubFetch(() => reply(503, { code: "encryption_not_configured", message: s2 }));
    expect(await saveMailSettings(INPUT)).toEqual({ ok: false, thongBao: s2 });
  });
});

describe("sendTestMail — POST test-messages", () => {
  it("POST { recipient } with the caller's Idempotency-Key", async () => {
    const fake = stubFetch(() => reply(200, { sent: true }));
    expect(await sendTestMail("canbo@xa.gov.vn", "khoa-gui-1")).toEqual({ ok: true, duLieu: null });
    const c = call(fake);
    expect(c.path).toBe("/api/v1/mail-settings/test-messages");
    expect(c.method).toBe("POST");
    expect(c.headers.get("Idempotency-Key")).toBe("khoa-gui-1");
    expect(c.body).toEqual({ recipient: "canbo@xa.gov.vn" });
  });

  it("a replayed 200 ({ replayed: true }) is a success", async () => {
    stubFetch(() => reply(200, { replayed: true }));
    expect(await sendTestMail("a@b.vn", "k")).toEqual({ ok: true, duLieu: null });
  });

  it("502 from the commune's mail server: the server's plain sentence, verbatim", async () => {
    const sentence = "Máy chủ thư từ chối tài khoản hoặc mật khẩu. Hãy nhập lại mật khẩu và lưu cấu hình.";
    stubFetch(() => reply(502, { code: "mail_auth_rejected", message: sentence }));
    expect(await sendTestMail("a@b.vn", "k")).toEqual({ ok: false, thongBao: sentence });
  });

  it("502/503 page from a proxy: 'no answer', not 'no connection'", async () => {
    stubFetch(() => new Response("<html>Bad Gateway</html>", { status: 502 }));
    expect(await sendTestMail("a@b.vn", "k")).toEqual({ ok: false, thongBao: TEST_UNREACHABLE_FALLBACK });
    stubFetch(() => new Response("", { status: 500 }));
    expect(await sendTestMail("a@b.vn", "k")).toEqual({ ok: false, thongBao: LOI_KHONG_RO });
  });

  it("409 mail_settings_missing is the server's sentence", async () => {
    const sentence = "Xã chưa lưu cấu hình máy chủ thư. Hãy lưu cấu hình trước khi gửi thử.";
    stubFetch(() => reply(409, { code: "mail_settings_missing", message: sentence }));
    expect(await sendTestMail("a@b.vn", "k")).toEqual({ ok: false, thongBao: sentence });
  });
});
