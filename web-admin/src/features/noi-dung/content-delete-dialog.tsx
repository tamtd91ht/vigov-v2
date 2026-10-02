"use client";

import { useState, type FormEvent } from "react";

import type { CallResult } from "@/lib/api/task-attachments";
import type { comms_noiDungRa } from "@/lib/api/schema.gen";

import {
  CONTENT_DELETE_NOTE,
  CONTENT_DELETE_REASON_LABEL,
  CONTENT_DELETE_SUBMIT,
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
      <form onSubmit={submit} aria-labelledby={TITLE_ID}>
        <h3 id={TITLE_ID}>{CONTENT_DELETE_TITLE}</h3>
        <p>
          <span className="ten-can-bo">{item.title}</span>
        </p>
        <p className="ghi-chu">{CONTENT_DELETE_NOTE}</p>

        <div className="o-nhap">
          <label htmlFor={REASON_ID}>{CONTENT_DELETE_REASON_LABEL}</label>
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
          <p className="ghi-chu" id={`${REASON_ID}-counter`} aria-live="polite">
            {deleteReasonCounter(reason)}
          </p>
        </div>

        {error !== null && (
          <p className="thong-bao-loi" role="alert">
            {error}
          </p>
        )}

        <div className="cum-nut">
          <button type="button" className="nut-phu" disabled={busy} onClick={dismiss}>
            {NHAN_NUT_HUY}
          </button>
          <button type="submit" className="nut-phu nut-xoa" disabled={busy || !ready}>
            {CONTENT_DELETE_SUBMIT}
          </button>
        </div>
      </form>
    </OverlayDialog>
  );
}
