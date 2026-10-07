/**
 * `🗑 Xoá đã chọn` (§2) — pure half: selection, order, the sequential run, and the words.
 *
 * THERE IS NO BATCH ROUTE, AND THAT IS THE DESIGN (require a037b76, user decision 28/09/2026): the
 * screen sends ONE soft-delete call (`xoaNhiemVu`, `DELETE /api/v1/tasks/{ma}` with a reason) per
 * selected task, one after another, each with the SAME reason. Each call is its own soft delete and
 * its own audit entry (rule 6), and each is refused or accepted on its own — so the result is
 * reported PER TASK, never as "đã xoá 5 nhiệm vụ" when two of the five were refused.
 *
 * `task.delete` gates the controls. That is convenience: every call is checked again by the route
 * (rule 5, forbidden #1), and a refusal shows the server's sentence verbatim.
 */

import type { KetQua } from "@/lib/api/goi";
import type { petitions_nhiemVuRa } from "@/lib/api/schema.gen";

/**
 * What the selection keeps of a row: the code, its parent's code for the order of the run, and the
 * title + assignee the confirm dialog lists (spec 02 §4: "Mỗi dòng gồm tên nhiệm vụ và tên người
 * thực hiện") — kept with the tick, because the row may be on a page no longer on screen.
 */
export type SelectedTask = Pick<petitions_nhiemVuRa, "code" | "parent" | "title" | "assignee">;

/** Selection keyed by register code. A `ReadonlyMap` so a render can never mutate it in place. */
export type TaskSelectionState = ReadonlyMap<string, SelectedTask>;

export const EMPTY_SELECTION: TaskSelectionState = new Map();

/**
 * What a list row or a Kanban card needs to draw its `☐ Chọn`. `null` at the call site = no
 * checkbox at all (no `task.delete`) — closed by default, so a caller that forgets the prop draws
 * fewer controls, never more.
 */
export type TaskSelection = {
  readonly selected: TaskSelectionState;
  readonly toggle: (task: SelectedTask) => void;
  /**
   * The header box of the list (spec 04 `SelectAllBox`): ticks every row shown, or unticks them all
   * when every one is ticked. Absent = no header box.
   */
  readonly toggleAll?: (tasks: readonly SelectedTask[]) => void;
  /** A batch is running: the selection is frozen until every call has answered. */
  readonly disabled: boolean;
};

/** Tick or untick one row — always a NEW map built from the old one, never the old one edited. */
export function toggleSelected(sel: TaskSelectionState, task: SelectedTask): TaskSelectionState {
  if (sel.has(task.code)) return new Map([...sel].filter(([code]) => code !== task.code));
  return new Map([...sel, [task.code, keptOf(task)]]);
}

/** Only the four fields — never the whole row object, which would pin a page's answer in memory. */
function keptOf(task: SelectedTask): SelectedTask {
  return { code: task.code, parent: task.parent, title: task.title, assignee: task.assignee };
}

/** The header box: all shown rows ticked → untick them; otherwise tick every shown row. */
export function toggleAllSelected(sel: TaskSelectionState, tasks: readonly SelectedTask[]): TaskSelectionState {
  if (tasks.length === 0) return sel;
  const all = tasks.every((t) => sel.has(t.code));
  if (all) {
    const shown = new Set(tasks.map((t) => t.code));
    return new Map([...sel].filter(([code]) => !shown.has(code)));
  }
  const next = new Map(sel);
  for (const t of tasks) if (!next.has(t.code)) next.set(t.code, keptOf(t));
  return next;
}

/** `checked` / `indeterminate` of the header box over the rows shown. */
export function selectAllState(sel: TaskSelectionState, tasks: readonly SelectedTask[]): "all" | "some" | "none" {
  const n = tasks.filter((t) => sel.has(t.code)).length;
  return n === 0 ? "none" : n === tasks.length ? "all" : "some";
}

/** Keep only the codes that FAILED — they stay selected so the clerk can see and retry them. */
export function keepFailed(
  sel: TaskSelectionState,
  results: readonly BatchDeleteResult[],
): TaskSelectionState {
  const failed = new Set(results.filter((r) => !r.ok).map((r) => r.code));
  return new Map([...sel].filter(([code]) => failed.has(code)));
}

/**
 * Children before parents. The server refuses a task that still has a live child (409, "còn N việc
 * con chưa xoá", ADR 0037 decision 3), so a selection holding a parent AND its child, sent
 * parent-first, fails on the parent for a reason the same batch was about to remove.
 *
 * DEPTH WITHIN THE SELECTION ONLY, from each row's `parent` code: a task goes after every selected
 * task below it. A child that is NOT selected still blocks its parent — that refusal is the server's,
 * shown verbatim. Ties keep the order of selection. A cycle cannot come from the server
 * (`kiemChuTrinh`); the walk is bounded anyway so bad data cannot hang the page.
 */
