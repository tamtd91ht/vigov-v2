/**
 * Words and decisions of the "Tự động hoá" tab (`docs/ui-ux/14-cau-hinh.md §9`, ADR 0058). Pure, so
 * every state the screen can say — never run, claimed but no result, off because never saved — has a
 * test without a DOM.
 *
 * WHAT THE SCREEN SAYS ABOUT RUNS COMES FROM `last_runs` AND NOTHING ELSE. A switch that is on does not
 * mean anything ran: the runners claim work at their own tick, and a commune may switch a job on long
 * before the first run is recorded. So there is no sentence here promising "chạy trong vòng 1 phút";
 * there is "Chưa chạy lần nào" until a run exists, and a "run now" mark that no run has picked up
 * yet is said as exactly that.
 */

import type { AutomationJob, AutomationSetting } from "@/lib/api/automation-jobs";
import type { identity_automationRunOut } from "@/lib/api/schema.gen";

import { formatDateTime } from "@/features/dashboard/period";

import { nhanLoaiViec } from "./nhan-thoi-han";

export const AUTOMATION_TITLE = "Tự động hoá";

/** §9's guidance sentence, verbatim. */
export const AUTOMATION_GUIDANCE =
  "Những việc phần mềm tự làm cho xã. Mặc định tắt hết — bật cái nào thì xã tự chọn, và giờ giấc " +
  "tính theo giờ Việt Nam. Đổi nhịp có hiệu lực ngay ở lượt chạy kế tiếp.";

type JobWords = { readonly title: string; readonly description: string };

/**
 * Does this schedule kind carry a weekday? `monthly_and_weekly` (`scheduled_reports`, ADR 0086 B2) is
 * weekday + HH:MM on the wire exactly like `weekly` — the 1st of the month is implied by the kind, not
 * a field (`service-identity/internal/domain/automation.go` LatestSlot).
 */
export function hasWeekday(scheduleKind: string): boolean {
  return scheduleKind === "weekly" || scheduleKind === "monthly_and_weekly";
}

/**
 * The jobs, in §9's order. Descriptions follow §9 with two corrections the decisions force:
 * reminders sweep petitions (đơn thư) too (ADR 0058 §4), and escalation thresholds are WORKING HOURS
 * from the SLA tab (ADR 0029, ADR 0058 §5) — §9's "số ngày" would tell the administrator a unit the
 * software does not count in.
 *
 * WHAT EACH JOB COVERS WAS DECIDED BY THE USER ON 29/09/2026 (ADR 0058, ADR 0029 §Bổ sung 29/09), and
 * both sentences name it so an administrator switching a job on knows what it touches: reminders cover
 * tasks, incoming documents, citizens' petitions and đơn thư; escalation covers tasks, petitions
 * (against their RESOLVE deadline, not the acknowledge one) and incoming documents — NOT đơn thư.
 */
// Bug sheet row 40: the sentences follow the prototype's (`automation.py:51-74`) word for word wherever
// they are true here. Kept against it, on purpose: "văn bản đến" and "đơn thư" (the scope ADR 0058 §4
// decided), "giờ làm việc" for "ngày" (rule 10 / ADR 0007 — the thresholds are working hours), and a
// SECOND threshold of its own instead of "trễ gấp đôi" (the Báo Chủ tịch column is set separately, ADR
// 0029), sent to "lãnh đạo trực tiếp" as that column is named.
const JOBS: Readonly<Record<string, JobWords>> = {
  sla_reminders: {
    title: "Nhắc việc sắp đến hạn và đã quá hạn",
    description:
      "Quét nhiệm vụ, văn bản đến, phản ánh và đơn thư; nhắc người phụ trách trước khi đến hạn và báo " +
      "khi đã quá hạn. Cả việc bộ phận giữ mà chưa phân công ai. Mốc sắp đến hạn lấy từ Thời hạn xử lý.",
  },
  escalation: {
    title: "Leo thang việc trễ hạn",
    description:
      "Việc trễ quá số giờ làm việc đã đặt thì báo lên lãnh đạo trực tiếp, trễ quá mốc thứ hai thì báo " +
      "lên Chủ tịch. Số giờ lấy từ Thời hạn xử lý. Áp cho nhiệm vụ, văn bản đến và phản ánh (tính theo " +
      "hạn xử lý xong).",
  },
  weekly_digest: {
    title: "Bản tin đầu tuần cho lãnh đạo",
    description:
      "Tóm tắt việc tồn, việc trễ và phản ánh nóng của tuần trước. Phản ánh nóng là phiếu đã quá hạn " +
      "hoặc bị người dân chấm 1–2 sao trong tuần.",
  },
  // ADR 0086 B ("theo prototype"): label and description verbatim from the reference system
  // (`vigov-require/apps/api/app/modules/admin/automation.py:91-99`). "Đầu tuần" holds for the
  // suggested Monday; the weekday the commune picks is drawn right under it.
  scheduled_reports: {
    title: "Gửi báo cáo định kỳ",
    description: "Báo cáo tuần vào đầu tuần, báo cáo tháng vào ngày mùng 1.",
  },
};

