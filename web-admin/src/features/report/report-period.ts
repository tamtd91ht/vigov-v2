/**
 * `/bao-cao` periods — the four named periods of `/tong-quan`, UNCHANGED, plus "Tuỳ chọn".
 *
 * Pure: no network, no DOM, and the clock is a PARAMETER. Every boundary is in Asia/Ho_Chi_Minh for
 * the reason `features/dashboard/period.ts` states in its header; this file adds no arithmetic of its
 * own for the named periods — it hands them to `periodWindows`, so `/bao-cao` and `/tong-quan` ask
 * the servers the very same query for the same period (spec 13 §10).
 *
 * THE CUSTOM PERIOD (ADR 0053, amendment 04/10/2026, B2 and B5e — a vendor choice, not the
 * customer's): "Từ ngày" D1 – "Đến ngày" D2 INCLUDES BOTH DAYS, i.e. `[D1 00:00, D2+1 00:00)` in
 * Viet Nam, N days; the comparison window is the N days immediately before, `[D1 − N days, D1)`.
 * Somebody who picks "đến ngày 17" means the 17th is in; half-open keeps the `[from, to)` rule of
 * every summary route. A day is exactly 24 h here because the zone has no daylight-saving time.
 *
 * Wall-clock arithmetic is for REPORTING PERIODS only — never a deadline (rule 10, forbidden #2).
 */

import {
  comparisonNote,
  formatDate,
  periodMetaLabel,
  periodWindows,
  toRfc3339,
} from "@/features/dashboard/period";
import type { Bounds, PeriodKind, PeriodWindows } from "@/features/dashboard/period";

const DAY_MS = 24 * 60 * 60 * 1000;

/** The picker's fifth option. Not a `PeriodKind`: `/tong-quan` has no such period. */
export const CUSTOM_PERIOD = "custom";

export const CUSTOM_BUTTON_LABEL = "Tuỳ chọn";

/** What the reader picked: a named period, or a validated day range. */
export type ReportSelection =
  | { readonly kind: PeriodKind }
  | { readonly kind: typeof CUSTOM_PERIOD; readonly from: string; readonly to: string };

/** A custom period's windows. `period` = `current`: the range is whole days, nothing is "elapsed". */
export type CustomWindows = {
  readonly kind: typeof CUSTOM_PERIOD;
  readonly period: Bounds;
  readonly current: Bounds;
  readonly previous: Bounds;
  /** N — the length of both windows, in days */
  readonly days: number;
};

export type ReportWindows = PeriodWindows | CustomWindows;

/** `2026-09-17` — the wall date in Viet Nam, the value a native `<input type="date">` holds. */
export function toDateInputValue(instant: number): string {
  return toRfc3339(instant).slice(0, 10);
}

const DATE_INPUT = /^\d{4}-\d{2}-\d{2}$/;

/**
 * 00:00 in Viet Nam of a `YYYY-MM-DD` date, or `null` for anything that is not a real calendar day.
 * The round trip refuses dates `Date` would silently roll over (`2026-02-30` → 2 March).
 */
export function zoneDayStart(value: string): number | null {
  if (!DATE_INPUT.test(value)) return null;
  const instant = Date.parse(`${value}T00:00:00+07:00`);
  if (Number.isNaN(instant) || toDateInputValue(instant) !== value) return null;
  return instant;
}

export const MISSING_DATES = "Chọn đủ Từ ngày và Đến ngày.";
export const INVALID_DATE = "Ngày không hợp lệ. Nhập theo dạng ngày/tháng/năm.";
export const REVERSED_RANGE = "Từ ngày phải trước hoặc trùng Đến ngày.";

/**
 * The custom windows of `[from, to]` (both `YYYY-MM-DD`, both days included), or the sentence that
 * says why the range is refused. A refused range is NEVER sent: the caller has no windows to send.
 */
export function customWindows(
  from: string,
  to: string,
): { ok: true; windows: CustomWindows } | { ok: false; message: string } {
  if (from.trim() === "" || to.trim() === "") return { ok: false, message: MISSING_DATES };
  const start = zoneDayStart(from);
  const lastDay = zoneDayStart(to);
  if (start === null || lastDay === null) return { ok: false, message: INVALID_DATE };
  if (start > lastDay) return { ok: false, message: REVERSED_RANGE };
  const end = lastDay + DAY_MS;
  const days = Math.round((end - start) / DAY_MS);
  const current: Bounds = { start, end };
  return {
    ok: true,
    windows: {
      kind: CUSTOM_PERIOD,
      period: current,
      current,
      previous: { start: start - days * DAY_MS, end: start },
      days,
    },
  };
}

/**
 * The windows of a selection. A named period is `periodWindows` VERBATIM — the shared code path is
 * what keeps the two pages' figures equal. A custom selection is assumed validated (it was built
 * from `customWindows`); an invalid one falls back to NOTHING — it throws, because a silent default
 * period here would put figures for a period nobody chose on a report.
 */
export function reportWindows(selection: ReportSelection, now: Date): ReportWindows {
  if (selection.kind !== CUSTOM_PERIOD) return periodWindows(selection.kind, now);
  const r = customWindows(selection.from, selection.to);
  if (!r.ok) throw new Error(`reportWindows: refused custom range — ${r.message}`);
  return r.windows;
}

/** `Kỳ tháng này: 1/9/2026 – 30/9/2026` · `Kỳ tuỳ chọn: 1/9/2026 – 17/9/2026 (17 ngày)`. */
export function reportPeriodLabel(w: ReportWindows): string {
  if (w.kind !== CUSTOM_PERIOD) return periodMetaLabel(w);
  return `Kỳ tuỳ chọn: ${formatDate(w.period.start)} – ${formatDate(w.period.end - 1)} (${w.days} ngày)`;
}

/**
 * What the comparison is (ADR 0053 B5a): a named period keeps `/tong-quan`'s sentence (same kind of
 * period, same elapsed portion); a custom one compares with the same number of days just before.
 */
export function reportComparisonNote(w: ReportWindows): string {
  if (w.kind !== CUSTOM_PERIOD) return comparisonNote(w);
  return (
    `So với cùng số ngày liền trước (${w.days} ngày): ` +
    `${formatDate(w.previous.start)} – ${formatDate(w.previous.end - 1)}.`
  );
}
