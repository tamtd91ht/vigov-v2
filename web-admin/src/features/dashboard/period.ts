/**
 * Reporting periods of the leadership overview — pure: no network, no DOM, and the clock is a
 * PARAMETER, never read here.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * EVERY BOUNDARY IS COMPUTED IN Asia/Ho_Chi_Minh, NEVER IN THE MACHINE'S LOCAL TIME.
 *
 * "This month" is the commune's month. `new Date(y, m, 1)` builds midnight in whatever zone the
 * browser — or the CI runner, pinned to UTC in `vitest.config.mts` — happens to be in, and 00:00
 * UTC is 07:00 in Viet Nam: every period would silently start seven hours late, dropping the first
 * morning of the month from every figure. Nothing on screen would look wrong.
 *
 * The zone has had no daylight-saving time since 1975, so a fixed +07:00 offset IS the zone, and
 * every instant here is shifted by it explicitly, then read with the UTC getters. `Date.UTC` also
 * normalises month and day overflow (month -1, day 0), which is what makes "previous month" of
 * January and "Monday of this week" on the 1st correct without a branch.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * WALL-CLOCK ARITHMETIC HERE IS FOR REPORTING PERIODS ONLY — never for a deadline. Deadlines count
 * working hours and belong to identity (rule 10, forbidden #2, ADR 0007); this file never computes
 * how late anything is.
 */

/** The four kinds of the period picker (spec §3). Default: month. */
export type PeriodKind = "week" | "month" | "quarter" | "year";

export const PERIOD_KINDS: readonly PeriodKind[] = ["week", "month", "quarter", "year"];

export const DEFAULT_PERIOD_KIND: PeriodKind = "month";

/** Button text of the picker, verbatim spec §3. */
export const PERIOD_BUTTON_LABEL: Readonly<Record<PeriodKind, string>> = {
  week: "Tuần này",
  month: "Tháng này",
  quarter: "Quý này",
  year: "Năm nay",
};

/** How the meta line names the period: `Kỳ tháng này: …`. */
const PERIOD_META_LABEL: Readonly<Record<PeriodKind, string>> = {
  week: "tuần này",
  month: "tháng này",
  quarter: "quý này",
  year: "năm nay",
};

/** The business zone's offset. Fixed: no DST in Viet Nam — see the header. */
const ZONE_OFFSET_MS = 7 * 60 * 60 * 1000;
const ZONE_OFFSET_TEXT = "+07:00";

type WallParts = {
  readonly year: number;
  /** 0-based, as `Date` counts */
  readonly month: number;
  readonly day: number;
  /** 0 = Sunday … 6 = Saturday */
  readonly weekday: number;
  readonly hour: number;
  readonly minute: number;
  readonly second: number;
};

function wallParts(instant: Date): WallParts {
  const w = new Date(instant.getTime() + ZONE_OFFSET_MS);
  return {
    year: w.getUTCFullYear(),
    month: w.getUTCMonth(),
    day: w.getUTCDate(),
    weekday: w.getUTCDay(),
    hour: w.getUTCHours(),
    minute: w.getUTCMinutes(),
    second: w.getUTCSeconds(),
  };
}

/** 00:00 in Viet Nam of the given wall date, as an instant. Overflowing fields normalise. */
function zoneMidnight(year: number, month: number, day: number): number {
  return Date.UTC(year, month, day) - ZONE_OFFSET_MS;
}

export type Bounds = {
  /** first instant of the period */
  readonly start: number;
  /** first instant AFTER the period — half-open */
  readonly end: number;
};

/** The period of `kind` that contains `now`, and the start of the one before it. */
function boundsAt(kind: PeriodKind, now: Date): { current: Bounds; previousStart: number } {
  const p = wallParts(now);
  switch (kind) {
    case "week": {
      // Monday starts the week (user decision). `(weekday + 6) % 7` is days since Monday.
      const monday = p.day - ((p.weekday + 6) % 7);
      return {
        current: {
          start: zoneMidnight(p.year, p.month, monday),
          end: zoneMidnight(p.year, p.month, monday + 7),
        },
        previousStart: zoneMidnight(p.year, p.month, monday - 7),
      };
    }
    case "month":
      return {
        current: {
          start: zoneMidnight(p.year, p.month, 1),
          end: zoneMidnight(p.year, p.month + 1, 1),
        },
        previousStart: zoneMidnight(p.year, p.month - 1, 1),
      };
    case "quarter": {
      const first = p.month - (p.month % 3);
      return {
        current: {
          start: zoneMidnight(p.year, first, 1),
          end: zoneMidnight(p.year, first + 3, 1),
        },
        previousStart: zoneMidnight(p.year, first - 3, 1),
      };
    }
    case "year":
      return {
        current: { start: zoneMidnight(p.year, 0, 1), end: zoneMidnight(p.year + 1, 0, 1) },
        previousStart: zoneMidnight(p.year - 1, 0, 1),
      };
  }
}

