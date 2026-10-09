import { describe, expect, it } from "vitest";

import type { identity_boPhanRa, petitions_taskUnitRowOut } from "@/lib/api/schema.gen";

import {
  assembleUnitRows,
  barPercent,
  onTimeCell,
  UNASSIGNED_UNIT_LABEL,
  UNKNOWN_UNIT_LABEL,
  unitRowsFrom,
} from "./unit-table";

function unit(id: string, name: string): identity_boPhanRa {
  return { id, code: id.toLowerCase(), name, parent_id: "", order: 0, staff_count: 0 };
}

function row(org_unit_id: string, total: number, extra: Partial<petitions_taskUnitRowOut> = {}): petitions_taskUnitRowOut {
  return { org_unit_id, total, completed: 0, on_time_sample: 0, on_time: 0, overdue: 0, ...extra };
}

const UNITS = [
  unit("01JBP1", "VĂN PHÒNG ĐẢNG ỦY"),
  unit("01JBP2", "THƯỜNG TRỰC HỘI ĐỒNG NHÂN DÂN"),
  unit("01JBP3", "LÃNH ĐẠO ỦY BAN NHÂN DÂN XÃ"),
];

describe("assembleUnitRows (ADR 0053 B3, B5d)", () => {
  it("every org unit is a row — a unit with no task shows zeros, it does not disappear", () => {
    const rows = assembleUnitRows(UNITS, [row("01JBP1", 21, { completed: 13 })]);
    expect(rows.map((r) => r.name)).toEqual([
      "VĂN PHÒNG ĐẢNG ỦY",
      "LÃNH ĐẠO ỦY BAN NHÂN DÂN XÃ",
      "THƯỜNG TRỰC HỘI ĐỒNG NHÂN DÂN",
    ]);
    expect(rows[1]).toMatchObject({ total: 0, completed: 0, onTime: 0, onTimeSample: 0, overdue: 0, kind: "unit" });
  });

  it("tasks with no unit (`\"\"`) are one 'Chưa xác định bộ phận' row — only when present", () => {
    expect(assembleUnitRows(UNITS, []).some((r) => r.kind === "unassigned")).toBe(false);
    const rows = assembleUnitRows(UNITS, [row("", 4, { overdue: 1 })]);
    const u = rows.find((r) => r.kind === "unassigned");
    expect(u).toMatchObject({ key: "", name: UNASSIGNED_UNIT_LABEL, total: 4, overdue: 1 });
  });

  it("a unit id no longer in the org-unit list gets ITS OWN row — never merged into 'Chưa xác định'", () => {
    const rows = assembleUnitRows(UNITS, [row("", 2), row("01JGONE", 5), row("01JGONE2", 1)]);
    const unknown = rows.filter((r) => r.kind === "unknown");
    expect(unknown.map((r) => [r.key, r.name, r.total])).toEqual([
      ["01JGONE", UNKNOWN_UNIT_LABEL, 5],
      ["01JGONE2", UNKNOWN_UNIT_LABEL, 1],
    ]);
    expect(rows.find((r) => r.kind === "unassigned")?.total).toBe(2);
    expect(rows).toHaveLength(UNITS.length + 3);
  });

  it("sorted by Tổng việc descending, then by name", () => {
    const rows = assembleUnitRows(UNITS, [row("01JBP3", 2), row("01JBP2", 2), row("", 9)]);
    expect(rows.map((r) => [r.name, r.total])).toEqual([
      [UNASSIGNED_UNIT_LABEL, 9],
      ["LÃNH ĐẠO ỦY BAN NHÂN DÂN XÃ", 2],
      ["THƯỜNG TRỰC HỘI ĐỒNG NHÂN DÂN", 2],
      ["VĂN PHÒNG ĐẢNG ỦY", 0],
    ]);
  });
});

describe("onTimeCell — the prototype's rounded % and threshold colour (D1)", () => {
  it("rounded percentage, no decimals: 100%, 33%", () => {
    expect(onTimeCell({ onTime: 2, onTimeSample: 2 })).toEqual({ text: "100%", tone: "good" });
    expect(onTimeCell({ onTime: 1, onTimeSample: 3 })).toEqual({ text: "33%", tone: "bad" });
    expect(onTimeCell({ onTime: 0, onTimeSample: 4 })).toEqual({ text: "0%", tone: "bad" });
  });

  it("≥ 80 good, ≥ 50 warning, else bad — on the unrounded rate", () => {
    expect(onTimeCell({ onTime: 4, onTimeSample: 5 })?.tone).toBe("good"); // 80%
    expect(onTimeCell({ onTime: 79, onTimeSample: 100 })?.tone).toBe("warning");
    expect(onTimeCell({ onTime: 1, onTimeSample: 2 })?.tone).toBe("warning"); // 50%
    expect(onTimeCell({ onTime: 49, onTimeSample: 100 })?.tone).toBe("bad");
  });

  it("an empty sample has no rate — null, never 0%", () => {
    expect(onTimeCell({ onTime: 0, onTimeSample: 0 })).toBeNull();
  });
});

describe("barPercent — a bar against its column's largest value", () => {
  it("0 draws nothing; the largest fills the track; a small non-zero never under 4%", () => {
    expect(barPercent(0, 21)).toBe(0);
    expect(barPercent(21, 21)).toBe(100);
    expect(barPercent(7, 21)).toBe(33);
    expect(barPercent(1, 100)).toBe(4);
  });

  it("a column of zeros (max floored at 1) draws no bar", () => {
    expect(barPercent(0, 0)).toBe(0);
  });
});

describe("unitRowsFrom — either answer failing fails the table", () => {
  const summaryOk = { ok: true as const, duLieu: { as_of: "2026-09-28T16:43:00+07:00", units: [row("01JBP1", 1)] } };
  const unitsOk = { ok: true as const, duLieu: { items: UNITS } };

  it("both answered: the assembled rows", () => {
    const r = unitRowsFrom(summaryOk, unitsOk);
    expect(r.ok && r.duLieu.length).toBe(3);
  });

  it("summary refused (403/400): its sentence, verbatim — even when the org units also failed", () => {
    expect(unitRowsFrom({ ok: false, thongBao: "Không có quyền." }, { ok: false, thongBao: "Lỗi khác." })).toEqual({
      ok: false,
      thongBao: "Không có quyền.",
    });
  });

  it("org units failed: no table of nameless rows", () => {
    expect(unitRowsFrom(summaryOk, { ok: false, thongBao: "Mất kết nối." })).toEqual({
      ok: false,
      thongBao: "Mất kết nối.",
    });
  });
});
