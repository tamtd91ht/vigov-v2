"use client";

import { Loader2, Trash2 } from "lucide-react";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";

import {
  BATCH_NOTE,
  BATCH_REASON_LABEL,
  batchDeleteConfirmLabel,
  batchDeleteTitle,
  batchProgressText,
  batchReason,
} from "./batch-delete";
import { INPUT_CLASS, LABEL_CLASS } from "./task-spec";

/** Id of the dialog's title — its accessible name. */
export const BATCH_DELETE_TITLE_ID = "tieu-de-xoa-da-chon";

/** One line of the dialog's list: what is about to go (spec 02 §4). */
export type BatchDeleteRow = { readonly code: string; readonly title: string; readonly holder: string };

/**
 * The confirm dialog of `Xoá đã chọn` — spec 02 §4 / prototype `BulkDeleteDialog`: the question, the
 * consequence, every selected task in ONE bordered list (title + assignee), then `[Huỷ] [Xoá N nhiệm
 * vụ]` with the delete button solid red.
 *
 * ONE DIFFERENCE, A HARD RULE (rule 7, ADR 0076 lần 2 #10): the mandatory reason, ONE for all — every
 * soft delete records who deleted it and why. The button stays off until the reason has text.
 *
 * The run is the page's (`runBatchDelete`, one call per task, children first); when it ends the page
 * closes this dialog and reports by toast. While it runs, Esc and the ✕ do nothing — the answer must
 * have somewhere to land.
 */
export function BatchDeleteDialog({
  tasks,
  progress,
  onRun,
  onClose,
}: {
  tasks: readonly BatchDeleteRow[];
  /** `{ done, total }` while running, else `null`. */
  progress: { readonly done: number; readonly total: number } | null;
  onRun: (reason: string) => void;
  onClose: () => void;
}) {
  const [text, setText] = useState("");
  const reason = batchReason(text);
  const running = progress !== null;
  const n = tasks.length;

  return (
    <ModalDialog titleId={BATCH_DELETE_TITLE_ID} closeDisabled={running} onDismiss={onClose}>
      <ModalDialogHeader titleId={BATCH_DELETE_TITLE_ID} title={batchDeleteTitle(n)} description={BATCH_NOTE} />
      <form
        className="m-0 flex min-h-0 flex-col gap-4"
        onSubmit={(e) => {
          e.preventDefault();
          if (reason !== null && !running && n > 0) onRun(reason);
        }}
      >
        <ul
          className="border-line m-0 max-h-56 list-none space-y-1.5 overflow-y-auto rounded-md border p-3 text-[12.5px]"
          aria-label="Các nhiệm vụ sẽ xoá"
        >
          {tasks.map((t) => (
            <li key={t.code} className="flex items-baseline gap-2">
              <span className="text-navy min-w-0 flex-1 font-medium">{t.title}</span>
              <span className="text-ink-muted shrink-0 text-[11.5px]">{t.holder}</span>
            </li>
          ))}
        </ul>

        <div>
          <label htmlFor="ly-do-xoa-da-chon" className={LABEL_CLASS}>
            {BATCH_REASON_LABEL}
          </label>
          <input
            id="ly-do-xoa-da-chon"
            name="ly-do-xoa-da-chon"
            value={text}
            required
            autoComplete="off"
            disabled={running}
            className={INPUT_CLASS}
            onChange={(e) => setText(e.target.value)}
          />
        </div>

        {/* ALWAYS IN THE DOM: a live region inserted later is one not every screen reader announces. */}
        <p role="status" className="text-ink-muted m-0 text-[12px] empty:hidden">
          {progress !== null ? batchProgressText(progress.done, progress.total) : ""}
        </p>

        <div className="flex flex-wrap justify-end gap-2">
          <Button type="button" variant="outline" disabled={running} onClick={onClose}>
            Huỷ
          </Button>
          <Button
            type="submit"
            variant="primary"
            className="bg-danger hover:not-disabled:bg-danger/90 text-white"
            icon={
              running ? (
                <Loader2 aria-hidden="true" focusable="false" className="size-4 animate-spin" />
              ) : (
                <Trash2 aria-hidden="true" focusable="false" className="size-4" />
              )
            }
            disabled={running || reason === null || n === 0}
            aria-busy={running || undefined}
          >
            {batchDeleteConfirmLabel(n)}
          </Button>
        </div>
      </form>
    </ModalDialog>
  );
}
