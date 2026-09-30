/**
 * Words and decisions of the "Tự động hoá" tab (`docs/ui-ux/14-cau-hinh.md §9`, ADR 0058). Pure, so
 * every state the screen can say — never run, claimed but no result, off because never saved — has a
 * test without a DOM.
 *
 * WHAT THE SCREEN SAYS ABOUT RUNS COMES FROM `last_runs` AND NOTHING ELSE. A switch that is on does not
 * mean anything ran: the runners claim work at their own tick, and a commune may switch a job on long
 * before the first run is recorded. So there is no sentence here promising "chạy trong vòng 1 phút";
 * there is "Chưa có lượt chạy nào" until a run exists, and a "run now" mark that no run has picked up
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

/**
 * Who receives what these jobs send (ADR 0058 §3): staff only, into the header bell. Said because a
 * switch named "Nhắc việc" reads as "the citizen gets reminded" to anyone who has run a one-stop shop.
 */
export const AUTOMATION_RECIPIENTS =
  "Lời nhắc, tin leo thang và bản tin đầu tuần chỉ gửi tới cán bộ của xã, vào chuông thông báo ở đầu " +
  "trang. Người dân không nhận tin nào từ các việc này.";

type JobWords = { readonly title: string; readonly description: string };

/**
 * The three jobs, in §9's order. Descriptions follow §9 with two corrections the decisions force:
 * reminders sweep petitions (đơn thư) too (ADR 0058 §4), and escalation thresholds are WORKING HOURS
 * from the SLA tab (ADR 0029, ADR 0058 §5) — §9's "số ngày" would tell the administrator a unit the
 * software does not count in.
 *
 * WHAT EACH JOB COVERS WAS DECIDED BY THE USER ON 29/09/2026 (ADR 0058, ADR 0029 §Bổ sung 29/09), and
 * both sentences name it so an administrator switching a job on knows what it touches: reminders cover
 * tasks, incoming documents, citizens' petitions and đơn thư; escalation covers tasks, petitions
 * (against their RESOLVE deadline, not the acknowledge one) and incoming documents — NOT đơn thư.
 */
const JOBS: Readonly<Record<string, JobWords>> = {
  sla_reminders: {
    title: "Nhắc việc sắp đến hạn và đã quá hạn",
    description:
      "Quét nhiệm vụ, văn bản đến, phản ánh của người dân và đơn thư; nhắc người phụ trách trước khi " +
      "đến hạn và báo khi đã quá hạn. Cả việc bộ phận giữ mà chưa phân công ai. Mốc \"sắp đến hạn\" " +
      "lấy từ tab Thời hạn xử lý.",
  },
  escalation: {
    title: "Leo thang việc trễ hạn",
    description:
      "Áp cho nhiệm vụ, văn bản đến và phản ánh của người dân (tính theo hạn xử lý xong). " +
      "Việc trễ quá ngưỡng thứ nhất thì báo lên lãnh đạo trực tiếp, trễ quá ngưỡng thứ hai thì báo " +
      "lên Chủ tịch. Hai ngưỡng tính bằng giờ làm việc, lấy từ cột Báo lãnh đạo trực tiếp và Báo Chủ " +
      "tịch của tab Thời hạn xử lý.",
  },
  weekly_digest: {
    title: "Bản tin đầu tuần cho lãnh đạo",
    description:
      "Tóm tắt việc tồn, việc trễ và phản ánh nóng của tuần trước. Phản ánh nóng là phiếu đã quá hạn " +
      "hoặc bị người dân chấm 1–2 sao trong tuần.",
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

/** "Cứ 15 phút một lần" · "Hằng ngày lúc 07:00" · "Thứ Hai hằng tuần, lúc 07:30". */
export function cadenceSentence(j: AutomationJob): string {
  switch (j.schedule_kind) {
    case "interval":
      return j.interval_minutes === null ? "—" : `Cứ ${j.interval_minutes} phút một lần`;
    case "daily":
      return `Hằng ngày lúc ${clock(j.run_hour, j.run_minute)}`;
    case "weekly":
      return `${weekdayLabel(j.weekday)} hằng tuần, lúc ${clock(j.run_hour, j.run_minute)}`;
    default:
      return "—";
  }
}

/** The card's state line. `configured: false` is OFF, and its cadence is only a suggestion. */
export function stateSentence(j: AutomationJob): string {
  if (!j.configured) {
    return "Đang tắt — xã chưa lưu cấu hình việc này. Nhịp dưới đây là nhịp gợi ý; bật rồi bấm Lưu thì việc mới chạy.";
  }
  return j.enabled ? "Đang bật" : "Đang tắt";
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
  if (j.schedule_kind === "weekly") {
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

export const NO_RUNS_YET = "Chưa có lượt chạy nào.";

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

export const SAVE_BUTTON = "Lưu";
export const SAVING = "Đang lưu…";
export const RUN_NOW_BUTTON = "Chạy ngay";
export const RUN_NOW_SENDING = "Đang gửi yêu cầu…";
export const RUN_NOW_NEEDS_ON = "Chạy ngay chỉ dùng được khi việc đang bật và đã lưu.";
export const SAVED_SENTENCE = "Đã lưu cấu hình việc này.";
export const ENABLE_LABEL = "Bật việc này";
