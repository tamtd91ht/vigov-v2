/**
 * "Cấu hình → Kênh Zalo" (ADR 0074 #6) — the pure half: labels, the form draft, the client-side check.
 * The check is a HINT; comms re-checks every rule and its sentence still reaches the form.
 */

import type { ZaloChannelSettings, ZaloChannelSettingsChange, ZaloReminderKind } from "@/lib/api/zalo";
import { ZALO_REMINDER_KINDS } from "@/lib/api/zalo";

export const ZALO_TAB_TITLE = "Kênh nhắc việc qua Zalo";

export const ZALO_TAB_DESCRIPTION =
  "Gửi nhắc việc cho cán bộ của xã qua Zalo Bot dùng chung của nền tảng, bên cạnh chuông thông báo. Cán bộ tự ghép nối Zalo của mình ở trang Cá nhân.";

/** The four kinds of wave 1, in the order of `ZALO_REMINDER_KINDS`. */
export const KIND_LABEL: Record<ZaloReminderKind, string> = {
  "sap-den-han": "Việc sắp đến hạn",
  "qua-han": "Việc quá hạn",
  "leo-thang": "Việc được chuyển lên cấp trên (leo thang)",
  "ban-tin-tuan": "Bản tin tổng hợp hằng tuần",
};

export const QUIET_HINT = "Trong khoảng giờ này hệ thống không gửi tin Zalo; tin được dời tới cuối khoảng. Giờ Việt Nam.";

export const SAVED_SENTENCE = "Đã lưu cấu hình kênh Zalo.";

export const NO_LINKED_STAFF = "Chưa cán bộ nào của xã ghép nối Zalo.";

export type ZaloChannelDraft = {
  isEnabled: boolean;
  kinds: ZaloReminderKind[];
  quietStart: string;
  quietEnd: string;
  /** Text of the number boxes — "" when empty, so a half-typed value is not turned into 0. */
  lateStartDays: string;
  lateRepeatDays: string;
};

export type ZaloChannelField = "kinds" | "quiet" | "lateStartDays" | "lateRepeatDays";

export function draftFromSettings(s: ZaloChannelSettings): ZaloChannelDraft {
  return {
    isEnabled: s.is_enabled,
    kinds: ZALO_REMINDER_KINDS.filter((k) => s.kinds.includes(k)),
    quietStart: s.quiet_start,
    quietEnd: s.quiet_end,
    lateStartDays: s.overdue_start_after_days === null ? "" : String(s.overdue_start_after_days),
    lateRepeatDays: s.overdue_repeat_every_days === null ? "" : String(s.overdue_repeat_every_days),
  };
}

export function toggleKind(draft: ZaloChannelDraft, kind: ZaloReminderKind, on: boolean): ZaloChannelDraft {
  const next = on ? [...draft.kinds, kind] : draft.kinds.filter((k) => k !== kind);
  // Kept in the fixed order, so the body never depends on click order.
  return { ...draft, kinds: ZALO_REMINDER_KINDS.filter((k) => next.includes(k)) };
}

const HH_MM = /^([01]\d|2[0-3]):[0-5]\d$/;

function positiveWholeDays(text: string): number | null {
  if (!/^\d+$/.test(text.trim())) return null;
  const n = Number(text.trim());
  return Number.isSafeInteger(n) && n >= 1 ? n : null;
}

/**
 * The PUT body, or the first field that is wrong.
 *
 *   - turning the channel ON needs at least one kind (the server refuses an empty list then);
 *   - the `qua-han` kind chosen → both reminder cadences REQUIRED, whole days ≥ 1; not chosen → both
 *     sent as null, so a value typed then un-ticked is not stored behind the operator's back.
 * These are reminder CADENCES set by the commune (ADR 0074), not a deadline: nothing here computes
 * or stores whether a task is late.
 */
export function buildChange(
  d: ZaloChannelDraft,
): { ok: true; change: ZaloChannelSettingsChange } | { ok: false; field: ZaloChannelField; text: string } {
  if (d.isEnabled && d.kinds.length === 0) {
    return { ok: false, field: "kinds", text: "Bật kênh Zalo thì phải chọn ít nhất một loại nhắc việc." };
  }
  if (!HH_MM.test(d.quietStart) || !HH_MM.test(d.quietEnd)) {
    return { ok: false, field: "quiet", text: "Giờ yên lặng phải theo dạng giờ:phút, ví dụ 21:00 và 06:00." };
  }
  const lateKindChosen = d.kinds.includes("qua-han");
  let start: number | null = null;
  let repeat: number | null = null;
  if (lateKindChosen) {
    start = positiveWholeDays(d.lateStartDays);
    if (start === null) {
      return {
        ok: false,
        field: "lateStartDays",
        text: "Đã chọn nhắc việc quá hạn: hãy nhập số ngày (từ 1 trở lên) kể từ khi quá hạn thì bắt đầu nhắc.",
      };
    }
    repeat = positiveWholeDays(d.lateRepeatDays);
    if (repeat === null) {
      return {
        ok: false,
        field: "lateRepeatDays",
        text: "Đã chọn nhắc việc quá hạn: hãy nhập số ngày (từ 1 trở lên) giữa hai lần nhắc.",
      };
    }
  }
  return {
    ok: true,
    change: {
      is_enabled: d.isEnabled,
      kinds: [...d.kinds],
      quiet_start: d.quietStart,
      quiet_end: d.quietEnd,
      overdue_start_after_days: start,
      overdue_repeat_every_days: repeat,
    },
  };
}
