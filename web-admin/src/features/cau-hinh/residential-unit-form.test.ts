import { describe, expect, it, vi } from "vitest";

import type { identity_loaiDonViDanCuRa, identity_thonToDanPhoRa } from "@/lib/api/schema.gen";

import { CHUA_CO_THAY_DOI, LOI_THU_TU } from "./nhan-so-do";
import {
  COUNT_ERROR_HOUSEHOLDS,
  COUNT_ERROR_POPULATION,
  activeToggleBody,
  createBody,
  draftForCreate,
  draftFromUnit,
  headOptions,
  openCreate,
  parseCount,
  submitResidentialUnitForm,
  toggleResidentialUnitActive,
  typeOptions,
  updateBody,
  type ResidentialUnitApi,
} from "./residential-unit-form";

/**
 * What goes up on the wire for add / edit / Ngừng dùng / Dùng lại, and which sentence comes back.
 * The rule this file guards above all: a blank count is `null` ("chưa nhập"), never 0.
 */

function unit(over: Partial<identity_thonToDanPhoRa> = {}): identity_thonToDanPhoRa {
  return {
    id: "01JTHON1",
    code: "thon-binh-an",
    name: "Thôn Bình An",
    type_code: "thon",
    type_label: "Thôn",
    household_count: 284,
    population_count: 1132,
    active: true,
    head_staff_code: "CB-001",
    head_staff_name: "Nguyễn Văn A",
    order: 2,
    ...over,
  };
}

function fakeApi(over: Partial<ResidentialUnitApi> = {}) {
  const create = vi.fn<ResidentialUnitApi["create"]>(async (b) => ({ ok: true, duLieu: unit({ name: b.name }) }));
  const update = vi.fn<ResidentialUnitApi["update"]>(async (_id, b) => ({
    ok: true,
    duLieu: unit(typeof b.active === "boolean" ? { active: b.active } : {}),
  }));
  return { create: over.create ?? create, update: over.update ?? update, createMock: create, updateMock: update };
}

describe("parseCount — blank is 'chưa nhập', never 0", () => {
  it("blank → null; 0 → 0 (an assertion); numbers, grouped the vi-VN way too", () => {
    expect(parseCount("")).toEqual({ ok: true, value: null });
    expect(parseCount("   ")).toEqual({ ok: true, value: null });
    expect(parseCount("0")).toEqual({ ok: true, value: 0 });
    expect(parseCount("284")).toEqual({ ok: true, value: 284 });
    expect(parseCount("1.132")).toEqual({ ok: true, value: 1132 });
    expect(parseCount("12 345")).toEqual({ ok: true, value: 12345 });
  });

  it("refused locally: negative, decimal, a word after the number", () => {
    for (const bad of ["-1", "1.5", "12 hộ", "abc", "1,5"]) expect(parseCount(bad).ok).toBe(false);
  });
});

describe("create body", () => {
  it("only the name: every blank optional box is ABSENT — counts included (not 0, not null)", () => {
    const r = createBody({ ...draftForCreate(), name: "Tổ dân phố 5" });
    expect(r).toEqual({ kind: "send", body: { name: "Tổ dân phố 5" } });
    if (r.kind !== "send") throw new Error("unreachable");
    expect(JSON.parse(JSON.stringify(r.body))).toEqual({ name: "Tổ dân phố 5" });
  });

  it("every box filled", () => {
    const r = createBody({
      name: "Thôn Bình An",
      code: " thon-binh-an ",
      typeCode: "thon",
      headStaffCode: "CB-001",
      households: "0",
      population: "1.132",
      order: "3",
    });
    expect(r).toEqual({
      kind: "send",
      body: {
        name: "Thôn Bình An",
        code: "thon-binh-an",
        type_code: "thon",
        head_staff_code: "CB-001",
        household_count: 0,
        population_count: 1132,
        order: 3,
      },
    });
  });

  it("a mistyped box is a local error — nothing sent", () => {
    expect(createBody({ ...draftForCreate(), name: "X", households: "nhiều" })).toEqual({
      kind: "error",
      message: COUNT_ERROR_HOUSEHOLDS,
    });
    expect(createBody({ ...draftForCreate(), name: "X", population: "-3" })).toEqual({
      kind: "error",
      message: COUNT_ERROR_POPULATION,
    });
    expect(createBody({ ...draftForCreate(), name: "X", order: "ba" })).toEqual({ kind: "error", message: LOI_THU_TU });
  });
});

