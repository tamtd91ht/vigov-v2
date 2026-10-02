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

/** What the selection keeps of a row: the code, and its parent's code for the order of the run. */
export type SelectedTask = Pick<petitions_nhiemVuRa, "code" | "parent">;

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
  /** A batch is running: the selection is frozen until every call has answered. */
  readonly disabled: boolean;
};

/** Tick or untick one row — always a NEW map built from the old one, never the old one edited. */
export function toggleSelected(sel: TaskSelectionState, task: SelectedTask): TaskSelectionState {
  if (sel.has(task.code)) return new Map([...sel].filter(([code]) => code !== task.code));
  return new Map([...sel, [task.code, { code: task.code, parent: task.parent }]]);
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
export const BATCH_CLEAR_BUTTON = "Bỏ chọn tất cả";
export const BATCH_NOTE =
  "Mỗi nhiệm vụ được xoá riêng, lần lượt từng việc, cùng một lý do. Xoá mềm: dòng ở lại cùng người " +
  "xoá và lý do, mã đã cấp không bao giờ cấp lại. Việc nào máy chủ từ chối thì vẫn được chọn, kèm " +
  "câu trả lời của máy chủ.";

/** `Đã chọn {N} nhiệm vụ` — verbatim §2. */
export function selectedCountLabel(n: number): string {
  return `Đã chọn ${n} nhiệm vụ`;
}

export function batchProgressText(done: number, total: number): string {
  return `Đang xoá ${done}/${total}… Mỗi việc chờ máy chủ trả lời rồi mới sang việc kế tiếp.`;
}

/** One line per task — which went, which did not and why, in the server's own words. */
export function batchResultLine(r: BatchDeleteResult): string {
  return r.ok ? `${r.code} — đã xoá.` : `${r.code} — chưa xoá: ${r.message}`;
}

export function batchSummary(results: readonly BatchDeleteResult[]): string {
  const ok = results.filter((r) => r.ok).length;
  const failed = results.length - ok;
  return failed === 0
    ? `Đã xoá ${ok}/${results.length} nhiệm vụ.`
    : `Đã xoá ${ok}/${results.length} nhiệm vụ; ${failed} việc chưa xoá được — xem từng dòng bên dưới.`;
}

/** Accessible name of a row's / card's checkbox: which task, among twenty. */
export function selectLabel(code: string): string {
  return `Chọn ${code}`;
}