/** What the screen asks the servers, and what it tells the reader it asked. */
export type PeriodWindows = {
  readonly kind: PeriodKind;
  /** The whole period, for the meta line: `1/9/2026 – 30/9/2026`. */
  readonly period: Bounds;
  /** `[period.start, now)` — what the current figures count. */
  readonly current: Bounds;
  /**
   * `[previous start, previous start + elapsed)` — the SAME ELAPSED LENGTH of the previous period
   * of the same kind (user decision). Comparing a whole August with the first 28 days of September
   * would report every period figure as falling.
   */
  readonly previous: Bounds;
};

/**
 * The current and comparison windows at instant `now`.
 *
 * `current.end` IS `now` ROUNDED UP TO THE NEXT WHOLE SECOND. Two reasons: the value goes on the
 * wire as RFC 3339 without fractions, and a window must never be empty — the servers refuse
 * `from >= to` with 400, which at the first instant of a period would blank the whole page. The
 * extra fraction of a second counts nothing that exists yet.
 *
 * `previous.end` IS CAPPED AT THE CURRENT PERIOD'S START. 30 days into March is past the end of
 * February; without the cap the "previous" window would overlap the current one and count the
 * same records twice.
 */
export function periodWindows(kind: PeriodKind, now: Date): PeriodWindows {
  const { current: period, previousStart } = boundsAt(kind, now);
  const roundedUp = Math.floor(now.getTime() / 1000) * 1000 + 1000;
  const currentEnd = Math.min(Math.max(roundedUp, period.start + 1000), period.end);
  const elapsed = currentEnd - period.start;
  return {
    kind,
    period,
    current: { start: period.start, end: currentEnd },
    previous: { start: previousStart, end: Math.min(previousStart + elapsed, period.start) },
  };
}

function pad2(n: number): string {
  return String(n).padStart(2, "0");
}

/** `2026-09-01T00:00:00+07:00` — RFC 3339 with the zone's offset, the shape the servers parse. */
export function toRfc3339(instant: number): string {
  const p = wallParts(new Date(instant));
  return (
    `${p.year}-${pad2(p.month + 1)}-${pad2(p.day)}` +
    `T${pad2(p.hour)}:${pad2(p.minute)}:${pad2(p.second)}${ZONE_OFFSET_TEXT}`
  );
}

/** A window as the query pair the summaries take. */
export function toQueryPeriod(b: Bounds): { from: string; to: string } {
  return { from: toRfc3339(b.start), to: toRfc3339(b.end) };
}

/** `1/9/2026` — the wall date in Viet Nam, unpadded as spec §2 prints it. */
export function formatDate(instant: number): string {
  const p = wallParts(new Date(instant));
  return `${p.day}/${p.month + 1}/${p.year}`;
}

/** `16:43 07/09/2026` — spec §3's `HH:mm dd/MM/yyyy`, in Viet Nam. */
export function formatDateTime(instant: number): string {
  const p = wallParts(new Date(instant));
  return `${pad2(p.hour)}:${pad2(p.minute)} ${pad2(p.day)}/${pad2(p.month + 1)}/${p.year}`;
}

/** The year, in Viet Nam, of an instant — the fiscal block is year-to-date. */
export function zoneYear(instant: number): number {
  return wallParts(new Date(instant)).year;
}

/**
 * `Kỳ tháng này: 1/9/2026 – 30/9/2026`. The last day is the day of the last instant INSIDE the
 * half-open period, not the day of `end` (which is the next period's first day).
 */
export function periodMetaLabel(w: PeriodWindows): string {
  return (
    `Kỳ ${PERIOD_META_LABEL[w.kind]}: ${formatDate(w.period.start)} – ` +
    formatDate(w.period.end - 1)
  );
}

/** States the comparison window, so the reader knows what "kỳ trước" means. */
export function comparisonNote(w: PeriodWindows): string {
  return (
    "So với cùng khoảng thời gian đã trôi qua của kỳ trước: " +
    `${formatDateTime(w.previous.start)} – ${formatDateTime(w.previous.end)}.`
  );
}
