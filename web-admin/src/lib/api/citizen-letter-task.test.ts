import { afterEach, describe, expect, it, vi } from "vitest";

import { createTaskFromLetter, letterTaskBody } from "./citizen-letter-task";

/**
 * POST /api/v1/citizen-letter-tasks (ADR 0085 A). Pinned: the path, the key, the body built field by
 * field — `due_at`, `source`, `source_id` NEVER sent (the server refuses each by name) — and refusals
 * returned verbatim.
 */

function stub(res: Response) {
  const fake = vi.fn(async (_url: string, _init?: RequestInit) => res);
  vi.stubGlobal("fetch", fake);
  return fake;
}
const json = (body: unknown, status: number) =>
  new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("letterTaskBody", () => {
  it("drops due_at / source / source_id and every empty optional key; keeps the rest", () => {
    const body = letterTaskBody("L1", {
      auto_code: true,
      type: "co-ban",
      title: "Khắc phục đường ngập",
      description: "",
      priority: "thuong",
      unit: "U1",
      assignee: "CB-00002",
      assigner: "",
      // Keys the create form may carry — the builder must leave them behind.
      ...({ due_at: "2026-10-20T10:00:00Z", source: "don-thu", source_id: "x" } as object),
      documents: [{ id: "old", group: "san-pham-dau-ra", summary: "Báo cáo", reference: "", date: "2026-10-09" }],
    });
    expect(body).toEqual({
      letter_id: "L1",
      auto_code: true,
      type: "co-ban",
      title: "Khắc phục đường ngập",
      priority: "thuong",
      unit: "U1",
      assignee: "CB-00002",
      documents: [{ group: "san-pham-dau-ra", summary: "Báo cáo", date: "2026-10-09" }],
    });
  });
});

describe("POST /api/v1/citizen-letter-tasks", () => {
  it("POST, the caller's Idempotency-Key, 201 → the task", async () => {
    const fake = stub(json({ code: "NV41", title: "Khắc phục đường ngập" }, 201));
    const r = await createTaskFromLetter({ letter_id: "L1", auto_code: true, type: "co-ban", title: "Khắc phục đường ngập" }, "khoa-9");
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/citizen-letter-tasks");
    const init = fake.mock.calls[0]?.[1];
    expect(init?.method).toBe("POST");
    expect((init?.headers as Record<string, string>)["Idempotency-Key"]).toBe("khoa-9");
    const sent = JSON.parse(String(init?.body)) as Record<string, unknown>;
    expect(sent).toEqual({ letter_id: "L1", auto_code: true, type: "co-ban", title: "Khắc phục đường ngập" });
    expect(r.ok && r.duLieu.code).toBe("NV41");
  });

  it("a wider object passed in still sends no due_at", async () => {
    const fake = stub(json({ code: "NV42" }, 201));
    const wide = { letter_id: "L1", auto_code: true, type: "co-ban", title: "T", due_at: "2026-10-20T10:00:00Z" };
    await createTaskFromLetter(wide, "k");
    expect(String(fake.mock.calls[0]?.[1]?.body)).not.toContain("due_at");
  });

  it("422 denunciation_no_task / assignment_required and 404: the server's sentence, verbatim", async () => {
    stub(json({ code: "denunciation_no_task", message: "Đơn tố cáo không chuyển thành nhiệm vụ để giữ bí mật người tố cáo.", trace_id: "" }, 422));
    expect(await createTaskFromLetter({ letter_id: "L1", auto_code: true, type: "x", title: "T" }, "k")).toEqual({
      ok: false,
      thongBao: "Đơn tố cáo không chuyển thành nhiệm vụ để giữ bí mật người tố cáo.",
    });
    stub(json({ code: "assignment_required", message: "Chọn bộ phận hoặc người thực hiện.", trace_id: "" }, 422));
    expect(await createTaskFromLetter({ letter_id: "L1", auto_code: true, type: "x", title: "T" }, "k")).toEqual({
      ok: false,
      thongBao: "Chọn bộ phận hoặc người thực hiện.",
    });
    stub(json({ code: "not_found", message: "Không tìm thấy đơn thư.", trace_id: "" }, 404));
    expect(await createTaskFromLetter({ letter_id: "L1", auto_code: true, type: "x", title: "T" }, "k")).toEqual({
      ok: false,
      thongBao: "Không tìm thấy đơn thư.",
    });
  });
});
