/**
 * The owner's Nhiệm vụ spec (02–10) as data and class strings, for the Nhiệm vụ screens only (ADR 0076
 * §"Sửa đổi 07/10/2026 (lần 2)"). Pure: no network, no DOM, no clock — "now" is always a parameter.
 *
 * TWO DIFFERENCES FROM THE SPEC TEXT, BOTH DELIBERATE (same as `features/giai-ngan/spec-classes.ts`):
 *   - The spec's `bg-surface` (page grey) is this app's `bg-canvas`; our `bg-surface` is white.
 *   - This app runs WITHOUT Tailwind preflight, so headings, paragraphs, lists and buttons keep the
 *     browser's margins and look unless a class says otherwise — the strings add `m-0`, `text-left`,
 *     `[font-family:inherit]` where the prototype relied on preflight. NEVER `border-solid` next to a
 *     one-side border (`border-b`): without preflight the other three sides then paint `medium` (3px).
 */

import type { petitions_loaiNhiemVuRa, petitions_nhiemVuRa } from "@/lib/api/schema.gen";

import {
  hoanThanhTreHan,
  ketThuc,
  nhanHoanThanhTreHan,
  tinhTrangHan,
  type BangNhanTrangThai, // vi-name-ok: existing type, imported not declared (rule 12 inv 3)
  type TrangThaiNhiemVu, // vi-name-ok: existing type, imported not declared (rule 12 inv 3)
} from "./nhan-nhiem-vu";

/* ── STATUS LABELS: FIXED, THE SPEC'S (owner 07/10/2026, ADR 0076 lần 2 #2) ─────────────────────
 *
 * The Nhiệm vụ screens show THESE words, not the commune's `/task-statuses` labels. The catalogue is
 * still read: it gives the codes' ORDER (Kanban columns) and nothing else changes — codes stay the
 * Vietnamese ones of ADR 0011 and every request sends them. This reverses decision #21 ON THESE
 * SCREENS ONLY (ADR 0076 lần 2, consequence (a): to be confirmed with the customer); Sổ tay lãnh đạo
 * and the Danh mục tab keep the commune's labels.
 */
export const SPEC_STATUS_LABELS: Readonly<Record<TrangThaiNhiemVu, string>> = {
  "moi-giao": "Chưa thực hiện",
  "da-tiep-nhan": "Đã tiếp nhận",
  "dang-thuc-hien": "Đang thực hiện",
  "cho-duyet": "Chờ duyệt",
  "hoan-thanh": "Hoàn thành",
  "tam-dung": "Tạm dừng",
  "chuyen-tiep": "Chuyển tiếp",
};

/** The commune's table with the spec's words in place of its labels — order untouched. */
export function withSpecLabels(table: BangNhanTrangThai): BangNhanTrangThai {
  return { nhan: SPEC_STATUS_LABELS, thuTu: table.thuTu };
}

/**
 * The sentence under the status strip — spec 10 `TASK_STATUS_HINT`, VERBATIM (owner 07/10/2026, ADR
 * 0076 lần 2 #3) even where it states a rule this system does not apply: completion needs no
 * leader's approval (ADR 0065), and "Chờ duyệt" requires no evidence (ADR 0076 #5). The server's rules
 * are unchanged; only the words are the spec's (consequence (c) of the ADR).
 */
export const TASK_STATUS_HINT: Readonly<Record<TrangThaiNhiemVu, string>> = {
  "moi-giao": "Đã giao nhưng người nhận chưa bấm tiếp nhận.",
  "da-tiep-nhan": "Người nhận đã nhận việc, chưa bắt tay làm.",
  "dang-thuc-hien": "Đang làm. Báo tiến độ và đính minh chứng ở cột bên phải.",
  "cho-duyet": "Đã làm xong, chờ lãnh đạo duyệt. Phải có minh chứng.",
  "hoan-thanh": "Lãnh đạo đã duyệt. Nhiệm vụ khép lại.",
  "tam-dung": "Dừng vì lý do khách quan. Đồng hồ hạn vẫn chạy.",
  "chuyen-tiep": "Đã chuyển tay, chờ người mới tiếp nhận.",
};

