/**
 * The RECORD TABS of a detail panel (owner, 05/10/2026: open A, close, open B — A's tab is still
 * there, B is focused; switch back to A, or ✕ it). Pure functions over an immutable state, so the
 * screen can fold them into its own reducer and the rules are testable without a DOM.
 *
 * Nhiệm vụ is the pilot; Đơn thư and Phản ánh will reuse it — which is why it knows nothing about
 * tasks: a tab is an `id` (the record's business code) and an opaque `data` payload.
 *
 *   open      present → activate it (and refresh `data`); absent → append at the RIGHT, activate
 *   limit     `max` tabs; one more evicts the LEAST RECENTLY USED — never the one being opened
 *   close     the active one closed → the right neighbour, else the left; the last → `active: null`
 *
 * "Used" is a logical clock (`clock`), not `Date.now()`: two opens inside one millisecond must still
 * order, and a clock read from the wall would make the eviction depend on the machine's time.
 */

export type RecordTab<T> = {
  /** The record's business code — what the address bar and the panel's read take. */
  readonly id: string;
  readonly data: T;
  /** The clock value of the last open or activation. Smallest = least recently used. */
  readonly used: number;
};

export type RecordTabsState<T> = {
  readonly tabs: readonly RecordTab<T>[];
  readonly active: string | null;
  readonly clock: number;
};

/** Eight: enough to compare a handful of records, few enough that every title stays readable. */
export const MAX_RECORD_TABS = 8;

export function emptyRecordTabs<T>(): RecordTabsState<T> {
  return { tabs: [], active: null, clock: 0 };
}

/** Drop least-recently-used tabs until `max` remain, never `keep`. */
function evict<T>(tabs: readonly RecordTab<T>[], keep: string | null, max: number): readonly RecordTab<T>[] {
  let out = tabs;
  while (out.length > max) {
    let victim: RecordTab<T> | null = null;
    for (const t of out) {
      if (t.id === keep) continue;
      if (victim === null || t.used < victim.used) victim = t;
    }
    if (victim === null) break;
    const v = victim;
    out = out.filter((t) => t !== v);
  }
  return out;
}

/** Open `id` (append or refresh) and make it the active tab. */
export function openRecordTab<T>(
  s: RecordTabsState<T>,
  id: string,
  data: T,
  max: number = MAX_RECORD_TABS,
): RecordTabsState<T> {
  const clock = s.clock + 1;
  const present = s.tabs.some((t) => t.id === id);
  const tabs = present
    ? s.tabs.map((t) => (t.id === id ? { id, data, used: clock } : t))
    : [...s.tabs, { id, data, used: clock }];
  return { tabs: evict(tabs, id, max), active: id, clock };
}

/** Activate a tab already present. An unknown id changes nothing — it has no label to show. */
export function activateRecordTab<T>(s: RecordTabsState<T>, id: string): RecordTabsState<T> {
  if (!s.tabs.some((t) => t.id === id)) return s;
  const clock = s.clock + 1;
  return { tabs: s.tabs.map((t) => (t.id === id ? { ...t, used: clock } : t)), active: id, clock };
}

/** Replace a tab's payload in place, without activating it or touching its recency. */
export function updateRecordTab<T>(s: RecordTabsState<T>, id: string, data: T): RecordTabsState<T> {
  if (!s.tabs.some((t) => t.id === id)) return s;
  return { ...s, tabs: s.tabs.map((t) => (t.id === id ? { ...t, data } : t)) };
}

/**
 * The record behind tab `from` now answers to `to` (a renamed register code): same position, same
 * recency. A tab already open under `to` merges into this one — two tabs for one record would be
 * two views of it that disagree.
 */
export function renameRecordTab<T>(
  s: RecordTabsState<T>,
  from: string,
  to: string,
  data: T,
): RecordTabsState<T> {
  if (from === to) return updateRecordTab(s, to, data);
  if (!s.tabs.some((t) => t.id === from)) return s;
  const tabs = s.tabs
    .filter((t) => t.id !== to)
    .map((t) => (t.id === from ? { id: to, data, used: t.used } : t));
  const active = s.active === from || s.active === to ? to : s.active;
  return { ...s, tabs, active };
}