export function deleteOrder(tasks: readonly SelectedTask[]): SelectedTask[] {
  const byCode = new Map(tasks.map((t) => [t.code, t]));
  const depth = (t: SelectedTask): number => {
    let d = 0;
    let p = t.parent;
    while (p !== "" && byCode.has(p) && d < tasks.length) {
      d += 1;
      p = (byCode.get(p) as SelectedTask).parent;
    }
    return d;
  };
  return tasks
    .map((t, i) => ({ t, i, d: depth(t) }))
    .sort((a, b) => b.d - a.d || a.i - b.i)
    .map((x) => x.t);
}

/** The shared reason, trimmed as the server trims it; `null` disables the button. */
export function batchReason(text: string): string | null {
  const s = text.trim();
  return s === "" ? null : s;
}

export type BatchDeleteResult =
  | { readonly code: string; readonly ok: true }
  | { readonly code: string; readonly ok: false; readonly message: string };

/**
 * One call per task, SEQUENTIALLY — the next starts only after the previous answered. Parallel
 * calls would race the parent/child order `deleteOrder` exists for, and put N concurrent writes on
 * one commune's register from one click.
 *
 * NEVER STOPS AT A REFUSAL: one task that cannot go does not make the others wrong. `onProgress`
 * reports how many are done, for the live line.
 */
export async function runBatchDelete(
  tasks: readonly SelectedTask[],
  reason: string,
  softDelete: (code: string, reason: string) => Promise<KetQua<void>>,
  onProgress?: (done: number) => void,
): Promise<BatchDeleteResult[]> {
  const results: BatchDeleteResult[] = [];
  for (const t of deleteOrder(tasks)) {
    const r = await softDelete(t.code, reason);
    results.push(r.ok ? { code: t.code, ok: true } : { code: t.code, ok: false, message: r.thongBao });
    onProgress?.(results.length);
  }
  return results;
}

/* ── words ─────────────────────────────────────────────────────────────────────────────────── */

/** The spec's `🗑` is drawn as a lucide `Trash2` beside the word (ADR 0068). */
export const BATCH_DELETE_BUTTON = "Xoá đã chọn";
export const BATCH_REASON_LABEL = "Lý do xoá (bắt buộc, dùng chung cho mọi nhiệm vụ đã chọn)";
/** Spec 02 §4 — the dialog's description, verbatim. */
export const BATCH_NOTE =
  "Những nhiệm vụ này sẽ khuất khỏi sổ theo dõi và khỏi báo cáo. Bản ghi và nhật ký vẫn được giữ trong " +
  "hệ thống, nhưng giao diện không có nút hoàn tác. Nhiệm vụ còn việc con sẽ bị từ chối.";
/** Accessible name of the header box (prototype `SelectAllBox`). */
export const SELECT_ALL_LABEL = "Chọn tất cả dòng đang hiển thị";

/** Spec 02 §4: `Xoá {n} nhiệm vụ khỏi sổ?` */
export function batchDeleteTitle(n: number): string {
  return `Xoá ${n} nhiệm vụ khỏi sổ?`;
}

/** Spec 02 §4: the confirm button, `Xoá {n} nhiệm vụ`. */
export function batchDeleteConfirmLabel(n: number): string {
  return `Xoá ${n} nhiệm vụ`;
}

/** `Đã chọn {N} nhiệm vụ` — verbatim §2. */
export function selectedCountLabel(n: number): string {
  return `Đã chọn ${n} nhiệm vụ`;
}

export function batchProgressText(done: number, total: number): string {
  return `Đang xoá ${done}/${total}… Mỗi việc chờ máy chủ trả lời rồi mới sang việc kế tiếp.`;
}

/** Spec 02 §4 success toast: `Đã xoá {n} nhiệm vụ.` — only what the server confirmed; `null` for none. */
export function batchSuccessToast(results: readonly BatchDeleteResult[]): string | null {
  const ok = results.filter((r) => r.ok).length;
  return ok === 0 ? null : `Đã xoá ${ok} nhiệm vụ.`;
}

/**
 * Spec 02 §4 failure toasts: `{k} nhiệm vụ không xoá được: {lý do}` — ONE per distinct refusal, the
 * server's sentence verbatim (it carries the number of child tasks still alive, ADR 0037 decision 3).
 * The contract returns a sentence, not a code, so the spec's per-code wording is not rebuilt here.
 */
export function batchFailureToasts(results: readonly BatchDeleteResult[]): string[] {
  const byReason = new Map<string, number>();
  for (const r of results) if (!r.ok) byReason.set(r.message, (byReason.get(r.message) ?? 0) + 1);
  return [...byReason].map(([message, count]) => `${count} nhiệm vụ không xoá được: ${message}`);
}

/** Accessible name of a row's / card's checkbox: which task, among twenty. */
export function selectLabel(code: string): string {
  return `Chọn ${code}`;
}