/** A job key the screen does not know is shown by its key — never hidden, never guessed. */
export function jobWords(job: string): JobWords {
  return JOBS[job] ?? { title: job, description: "" };
}

export const WEEKDAYS: readonly { readonly value: number; readonly label: string }[] = [
  { value: 1, label: "Thứ Hai" },
  { value: 2, label: "Thứ Ba" },
  { value: 3, label: "Thứ Tư" },
  { value: 4, label: "Thứ Năm" },
  { value: 5, label: "Thứ Sáu" },
  { value: 6, label: "Thứ Bảy" },
  { value: 7, label: "Chủ nhật" },
];

export function weekdayLabel(n: number | null): string {
  return WEEKDAYS.find((w) => w.value === n)?.label ?? "—";
}

/** The server always sends `Asia/Ho_Chi_Minh`; said in words. Anything else is shown as sent. */
export function timezoneLabel(tz: string): string {
  return tz === "Asia/Ho_Chi_Minh" ? "Giờ Việt Nam" : tz;
}

const pad = (n: number) => String(n).padStart(2, "0");

export function clock(hour: number | null, minute: number | null): string {
  return hour === null || minute === null ? "—" : `${pad(hour)}:${pad(minute)}`;
}

/** The label above the clock field. Vietnam time is the guidance's promise; any other zone is named. */
export function clockFieldLabel(tz: string): string {
  return tz === "Asia/Ho_Chi_Minh" ? "Lúc" : `Lúc (${timezoneLabel(tz)})`;
}

/**
 * The card's run line (spec 09 header): the LATEST claim over every kind of work. Not `last_runs[0]`:
 * the server orders the list by job and kind of work (`service-identity/internal/store/automation.go`,
 * `ORDER BY sc.job, sc.work_kind`), so the first entry is the alphabetically first kind, not the newest.
 */
export function lastRunSentence(j: AutomationJob): string {
  let latest = Number.NaN;
  let latestIso: string | null = null;
  for (const r of j.last_runs) {
    const t = Date.parse(r.claimed_at);
    if (!Number.isNaN(t) && (Number.isNaN(latest) || t > latest)) {
      latest = t;
      latestIso = r.claimed_at;
    }
  }
  return latestIso === null ? NEVER_RAN : `Chạy lần cuối ${when(latestIso)}`;
}

export const NEVER_RAN = "Chưa chạy lần nào";

/** The switch's own word, beside it (spec 09). */
export function switchWord(on: boolean): string {
  return on ? "Đang bật" : "Đang tắt";
}

/** Toast after a switch was saved (spec 09): `Đã lưu: …` when switched on, `Đã tắt: …` when off. */
export function toggledToast(title: string, on: boolean): string {
  return on ? `Đã lưu: ${title}` : `Đã tắt: ${title}`;
}

/** Toast after "Lưu nhịp". */
export function cadenceSavedToast(title: string): string {
  return `Đã lưu: ${title}`;
}

/** Toast after "Chạy ngay" — the request is RECORDED; nothing has run yet (ADR 0058 §7). */
export function runRequestedToast(title: string): string {
  return `Đã ghi yêu cầu chạy ngay: ${title}`;
}

/* ---- the interval choices -------------------------------------------------------------------- */

/** Spec 09's "Cứ mỗi" list: 5, 10, 15, 30 phút, 1, 3, 6, 12 giờ, 1 ngày — in minutes. */
export const INTERVAL_CHOICES: readonly number[] = [5, 10, 15, 30, 60, 180, 360, 720, 1440];

/** "15 phút" · "3 giờ" · "1 ngày"; a minute count that is not a whole hour or day stays in minutes. */
export function intervalLabel(minutes: number): string {
  if (minutes >= 1440 && minutes % 1440 === 0) return `${minutes / 1440} ngày`;
  if (minutes >= 60 && minutes % 60 === 0) return `${minutes / 60} giờ`;
  return `${minutes} phút`;
}

/**
 * The options of "Cứ mỗi": the spec's list from the server's `min_interval_minutes` up, plus the
 * value already saved when the list does not hold it (e.g. 20 minutes) — a select that cannot show
 * the saved value would silently show, and on "Lưu nhịp" send, another one.
 */
export function intervalOptions(j: AutomationJob, current: string): readonly number[] {
  const min = j.min_interval_minutes ?? 0;
  const list = INTERVAL_CHOICES.filter((m) => m >= min);
  const n = Number(current);
  if (current.trim() !== "" && Number.isInteger(n) && n > 0 && !list.includes(n)) {
    return [...list, n].sort((a, b) => a - b);
  }
  return list;
}

/* ---- the cadence form ------------------------------------------------------------------------ */

/** What is being typed. Strings, as inputs give them. */
export type AutomationDraft = {
  readonly enabled: boolean;
  readonly interval: string;
  /** "HH:MM", what `<input type="time">` gives. */
  readonly time: string;
  readonly weekday: string;
};

