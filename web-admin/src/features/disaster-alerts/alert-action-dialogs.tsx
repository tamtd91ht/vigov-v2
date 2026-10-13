"use client";

import { CircleStop, LoaderCircle, Trash2, X } from "lucide-react";
import { useState, type FormEvent } from "react";

import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { IconButton } from "@/components/ui/icon-button";
import {
  deleteReasonCounter,
  deleteReasonLength,
  deleteReasonReady,
  DELETE_REASON_MAX_CHARS,
} from "@/features/noi-dung/nhan-noi-dung";
import { OverlayDialog, SMALL_DIALOG_CLASS } from "@/features/noi-dung/overlay-dialog";
import type { DisasterAlert } from "@/lib/api/disaster-alerts";
import type { KetQua } from "@/lib/api/goi";
import type { CallResult } from "@/lib/api/task-attachments";

import {
  CANCEL_BUTTON,
  CLOSE_LABEL,
  DELETE_BUTTON,
  DELETE_NOTE,
  DELETE_REASON_LABEL,
  DELETE_TITLE,
  END_BUTTON,
  END_CONFIRM_NOTE,
  END_CONFIRM_TITLE,
  kindLabel,
} from "./alert-form";

const END_TITLE_ID = "disaster-alert-end-title";
const DELETE_TITLE_ID = "disaster-alert-delete-title";
const REASON_ID = "disaster-alert-delete-reason";

function AlertName({ alert }: { alert: DisasterAlert }) {
  return (
    <p className="m-0 rounded-xl border border-line bg-surface-muted px-3.5 py-3 text-sm">
      <span className="text-ink-muted">{kindLabel(alert.kind)} · </span>
      <span className="font-semibold text-ink-900">{alert.title}</span>
    </p>
  );
}

/**
 * `Kết thúc sớm` — asks first: ending takes the alert off every resident's Mini App at once and is final
 * (the server answers 409 to any edit after it). The server's refusal is shown verbatim, the dialog stays.
 */
export function EndAlertDialog({
  alert,
  end,
  ended,
  close,
}: {
  alert: DisasterAlert;
  end: (id: string) => Promise<KetQua<DisasterAlert>>;
  ended: (row: DisasterAlert) => void;
  close: () => void;
}) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  function dismiss(): void {
    if (!busy) close();
  }

  function confirm(): void {
    if (busy) return;
    setBusy(true);
    setError(null);
    void end(alert.id).then((r) => {
      setBusy(false);
      if (r.ok) ended(r.duLieu);
      else setError(r.thongBao);
    });
  }

  return (
    <OverlayDialog titleId={END_TITLE_ID} onDismiss={dismiss} className={SMALL_DIALOG_CLASS}>
      <div className="flex flex-col gap-4">
        <div className="flex items-center gap-3 border-b border-line pb-3">
          <span aria-hidden="true" className="grid size-9 shrink-0 place-items-center rounded-full bg-danger-50 text-danger-600">
            <CircleStop className="size-[18px]" strokeWidth={1.8} focusable="false" />
          </span>
          <h3 id={END_TITLE_ID} className="m-0 min-w-0 flex-1 text-base font-semibold text-ink-900">
            {END_CONFIRM_TITLE}
          </h3>
          <IconButton type="button" label={CLOSE_LABEL} className="min-h-0" disabled={busy} onClick={dismiss}>
            <X aria-hidden="true" focusable="false" />
          </IconButton>
        </div>
        <AlertName alert={alert} />
        <p className="ghi-chu m-0 text-[13px]">{END_CONFIRM_NOTE}</p>
        {error !== null && (
          <p className="thong-bao-loi m-0" role="alert">
            {error}
          </p>
        )}
        <div className="flex flex-wrap justify-end gap-2 border-t border-line pt-3">
          <Button type="button" variant="secondary" disabled={busy} onClick={dismiss}>
            {CANCEL_BUTTON}
          </Button>
          <Button
            type="button"
            variant="danger"
            disabled={busy}
            aria-busy={busy || undefined}
            onClick={confirm}
            icon={
              busy ? (
                <LoaderCircle aria-hidden="true" focusable="false" className="animate-spin" />
              ) : (
                <CircleStop aria-hidden="true" focusable="false" />
              )
            }
          >
            {END_BUTTON}
          </Button>
        </div>
      </div>
    </OverlayDialog>
  );
}

/**
 * Delete — a SOFT delete with a MANDATORY reason (rule 7), the content screen's dialog in shape and rules
 * (`content-delete-dialog.tsx`): 204 → `deleted`; 404 → `gone` (deleted meanwhile, or not this commune's —
 * one answer); anything else → the server's sentence, the typed reason kept. The reason counts as the
 * server counts it (runes after trimming, 500).
 */
export function DeleteAlertDialog({
  alert,
  remove,
  deleted,
  gone,
  close,
}: {
  alert: DisasterAlert;
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
    void remove(alert.id, reason.trim()).then((r) => {
      setBusy(false);
      if (r.ok) deleted();
      else if (r.status === 404) gone();
      else setError(r.message);
    });
  }

  return (
    <OverlayDialog titleId={DELETE_TITLE_ID} onDismiss={dismiss} className={SMALL_DIALOG_CLASS}>
      <form className="flex flex-col gap-4" onSubmit={submit} aria-labelledby={DELETE_TITLE_ID}>
        <div className="flex items-center gap-3 border-b border-line pb-3">
          <span aria-hidden="true" className="grid size-9 shrink-0 place-items-center rounded-full bg-danger-50 text-danger-600">
            <Trash2 className="size-[18px]" strokeWidth={1.8} focusable="false" />
          </span>
          <h3 id={DELETE_TITLE_ID} className="m-0 min-w-0 flex-1 text-base font-semibold text-ink-900">
            {DELETE_TITLE}
          </h3>
          <IconButton type="button" label={CLOSE_LABEL} className="min-h-0" disabled={busy} onClick={dismiss}>
            <X aria-hidden="true" focusable="false" />
          </IconButton>
        </div>
        <AlertName alert={alert} />
        <p className="ghi-chu m-0 text-[13px]">{DELETE_NOTE}</p>
        <div className="flex flex-col gap-1.5">
          <Field
            label={DELETE_REASON_LABEL}
            htmlFor={REASON_ID}
            required
            grow="auto"
            className="[&_textarea]:py-2.5 [&_textarea[aria-invalid=true]]:border-danger-600"
          >
            <textarea
              id={REASON_ID}
              rows={3}
              required
              value={reason}
              aria-describedby={`${REASON_ID}-counter`}
              aria-invalid={tooLong}
              onChange={(e) => setReason(e.target.value)}
            />
          </Field>
          <p
            id={`${REASON_ID}-counter`}
            className={tooLong ? "ghi-chu m-0 text-right text-danger-600" : "ghi-chu m-0 text-right"}
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
            {CANCEL_BUTTON}
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
            {DELETE_BUTTON}
          </Button>
        </div>
      </form>
    </OverlayDialog>
  );
}