/** Close `id`. Closing the active tab activates its right neighbour, else its left one. */
export function closeRecordTab<T>(s: RecordTabsState<T>, id: string): RecordTabsState<T> {
  const i = s.tabs.findIndex((t) => t.id === id);
  if (i < 0) return s;
  const tabs = s.tabs.filter((t) => t.id !== id);
  if (s.active !== id) return { ...s, tabs };
  const next = tabs[i] ?? tabs[i - 1] ?? null;
  if (next === null) return { ...s, tabs, active: null };
  return activateRecordTab({ ...s, tabs }, next.id);
}

export function closeAllRecordTabs<T>(s: RecordTabsState<T>): RecordTabsState<T> {
  return { tabs: [], active: null, clock: s.clock };
}

/**
 * Fold tabs read back from storage under the ones already open. What is open NOW wins on a
 * conflict (it was opened after the stored copy was written) and keeps the active tab; the stored
 * ones go to the left in their stored order. Recency is rebased so every current tab stays newer
 * than every stored one, then the limit applies.
 */
export function mergeRecordTabs<T>(
  s: RecordTabsState<T>,
  stored: readonly RecordTab<T>[],
  max: number = MAX_RECORD_TABS,
): RecordTabsState<T> {
  const current = new Set(s.tabs.map((t) => t.id));
  const seen = new Set<string>();
  const older: RecordTab<T>[] = [];
  // Stored order by recency → clock values 1..n, current tabs shifted above them.
  const byUse = [...stored].sort((a, b) => a.used - b.used);
  const rank = new Map(byUse.map((t, k) => [t.id, k + 1]));
  for (const t of stored) {
    if (current.has(t.id) || seen.has(t.id)) continue;
    seen.add(t.id);
    older.push({ id: t.id, data: t.data, used: rank.get(t.id) ?? 0 });
  }
  const base = stored.length;
  const newer = s.tabs.map((t) => ({ ...t, used: t.used + base }));
  const tabs = evict([...older, ...newer], s.active, max);
  return { tabs, active: s.active, clock: s.clock + base };
}

/**
 * sessionStorage, read and written so that NO FAILURE reaches the screen: storage disabled, full,
 * blocked by policy, or holding something unparsable all read as "no stored tabs". The tabs are a
 * convenience; losing them must never cost the officer the record they are opening.
 *
 * `parse` validates every stored entry and returns `null` for one it refuses — what comes back from
 * storage is input like any other (a value another script wrote is not trusted).
 */
export function readStoredRecordTabs<T>(
  key: string,
  parse: (data: unknown) => T | null,
): RecordTab<T>[] {
  try {
    const raw = window.sessionStorage.getItem(key);
    if (raw === null) return [];
    const value: unknown = JSON.parse(raw);
    if (!Array.isArray(value)) return [];
    const out: RecordTab<T>[] = [];
    for (const item of value as unknown[]) {
      if (typeof item !== "object" || item === null) continue;
      const { id, data, used } = item as Record<string, unknown>;
      if (typeof id !== "string" || id === "" || typeof used !== "number" || !Number.isFinite(used)) continue;
      const parsed = parse(data);
      if (parsed !== null) out.push({ id, data: parsed, used });
    }
    return out.slice(-MAX_RECORD_TABS);
  } catch {
    return [];
  }
}

export function writeStoredRecordTabs<T>(key: string, tabs: readonly RecordTab<T>[]): void {
  try {
    if (tabs.length === 0) window.sessionStorage.removeItem(key);
    else window.sessionStorage.setItem(key, JSON.stringify(tabs));
  } catch {
    // Quota, privacy mode, a policy: the tabs simply do not survive a reload.
  }
}