export function draftFrom(j: AutomationJob): AutomationDraft {
  return {
    enabled: j.enabled,
    interval: j.interval_minutes === null ? "" : String(j.interval_minutes),
    time: j.run_hour === null || j.run_minute === null ? "" : clock(j.run_hour, j.run_minute),
    weekday: j.weekday === null ? "" : String(j.weekday),
  };
}

/** "Lưu nhịp" shows only when the cadence on screen differs from the saved one (spec 09). */
export function cadenceChanged(j: AutomationJob, d: AutomationDraft): boolean {
  const saved = draftFrom(j);
  return d.interval !== saved.interval || d.time !== saved.time || d.weekday !== saved.weekday;
}

export const INTERVAL_NOT_A_NUMBER = "Nhịp nhắc việc phải là một số phút nguyên.";
export const TIME_MISSING = "Chọn giờ chạy (giờ và phút).";
export const WEEKDAY_MISSING = "Chọn thứ trong tuần.";

function wholeNumber(raw: string): number | null {
  const t = raw.trim();
  if (t === "") return null;
  const n = Number(t);
  return Number.isInteger(n) ? n : null;
}

/**
 * Draft → PUT body. ONLY the fields of the job's kind are sent; the others go as `null`, which the
 * server requires ("the others must be absent or null — refused rather than ignored").
 *
 * THE BOUNDS ARE NOT CHECKED HERE (5–10080 minutes, 0–23, 0–59, 1–7): the server checks them and
 * answers with a sentence naming the bound; a copy here is a second rule set that drifts (rule 9).
 * Only what cannot be SENT is refused here — an empty or non-numeric field.
 */
export function settingBody(
  j: AutomationJob,
  d: AutomationDraft,
): { ok: true; body: AutomationSetting } | { ok: false; message: string } {
  const body: AutomationSetting = {
    enabled: d.enabled,
    interval_minutes: null,
    run_hour: null,
    run_minute: null,
    weekday: null,
  };
  if (j.schedule_kind === "interval") {
    const n = wholeNumber(d.interval);
    if (n === null) return { ok: false, message: INTERVAL_NOT_A_NUMBER };
    return { ok: true, body: { ...body, interval_minutes: n } };
  }
  const m = /^(\d{1,2}):(\d{2})$/.exec(d.time.trim());
  if (m === null) return { ok: false, message: TIME_MISSING };
  const timed = { ...body, run_hour: Number(m[1]), run_minute: Number(m[2]) };
  if (hasWeekday(j.schedule_kind)) {
    const w = wholeNumber(d.weekday);
    if (w === null) return { ok: false, message: WEEKDAY_MISSING };
    return { ok: true, body: { ...timed, weekday: w } };
  }
  return { ok: true, body: timed };
}

/* ---- runs ------------------------------------------------------------------------------------ */

/** "14:05 22/09/2026", Vietnam time. An unreadable instant says so rather than showing "Invalid Date". */
export function when(iso: string | null): string {
  if (iso === null) return "—";
  const t = Date.parse(iso);
  return Number.isNaN(t) ? "mốc thời gian không đọc được" : formatDateTime(t);
}

/** Outcome of a run. `null` = claimed and not reported: a runner that died shows as exactly that. */
export function outcomeLabel(outcome: string | null): string {
  switch (outcome) {
    case null:
      return "Chưa báo kết quả";
    case "succeeded":
      return "Hoàn thành";
    case "configuration_missing":
      return "Thiếu cấu hình của xã";
    case "dependency_unavailable":
      return "Dịch vụ liên quan không phản hồi";
    case "failed":
      return "Lỗi";
    default:
      return `Kết quả không rõ (${outcome})`;
  }
}

export function triggerLabel(trigger: string): string {
  switch (trigger) {
    case "schedule":
      return "Theo lịch";
    case "request":
      return "Chạy ngay";
    default:
      return trigger;
  }
}

export function workKindLabel(kind: string): string {
  return nhanLoaiViec(kind);
}

export function count(n: number | null): string {
  return n === null ? "—" : String(n);
}

/**
 * The "run now" line, from data only: when it was pressed, and whether any run has picked it up —
 * a run of trigger `request` claimed at or after the mark. Never "sẽ chạy trong 1 phút".
 */
export function runRequestSentence(j: AutomationJob): string | null {
  if (j.run_requested_at === null) return null;
  const mark = new Date(j.run_requested_at).getTime();
  const picked = j.last_runs.some(
    (r: identity_automationRunOut) =>
      r.trigger === "request" && !Number.isNaN(mark) && new Date(r.claimed_at).getTime() >= mark,
  );
  const at = when(j.run_requested_at);
  return picked
    ? `Yêu cầu chạy ngay lúc ${at} đã được nhận — xem lượt chạy bên dưới.`
    : `Đã ghi yêu cầu chạy ngay lúc ${at}. Chưa có lượt chạy nào nhận yêu cầu này.`;
}

export const SAVE_CADENCE_BUTTON = "Lưu nhịp";
export const RUN_NOW_BUTTON = "Chạy ngay";
export const RUN_NOW_SENDING = "Đang gửi yêu cầu…";
