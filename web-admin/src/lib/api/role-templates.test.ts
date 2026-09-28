import { afterEach, describe, expect, it, vi } from "vitest";

import { seedRoleTemplates } from "./role-templates";

function reply(status: number, body: unknown) {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

function stubFetch(answer: () => Response) {
  const fake = vi.fn(async () => answer());
  vi.stubGlobal("fetch", fake);
  return fake;
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

const RESULT = {
  created: [{ code: "chu-tich-ubnd", name: "Chủ tịch UBND" }],
  skipped_existing: [{ code: "ke-toan", name: "Kế toán" }],
  skipped_deleted: [],
};

describe("seedRoleTemplates — POST /api/v1/roles/defaults", () => {
  it("POST, no body, no Content-Type, no Idempotency-Key, no tenant anywhere", async () => {
    const fake = stubFetch(() => reply(200, RESULT));
    const r = await seedRoleTemplates();
    expect(r).toEqual({ ok: true, duLieu: RESULT });

    const [path, init] = fake.mock.calls[0] as unknown as [string, RequestInit];
    expect(path).toBe("/api/v1/roles/defaults");
    expect(init.method).toBe("POST");
    expect(init.body).toBeUndefined();
    const headers = new Headers(init.headers);
    expect(headers.get("Content-Type")).toBeNull();
    expect(headers.get("Idempotency-Key")).toBeNull();
    expect(path).not.toMatch(/tenant/i);
  });

  it("403 permission_escalation reaches the screen as the server's sentence, naming the keys", async () => {
    const sentence =
      "Chỉ người đang giữ đủ mọi quyền mà bộ vai trò mẫu cấp mới gieo được bộ này. Tài khoản của bạn còn thiếu: budget.confirm, task.approve.";
    stubFetch(() => reply(403, { code: "permission_escalation", message: sentence, trace_id: "t" }));
    expect(await seedRoleTemplates()).toEqual({ ok: false, thongBao: sentence });
  });

  it("201 is not the contract's 200 — refused rather than read as success", async () => {
    stubFetch(() => reply(201, RESULT));
    expect((await seedRoleTemplates()).ok).toBe(false);
  });
});
