/**
 * "Cấu hình → Kênh Zalo" (ADR 0074 #6, laid out by spec 11 under ADR 0079) — the pure half: the
 * words, the event table, the draft, the client-side check. The check is a HINT; comms re-checks every
 * rule (`service-comms/internal/domain/zalo_link.go` NormalizeZaloChannelSetting) and its sentence
 * still reaches the screen.
 */

import type { comms_communeZaloBotCurrentOut, comms_communeZaloBotIn } from "@/lib/api/schema.gen";
import type { ZaloChannelSettings, ZaloChannelSettingsChange } from "@/lib/api/zalo";

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
 * Spec 11 §2 with "đổi ở đây là đổi cho cả hai" DROPPED — ADR 0079 lô 2 Q1 #7 (đổi ở lô 4 Q10 — ô sửa
 * được, lưu riêng cho Zalo). Until that field ships, the threshold is not changed on this tab.
 */
export const WHEN_DESCRIPTION =
  "Ngưỡng “sắp đến hạn” dùng chung với cái chuông và danh sách “Sắp đến hạn”, nên con số trong tin nhắn luôn bằng con số trên màn hình.";

/** Spec 11 §2, "và thư" dropped for the reason given on `CHANNEL_DESCRIPTION`. */
export const EVENTS_HINT = "Bỏ chọn thì việc đó vẫn vào chuông như cũ, chỉ không nhắn Zalo.";

/**
 * One row of the event table. `kind` is the per-domain kind comms sends this event under
 * (`service-comms/internal/domain/zalo_link.go` ZaloReminderKinds, migration 0021 — ADR 0079 Q1 #6:
 * "web ánh xạ sang mã spec"), or null when NOTHING produces it yet (ADR 0079 Q3, second phase): drawn
 * disabled with "?". `alsoKinds` are switched TOGETHER with `kind` by the same box. A row any of whose
 * kinds is not in the server's `supported_events` is drawn disabled with "?" too — the server, not this
 * table, says what may be ticked.
 *
 * THE BOX IS CHECKED WHEN `kind` IS SELECTED, whatever `alsoKinds` hold. A stored half-state (e.g.
 * `van-ban.chua-cu-nguoi` alone, from a save made outside this screen) reads unchecked while `qua-han`
 * is off and checked while it is on; the next tick or untick then sets all three alike.
 */
export type ZaloEventRow = {
  code: string;
  label: string;
  hint?: string;
  kind: string | null;
  alsoKinds?: readonly string[];
};

/** Every kind one row switches: `kind` first, then `alsoKinds`. Empty for a row with no producer. */
export function rowKinds(row: ZaloEventRow): readonly string[] {
  return row.kind === null ? [] : [row.kind, ...(row.alsoKinds ?? [])];
}

export type ZaloEventGroup = { title: string; events: readonly ZaloEventRow[] };

/**
 * Spec 11 §2's eighteen rows, in its order and words — nothing more (owner, 08/10/2026, "theo
 * prototype"). Comms has four kinds the spec has no row for: document / petition escalation and
 * "unassigned". The domain's "quá hạn" row switches them (`alsoKinds`), so every kind in
 * `supported_events` still has exactly one box — none can stay selected with no control on screen. The
 * task rows keep their own kinds.
 */