/** The hint of a status code; `""` for a code the screen does not know. */
export function taskStatusHint(code: string): string {
  return (TASK_STATUS_HINT as Readonly<Record<string, string>>)[code] ?? "";
}

/** Spec 10 `TASK_STATUS_META[…]`: the column dot and the status chip, per Vietnamese code. */
const STATUS_TONE: Readonly<Record<TrangThaiNhiemVu, { dot: string; chip: string; active: string }>> = {
  "moi-giao": { dot: "bg-ink-muted", chip: "bg-ink-muted/12 text-ink border-line", active: "bg-ink-muted" },
  "da-tiep-nhan": { dot: "bg-teal", chip: "bg-teal/12 text-teal border-teal/25", active: "bg-teal" },
  "dang-thuc-hien": { dot: "bg-brand", chip: "bg-brand/12 text-brand border-brand/25", active: "bg-brand" },
  "cho-duyet": { dot: "bg-violet", chip: "bg-violet/12 text-violet border-violet/25", active: "bg-violet" },
  "hoan-thanh": { dot: "bg-leaf", chip: "bg-leaf/12 text-leaf border-leaf/25", active: "bg-leaf" },
  "tam-dung": { dot: "bg-tangerine", chip: "bg-tangerine/12 text-tangerine border-tangerine/25", active: "bg-tangerine" },
  "chuyen-tiep": { dot: "bg-ink", chip: "bg-ink/12 text-ink border-line", active: "bg-ink" },
};

const UNKNOWN_TONE = { dot: "bg-ink-muted", chip: "bg-ink-muted/12 text-ink border-line", active: "bg-ink-muted" };

function toneOf(code: string) {
  return (STATUS_TONE as Readonly<Record<string, typeof UNKNOWN_TONE>>)[code] ?? UNKNOWN_TONE;
}

/** The 8px dot before a Kanban column title (spec 03). Decorative: the title beside it is the word. */
export function statusDotClass(code: string): string {
  return toneOf(code).dot;
}

/** Fill of the CURRENT step of the status strip (spec 07 §2), always with white text. */
export function statusActiveClass(code: string): string {
  return toneOf(code).active;
}

/**
 * Overdue NOW — DERIVED from `due_at` and the clock (rule 10, invariant 3), never read from a column.
 * A finished task is never "late": it owes nothing; "Hoàn thành trễ hạn" is said instead (spec 10:
 * "việc đã xong thì không còn là việc đang trễ").
 */
export function isOverdueNow(task: Pick<petitions_nhiemVuRa, "due_at" | "status">, now: Date): boolean {
  return !ketThuc(task.status) && tinhTrangHan(task.due_at, now).loai === "tre";
}

/** Whole calendar days past the deadline (spec `days_overdue`), `0` when not late. */
export function daysOverdue(task: Pick<petitions_nhiemVuRa, "due_at">, now: Date): number {
  const t = tinhTrangHan(task.due_at, now);
  return t.loai === "tre" ? t.soNgay : 0;
}

/**
 * The badge a row shows (spec 10 `taskDisplayState`): finished late, then overdue, override the
 * status — both derived, both from dates. The label of the status itself comes from `labels`.
 */
export function taskDisplayState(
  task: Pick<petitions_nhiemVuRa, "due_at" | "status" | "completed_at" | "original_due_at">,
  labels: BangNhanTrangThai,
  now: Date,
): { readonly label: string; readonly chip: string; readonly tone: "late" | "finished-late" | "status" } {
  if (hoanThanhTreHan(task.completed_at, task.original_due_at)) {
    // From the table's `hoan-thanh` word — "Hoàn thành trễ hạn" with the spec's words (one source).
    return { label: nhanHoanThanhTreHan(labels), chip: "bg-tangerine/12 text-tangerine border-tangerine/25", tone: "finished-late" };
  }
  if (isOverdueNow(task, now)) {
    return { label: "Trễ hạn", chip: "bg-danger/12 text-danger border-danger/25", tone: "late" };
  }
  const known = (labels.nhan as Readonly<Record<string, string>>)[task.status];
  return { label: known ?? task.status, chip: toneOf(task.status).chip, tone: "status" };
}

