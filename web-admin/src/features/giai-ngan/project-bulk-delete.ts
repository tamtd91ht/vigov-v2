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

/**
 * Spec 02 §9's description, MADE TRUE FOR OUR SERVER (ADR 0068 lần 6 #9 brief): the spec says the
 * projects leave "cùng với các khoản chi đã ghi" and that only projects with LOCKED vouchers are
 * refused; our server refuses ANY project that still has vouchers (409 `project_has_vouchers`, ADR 0075
 * #4a), so no voucher ever leaves with a project.
 */
export const BULK_DELETE_NOTE =
  "Những dự án này sẽ khuất khỏi danh sách và khỏi báo cáo tiến độ. Bản ghi vẫn được giữ trong hệ " +
  "thống, nhưng giao diện không có nút hoàn tác. Dự án đã có phiếu chi sẽ bị từ chối.";

export function bulkProgressText(done: number, total: number): string {
  return `Đang xoá ${done}/${total}…`;
}

/** Spec 02 §9 success toast, counting only what the server accepted; `null` when nothing went. */
export function bulkSuccessToast(results: readonly ProjectDeleteResult[]): string | null {
  const ok = results.filter((r) => r.ok).length;
  return ok === 0 ? null : `Đã xoá ${ok} dự án.`;
}

/**
 * Spec 02 §9 error toasts, one per distinct refusal: "{n} dự án không xoá được: {lý do}". The reason
 * is THE SERVER'S sentence — the spec's own reasons are keyed by code, and `KetQua` carries none.
 */
export function bulkFailureToasts(results: readonly ProjectDeleteResult[]): string[] {
  const byReason = new Map<string, number>();
  for (const r of results) {
    if (!r.ok) byReason.set(r.message, (byReason.get(r.message) ?? 0) + 1);
  }
  return [...byReason].map(([message, n]) => `${n} dự án không xoá được: ${message}`);
}
