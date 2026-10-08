/**
 * "Cấu hình → Kênh Zalo" (ADR 0074 #6, laid out by spec 11 under ADR 0079) — the pure half: the
 * words, the event table, the draft, the client-side check. The check is a HINT; comms re-checks every
 * rule (`service-comms/internal/domain/zalo_link.go` NormalizeZaloChannelSetting) and its sentence
 * still reaches the screen.
 */

import type { ZaloChannelSettings, ZaloChannelSettingsChange, ZaloReminderKind } from "@/lib/api/zalo";
import { ZALO_REMINDER_KINDS } from "@/lib/api/zalo";

export const NO_LINKED_STAFF = "Chưa cán bộ nào của xã ghép nối Zalo.";

/** Toasts of the autosave (spec 11, top). */
export const SAVED_TOAST = "Đã lưu.";
export const SAVE_FAILED_TOAST = "Chưa lưu được. Vui lòng thử lại.";

/**
 * Spec 11 §1, "song song với chuông và thư" with "và thư" DROPPED: a reminder goes to the bell and,
 * since ADR 0074, to Zalo — no producer mails it (`service-comms/internal/app/staff_notification.go`
 * Deliver writes the bell row and the Zalo outbox, nothing else). A sentence promising an e-mail that
 * never comes is the authority telling its staff something false.
 */
export const CHANNEL_DESCRIPTION =
  "Bật rồi thì cán bộ nào đã ghép nối tài khoản sẽ nhận nhắc việc trong Zalo, song song với chuông. Cán bộ chưa ghép nối không bị ảnh hưởng gì.";

/** Spec 11 §1 hint, "ViGov" → "hệ thống" (ADR 0068 §13: no product name in the body of a screen). */
export const QUIET_HINT = "Trong khoảng này hệ thống không nhắn Zalo; chuông trong web vẫn kêu.";

/**
 * Spec 11 §2 with "đổi ở đây là đổi cho cả hai" DROPPED (ADR 0079 Q6, "giữ spec, chỉ bỏ phần sai"):
 * the threshold is read from the SLA column and cannot be changed on this tab.
 */
export const WHEN_DESCRIPTION =
  "Ngưỡng “sắp đến hạn” dùng chung với cái chuông và danh sách “Sắp đến hạn”, nên con số trong tin nhắn luôn bằng con số trên màn hình.";

/** Spec 11 §2, "và thư" dropped for the reason given on `CHANNEL_DESCRIPTION`. */
export const EVENTS_HINT = "Bỏ chọn thì việc đó vẫn vào chuông như cũ, chỉ không nhắn Zalo.";

/**
 * One row of spec 11's event table. `kind` is the server kind that ACTUALLY sends this event today,
 * or null when nothing does (drawn disabled with "?"). Several rows share one kind, hence one
 * checkbox state — what was found in the producers on 08/10/2026:
 *   - `sap-den-han`: the daily per-person digest of tasks, petitions (service-petitions
 *     `domain.DueSoonNotice`) and incoming documents (service-documents `slaReminders`);
 *   - `qua-han`: the same three, AND "unit has named nobody past the threshold"
 *     (`domain.UnassignedNotice` is sent with kind OVERDUE — comms has no kind of its own for it);
 *   - `leo-thang`: escalation of those three; the spec's one escalation row is the task one;
 *   - `ban-tin-tuan`: the Monday digest.
 */
export type ZaloEventRow = { code: string; label: string; hint?: string; kind: ZaloReminderKind | null };

export type ZaloEventGroup = { title: string; events: readonly ZaloEventRow[] };

