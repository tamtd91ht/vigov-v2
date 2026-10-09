import { afterEach, describe, expect, it, vi } from "vitest";

import { getStaffTallies } from "./can-bo";

/** A `fetch` stub answering one status + body, recording the calls. */
function answer(status: number, body: unknown) {
  const fake = vi.fn(
    async () =>
      new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } }),
  );
  vi.stubGlobal("fetch", fake);
  return fake;
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("getStaffTallies — GET /api/v1/staff-counts, every figure checked", () => {
  it("one GET to the bare path (no query: the commune comes from Host), the three figures out", async () => {
    const fake = answer(200, {
      total: 12,
      published: 5,
      no_department: { total: 2, published: 0 },
      departments: [{ id: "BP-LE", total: 10, published: 5 }],
    });
    const kq = await getStaffTallies();
    expect(kq).toEqual({
      ok: true,
      duLieu: { total: 12, published: 5, departments: [{ id: "BP-LE", total: 10, published: 5 }] },
    });
    expect(fake).toHaveBeenCalledTimes(1);
    expect(String((fake.mock.calls[0] as unknown as [string])[0])).toBe("/api/v1/staff-counts");
  });

  it.each([
    ["a missing total", { published: 1, departments: [] }],
    ["a negative published", { total: 3, published: -1, departments: [] }],
    ["a fractional department total", { total: 3, published: 1, departments: [{ id: "x", total: 1.5, published: 0 }] }],
    ["no departments list", { total: 3, published: 1 }],
  ])("%s → unreadable, never a guessed figure", async (_name, body) => {
    answer(200, body);
    const kq = await getStaffTallies();
    expect(kq.ok).toBe(false);
  });

  it("a refusal keeps the server's sentence", async () => {
    answer(403, { code: "forbidden", message: "Tài khoản không có quyền quản lý người dùng.", trace_id: "t" });
    const kq = await getStaffTallies();
    expect(kq).toEqual({ ok: false, thongBao: "Tài khoản không có quyền quản lý người dùng." });
  });
});
