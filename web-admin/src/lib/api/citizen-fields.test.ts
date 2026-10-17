import { afterEach, describe, expect, it, vi } from "vitest";

import {
  CITIZEN_FIELD_CODE_TAKEN,
  CITIZEN_FIELD_ORDER_PATH,
  citizenFieldPath,
  citizenFieldsPath,
  createCitizenField,
  listCitizenFields,
  reorderCitizenFields,
  retireCitizenField,
  updateCitizenField,
  type CitizenFieldCreateBody,
  type CitizenFieldUpdateBody,
} from "./citizen-fields";

/** The citizen-field routes (ADR 0101): paths, methods, bodies, the key on create, each route's one success status. */

function stub(res: Response) {
  const fake = vi.fn(async (_url: string, _init?: RequestInit) => res);
  vi.stubGlobal("fetch", fake);
  return fake;
}
const json = (body: unknown, status: number) =>
  new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
const empty = (status: number) => new Response(null, { status });
const sentBody = (fake: ReturnType<typeof stub>) => JSON.parse(String(fake.mock.calls[0]?.[1]?.body)) as unknown;
const sentHeaders = (fake: ReturnType<typeof stub>) => fake.mock.calls[0]?.[1]?.headers as Record<string, string> | undefined;

const CREATE: CitizenFieldCreateBody = {
  code: "hongheo1",
  label: "Phân loại hộ",
  field_type: "singleBox",
  placeholder: "",
  options: [
    { value: "ho-ngheo", label: "Hộ nghèo", retired: false },
    { value: "can-ngheo", label: "Cận nghèo", retired: false },
  ],
  number_kind: "",
  date_pattern: "",
  required: true,
  show_in_list: true,
};

const UNCHANGED: CitizenFieldUpdateBody = {
  code: null,
  label: null,
  field_type: null,
  placeholder: null,
  options: null,
  number_kind: null,
  date_pattern: null,
  required: null,
  show_in_list: null,
};

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("citizen-fields client", () => {
  it("list: GET /citizen-fields; `?include_retired=true` only when asked", async () => {
    expect(citizenFieldsPath(false)).toBe("/api/v1/citizen-fields");
    expect(citizenFieldsPath(true)).toBe("/api/v1/citizen-fields?include_retired=true");
    const fake = stub(json({ items: [] }, 200));
    expect(await listCitizenFields()).toEqual({ ok: true, duLieu: { items: [] } });
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/citizen-fields");
    expect(fake.mock.calls[0]?.[1]?.method).toBe("GET");
    const fake2 = stub(json({ items: [] }, 200));
    await listCitizenFields(true);
    expect(fake2.mock.calls[0]?.[0]).toBe("/api/v1/citizen-fields?include_retired=true");
  });

  it("create: POST with the caller's Idempotency-Key and the body verbatim; 201 → ok, any other success status is not the contract", async () => {
    const fake = stub(json({ ...CREATE, sort_order: 1, retired_at: null }, 201));
    expect(await createCitizenField(CREATE, "khoa-1")).toEqual({ ok: true, duLieu: null });
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/citizen-fields");
    expect(fake.mock.calls[0]?.[1]?.method).toBe("POST");
    expect(sentHeaders(fake)?.["Idempotency-Key"]).toBe("khoa-1");
    expect(sentBody(fake)).toEqual(CREATE);
    stub(json({}, 200));
    expect((await createCitizenField(CREATE, "khoa-1")).ok).toBe(false);
  });

  it("create refused: the server's sentence AND its code (taken code, limit)", async () => {
    stub(json({ code: "citizen_field_code_taken", message: "Mã đã dùng.", trace_id: "" }, 409));
    expect(await createCitizenField(CREATE, "k")).toEqual({ ok: false, thongBao: "Mã đã dùng.", code: CITIZEN_FIELD_CODE_TAKEN });
    stub(json({ code: "citizen_field_limit", message: "Đã đủ 50.", trace_id: "" }, 409));
    expect(await createCitizenField(CREATE, "k")).toEqual({ ok: false, thongBao: "Đã đủ 50.", code: "citizen_field_limit" });
  });

  it("update: PATCH /citizen-fields/{code}, unchanged keys null; 200 with the field → ok", async () => {
    const fake = stub(json({ ...CREATE, label: "Hộ", required: false }, 200));
    expect(await updateCitizenField("hongheo1", { ...UNCHANGED, label: "Hộ", required: false })).toEqual({ ok: true, duLieu: null });
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/citizen-fields/hongheo1");
    expect(fake.mock.calls[0]?.[1]?.method).toBe("PATCH");
    expect(sentBody(fake)).toEqual({ ...UNCHANGED, label: "Hộ", required: false });
    expect(sentHeaders(fake)?.["Idempotency-Key"]).toBeUndefined();
  });

  it("retire: DELETE /citizen-fields/{code} with `{reason}`", async () => {
    const fake = stub(empty(204));
    expect(await retireCitizenField("hongheo1", "Không dùng nữa")).toEqual({ ok: true, duLieu: null });
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/citizen-fields/hongheo1");
    expect(fake.mock.calls[0]?.[1]?.method).toBe("DELETE");
    expect(sentBody(fake)).toEqual({ reason: "Không dùng nữa" });
  });

  it("reorder: PUT the whole live order to the one order path", async () => {
    expect(CITIZEN_FIELD_ORDER_PATH).toBe("/api/v1/citizen-field-order");
    const fake = stub(empty(204));
    expect(await reorderCitizenFields(["sonhan01", "hongheo1"])).toEqual({ ok: true, duLieu: null });
    expect(fake.mock.calls[0]?.[0]).toBe(CITIZEN_FIELD_ORDER_PATH);
    expect(fake.mock.calls[0]?.[1]?.method).toBe("PUT");
    expect(sentBody(fake)).toEqual({ codes: ["sonhan01", "hongheo1"] });
  });

  it("a refused write keeps the server's sentence", async () => {
    stub(json({ code: "forbidden", message: "Không có quyền.", trace_id: "" }, 403));
    expect(await reorderCitizenFields(["a"])).toEqual({ ok: false, thongBao: "Không có quyền." });
  });

  it("the code is percent-encoded; no path names a tenant", () => {
    expect(citizenFieldPath("a/b")).toBe("/api/v1/citizen-fields/a%2Fb");
    for (const p of [citizenFieldsPath(true), citizenFieldPath("x"), CITIZEN_FIELD_ORDER_PATH]) expect(p).not.toMatch(/tenant/i);
  });
});
