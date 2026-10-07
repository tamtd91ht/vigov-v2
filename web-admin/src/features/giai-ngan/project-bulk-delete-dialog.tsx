"use client";

import { Trash2 } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";
import { xoaDuAn } from "@/lib/api/giai-ngan"; // vi-name-ok: existing delete call (rule 12, invariant 3)

import { nhanTien } from "./nhan-du-an"; // vi-name-ok: existing formatter (rule 12, invariant 3)
import {
  BULK_DELETE_NOTE,
  bulkDeleteConfirmLabel,
  bulkDeleteTitle,
  bulkFailureToasts,
  bulkProgressText,
  bulkSuccessToast,
  deleteProjectsInTurn,
  type SelectedProject,
} from "./project-bulk-delete";
import { Glyph } from "./project-ui";

const TITLE_ID = "tieu-de-xoa-du-an-da-chon";

/**
 * The confirm dialog of `Xoá đã chọn` (spec 02 §9, prototype `BulkDeleteDialog`): the specific question,
 * the consequence, every selected project in ONE bordered list as `name` + `code · plan`, then
 * `[Huỷ] [Xoá N dự án]` with the delete button solid red.
 *
 * AFTER THE RUN THE DIALOG CLOSES and the outcome is a toast, as the prototype does: "Đã xoá N dự án."
 * for what went, and one error toast per distinct refusal — "N dự án không xoá được: {the server's
 * sentence}". Every project is still deleted ONE BY ONE and reported by the server one by one
 * (`project-bulk-delete.ts`); nothing is summed into a success that did not happen.
 *
 * `projects` IS A SNAPSHOT taken when the dialog opened: the register re-reads as soon as the run ends
 * (`onFinished`).
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
  const running = progress !== null;
  const total = projects.length;

  async function run(): Promise<void> {
    if (running || total === 0) return;
    setProgress(0);
    const out = await deleteProjectsInTurn(projects, (id) => softDelete(id), setProgress);
    setProgress(null);
    const ok = bulkSuccessToast(out);
    if (ok !== null) toast.success(ok);
    for (const line of bulkFailureToasts(out)) toast.error(line);
    onFinished();
    onClose();
  }

  return (
    <ModalDialog titleId={TITLE_ID} className="max-w-125" onDismiss={() => !running && onClose()}>
      <ModalDialogHeader titleId={TITLE_ID} title={bulkDeleteTitle(total)} description={BULK_DELETE_NOTE} />

      <ul
        className="border-line m-0 max-h-56 list-none space-y-1.5 overflow-y-auto rounded-md border border-solid p-3 text-[12.5px]"
        aria-label="Các dự án sẽ xoá"
      >
        {projects.map((p) => (
          <li key={p.id} className="flex flex-wrap items-baseline justify-between gap-x-3">
            <span className="text-navy font-medium">{p.name}</span>
            <span className="text-ink-muted text-[11.5px] tabular-nums">
              {p.code} · {nhanTien(p.planned_amount)}
            </span>
          </li>
        ))}
      </ul>

      {/* ALWAYS IN THE DOM: a live region inserted later is one not every screen reader announces. */}
      <p role="status" className="text-ink-muted m-0 text-[12px] empty:hidden">
        {progress !== null ? bulkProgressText(progress, total) : ""}
      </p>

      <div className="flex shrink-0 flex-wrap justify-end gap-2">
        <Button type="button" variant="outline" disabled={running} onClick={onClose}>
          Huỷ
        </Button>
        <Button
          type="button"
          variant="primary"
          className="bg-danger hover:not-disabled:bg-danger/90 text-white"
          icon={<Glyph icon={Trash2} className="size-4" />}
          disabled={running || total === 0}
          aria-busy={running || undefined}
          onClick={() => void run()}
        >
          {bulkDeleteConfirmLabel(total)}
        </Button>
      </div>
    </ModalDialog>
  );
}
