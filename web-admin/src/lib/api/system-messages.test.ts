import { afterEach, describe, expect, it, vi } from "vitest";

import { listSystemMessages, restoreSystemMessage, rewordSystemMessage } from "./system-messages";

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

  it("404 is the server's sentence", async () => {
    stubFetch(() => reply(404, { code: "not_found", message: "Không tìm thấy câu hệ thống này." }));
    expect(await restoreSystemMessage("petitions", "feedback.khong_co")).toEqual({
      ok: false,
      thongBao: "Không tìm thấy câu hệ thống này.",
    });
  });
});
