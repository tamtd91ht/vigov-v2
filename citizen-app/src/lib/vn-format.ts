/**
 * DATES THE WAY THE CUSTOMER PROTOTYPE PRINTS THEM — `../vigov-require/apps/miniapp/src/lib/format.ts`
 * (owner 11/10/2026: "làm hết theo prototype"). Used by the commune app's screens; the shared app keeps
 * `thoi-diem.ts`'s "dd/MM/yyyy HH:mm".
 *
 *   formatDate       "24/8/2026"            — no leading zeros, as a Vietnamese reader writes a date
 *   formatDateTime   "14:30 ngày 22/8/2026"
 *   formatDeadline   "15h ngày 2/10"        — the administrative way of naming a deadline; "15h30 ngày 2/10"
 *                                             when the minutes are not zero (the prototype drops them, which
 *                                             would name a deadline earlier than the one the commune committed
 *                                             to — rule 10: a deadline is a public authority's commitment)
 *   formatDayGroup   "Hôm nay" · "Hôm qua" · "Thứ Ba, 22/8/2026"
 *
 * PINNED TO +07, like `thoi-diem.ts` and for the same reason: a phone set to another time zone must not move
 * a commune's deadline by hours (Vietnam has no daylight saving, so +07 is exact all year). No `Intl` time-zone
 * data is relied on. Every function returns `null` for an unreadable input — the caller has its own words for
 * that case, never "Invalid Date". PURE; nothing here reads the clock unless the caller passes it.
 */
const VN_OFFSET_MS = 7 * 60 * 60 * 1000;

type VnParts = { year: number; month: number; day: number; hour: number; minute: number };

function vnParts(iso: string): VnParts | null {
  const ms = new Date(iso).getTime();
  if (Number.isNaN(ms)) return null;
  const t = new Date(ms + VN_OFFSET_MS);
  return {
    year: t.getUTCFullYear(),
    month: t.getUTCMonth() + 1,
    day: t.getUTCDate(),
    hour: t.getUTCHours(),
    minute: t.getUTCMinutes(),
  };
}

const two = (n: number) => String(n).padStart(2, "0");

/** "24/8/2026", or `null`. */
export function formatDate(iso: string): string | null {
  const p = vnParts(iso);
  return p === null ? null : `${p.day}/${p.month}/${p.year}`;
}

/** "14:30 ngày 22/8/2026", or `null`. */
export function formatDateTime(iso: string): string | null {
  const p = vnParts(iso);
  return p === null ? null : `${two(p.hour)}:${two(p.minute)} ngày ${p.day}/${p.month}/${p.year}`;
}

/**
 * "15h ngày 2/10" — or "15h30 ngày 2/10" when the minutes are not zero — or `null`. ONLY ever called with a
 * deadline the SERVER stored (rule 10 #2): this formats a commitment, it never computes one.
 */
export function formatDeadline(iso: string): string | null {
  const p = vnParts(iso);
  if (p === null) return null;
  const time = p.minute === 0 ? `${p.hour}h` : `${p.hour}h${two(p.minute)}`;
  return `${time} ngày ${p.day}/${p.month}`;
}

/** The weekday names `Intl.DateTimeFormat("vi-VN", { weekday: "long" })` prints, Sunday first. */
const WEEKDAYS = ["Chủ Nhật", "Thứ Hai", "Thứ Ba", "Thứ Tư", "Thứ Năm", "Thứ Sáu", "Thứ Bảy"] as const;

/** A `YYYY-MM-DD` day as UTC midnight, or `null` for a malformed or impossible day. */
function dayMs(day: string): number | null {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(day);
  if (m === null) return null;
  const [y, mo, d] = [Number(m[1]), Number(m[2]), Number(m[3])];
  const ms = Date.UTC(y, mo - 1, d);
  const t = new Date(ms);
  return t.getUTCFullYear() === y && t.getUTCMonth() === mo - 1 && t.getUTCDate() === d ? ms : null;
}

/**
 * The heading of one day's group on a list (the prototype's broadcast library): "Hôm nay", "Hôm qua", or
 * "Thứ Ba, 22/8/2026". Both arguments are DAYS (`YYYY-MM-DD`, Vietnam's calendar), so no hour is ever claimed.
 * `null` for an unreadable day.
 */
export function formatDayGroup(day: string, today: string): string | null {
  const ms = dayMs(day);
  if (ms === null) return null;
  const now = dayMs(today);
  if (now !== null) {
    const diff = Math.round((now - ms) / 86_400_000);
    if (diff === 0) return "Hôm nay";
    if (diff === 1) return "Hôm qua";
  }
  const t = new Date(ms);
  return `${WEEKDAYS[t.getUTCDay()]}, ${t.getUTCDate()}/${t.getUTCMonth() + 1}/${t.getUTCFullYear()}`;
}
