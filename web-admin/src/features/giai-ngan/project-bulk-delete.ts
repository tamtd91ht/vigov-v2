/**
 * `Xoá đã chọn` of the project register — pure half: the sequential run and its words
 * (prototype `BudgetWorkspace.tsx:99-113`, `deleteEachOf` + `reportBulkDelete`).
 *
 * THERE IS NO BATCH ROUTE: one `DELETE /api/v1/investment-projects/{id}` per selected project, one
 * after another. Each call is its own soft delete and its own audit entry (rule 6), and each is
 * accepted or refused on its own — so the result is reported PER PROJECT, never as "đã xoá 5 dự án"
 * when two of the five were refused (a project with locked vouchers answers 409).
 *
 * NO REASON (user decision 06/10/2026): the server writes its fixed sentence into `delete_reason`
 * (`xoaDuAn`). `budget.update` gates the controls; every call is checked again by the route (rule 5).
 */

import type { KetQua } from "@/lib/api/goi"; // vi-name-ok: existing result type of goi.ts (rule 12 invariant 3)
import type { finance_duAnRa } from "@/lib/api/schema.gen";

/** What the run and the confirm list need of a row. */
export type SelectedProject = Pick<finance_duAnRa, "id" | "code" | "name" | "planned_amount">;

export type ProjectDeleteResult =
  | { readonly code: string; readonly ok: true }
  | { readonly code: string; readonly ok: false; readonly message: string };

/**
 * One call per project, SEQUENTIALLY — the next starts only after the previous answered: N concurrent
 * writes on one commune's register from one click is load nobody asked for, and the per-row result
 * would arrive in an order the list does not show.
 *
 * NEVER STOPS AT A REFUSAL: one project that cannot go does not make the others wrong.
 */
export async function deleteProjectsInTurn(
  projects: readonly SelectedProject[],
  softDelete: (id: string) => Promise<KetQua<unknown>>,
  onProgress?: (done: number) => void,
): Promise<ProjectDeleteResult[]> {
  const results: ProjectDeleteResult[] = [];
  for (const p of projects) {
    const r = await softDelete(p.id);
    results.push(r.ok ? { code: p.code, ok: true } : { code: p.code, ok: false, message: r.thongBao });
    onProgress?.(results.length);
  }
  return results;
}

/* ── words ─────────────────────────────────────────────────────────────────────────────────── */

export const BULK_DELETE_BUTTON = "Xoá đã chọn";

/** `Đã chọn N dự án` — the prototype's bar. */
export function selectedProjectsLabel(n: number): string {
  return `Đã chọn ${n} dự án`;
}

export function bulkDeleteTitle(n: number): string {
  return `Xoá ${n} dự án khỏi danh sách?`;
}

export function bulkDeleteConfirmLabel(n: number): string {
  return `Xoá ${n} dự án`;
}

export const BULK_DELETE_NOTE =
  "Từng dự án được xoá lần lượt. Dự án sẽ không còn trong danh sách và trong số liệu của năm, nhưng " +
  "bản ghi vẫn được giữ trong hệ thống cùng người xoá, và mã dự án không được cấp lại. Dự án nào máy " +
  "chủ từ chối thì vẫn còn trong danh sách, kèm câu trả lời của máy chủ.";

export function bulkProgressText(done: number, total: number): string {
  return `Đang xoá ${done}/${total}… Mỗi dự án chờ máy chủ trả lời rồi mới sang dự án kế tiếp.`;
}

/** One line per project — which went, which did not and why, in the server's own words. */
export function bulkResultLine(r: ProjectDeleteResult): string {
  return r.ok ? `${r.code} — đã xoá.` : `${r.code} — chưa xoá: ${r.message}`;
}

export function bulkSummary(results: readonly ProjectDeleteResult[]): string {
  const ok = results.filter((r) => r.ok).length;
  const failed = results.length - ok;
  return failed === 0
    ? `Đã xoá ${ok}/${results.length} dự án.`
    : `Đã xoá ${ok}/${results.length} dự án; ${failed} dự án chưa xoá được — xem từng dòng bên dưới.`;
}
