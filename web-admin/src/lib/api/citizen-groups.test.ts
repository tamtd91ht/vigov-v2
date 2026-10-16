import { afterEach, describe, expect, it, vi } from "vitest";

import { createCitizenGroup, deleteCitizenGroup, listCitizenGroups, newCitizenGroupBody, updateCitizenGroup } from "./citizen-groups";

/** The group routes of the citizen directory (ADR 0097 D4/D7): paths, the key on create, the OPTIONAL reason. */

function stub(res: Response) {
  const fake = vi.fn(async (_url: string, _init?: RequestInit) => res);
  vi.stubGlobal("fetch", fake);
  return fake;
}
const json = (body: unknown, status: number) =>
  new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
const sentBody = (fake: ReturnType<typeof stub>) => JSON.parse(String(fake.mock.calls[0]?.[1]?.body)) as unknown;

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("citizen groups", () => {
  it("list: GET /api/v1/citizen-groups, the items with their member counts", async () => {
    const items = [{ id: "01JG", name: "Tổ trưởng dân phố", description: "", sort_order: 0, member_count: 8, created_at: "", updated_at: "" }];
    const fake = stub(json({ items }, 200));
    expect(await listCitizenGroups()).toEqual({ ok: true, duLieu: { items } });
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/citizen-groups");
    expect(fake.mock.calls[0]?.[1]?.method).toBe("GET");
  });

  it("a new group from the tab's one input: name as typed, empty description, order 0 — all three sent", () => {
    expect(newCitizenGroupBody("Hộ kinh doanh")).toEqual({ name: "Hộ kinh doanh", description: "", sort_order: 0 });
  });

  it("create: POST with the caller's Idempotency-Key, 201 → the id; 409 name_taken is the server's sentence", async () => {
    const fake = stub(json({ id: "01JG" }, 201));
    expect(await createCitizenGroup(newCitizenGroupBody("Hộ kinh doanh"), "khoa-1")).toEqual({ ok: true, duLieu: { id: "01JG" } });
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/citizen-groups");
    expect((fake.mock.calls[0]?.[1]?.headers as Record<string, string>)["Idempotency-Key"]).toBe("khoa-1");
    expect(sentBody(fake)).toEqual({ name: "Hộ kinh doanh", description: "", sort_order: 0 });
    stub(json({ code: "name_taken", message: "Xã đã có nhóm tên này.", trace_id: "" }, 409));
    expect(await createCitizenGroup(newCitizenGroupBody("Hộ kinh doanh"), "khoa-1")).toEqual({ ok: false, thongBao: "Xã đã có nhóm tên này." });
  });

  it("update: PATCH /citizen-groups/{id}, 204 → null", async () => {
    const fake = stub(new Response(null, { status: 204 }));
    expect(await updateCitizenGroup("01JG", { name: "Tổ trưởng", description: "", sort_order: 1 })).toEqual({ ok: true, duLieu: null });
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/citizen-groups/01JG");
    expect(fake.mock.calls[0]?.[1]?.method).toBe("PATCH");
  });

  it("delete: POST /citizen-groups/{id}/deletion — an empty reason is SENT as empty, never dropped", async () => {
    const fake = stub(new Response(null, { status: 204 }));
    expect(await deleteCitizenGroup("01JG", "")).toEqual({ ok: true, duLieu: null });
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/citizen-groups/01JG/deletion");
    expect(fake.mock.calls[0]?.[1]?.method).toBe("POST");
    expect(sentBody(fake)).toEqual({ reason: "" });
    const fake2 = stub(new Response(null, { status: 204 }));
    await deleteCitizenGroup("01JG", "Trùng nhóm cũ");
    expect(sentBody(fake2)).toEqual({ reason: "Trùng nhóm cũ" });
  });
});
