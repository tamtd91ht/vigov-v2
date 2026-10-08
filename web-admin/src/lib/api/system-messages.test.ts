import { afterEach, describe, expect, it, vi } from "vitest";

import {
  createCommuneMessage,
  deleteCommuneMessage,
  editCommuneMessage,
  listSystemMessages,
  restoreSystemMessage,
  rewordSystemMessage,
  switchSystemMessage,
} from "./system-messages";

function reply(status: number, body?: unknown) {
  return body === undefined
    ? new Response(null, { status })
    : new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
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
    body: init.body === undefined ? undefined : (JSON.parse(String(init.body)) as unknown),
  };
}

const MESSAGE = {
  code: "feedback.never_public",
  description: "Hiện khi cán bộ cho hiện công khai một phiếu thuộc lĩnh vực thái độ, tác phong cán bộ",
  default_text: "Phản ánh về thái độ, tác phong cán bộ không được hiển thị công khai.",
  current_text: "Xã không công khai phản ánh liên quan đến cán bộ.",
  overridden: true,
  updated_at: "2026-09-28T10:00:00Z",
  updated_by: "CB-00123",
};

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("listSystemMessages", () => {
  it.each([
    ["petitions", "/api/v1/petitions-system-messages"],
    ["finance", "/api/v1/finance-system-messages"],
    ["reporting", "/api/v1/reporting-system-messages"],
  ] as const)("%s: GET %s, returns the items", async (module, path) => {
    const fake = stubFetch(() => reply(200, { items: [MESSAGE] }));
    expect(await listSystemMessages(module)).toEqual({ ok: true, duLieu: [MESSAGE] });
    expect(call(fake).path).toBe(path);
    expect(call(fake).method).toBe("GET");
  });

  it("403 is the server's sentence", async () => {
    stubFetch(() => reply(403, { code: "forbidden", message: "Bạn không có quyền thực hiện thao tác này." }));
    expect(await listSystemMessages("finance")).toEqual({
      ok: false,
      thongBao: "Bạn không có quyền thực hiện thao tác này.",
    });
  });
});

describe("rewordSystemMessage — PUT …/{code}/override", () => {
  it("PUT { text } to the module's override route, code encoded, NO Idempotency-Key", async () => {
    const fake = stubFetch(() => reply(200, MESSAGE));
    expect(await rewordSystemMessage("petitions", "feedback.never_public", "Câu mới.")).toEqual({
      ok: true,
      duLieu: MESSAGE,
    });
    const c = call(fake);
    expect(c.path).toBe("/api/v1/petitions-system-messages/feedback.never_public/override");
    expect(c.method).toBe("PUT");
    expect(c.body).toEqual({ text: "Câu mới." });
    // The route declares no idempotency (contract: no x-vigov-idempotency) — none is invented.
    expect(c.headers.has("Idempotency-Key")).toBe(false);
  });

  it("reporting: PUT to the reporting override route", async () => {
    const fake = stubFetch(() => reply(200, { ...MESSAGE, code: "report.title" }));
    const r = await rewordSystemMessage("reporting", "report.title", "Báo cáo điều hành của xã");
    expect(r.ok).toBe(true);
    const c = call(fake);
    expect(c.path).toBe("/api/v1/reporting-system-messages/report.title/override");
    expect(c.method).toBe("PUT");
    expect(c.body).toEqual({ text: "Báo cáo điều hành của xã" });
  });

  it("a code with a slash cannot escape its path segment", async () => {
    const fake = stubFetch(() => reply(404, { code: "not_found", message: "Không tìm thấy câu." }));
    await rewordSystemMessage("finance", "a/../b", "x");
    expect(call(fake).path).toBe("/api/v1/finance-system-messages/a%2F..%2Fb/override");
  });

  it("400 from the server (markup, control characters…) is its sentence, verbatim", async () => {
    const sentence = "Nội dung câu không được chứa dấu < hoặc >.";
    stubFetch(() => reply(400, { code: "invalid_text", message: sentence }));
    expect(await rewordSystemMessage("finance", "budget.scope_notice", "<b>x</b>")).toEqual({
      ok: false,
      thongBao: sentence,
    });
  });
});