describe("edit body — only what changed", () => {
  it("opening the form: null counts are EMPTY boxes, never '0'", () => {
    const d = draftFromUnit(unit({ household_count: null, population_count: 0 }));
    expect(d.households).toBe("");
    expect(d.population).toBe("0");
  });

  it("nothing changed → nothing sent", () => {
    expect(updateBody(unit(), draftFromUnit(unit()))).toEqual({ kind: "unchanged" });
  });

  it("emptying a count box that held a number sends null (CLEAR), not 0", () => {
    const u = unit();
    const r = updateBody(u, { ...draftFromUnit(u), households: "" });
    expect(r).toEqual({ kind: "send", body: { household_count: null } });
    if (r.kind !== "send") throw new Error("unreachable");
    // `null` goes up on the wire — that is the server's "clear".
    expect(JSON.stringify(r.body)).toBe('{"household_count":null}');
  });

  it("a count that was null and stays blank is not sent", () => {
    const u = unit({ population_count: null });
    expect(updateBody(u, draftFromUnit(u))).toEqual({ kind: "unchanged" });
  });

  it("clearing type and head sends '' (clear); code never goes up; emptied order is no change", () => {
    const u = unit();
    const r = updateBody(u, { ...draftFromUnit(u), typeCode: "", headStaffCode: "", order: "", code: "doi-ma" });
    expect(r).toEqual({ kind: "send", body: { type_code: "", head_staff_code: "" } });
  });

  it("rename + new count", () => {
    const u = unit();
    expect(updateBody(u, { ...draftFromUnit(u), name: "Thôn Bình An Mới", population: "1.200" })).toEqual({
      kind: "send",
      body: { name: "Thôn Bình An Mới", population_count: 1200 },
    });
  });
});

describe("Ngừng dùng / Dùng lại — the active toggle body", () => {
  it("active unit → { active: false }; out-of-use unit → { active: true }; nothing else rides along", () => {
    expect(activeToggleBody(unit({ active: true }))).toEqual({ active: false });
    expect(activeToggleBody(unit({ active: false }))).toEqual({ active: true });
  });

  it("sends the PATCH on the unit's id and says what happened", async () => {
    const api = fakeApi();
    const r = await toggleResidentialUnitActive(unit(), api);
    expect(api.updateMock).toHaveBeenCalledWith("01JTHON1", { active: false });
    expect(r).toEqual({
      kind: "done",
      sentence: "Đã ngừng dùng Thôn Bình An. Hồ sơ đã lập ở địa bàn này vẫn giữ nguyên tên địa bàn.",
    });
    const back = await toggleResidentialUnitActive(unit({ active: false }), api);
    expect(back).toEqual({ kind: "done", sentence: "Đã đưa Thôn Bình An vào dùng lại." });
  });

  it("a refusal is the server's sentence, verbatim", async () => {
    const api = fakeApi({ update: async () => ({ ok: false, thongBao: "Không tìm thấy thôn / tổ dân phố." }) });
    expect(await toggleResidentialUnitActive(unit(), api)).toEqual({
      kind: "serverError",
      message: "Không tìm thấy thôn / tổ dân phố.",
    });
  });
});