/**
 * The type the Nhiệm vụ create dialog pre-selects — spec 06: the ACTIVE `is_default` row, ELSE THE
 * FIRST ACTIVE ROW (owner 07/10/2026, ADR 0076 lần 2 #2, reversing a1e5e64f's "leave it empty" on this
 * screen). Biên bản and Phản ánh keep `defaultTaskType` (no first-row fallback).
 */
export function specDefaultTaskType(types: readonly petitions_loaiNhiemVuRa[]): string {
  const active = types.filter((t) => t.active);
  return active.find((t) => t.is_default)?.code ?? active[0]?.code ?? "";
}

/**
 * Initials of a person for the timeline's avatar (spec 08): the first letter of the LAST TWO words
 * of the name, upper-cased ("Huỳnh Văn 9" → "V9"); `""` when there is no name.
 */
export function staffInitials(fullName: string): string {
  return fullName
    .trim()
    .split(/\s+/)
    .filter((w) => w !== "")
    .slice(-2)
    .map((w) => w.charAt(0))
    .join("")
    .toLocaleUpperCase("vi");
}

/* ── CLASS STRINGS (spec 00/02–09, preflight-safe) ──────────────────────────────────────────── */

/** Native select of the filter row (spec 02 §2). */
export const FILTER_SELECT_CLASS =
  "border-line focus-visible:ring-ring/50 h-9 rounded-md border bg-white px-3 text-[12.5px] text-ink [font-family:inherit] outline-none focus-visible:ring-[3px]";

/** Native select of the create form (spec 06 — page-grey fill, unlike the filter row). */
export const FORM_SELECT_CLASS =
  "border-line bg-canvas focus-visible:ring-ring/50 h-9 w-full rounded-md border px-3 text-[13px] text-ink [font-family:inherit] outline-none focus-visible:ring-[3px]";

/** shadcn `Input` / `Textarea` (spec 00 §5) — 36px, 14px from 768px, 16px below (iOS zoom). */
export const INPUT_CLASS =
  "border-line focus-visible:ring-ring/50 h-9 w-full min-w-0 rounded-md border bg-white px-3 text-base text-ink [font-family:inherit] outline-none focus-visible:ring-[3px] disabled:cursor-not-allowed disabled:opacity-50 md:text-sm";
export const TEXTAREA_CLASS =
  "border-line focus-visible:ring-ring/50 w-full min-w-0 rounded-md border bg-white px-3 py-2 text-base text-ink [font-family:inherit] outline-none focus-visible:ring-[3px] md:text-sm";

/** shadcn `Label` / `Field` label (spec 00 §5). */
export const LABEL_CLASS = "text-ink mb-1.5 block text-[13px] font-semibold";

/** A field's hint line under it (spec 06). */
export const HINT_CLASS = "text-ink-muted m-0 mt-1 text-[11.5px]";

/** Native checkbox (spec 00 §4). */
export const CHECKBOX_CLASS = "accent-brand m-0 size-3.5";

/** shadcn `Badge` + a chip tone (spec 04 `Trạng thái`). */
export const BADGE_CLASS =
  "inline-flex h-5 w-fit shrink-0 items-center justify-center gap-1 overflow-hidden rounded-4xl border px-2 py-0.5 text-xs font-medium whitespace-nowrap [&>svg]:size-3!";

/** Table frame of the list and the register (spec 04/05): white, rounded 10px, horizontal scroll only. */
export const TABLE_FRAME_CLASS = "border-line overflow-hidden rounded-[10px] border bg-white";
export const TABLE_CLASS = "w-full caption-bottom border-collapse text-sm text-ink";
export const TH_CLASS = "text-navy h-10 px-2 text-left align-middle font-medium whitespace-nowrap";
export const TD_CLASS = "p-2 align-middle whitespace-nowrap";
/** A body row; `late` adds the spec's pink tint. */
export function rowClass(late: boolean): string {
  return late
    ? "border-line bg-danger/6 hover:bg-danger/10 cursor-pointer border-b transition-colors"
    : "border-line hover:bg-muted/50 cursor-pointer border-b transition-colors";
}

/** A drawer section card (spec 07 §5 `Section`). */
export const SECTION_CLASS = "border-line shadow-card m-0 mb-3 rounded-[12px] border bg-white p-4";
export const SECTION_TITLE_CLASS = "text-navy m-0 mb-2.5 text-[12.5px] font-bold";