describe("restoreSystemMessage — DELETE …/{code}/override", () => {
  it("DELETE, no body, 204 is success", async () => {
    const fake = stubFetch(() => reply(204));
    expect(await restoreSystemMessage("finance", "budget.scope_notice")).toEqual({ ok: true, duLieu: null });
    const c = call(fake);
    expect(c.path).toBe("/api/v1/finance-system-messages/budget.scope_notice/override");
    expect(c.method).toBe("DELETE");
    expect(c.body).toBeUndefined();
    expect(c.headers.has("Idempotency-Key")).toBe(false);
  });

  it("reporting: DELETE the reporting override route, 204", async () => {
    const fake = stubFetch(() => reply(204));
    expect(await restoreSystemMessage("reporting", "report.block.alerts")).toEqual({ ok: true, duLieu: null });
    expect(call(fake).path).toBe("/api/v1/reporting-system-messages/report.block.alerts/override");
    expect(call(fake).method).toBe("DELETE");
  });

  it("404 is the server's sentence", async () => {
    stubFetch(() => reply(404, { code: "not_found", message: "Không tìm thấy câu hệ thống này." }));
    expect(await restoreSystemMessage("petitions", "feedback.khong_co")).toEqual({
      ok: false,
      thongBao: "Không tìm thấy câu hệ thống này.",
    });
  });
});

describe("switchSystemMessage — PATCH …/{code}/override", () => {
  it.each(["petitions", "finance", "reporting"] as const)("%s: PATCH { is_active } to the override route", async (module) => {
    const fake = stubFetch(() => reply(200, { ...MESSAGE, is_active: false }));
    expect((await switchSystemMessage(module, "x.y", false)).ok).toBe(true);
    const c = call(fake);
    expect(c.path).toBe(`/api/v1/${module}-system-messages/x.y/override`);
    expect(c.method).toBe("PATCH");
    expect(c.body).toEqual({ is_active: false });
    expect(c.headers.has("Idempotency-Key")).toBe(false);
  });

  it("409 no_commune_wording is the server's sentence", async () => {
    const sentence = "Câu này đang dùng lời gốc của phần mềm, chưa có lời của xã để tắt hoặc bật.";
    stubFetch(() => reply(409, { code: "no_commune_wording", message: sentence }));
    expect(await switchSystemMessage("petitions", "feedback.never_public", false)).toEqual({ ok: false, thongBao: sentence });
  });
});

describe("commune sentences — POST / PATCH / DELETE …/{code}", () => {
  it("POST to the list route with the Idempotency-Key; 201 is the new sentence", async () => {
    const created = { ...MESSAGE, code: "chung.loi-chao", origin: "commune" };
    const fake = stubFetch(() => reply(201, created));
    const r = await createCommuneMessage(
      "petitions",
      { group_code: "chung", code: "chung.loi-chao", text: "Xin chào." },
      "idem-1",
    );
    expect(r).toEqual({ ok: true, duLieu: created });
    const c = call(fake);
    expect(c.path).toBe("/api/v1/petitions-system-messages");
    expect(c.method).toBe("POST");
    expect(c.headers.get("Idempotency-Key")).toBe("idem-1");
    expect(c.body).toEqual({ group_code: "chung", code: "chung.loi-chao", text: "Xin chào." });
  });

  it("POST 409 message_code_taken is the server's sentence", async () => {
    stubFetch(() => reply(409, { code: "message_code_taken", message: "Mã này đã được dùng cho một câu khác." }));
    expect(
      await createCommuneMessage("finance", { group_code: "giai-ngan", code: "giai-ngan.a", text: "B." }, "k"),
    ).toEqual({ ok: false, thongBao: "Mã này đã được dùng cho một câu khác." });
  });

  it("PATCH …/{code} (NOT /override), code encoded, no Idempotency-Key", async () => {
    const fake = stubFetch(() => reply(200, MESSAGE));
    await editCommuneMessage("finance", "giai-ngan/x", { is_active: true });
    const c = call(fake);
    expect(c.path).toBe("/api/v1/finance-system-messages/giai-ngan%2Fx");
    expect(c.method).toBe("PATCH");
    expect(c.body).toEqual({ is_active: true });
    expect(c.headers.has("Idempotency-Key")).toBe(false);
  });

  it("DELETE …/{code} carries { reason } in the body; 204 is success", async () => {
    const fake = stubFetch(() => reply(204));
    expect(await deleteCommuneMessage("petitions", "phan-anh.a", "Trùng.")).toEqual({ ok: true, duLieu: null });
    const c = call(fake);
    expect(c.path).toBe("/api/v1/petitions-system-messages/phan-anh.a");
    expect(c.method).toBe("DELETE");
    expect(c.body).toEqual({ reason: "Trùng." });
  });
});