export const ZALO_EVENT_GROUPS: readonly ZaloEventGroup[] = [
  {
    title: "Nhiệm vụ",
    events: [
      { code: "task.assigned", label: "Giao việc mới", hint: "Nhắn ngay cho người được giao.", kind: null },
      {
        code: "task.due_soon",
        label: "Việc sắp đến hạn",
        hint: "Gộp thành một bản tin mỗi sáng, theo ngưỡng bên dưới.",
        kind: "sap-den-han",
      },
      { code: "task.overdue", label: "Việc quá hạn", hint: "Theo nhịp đặt ở phần dưới.", kind: "qua-han" },
      {
        code: "task.escalated",
        label: "Việc bị đôn đốc lên cấp trên",
        hint: "Lãnh đạo nhận khi việc trễ quá lâu.",
        kind: "leo-thang",
      },
      { code: "task.unassigned_too_long", label: "Bộ phận chưa cử người làm", kind: "qua-han" },
      {
        code: "task.extension_requested",
        label: "Đề nghị lùi hạn chờ duyệt",
        hint: "Gửi cho lãnh đạo giao việc.",
        kind: null,
      },
      { code: "task.approval_requested", label: "Việc chờ duyệt", kind: null },
      {
        code: "task.comment_mention",
        label: "Được nhắc tên trong trao đổi",
        hint: "Loại này nổ nhiều; bật khi xã thật sự cần.",
        kind: null,
      },
    ],
  },
  {
    title: "Văn bản và đơn thư",
    events: [
      {
        code: "document.transferred",
        label: "Văn bản, đơn thư chuyển tới mình",
        hint: "Gửi ngay lúc văn thư phân công cho cán bộ xử lý.",
        kind: null,
      },
      { code: "document.due_soon", label: "Văn bản, đơn thư sắp đến hạn", kind: "sap-den-han" },
      { code: "document.overdue", label: "Văn bản, đơn thư quá hạn", kind: "qua-han" },
    ],
  },
  {
    title: "Phản ánh người dân",
    events: [
      { code: "feedback.assigned", label: "Phản ánh được phân công", kind: null },
      { code: "feedback.due_soon", label: "Phản ánh sắp đến hạn", kind: "sap-den-han" },
      { code: "feedback.overdue", label: "Phản ánh quá hạn", kind: "qua-han" },
      {
        code: "feedback.reopened",
        label: "Phản ánh bị mở lại",
        hint: "Người dân chấm một hoặc hai sao thì phiếu mở lại.",
        kind: null,
      },
    ],
  },
  {
    title: "Khác",
    events: [
      { code: "announcement.published", label: "Thông báo mới gửi cho mình", kind: null },
      { code: "report.ready", label: "Báo cáo điều hành đã sẵn sàng", kind: null },
      { code: "digest.weekly", label: "Bản tin đầu tuần", kind: "ban-tin-tuan" },
    ],
  },
];

/** One person of `GET /api/v1/staff-directory` — exactly the four fields it returns, no phone, no e-mail. */
export type DirectoryPerson = { code: string; full_name: string; position: string; department_id: string };

/** One unit of `GET /api/v1/org-units`, only what the join reads. */
export type UnitName = { id: string; name: string };

/** One line of "Ai đã ghép nối". `detail` is "chức danh · bộ phận" (prototype `ZaloChannelPanel.tsx:235`). */
export type StaffLinkRow = { code: string; name: string; detail: string; linked: boolean };

/**
 * The staff directory joined with the linked list by STAFF CODE (the only key both carry; comms keys
 * its links by business code, identity returns it as `code`).
 *
 * The directory holds only people with a live account (`service-identity/internal/store/
 * can_bo_chon_nguoi.go:57`: not deleted, has an account, active). A link whose person is NOT in it —
 * an account locked or retired after pairing — is still a live link comms can message, so it is
 * listed too, after the directory, with its code in place of the title line, and counted: a total
 * that hid it would show fewer linked people than comms actually messages.
 */
export function joinStaffLinks(
  directory: readonly DirectoryPerson[],
  units: readonly UnitName[],
  links: readonly { staff_code: string; staff_name: string }[],
): { rows: StaffLinkRow[]; linked: number; total: number } {
  const unitName = new Map(units.map((u) => [u.id, u.name]));
  const linkedCodes = new Set(links.map((l) => l.staff_code));
  const rows: StaffLinkRow[] = directory.map((p) => ({
    code: p.code,
    name: p.full_name,
    detail: [p.position, unitName.get(p.department_id) ?? ""].filter((s) => s.trim() !== "").join(" · "),
    linked: linkedCodes.has(p.code),
  }));
  const inDirectory = new Set(directory.map((p) => p.code));
  for (const l of links) {
    if (!inDirectory.has(l.staff_code)) rows.push({ code: l.staff_code, name: l.staff_name, detail: l.staff_code, linked: true });
  }
  return { rows, linked: rows.filter((r) => r.linked).length, total: rows.length };
}

export type ZaloChannelDraft = {
  isEnabled: boolean;
  kinds: ZaloReminderKind[];
  /** "HH:MM", as the server stores it. The selects offer whole hours; another stored value is kept. */
  quietStart: string;
  quietEnd: string;
  /** null = not set. The two are set together or not at all (server: overdue_cadence_incomplete). */
  lateStartDays: number | null;
  lateRepeatDays: number | null;
};

