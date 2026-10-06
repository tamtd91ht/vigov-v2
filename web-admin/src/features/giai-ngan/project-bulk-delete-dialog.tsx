"use client";

import { Trash2 } from "lucide-react";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";
import { BUSY_DELETING, BusyLabel } from "@/features/danh-ba/busy-label";
import { xoaDuAn } from "@/lib/api/giai-ngan"; // vi-name-ok: existing delete call (rule 12, invariant 3)

import { nhanTien } from "./nhan-du-an"; // vi-name-ok: existing formatter (rule 12, invariant 3)
import {
  BULK_DELETE_NOTE,
  bulkDeleteConfirmLabel,
  bulkDeleteTitle,
  bulkProgressText,
  bulkResultLine,
  bulkSummary,
  deleteProjectsInTurn,
  type ProjectDeleteResult,
  type SelectedProject,
} from "./project-bulk-delete";
import { Glyph } from "./project-ui";

const TITLE_ID = "tieu-de-xoa-du-an-da-chon";

/**
 * The confirm dialog of `Xoá đã chọn` (prototype `BulkDeleteDialog`, `BudgetWorkspace.tsx:409-422`):
 * the specific question, the consequence, every selected project as `name` over `code · plan`, then
 * the run — progress while it goes, one line per project when it is done.
 *
 * `projects` IS A SNAPSHOT taken when the dialog opened: the register re-reads as soon as the run ends
 * (`onFinished`), and the result lines must keep naming the projects that were sent, not the new list.
 *
 * Stays open after the run so the per-project result can be read; `Đóng` closes it.
 */
export function ProjectBulkDeleteDialog({
  projects,
  onClose,
  onFinished,
  softDelete = xoaDuAn,
}: {
  projects: readonly SelectedProject[];
  onClose: () => void;
  /** The run ended (whatever its results): the register re-reads. */
  onFinished: () => void;
  /** Injected in tests; the real call otherwise. */
  softDelete?: (id: string) => ReturnType<typeof xoaDuAn>;
}) {
  const [progress, setProgress] = useState<number | null>(null);
  const [results, setResults] = useState<readonly ProjectDeleteResult[] | null>(null);
  const running = progress !== null;
  const total = projects.length;

  async function run(): Promise<void> {
    if (running || results !== null || total === 0) return;
    setProgress(0);
    const out = await deleteProjectsInTurn(projects, (id) => softDelete(id), setProgress);
    setProgress(null);
    setResults(out);
    onFinished();
  }

  return (
    <ModalDialog titleId={TITLE_ID} onDismiss={() => !running && onClose()}>
      <ModalDialogHeader titleId={TITLE_ID} title={bulkDeleteTitle(total)} description={BULK_DELETE_NOTE} />

      <div className="min-h-0 overflow-y-auto">
        {results === null ? (
          <ul className="m-0 flex list-none flex-col gap-1.5 p-0" aria-label="Các dự án sẽ xoá">
            {projects.map((p) => (
              <li key={p.id} className="rounded-control border border-solid border-line px-3 py-2">
                <span className="block text-sm font-semibold text-ink-900">{p.name}</span>
                <span className="block text-xs text-ink-500 tabular-nums">
                  {p.code} · {nhanTien(p.planned_amount)}
                </span>
              </li>
            ))}
          </ul>
        ) : (
          <ul className="m-0 flex list-none flex-col gap-1 p-0" aria-label="Kết quả xoá từng dự án">
            {results.map((r) => (
              <li key={r.code} data-result={r.ok ? "ok" : "refused"} className={r.ok ? "text-sm text-ink-700" : "thong-bao-loi m-0"}>
                {bulkResultLine(r)}
              </li>
            ))}
          </ul>
        )}
      </div>

      {/* ALWAYS IN THE DOM: a live region inserted later is one not every screen reader announces. */}
      <p role="status" className="m-0 text-sm text-ink-700 empty:hidden">
        {progress !== null ? bulkProgressText(progress, total) : results !== null ? bulkSummary(results) : ""}
      </p>

      <div className="flex shrink-0 flex-wrap justify-end gap-2">
        {results === null && (
          <Button
            type="button"
            variant="danger"
            icon={<Glyph icon={Trash2} />}
            disabled={running || total === 0}
            aria-busy={running || undefined}
            onClick={() => void run()}
          >
            <BusyLabel busy={running} label={bulkDeleteConfirmLabel(total)} busyText={BUSY_DELETING} />
          </Button>
        )}
        <Button type="button" variant="secondary" disabled={running} onClick={onClose}>
          {results === null ? "Huỷ" : "Đóng"}
        </Button>
      </div>
    </ModalDialog>
  );
}
