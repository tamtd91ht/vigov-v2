import { afterEach, describe, expect, it, vi } from "vitest";

import { createResidentialUnit, updateResidentialUnit, type UpdateResidentialUnitBody } from "./thon-to-dan-pho";

/**
 * The two write routes of residential units, on the wire: the create carries the caller's
 * `Idempotency-Key`; the PATCH never carries `code`, and keeps "absent" (leave alone) apart from
 * `null` (clear) for the two counts.
 */

const UNIT = {
  id: "01JTHON1",
  code: "thon-binh-an",
  name: "Thôn Bình An",
  type_code: "",
  type_label: "",
  household_count: null,
  population_count: null,
  active: true,
  head_staff_code: "",
  head_staff_name: "",
  order: 0,
};

function stubFetch(status: number, body: unknown) {
  const f = vi.fn(async () => new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } }));
  vi.stubGlobal("fetch", f);
  return f;
}

function call(f: ReturnType<typeof stubFetch>) {
  const [path, init] = f.mock.calls[0] as unknown as [string, RequestInit];
  return { path, method: init.method, headers: new Headers(init.headers), raw: String(init.body) };
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("createResidentialUnit — POST /api/v1/residential-units", () => {
  it("sends the caller's Idempotency-Key and only the fields given; 201 → the unit", async () => {
    const f = stubFetch(201, UNIT);
    const r = await createResidentialUnit({ name: "Thôn Bình An", household_count: 0 }, "khoa-1");
    expect(r).toEqual({ ok: true, duLieu: UNIT });
    const c = call(f);
    expect(c.path).toBe("/api/v1/residential-units");
    expect(c.method).toBe("POST");
    expect(c.headers.get("Idempotency-Key")).toBe("khoa-1");
    expect(JSON.parse(c.raw)).toEqual({ name: "Thôn Bình An", household_count: 0 });
  });

  it("a 409 is the server's sentence", async () => {
    stubFetch(409, { code: "residential_unit_name_taken", message: "Xã đã có một thôn / tổ dân phố cùng tên.", trace_id: "t" });
    expect(await createResidentialUnit({ name: "Thôn Bình An" }, "k")).toEqual({
      ok: false,
      thongBao: "Xã đã có một thôn / tổ dân phố cùng tên.",
    });
  });
});

describe("updateResidentialUnit — PATCH /api/v1/residential-units/{id}", () => {
  it("null goes up (clear), absent stays absent, and `code` cannot ride along", async () => {
    const f = stubFetch(200, UNIT);
    const sneaky = { household_count: null, code: "doi-ma" } as unknown as UpdateResidentialUnitBody;
    await updateResidentialUnit("01J/X", sneaky);
    const c = call(f);
    expect(c.path).toBe("/api/v1/residential-units/01J%2FX");
    expect(c.method).toBe("PATCH");
    expect(c.raw).toBe('{"household_count":null}');
    expect(c.headers.get("Idempotency-Key")).toBeNull();
  });

  it("the active toggle is the one field", async () => {
    const f = stubFetch(200, { ...UNIT, active: false });
    await updateResidentialUnit("01JTHON1", { active: false });
    expect(call(f).raw).toBe('{"active":false}');
  });
});