export function draftFromSettings(s: ZaloChannelSettings): ZaloChannelDraft {
  return {
    isEnabled: s.is_enabled,
    kinds: ZALO_REMINDER_KINDS.filter((k) => s.kinds.includes(k)),
    quietStart: s.quiet_start,
    quietEnd: s.quiet_end,
    lateStartDays: s.overdue_start_after_days,
    lateRepeatDays: s.overdue_repeat_every_days,
  };
}

export function toggleKind(draft: ZaloChannelDraft, kind: ZaloReminderKind, on: boolean): ZaloChannelDraft {
  const next = on ? [...draft.kinds, kind] : draft.kinds.filter((k) => k !== kind);
  // Kept in the fixed order, so the body never depends on click order.
  return { ...draft, kinds: ZALO_REMINDER_KINDS.filter((k) => next.includes(k)) };
}

/** The select ranges of spec 11 §2 ("từ min đến 14"). The server allows up to 365; a larger stored value is shown as its own option. */
export const DAYS_MAX = 14;
const SERVER_DAYS_MAX = 365;

const HH_MM = /^([01]\d|2[0-3]):[0-5]\d$/;

/** Whole-hour options 00:00…23:00, plus the stored value when it is not one of them. */
export function hourOptions(current: string): string[] {
  const hours = Array.from({ length: 24 }, (_, h) => `${String(h).padStart(2, "0")}:00`);
  return hours.includes(current) || !HH_MM.test(current) ? hours : [...hours, current].sort();
}

/** min…14, plus the stored value when it is outside that range. */
export function dayOptions(min: number, current: number | null): number[] {
  const days = Array.from({ length: DAYS_MAX + 1 - min }, (_, i) => i + min);
  return current === null || days.includes(current) ? days : [...days, current].sort((a, b) => a - b);
}

export type BuildRefusal = {
  ok: false;
  text: string;
  /**
   * true when the draft is only HALF-WAY through a legitimate edit — one of the two cadence numbers
   * picked, the other still unset. Held on screen and not sent; any other refusal is reverted.
   */
  held: boolean;
};

/**
 * The PUT body (the server's PUT replaces the whole row, so every field is sent), or why not. Mirrors
 * the server's refusals, in its words:
 *   - turning the channel ON needs at least one kind;
 *   - the quiet window cannot start and end at the same time;
 *   - the two overdue cadences are both set or both unset; `qua-han` chosen → both REQUIRED.
 * These are reminder CADENCES set by the commune (ADR 0074), not a deadline: nothing here computes
 * or stores whether a task is late.
 */
export function buildChange(d: ZaloChannelDraft): { ok: true; change: ZaloChannelSettingsChange } | BuildRefusal {
  if (d.isEnabled && d.kinds.length === 0) {
    return { ok: false, held: false, text: "Bật kênh Zalo thì phải chọn ít nhất một loại nhắc việc." };
  }
  if (!HH_MM.test(d.quietStart) || !HH_MM.test(d.quietEnd)) {
    return { ok: false, held: false, text: "Giờ yên tĩnh phải có dạng HH:MM, từ 00:00 đến 23:59." };
  }
  if (d.quietStart === d.quietEnd) {
    return { ok: false, held: false, text: "Giờ bắt đầu và giờ kết thúc yên tĩnh không được trùng nhau." };
  }
  if ((d.lateStartDays === null) !== (d.lateRepeatDays === null)) {
    return {
      ok: false,
      held: true,
      text: "Chọn đủ hai số của nhịp nhắc quá hạn (bắt đầu nhắc sau, nhắc lại mỗi) thì mới lưu.",
    };
  }
  if (d.kinds.includes("qua-han") && d.lateStartDays === null) {
    return {
      ok: false,
      held: false,
      text: "Đã chọn nhắc việc quá hạn thì phải đặt nhịp nhắc: bắt đầu sau bao nhiêu ngày và nhắc lại mỗi bao nhiêu ngày.",
    };
  }
  const start = d.lateStartDays;
  const repeat = d.lateRepeatDays;
  if (start !== null && (!Number.isSafeInteger(start) || start < 0 || start > SERVER_DAYS_MAX)) {
    return { ok: false, held: false, text: `Số ngày bắt đầu nhắc việc quá hạn phải từ 0 đến ${SERVER_DAYS_MAX}.` };
  }
  if (repeat !== null && (!Number.isSafeInteger(repeat) || repeat < 1 || repeat > SERVER_DAYS_MAX)) {
    return { ok: false, held: false, text: `Số ngày nhắc lại việc quá hạn phải từ 1 đến ${SERVER_DAYS_MAX}.` };
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
