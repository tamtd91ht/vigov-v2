/**
 * "Tình hình thực hiện theo bộ phận" — the rows of `/bao-cao`'s unit table, assembled from TWO
 * answers: `GET /api/v1/task-unit-summary` (figures, one row per unit that holds tasks) and
 * `GET /api/v1/org-units` (names, and the units that hold nothing). Pure, so every row rule below is
 * tested without a browser.
 *
 * DECIDED BY THE OWNER (ADR 0053, amendment 04/10/2026, B3) — not chosen here:
 *   · the name is "Tình hình thực hiện theo bộ phận", never "Xếp hạng": a People's Committee report
 *     does not rank the Party committee, the People's Council or the Fatherland Front;
 *   · EVERY unit of the commune is a row, a unit with no task included (zeros, not absent);
 *   · tasks with no unit are one row, "Chưa xác định bộ phận";
 *   · sorted by "Tổng việc", descending.
 * VENDOR CHOICES (B5 — the customer may reword them):
 *   · a unit id the org-unit list no longer holds (soft-deleted unit still holding tasks) gets ITS
 *     OWN row (B5d). Merging it into "Chưa xác định" would hand a real unit's tasks to nobody — a
 *     false statement of responsibility;
 *   · "Quá hạn" is a plain count of the CURRENT stock (B5b): no bar of overdue/total, because the
 *     numerator is "now" and the denominator is "the period" — a ratio nobody could read right.
 */

import { formatCount, formatPercent, NO_VALUE, ratioPercent } from "@/features/dashboard/figures";
import type { KetQua } from "@/lib/api/goi";
import type {
  identity_boPhanRa,
  identity_danhSachBoPhanRa,
  petitions_taskUnitRowOut,
  petitions_taskUnitSummaryOut,
} from "@/lib/api/schema.gen";

export const UNIT_TABLE_TITLE = "Tình hình thực hiện theo bộ phận";

export const UNASSIGNED_UNIT_LABEL = "Chưa xác định bộ phận";

export const UNKNOWN_UNIT_LABEL = "Bộ phận không còn trong danh mục";

/** Column headers, in spec order minus the bar: Bộ phận · Tổng việc · Hoàn thành · Đúng hạn · Quá hạn. */
export const UNIT_COLUMNS = {
  unit: "Bộ phận",
  total: "Tổng việc",
  completed: "Hoàn thành",
  onTime: "Đúng hạn",
  overdue: "Quá hạn (hiện tại)",
} as const;

/** The note under the table: what "Tổng việc" counts, and that "Quá hạn" is not a period figure. */
export const UNIT_TABLE_NOTE =
  "Tổng việc là số nhiệm vụ bộ phận đang nắm trong kỳ: tạo trước cuối kỳ và chưa hoàn thành trước " +
  "đầu kỳ. Quá hạn tính tại thời điểm xem, không theo kỳ.";

export const UNIT_TABLE_EMPTY = "Xã chưa có bộ phận nào trong danh mục và không có nhiệm vụ nào trong kỳ.";

export type UnitRowKind = "unit" | "unassigned" | "unknown";

export type UnitRow = {
  /** org unit id; `""` for the unassigned row */
  readonly key: string;
  readonly name: string;
  readonly kind: UnitRowKind;
  readonly total: number;
  readonly completed: number;
  readonly onTime: number;
  readonly onTimeSample: number;
  readonly overdue: number;
};

const ZERO = { total: 0, completed: 0, onTime: 0, onTimeSample: 0, overdue: 0 } as const;

function figuresOf(r: petitions_taskUnitRowOut) {
  return {
    total: r.total,
    completed: r.completed,
    onTime: r.on_time,
    onTimeSample: r.on_time_sample,
    overdue: r.overdue,
  };
}

/**
 * Every org unit (zeros where the summary has no row) + the unassigned row if the summary has one +
 * one row per summary id the org-unit list does not hold. Sorted by Tổng việc desc, then name.
 */
export function assembleUnitRows(
  orgUnits: readonly identity_boPhanRa[],
  summary: readonly petitions_taskUnitRowOut[],
): UnitRow[] {
  const byId = new Map(summary.map((r) => [r.org_unit_id, r]));
  const known = new Set(orgUnits.map((u) => u.id));
  const rows: UnitRow[] = orgUnits.map((u) => {
    const s = byId.get(u.id);
    return { key: u.id, name: u.name, kind: "unit", ...(s === undefined ? ZERO : figuresOf(s)) };
  });
  for (const s of summary) {
    if (s.org_unit_id === "") {
      rows.push({ key: "", name: UNASSIGNED_UNIT_LABEL, kind: "unassigned", ...figuresOf(s) });
    } else if (!known.has(s.org_unit_id)) {
      rows.push({ key: s.org_unit_id, name: UNKNOWN_UNIT_LABEL, kind: "unknown", ...figuresOf(s) });
    }
  }
  return rows.sort(
    (a, b) =>
      b.total - a.total ||
      a.name.localeCompare(b.name, "vi") ||
      (a.key < b.key ? -1 : a.key > b.key ? 1 : 0),
  );
}

/**
 * The table's state from its two answers. Either failing fails the table — rows without names, or
 * names without figures, would both misstate a unit. The summary's sentence wins when both fail: it
 * is the route that carries this screen's permission check.
 */
export function unitRowsFrom(
  summary: KetQua<petitions_taskUnitSummaryOut>,
  orgUnits: KetQua<identity_danhSachBoPhanRa>,
): KetQua<UnitRow[]> {
  if (!summary.ok) return summary;
  if (!orgUnits.ok) return orgUnits;
  return { ok: true, duLieu: assembleUnitRows(orgUnits.duLieu.items, summary.duLieu.units) };
}

/**
 * `{n} — {tỷ lệ}%` with a decimal comma (`3 — 33,3%`), or `—` when the unit completed no task with
 * an original deadline: an empty sample has no rate, and printing 0% would call the unit late for
 * doing nothing wrong (`ratioPercent`, the same rule as `/tong-quan`'s on-time figure).
 */
export function onTimeText(row: Pick<UnitRow, "onTime" | "onTimeSample">): string {
  const p = ratioPercent(row.onTime, row.onTimeSample);
  return p === null ? NO_VALUE : `${formatCount(row.onTime)} — ${formatPercent(p)}`;
}