export const ZALO_EVENT_GROUPS: readonly ZaloEventGroup[] = [
  {
    title: "Nhiệm vụ",
    events: [
      { code: "task.assigned", label: "Giao việc mới", hint: "Nhắn ngay cho người được giao.", kind: null },
      {
        code: "task.due_soon",
        label: "Việc sắp đến hạn",
        hint: "Gộp thành một bản tin mỗi sáng, theo ngưỡng bên dưới.",
        kind: "nhiem-vu.sap-den-han",
      },
      { code: "task.overdue", label: "Việc quá hạn", hint: "Theo nhịp đặt ở phần dưới.", kind: "nhiem-vu.qua-han" },
      {
        code: "task.escalated",
        label: "Việc bị đôn đốc lên cấp trên",
        hint: "Lãnh đạo nhận khi việc trễ quá lâu.",
        kind: "nhiem-vu.leo-thang",
      },
      { code: "task.unassigned_too_long", label: "Bộ phận chưa cử người làm", kind: "nhiem-vu.chua-cu-nguoi" },
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
      { code: "document.due_soon", label: "Văn bản, đơn thư sắp đến hạn", kind: "van-ban.sap-den-han" },
      {
        code: "document.overdue",
        label: "Văn bản, đơn thư quá hạn",
        kind: "van-ban.qua-han",
        alsoKinds: ["van-ban.chua-cu-nguoi", "van-ban.leo-thang"],
      },
    ],
  },
  {
    title: "Phản ánh người dân",
    events: [
      { code: "feedback.assigned", label: "Phản ánh được phân công", kind: null },
      { code: "feedback.due_soon", label: "Phản ánh sắp đến hạn", kind: "phan-anh.sap-den-han" },
      {
        code: "feedback.overdue",
        label: "Phản ánh quá hạn",
        kind: "phan-anh.qua-han",
        alsoKinds: ["phan-anh.chua-cu-nguoi", "phan-anh.leo-thang"],
      },
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

/**
 * The kinds that need the overdue cadence — comms' `zaloOverdueKinds` (0021
 * `zalo_channel_setting_overdue_kinds_need_cadence`). Unassigned and escalation notices are one-shot.
 */
export const OVERDUE_KINDS: readonly string[] = ["nhiem-vu.qua-han", "van-ban.qua-han", "phan-anh.qua-han"];

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
  /** As the server sent them (its canonical order). A kind this screen has no row for is KEPT, never dropped on save. */
  kinds: string[];
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
    kinds: [...s.kinds],
    quietStart: s.quiet_start,
    quietEnd: s.quiet_end,
    lateStartDays: s.overdue_start_after_days,
    lateRepeatDays: s.overdue_repeat_every_days,
  };
}

/**
 * `order` is the server's `supported_events` — its canonical order, so the body never depends on click
 * order. A selected kind outside it stays, after the ordered ones.
 */
export function toggleKind(
  draft: ZaloChannelDraft,
  kind: string | readonly string[],
  on: boolean,
  order: readonly string[],
): ZaloChannelDraft {
  const kinds: readonly string[] = typeof kind === "string" ? [kind] : kind;
  const next = on
    ? [...draft.kinds, ...kinds.filter((k) => !draft.kinds.includes(k))]
    : draft.kinds.filter((k) => !kinds.includes(k));
  const ordered = order.filter((k) => next.includes(k));
  return { ...draft, kinds: [...ordered, ...next.filter((k) => !order.includes(k))] };
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
 *   - the two overdue cadences are both set or both unset; an overdue kind chosen → both REQUIRED.
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
  if (d.kinds.some((k) => OVERDUE_KINDS.includes(k)) && d.lateStartDays === null) {
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

// ---- §0 and §3: the commune's own bot (ADR 0079 Q1 #1–#5) ----------------------------------------

/** Spec 11 §0, verbatim — shown when the GET says `platform_ready: false` (no bot can serve the commune). */
export const PLATFORM_NOT_READY =
  "Chưa có con bot nào phục vụ xã, nên chưa tin nào gửi đi được. Hoặc chờ ViHAT bật bot chung, hoặc nhập mã bot riêng của xã ở phần dưới.";

/** Spec 11 §3 description, by `has_own_bot`. */
export function botDescription(hasOwnBot: boolean): string {
  return hasOwnBot
    ? "Xã đang dùng bot riêng. Mọi tin nhắc việc của xã đi bằng con bot này."
    : "Xã đang dùng bot chung của nền tảng. Để trống phần dưới là giữ nguyên như vậy — chỉ nhập mã bot khi xã muốn bot mang tên mình.";
}

/** Spec 11 §3 pill: "Bot riêng · {tên}" or "Bot chung". */
export function botPill(current: comms_communeZaloBotCurrentOut): string {
  return current.has_own_bot && current.bot !== null ? `Bot riêng · ${current.bot.bot_name}` : "Bot chung";
}

/** Spec 11 §3 token hint, by `has_own_bot`. */
export function botTokenHint(hasOwnBot: boolean): string {
  return hasOwnBot
    ? "Đã có mã. Để trống nếu giữ nguyên; nhập mã mới để thay."
    : "Lấy trong Mini App “Zalo Bot Creator”. Để trống là dùng bot chung.";
}

/**
 * The sentence for each Zalo call outcome comms records (`service-comms/internal/domain/zalo_bot.go`,
 * ZaloCall*). ZALO IS NEVER QUOTED: comms sends the class only. Commune words — "mã bot", the Mini App
 * the commune got it from — not the operator console's.
 */
const OUTCOME_TEXT: Readonly<Record<string, { long: string; short: string }>> = {
  "thanh-cong": { long: "Kết nối tốt: Zalo nhận mã bot của xã.", short: "kết nối tốt" },
  "chua-cau-hinh": { long: "Xã chưa có mã bot nên hệ thống chưa hỏi Zalo.", short: "chưa có mã bot" },
  "token-bi-tu-choi": {
    long: "Zalo không nhận mã bot (mã sai, đã bị thu hồi, hoặc bot không còn). Lấy mã mới trong Mini App “Zalo Bot Creator” rồi lưu lại.",
    short: "Zalo không nhận mã bot",
  },
  "gioi-han-tan-suat": {
    long: "Zalo đang giới hạn số lần gọi. Vui lòng thử lại sau ít phút.",
    short: "Zalo đang giới hạn số lần gọi",
  },
  "khong-kha-dung": {
    long: "Không nhận được trả lời từ Zalo. Vui lòng thử lại sau ít phút.",
    short: "Zalo không trả lời",
  },
  "bi-tu-choi": {
    long: "Zalo từ chối yêu cầu. Kiểm tra cấu hình bot trong Mini App “Zalo Bot Creator”.",
    short: "Zalo từ chối yêu cầu",
  },
  "phan-hoi-sai-dang": {
    long: "Zalo trả lời theo dạng hệ thống không đọc được. Hãy báo đơn vị vận hành hệ thống kèm thời điểm thử.",
    short: "Zalo trả lời sai dạng",
  },
};

/** A class this build does not know is a failure of unknown cause — never a success. */
const UNKNOWN_OUTCOME = {
  long: "Không rõ kết quả lần hỏi Zalo. Hãy báo đơn vị vận hành hệ thống kèm thời điểm thử.",
  short: "không rõ kết quả",
};

function outcome(result: string): { long: string; short: string } {
  // Own keys only: `"toString" in OUTCOME_TEXT` is true and would print a function as a sentence.
  return (Object.prototype.hasOwnProperty.call(OUTCOME_TEXT, result) ? OUTCOME_TEXT[result] : undefined) ?? UNKNOWN_OUTCOME;
}

export const isOutcomeOk = (result: string): boolean => result === "thanh-cong";

/** "Kiểm tra kết nối" toast. `account_name` is the BOT's name as Zalo knows it — not personal data. */
export function checkResultText(result: string, accountName?: string): string {
  if (isOutcomeOk(result) && accountName !== undefined && accountName.trim() !== "") {
    return `Kết nối tốt: Zalo nhận mã bot của xã (${accountName}).`;
  }
  return outcome(result).long;
}

/** "Lần kiểm gần nhất: {lúc} · {kết quả}" — `at` already formatted by the caller. */
export function lastCheckText(at: string, result: string): string {
  return `Lần kiểm gần nhất: ${at} · ${outcome(result).short}`;
}

/**
 * After "Đăng ký webhook". `khong-kha-dung` / `phan-hoi-sai-dang` are AMBIGUOUS: Zalo may already hold
 * the new secret (comms keeps it pending and REUSES it on the next press), so staff are told to press
 * again, not that it failed.
 */
export function webhookResultText(result: string): string {
  if (isOutcomeOk(result)) return "Đã đăng ký webhook với Zalo. Tin cán bộ nhắn cho bot từ giờ về hệ thống.";
  if (result === "khong-kha-dung" || result === "phan-hoi-sai-dang") {
    return `${outcome(result).long} Chưa rõ Zalo đã nhận đăng ký hay chưa: bấm “Đăng ký webhook” lần nữa.`;
  }
  return `Chưa đăng ký được. ${outcome(result).long}`;
}

/** The one-time secret box (ADR 0079 Q1 #3). Says it BEFORE it is too late. */
export const WEBHOOK_SECRET_ONCE =
  "Mã này chỉ hiện một lần, ngay bây giờ. Hệ thống không lưu bản đọc được nên không hiện lại — chép ngay nếu xã tự đăng ký webhook bằng tay trong ứng dụng Zalo.";

/**
 * ADR 0079 Q1 #4: a switch ends EVERY live link — the confirmation names how many. `live_link_count` is
 * the server's figure; nothing is estimated here.
 */
export function relinkSentence(liveLinkCount: number): string {
  return liveLinkCount > 0
    ? `${liveLinkCount} cán bộ đang ghép nối sẽ phải ghép nối lại.`
    : "Hiện chưa cán bộ nào ghép nối, nên không ai phải ghép nối lại.";
}

/** After a switch: what the server says actually ended. */
export function endedLinksText(count: number): string {
  return count > 0 ? ` ${count} cán bộ cần ghép nối lại.` : "";
}

/**
 * Which switch a save is. `adopt`: shared → own (token typed, no own bot). `replace`: a new token on an
 * own bot — a switch only if it is ANOTHER bot account, which only Zalo can tell, so it is confirmed as
 * one. `edit`: name / link only, no token — no link ends, no confirmation.
 */
export type BotSaveKind = "adopt" | "replace" | "edit";

export type BotDraft = { token: string; name: string; chatUrl: string };

export const RETIRE_REASON_MAX = 200;

function isHttpsUrl(v: string): boolean {
  try {
    const u = new URL(v);
    return u.protocol === "https:" && u.hostname !== "" && u.username === "" && u.password === "";
  } catch {
    return false;
  }
}

/**
 * The PUT body, or the first thing wrong — in comms' own words (`zalo_commune_bot.go`
 * communeBotRefusals). A HINT: comms re-checks every rule and its sentence still reaches the screen.
 * The token is VERBATIM: a trimmed copy is a different token.
 */
export function buildBotChange(
  d: BotDraft,
  hasOwnBot: boolean,
): { ok: true; body: comms_communeZaloBotIn; kind: BotSaveKind } | { ok: false; text: string } {
  if (!hasOwnBot && d.token === "") return { ok: false, text: "Xã chưa có bot riêng nên phải nhập mã bot." };
  const name = d.name.trim();
  if (!name.startsWith("Bot")) return { ok: false, text: "Tên bot phải bắt đầu bằng “Bot”." };
  const chatUrl = d.chatUrl.trim();
  if (!isHttpsUrl(chatUrl)) {
    return { ok: false, text: "Đường mở khung chat phải là một liên kết https:// đầy đủ, ví dụ https://zalo.me/…" };
  }
  const body: comms_communeZaloBotIn = { bot_name: name, chat_url: chatUrl };
  if (d.token !== "") body.bot_token = d.token;
  return { ok: true, body, kind: d.token === "" ? "edit" : hasOwnBot ? "replace" : "adopt" };
}

/** The retire reason, trimmed, or why not (comms: required, at most 200 characters). */
export function checkRetireReason(raw: string): { ok: true; reason: string } | { ok: false; text: string } {
  const reason = raw.trim();
  if (reason === "" || [...reason].length > RETIRE_REASON_MAX) {
    return { ok: false, text: `Cần nhập lý do quay về bot chung (tối đa ${RETIRE_REASON_MAX} ký tự).` };
  }
  return { ok: true, reason };
}