describe("submit — create and edit", () => {
  it("create uses the key minted at OPEN, the same key on a retry", async () => {
    const mint = vi.fn(() => "khoa-1");
    const open = openCreate(mint);
    const taken =
      "Xã đã có một thôn / tổ dân phố cùng tên (kể cả đơn vị đã ngưng dùng). Nếu đó là đơn vị cũ, hãy dùng lại đơn vị ấy thay vì tạo mới.";
    const create = vi
      .fn<ResidentialUnitApi["create"]>()
      .mockResolvedValueOnce({ ok: false, thongBao: taken })
      .mockResolvedValueOnce({ ok: true, duLieu: unit({ name: "Thôn Mới" }) });
    const api = fakeApi({ create });
    const d = { ...draftForCreate(), name: "Thôn Mới" };

    expect(await submitResidentialUnitForm(open, d, api)).toEqual({ kind: "serverError", message: taken });
    expect(await submitResidentialUnitForm(open, d, api)).toEqual({ kind: "done", sentence: "Đã thêm Thôn Mới." });
    expect(create.mock.calls.map((c) => c[1])).toEqual(["khoa-1", "khoa-1"]);
    expect(mint).toHaveBeenCalledTimes(1);
  });

  it("edit with nothing changed says so and sends nothing", async () => {
    const api = fakeApi();
    const u = unit();
    expect(await submitResidentialUnitForm({ kind: "edit", unit: u }, draftFromUnit(u), api)).toEqual({
      kind: "localError",
      message: CHUA_CO_THAY_DOI,
    });
    expect(api.updateMock).not.toHaveBeenCalled();
  });

  it("edit sends only the diff and says it was saved", async () => {
    const api = fakeApi();
    const u = unit();
    const r = await submitResidentialUnitForm({ kind: "edit", unit: u }, { ...draftFromUnit(u), households: "300" }, api);
    expect(api.updateMock).toHaveBeenCalledWith("01JTHON1", { household_count: 300 });
    expect(r).toEqual({ kind: "done", sentence: "Đã lưu Thôn Bình An." });
  });

  it("each server refusal is shown as the server wrote it", async () => {
    for (const msg of [
      "Mã này đã được cấp trong xã (kể cả cho đơn vị đã ngưng dùng — mã đã cấp không cấp lại). Hãy chọn mã khác.",
      "Danh sách thôn / tổ dân phố của xã đã tới trần, không thêm được nữa.",
      "Loại đơn vị dân cư được chọn không có hoặc đã ngưng dùng trong xã. Hãy tải lại danh mục.",
      "Không tìm thấy cán bộ đang làm việc của xã có mã được chọn làm trưởng thôn / tổ trưởng.",
    ]) {
      const api = fakeApi({ create: async () => ({ ok: false, thongBao: msg }) });
      expect(await submitResidentialUnitForm(openCreate(() => "k"), { ...draftForCreate(), name: "X" }, api)).toEqual({
        kind: "serverError",
        message: msg,
      });
    }
  });
});

function type(code: string, label: string, active: boolean): identity_loaiDonViDanCuRa {
  return { id: `id-${code}`, code, label, active, is_default: false, order: 0, source: "system", tier: 1 };
}

describe("pickers exclude what is out of use", () => {
  const types = [type("thon", "Thôn", true), type("to-dan-pho", "Tổ dân phố", true), type("ap", "Ấp", false)];

  it("type picker: only types in use", () => {
    expect(typeOptions(types, null)).toEqual([
      { value: "thon", label: "Thôn" },
      { value: "to-dan-pho", label: "Tổ dân phố" },
    ]);
  });

  it("type picker on edit: the unit's CURRENT out-of-use type stays, marked — no silent re-classification", () => {
    const opts = typeOptions(types, { code: "ap", label: "Ấp" });
    expect(opts.map((o) => o.value)).toEqual(["thon", "to-dan-pho", "ap"]);
    expect(opts[2]!.label).toMatch(/^Ấp \(đã ngừng dùng/);
    // A current type that is in use is not duplicated.
    expect(typeOptions(types, { code: "thon", label: "Thôn" })).toHaveLength(2);
  });

  it("head picker: the directory (live staff only), sent back by business code", () => {
    const dir = [{ code: "CB-002", full_name: "Trần Thị B", position: "Công chức", department_id: "" }];
    expect(headOptions(dir, null)).toEqual([{ value: "CB-002", label: "Trần Thị B — Công chức (CB-002)" }]);
    const withGone = headOptions(dir, { code: "CB-001", name: "Nguyễn Văn A" });
    expect(withGone[0]).toEqual({ value: "CB-001", label: "Nguyễn Văn A (không còn trong danh bạ cán bộ)" });
  });
});
