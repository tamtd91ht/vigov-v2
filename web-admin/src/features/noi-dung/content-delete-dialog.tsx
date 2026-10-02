"use client";

import { LoaderCircle, Trash2, X } from "lucide-react";
import { useState, type FormEvent } from "react";

import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { IconButton } from "@/components/ui/icon-button";
import type { CallResult } from "@/lib/api/task-attachments";
import type { comms_noiDungRa } from "@/lib/api/schema.gen";

import {
  CONTENT_DELETE_NOTE,
  CONTENT_DELETE_REASON_LABEL,
  CONTENT_DELETE_SUBMIT,
  CLOSE_LABEL,
  CONTENT_DELETE_TITLE,
  deleteReasonCounter,
  deleteReasonLength,
  deleteReasonReady,
  DELETE_REASON_MAX_CHARS,
  NHAN_NUT_HUY,
} from "./nhan-noi-dung";
import { OverlayDialog } from "./overlay-dialog";

const TITLE_ID = "content-delete-title";
const REASON_ID = "content-delete-reason";

/**
 * §6's per-row delete, in the overlay — a SOFT delete with a MANDATORY reason (rule 7).
 *
 * THE DIALOG OWNS THE CALL'S OUTCOME, the parent owns the list. Three outcomes, three exits:
 *
 *   - 204 → `deleted()`: the parent closes this and reloads the page it was on.
 *   - 404 → `gone()`: the item is no longer there (deleted by someone else — or never this commune's;
 *     the server gives one answer for both). Keeping the dialog open would offer a retry that can only
 *     fail again, so the parent closes it, says so, and reloads.
 *   - anything else → the server's sentence, verbatim, and the dialog STAYS OPEN with the typed reason:
 *     a 400 on the reason is fixed by editing it, a network error by pressing again.
 *
 * NO `maxLength` ON THE BOX, unlike the category's: `maxLength` counts UTF-16 units and would cut a
 * pasted reason silently, while the server counts runes after trimming. The counter shows the server's
 * count and `Xoá` stays off past 500 — the officer sees why instead of losing the tail of the text.
 *
 * WHILE IN FLIGHT, Esc and `Huỷ` are refused: closing would hide whether the record was deleted.
 *
 * `remove` is injected so the tests can drive every answer without a network.
 */
export function ContentDeleteDialog({
  item,
  remove,
  deleted,
  gone,
  close,
}: {
  item: comms_noiDungRa;
  remove: (id: string, reason: string) => Promise<CallResult<null>>;
  deleted: () => void;
  gone: () => void;
  close: () => void;
}) {
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const ready = deleteReasonReady(reason);
  const tooLong = deleteReasonLength(reason) > DELETE_REASON_MAX_CHARS;

  function dismiss(): void {
    if (!busy) close();
  }

  function submit(e: FormEvent): void {
    e.preventDefault();
    if (busy || !ready) return;
    setBusy(true);
    setError(null);
    remove(item.id, reason.trim()).then((r) => {
      setBusy(false);
      if (r.ok) {
        deleted();
        return;
      }
      if (r.status === 404) {
        gone();
        return;
      }
      setError(r.message);
    });
  }

  return (
    <OverlayDialog titleId={TITLE_ID} onDismiss={dismiss}>
      <form className="flex max-w-xl flex-col gap-4" onSubmit={submit} aria-labelledby={TITLE_ID}>
        <div className="flex items-center gap-3 border-b border-line pb-3">
          <span aria-hidden="true" className="grid size-9 shrink-0 place-items-center rounded-full bg-danger-50 text-danger-600">
            <Trash2 className="size-[18px]" strokeWidth={1.8} focusable="false" />
          </span>
          <h3 id={TITLE_ID} className="m-0 min-w-0 flex-1 text-base font-semibold text-ink-900">
            {CONTENT_DELETE_TITLE}
          </h3>
          {/* Same exit as `Huỷ`, refused while in flight for the same reason. */}
          <IconButton type="button" label={CLOSE_LABEL} className="min-h-0" disabled={busy} onClick={dismiss}>
            <X aria-hidden="true" focusable="false" />
          </IconButton>
        </div>
        <p className="m-0 rounded-xl border border-line bg-surface-muted px-3.5 py-3">
          <span className="ten-can-bo">{item.title}</span>
        </p>
        <p className="ghi-chu m-0 text-[13px]">{CONTENT_DELETE_NOTE}</p>

        <div className="flex flex-col gap-1.5">
          <Field label={CONTENT_DELETE_REASON_LABEL} htmlFor={REASON_ID} grow="auto"
            // Past 500 the frame turns red too — the counter's words carry the meaning, colour is second.
            className="[&_textarea]:py-2.5 [&_textarea[aria-invalid=true]]:border-danger-600"
          >
            <textarea
              id={REASON_ID}
              name={REASON_ID}
              rows={3}
              required
              value={reason}
              aria-describedby={`${REASON_ID}-counter`}
              aria-invalid={tooLong}
              onChange={(e) => setReason(e.target.value)}
            />
          </Field>
          <p
            className={tooLong ? "ghi-chu m-0 text-right text-danger-600" : "ghi-chu m-0 text-right"}
            id={`${REASON_ID}-counter`}
            aria-live="polite"
          >
            {deleteReasonCounter(reason)}
          </p>
        </div>

        {error !== null && (
          <p className="thong-bao-loi m-0" role="alert">
            {error}
          </p>
        )}

        <div className="flex flex-wrap justify-end gap-2 border-t border-line pt-3">
          <Button type="button" variant="secondary" disabled={busy} onClick={dismiss}>
            {NHAN_NUT_HUY}
          </Button>
          <Button
            type="submit"
            variant="danger"
            disabled={busy || !ready}
            aria-busy={busy || undefined}
            icon={
              busy ? (
                <LoaderCircle aria-hidden="true" focusable="false" className="animate-spin" />
              ) : (
                <Trash2 aria-hidden="true" focusable="false" />
              )
            }
          >
            {CONTENT_DELETE_SUBMIT}
          </Button>
        </div>
      </form>
    </OverlayDialog>
  );
}
