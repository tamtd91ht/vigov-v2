import { afterEach, describe, expect, it, vi } from "vitest";

import { createRole, deleteRole, updateRole } from "./roles";

function reply(status: number, body: unknown) {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

function stubFetch(answer: () => Response) {
  const fake = vi.fn(async () => answer());
  vi.stubGlobal("fetch", fake);
  return fake;
}

function sent(fake: ReturnType<typeof stubFetch>): { path: string; init: RequestInit } {
  const [path, init] = fake.mock.calls[0] as unknown as [string, RequestInit];
  return { path, init };
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

const ROLE = { id: "vt-9", code: "pho-chu-tich", name: "Phó Chủ tịch UBND", is_leader: true };

describe("createRole — POST /api/v1/roles", () => {
  it("sends exactly {name, is_leader}, no tenant anywhere, and reads the 201 role", async () => {
    const fake = stubFetch(() => reply(201, ROLE));
    const r = await createRole({ name: "Phó Chủ tịch UBND", is_leader: true });
    expect(r).toEqual({ ok: true, duLieu: ROLE });
    const { path, init } = sent(fake);
    expect(path).toBe("/api/v1/roles");
    expect(init.method).toBe("POST");
    expect(JSON.parse(String(init.body))).toEqual({ name: "Phó Chủ tịch UBND", is_leader: true });
    expect(`${path}${String(init.body)}`).not.toMatch(/tenant/i);
  });

  it("drops a stray key the caller passed — the body is built key by key", async () => {
    const fake = stubFetch(() => reply(201, ROLE));
    await createRole({ name: "Kế toán", is_leader: false, tenant_id: "x" } as never);
    expect(JSON.parse(String(sent(fake).init.body))).toEqual({ name: "Kế toán", is_leader: false });
  });

  it("409 duplicate_name: the server's sentence verbatim, with its code for the field placement", async () => {
    const sentence = "Đơn vị đã có vai trò mang tên này.";
    stubFetch(() => reply(409, { code: "duplicate_name", message: sentence, trace_id: "t" }));
    expect(await createRole({ name: "Kế toán", is_leader: false })).toEqual({
      ok: false,
      thongBao: sentence,
      code: "duplicate_name",
    });
  });

  it("200 is not the contract's 201 — refused rather than read as success", async () => {
    stubFetch(() => reply(200, ROLE));
    expect((await createRole({ name: "Kế toán", is_leader: false })).ok).toBe(false);
  });
});

describe("updateRole — PUT /api/v1/roles/{id}", () => {
  it("PUTs {name, is_leader} to the encoded id and reads the 200 role", async () => {
    const fake = stubFetch(() => reply(200, ROLE));
    const r = await updateRole("vt/9", { name: "Phó Chủ tịch UBND", is_leader: true });
    expect(r.ok).toBe(true);
    const { path, init } = sent(fake);
    expect(path).toBe("/api/v1/roles/vt%2F9");
    expect(init.method).toBe("PUT");
    expect(JSON.parse(String(init.body))).toEqual({ name: "Phó Chủ tịch UBND", is_leader: true });
  });

  it("409 system_role keeps the server's sentence and code", async () => {
    const sentence = "Không đổi tên được vai trò quản trị hệ thống.";
    stubFetch(() => reply(409, { code: "system_role", message: sentence, trace_id: "t" }));
    expect(await updateRole("vt-0", { name: "X", is_leader: false })).toEqual({
      ok: false,
      thongBao: sentence,
      code: "system_role",
    });
  });
});

describe("deleteRole — POST /api/v1/roles/{id}/deletion", () => {
  it("POSTs exactly {reason} to the deletion resource; 200 is success", async () => {
    const fake = stubFetch(() => reply(200, {}));
    expect(await deleteRole("vt-9", "Sáp nhập chức danh")).toEqual({ ok: true, duLieu: null });
    const { path, init } = sent(fake);
    expect(path).toBe("/api/v1/roles/vt-9/deletion");
    expect(init.method).toBe("POST");
    expect(JSON.parse(String(init.body))).toEqual({ reason: "Sáp nhập chức danh" });
  });

  it("201 is success too — the contract names both", async () => {
    stubFetch(() => new Response(null, { status: 201 }));
    expect(await deleteRole("vt-9", "Sáp nhập chức danh")).toEqual({ ok: true, duLieu: null });
  });

  it("204 is neither — refused", async () => {
    stubFetch(() => new Response(null, { status: 204 }));
    expect((await deleteRole("vt-9", "Sáp nhập chức danh")).ok).toBe(false);
  });

  it("409 role_in_use: the server's sentence (how many staff) verbatim", async () => {
    const sentence = "Còn 3 cán bộ đang giữ vai trò này. Chuyển họ sang vai trò khác trước khi xoá.";
    stubFetch(() => reply(409, { code: "role_in_use", message: sentence, trace_id: "t" }));
    expect(await deleteRole("vt-9", "x")).toEqual({ ok: false, thongBao: sentence, code: "role_in_use" });
  });
});
