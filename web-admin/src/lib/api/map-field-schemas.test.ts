import { afterEach, describe, expect, it, vi } from "vitest";

import { createMapField, deleteMapField, listMapFields, updateMapField } from "./map-field-schemas";
import type { UpdateMapFieldIn } from "./map-field-schemas";

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

const ROW = {
  id: "01JMF1",
  asset_type_code: "doanh-nghiep",
  field_code: "legal_form",
  label: "Loại hình doanh nghiệp",
  value_type: "chon",
  options: [{ value: "tnhh", label: "Công ty TNHH" }],
  is_required: false,
  sort_order: 1,
  is_active: true,
};

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("listMapFields — GET /api/v1/map-field-schemas?asset_type_code=", () => {
  it("one asset type per call, the code in the query", async () => {
    const fake = stubFetch(() => reply(200, { items: [ROW] }));
    expect(await listMapFields("doanh-nghiep")).toEqual({ ok: true, duLieu: { items: [ROW] } });
    const c = call(fake);
    expect(c.path).toBe("/api/v1/map-field-schemas?asset_type_code=doanh-nghiep");
    expect(c.method).toBe("GET");
  });

  it("403 (no asset.read) is the server's sentence", async () => {
    stubFetch(() => reply(403, { code: "forbidden", message: "Bạn không có quyền xem bản đồ tài nguyên." }));
    expect(await listMapFields("doanh-nghiep")).toEqual({
      ok: false,
      thongBao: "Bạn không có quyền xem bản đồ tài nguyên.",
    });
  });
});

describe("createMapField — POST", () => {
  it("POST with the caller's Idempotency-Key; options only for `chon`", async () => {
    const fake = stubFetch(() => reply(201, ROW));
    await createMapField(
      {
        asset_type_code: "doanh-nghiep",
        field_code: "legal_form",
        label: "Loại hình",
        value_type: "chon",
        options: [{ value: "tnhh", label: "Công ty TNHH" }],
        is_required: true,
        sort_order: 2,
      },
      "khoa-mo-form",
    );
    const c = call(fake);
    expect(c.path).toBe("/api/v1/map-field-schemas");
    expect(c.method).toBe("POST");
    expect(c.headers.get("Idempotency-Key")).toBe("khoa-mo-form");
    expect(c.body).toEqual({
      asset_type_code: "doanh-nghiep",
      field_code: "legal_form",
      label: "Loại hình",
      value_type: "chon",
      options: [{ value: "tnhh", label: "Công ty TNHH" }],
      is_required: true,
      sort_order: 2,
    });
  });

  it("a non-`chon` type never sends options, even if the caller passes some", async () => {
    const fake = stubFetch(() => reply(201, ROW));
    await createMapField(
      { asset_type_code: "a", field_code: "b", label: "c", value_type: "so-nguyen", options: [{ value: "x", label: "y" }] },
      "k",
    );
    expect(call(fake).body).not.toHaveProperty("options");
  });

  it("409 field_code_retired reaches the screen verbatim", async () => {
    const sentence = "Mã này đã dùng cho một trường đã xoá của nhóm. Mã đã cấp thì không cấp lại…";
    stubFetch(() => reply(409, { code: "field_code_retired", message: sentence }));
    expect(
      await createMapField({ asset_type_code: "a", field_code: "b", label: "c", value_type: "van-ban" }, "k"),
    ).toEqual({ ok: false, thongBao: sentence });
  });
});

describe("updateMapField — PATCH", () => {
  it("only the fields passed; Tắt sends is_active alone", async () => {
    const fake = stubFetch(() => reply(200, { ...ROW, is_active: false }));
    await updateMapField("01J/MF", { is_active: false });
    const c = call(fake);
    expect(c.path).toBe("/api/v1/map-field-schemas/01J%2FMF");
    expect(c.method).toBe("PATCH");
    expect(c.body).toEqual({ is_active: false });
  });

  it("the three immutable fields never go up, even smuggled past the type", async () => {
    const fake = stubFetch(() => reply(200, ROW));
    const smuggled = {
      label: "Mới",
      field_code: "khac",
      value_type: "so-nguyen",
      asset_type_code: "khac",
    } as unknown as UpdateMapFieldIn;
    await updateMapField("01JMF1", smuggled);
    expect(call(fake).body).toEqual({ label: "Mới" });
  });

  it("409 option_removed is the server's sentence", async () => {
    const sentence = "truong_ban_do: không bỏ được một lựa chọn đã có — có thể đổi nhãn hoặc thêm lựa chọn mới";
    stubFetch(() => reply(409, { code: "option_removed", message: sentence }));
    // §4.3: the `truong_ban_do: ` tag is stripped for display (`goi.ts`).
    expect(await updateMapField("01JMF1", { options: [] })).toEqual({
      ok: false,
      thongBao: "Không bỏ được một lựa chọn đã có — có thể đổi nhãn hoặc thêm lựa chọn mới",
    });
  });
});

describe("deleteMapField — DELETE", () => {
  it("reason in the body, 204 is success", async () => {
    const fake = stubFetch(() => new Response(null, { status: 204 }));
    expect(await deleteMapField("01JMF1", "Trùng trường khác")).toEqual({ ok: true, duLieu: null });
    const c = call(fake);
    expect(c.method).toBe("DELETE");
    expect(c.path).toBe("/api/v1/map-field-schemas/01JMF1");
    expect(c.body).toEqual({ reason: "Trùng trường khác" });
    expect(c.headers.get("Idempotency-Key")).toBeNull();
  });
});
