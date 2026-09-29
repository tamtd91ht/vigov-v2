/**
 * Merging five services' trails into one list, newest first — ADR 0054 §6–§7. Pure functions, no
 * network, no DOM: the three cases that matter most (order, the safe boundary, one service failing)
 * are exactly the ones nobody sees on a screen where all five answer quickly.
 *
 * THE SAFE BOUNDARY, and why a row may be loaded and still held back.
 *
 *   Service A has loaded down to 10:00 and has more; service B has loaded down to 08:00. B's 09:00
 *   row is loaded — but A may still hold a 09:30 row on its next page. Showing B's 09:00 now puts it
 *   ABOVE a row that is newer, and the order of a trail read by an inspector is not cosmetic. So a
 *   row is shown only when it is not older than the last loaded row of EVERY source that still has
 *   more — i.e. the NEWEST of those frontiers. Held rows stay in memory and appear once "Xem thêm"
 *   moves that frontier down. The source holding the boundary is the one "Xem thêm" loads.
 *
 * A FAILED SOURCE LEAVES THE BOUNDARY (§7) and is named on screen: its unknown rows cannot hold
 * everyone else back forever, and the visible error line is what keeps the partial list from reading
 * as complete. What it had already loaded stays shown — those rows are true.
 */

import type { KetQua } from "@/lib/api/request";
import type { audit_EntryView, page_Result_audit_EntryView } from "@/lib/api/schema.gen";
import { AUDIT_SOURCES, type AuditSourceKey } from "@/lib/api/audit-entries";

export type SourceState = {
  readonly key: AuditSourceKey;
  /** Every row loaded so far, in the server's order (`at DESC, id DESC`). Only ever appended to. */
  readonly items: readonly audit_EntryView[];
  /** That service's `next_cursor`; "" before the first page. */
  readonly cursor: string;
  /** `true` before the first page answers: an unanswered source may hold anything. */
  readonly hasMore: boolean;
  /** The server's sentence when the LAST request to this source failed, else `null`. */
  readonly error: string | null;
  readonly loading: boolean;
};

export type MergedRow = {
  readonly source: AuditSourceKey;
  readonly entry: audit_EntryView;
  /** Stable React key: rows are only appended, so a source's index never changes. */
  readonly rowKey: string;
};

export function initialSources(): SourceState[] {
  return AUDIT_SOURCES.map((s) => ({
    key: s.key,
    items: [],
    cursor: "",
    hasMore: true,
    error: null,
    loading: true,
  }));
}

/** Mark one source as loading (a "Xem thêm" or a retry), keeping everything it has. */
export function markLoading(sources: readonly SourceState[], key: AuditSourceKey): SourceState[] {
  return sources.map((s) => (s.key === key ? { ...s, loading: true } : s));
}

/**
 * Fold one page's answer into its source. Success appends and takes the new cursor; failure keeps
 * the rows and the cursor (so a retry asks for the SAME page again) and records the sentence.
 */
export function applyPage(
  sources: readonly SourceState[],
  key: AuditSourceKey,
  result: KetQua<page_Result_audit_EntryView>,
): SourceState[] {
  return sources.map((s) => {
    if (s.key !== key) return s;
    if (!result.ok) return { ...s, loading: false, error: result.thongBao };
    return {
      ...s,
      items: [...s.items, ...result.duLieu.items],
      cursor: result.duLieu.next_cursor,
      // `has_more` with an empty cursor could never be followed; treat it as the end rather than
      // re-reading page one forever.
      hasMore: result.duLieu.has_more && result.duLieu.next_cursor !== "",
      error: null,
      loading: false,
    };
  });
}

/**
 * An `at` as a comparable pair: epoch milliseconds, then the sub-millisecond digits Go writes
 * (RFC 3339 with nanoseconds). Two rows 300 µs apart in two services must not compare as equal at
 * the boundary. An unreadable `at` sorts oldest — it is a contract fault, never a reason to jump
 * ahead of real rows.
 */
type AtKey = readonly [number, number];

export function atKey(at: string): AtKey {
  const ms = Date.parse(at);
  if (Number.isNaN(ms)) return [Number.NEGATIVE_INFINITY, 0];
  const frac = /\.(\d+)/.exec(at)?.[1] ?? "";
  const subMs = Number((frac.slice(3, 9) + "000000").slice(0, 6));
  return [ms, subMs];
}

/** Negative when `a` is OLDER than `b`. */
export function compareAt(a: AtKey, b: AtKey): number {
  if (a[0] !== b[0]) return a[0] < b[0] ? -1 : 1;
  return a[1] - b[1];
}

export type Boundary =
  /** No source with more to load: every loaded row is safe. */
  | { readonly kind: "none" }
  /** A source that has more but has not answered yet: nothing is safe. */
  | { readonly kind: "blocked"; readonly holder: AuditSourceKey }
  | { readonly kind: "at"; readonly at: AtKey; readonly holder: AuditSourceKey };

/** The newest frontier among the sources that still have more and did not fail. */
export function safeBoundary(sources: readonly SourceState[]): Boundary {
  let best: Boundary = { kind: "none" };
  for (const s of sources) {
    if (s.error !== null || !s.hasMore) continue;
    const last = s.items[s.items.length - 1];
    if (last === undefined) return { kind: "blocked", holder: s.key };
    const at = atKey(last.at);
    if (best.kind === "none" || (best.kind === "at" && compareAt(at, best.at) > 0)) {
      best = { kind: "at", at, holder: s.key };
    }
  }
  return best;
}

/** The rows safe to show, newest first. Ties keep source order, then each server's own order. */
export function visibleRows(sources: readonly SourceState[]): MergedRow[] {
  const boundary = safeBoundary(sources);
  if (boundary.kind === "blocked") return [];
  const all: { row: MergedRow; at: AtKey }[] = [];
  for (const s of sources) {
    s.items.forEach((entry, i) => {
      all.push({ row: { source: s.key, entry, rowKey: `${s.key}:${i}` }, at: atKey(entry.at) });
    });
  }
  const kept =
    boundary.kind === "none" ? all : all.filter((x) => compareAt(x.at, boundary.at) >= 0);
  // Array.prototype.sort is stable: equal instants keep the push order above.
  kept.sort((x, y) => compareAt(y.at, x.at));
  return kept.map((x) => x.row);
}

/** Which source "Xem thêm" loads next — the one holding the boundary — or `null` when none can. */
export function nextToLoad(sources: readonly SourceState[]): AuditSourceKey | null {
  const b = safeBoundary(sources);
  return b.kind === "none" ? null : b.holder;
}

/** Rows loaded but held back by the boundary — said on screen so "Xem thêm" is not a mystery. */
export function heldBackCount(sources: readonly SourceState[]): number {
  const loaded = sources.reduce((n, s) => n + s.items.length, 0);
  return loaded - visibleRows(sources).length;
}
