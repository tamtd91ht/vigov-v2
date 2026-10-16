"use client";

import { Trash2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Field } from "@/components/ui/field";
import type { CitizenContact } from "@/lib/api/citizen-contacts";

import {
  CANCEL_BUTTON,
  DELETE_CONFIRM_BUTTON,
  DELETE_CONTACT_EXPLANATION,
  DELETE_REASON_LABEL,
  deleteContactQuestion,
} from "./citizen-directory-words";

export const DELETE_REASON_INPUT_ID = "ly-do-go-nguoi-dan"; // vi-name-ok: DOM id, same convention as the screen's other ids

/**
 * The confirm box of the trash icon (chapter 16 §4, ADR 0097 D7): the specific question, the consequence,
 * a MANDATORY reason, Huỷ and the red button. Drawn above the table, not inside the row (the shape
 * `HopXoa` uses: a box nested in a 320px row is unreadable).
 *
 * PRESENTATION ONLY: the reason, its error, the busy flag and the handlers are the workspace's. The
 * server refuses an empty reason too (400); the box refuses it first (`deleteReasonProblem`) so the
 * person is told under the field, not by a toast.
 */
export function CitizenContactDeleteDialog({
  contact,
  reason,
  setReason,
  error,
  serverError,
  busy,
  onConfirm,
  onCancel,
}: {
  contact: CitizenContact;
  reason: string;
  setReason: (v: string) => void;
  /** The local refusal (empty reason), under the field. */
  error: string | null;
  /** The server's sentence, verbatim, or "". */
  serverError: string;
  busy: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}) {
  return (
    <ConfirmDialog
      as="form"
      tone="danger"
      titleAs="h3"
      title={deleteContactQuestion(contact.full_name)}
      aria-label={deleteContactQuestion(contact.full_name)}
      className="mb-4"
      // The empty reason is refused by `deleteReasonProblem` and said UNDER the field, in Vietnamese —
      // the browser's own bubble is in the browser's language and vanishes.
      noValidate
      onSubmit={(e) => {
        e.preventDefault();
        onConfirm();
      }}
      actions={
        <>
          <Button type="button" variant="secondary" onClick={onCancel} disabled={busy}>
            {CANCEL_BUTTON}
          </Button>
          <Button type="submit" variant="danger" icon={<Trash2 aria-hidden="true" />} disabled={busy} aria-busy={busy || undefined}>
            {DELETE_CONFIRM_BUTTON}
          </Button>
        </>
      }
    >
      <p className="m-0">{DELETE_CONTACT_EXPLANATION}</p>
      <Field label={DELETE_REASON_LABEL} htmlFor={DELETE_REASON_INPUT_ID} required grow="auto" error={error ?? undefined}>
        <input
          id={DELETE_REASON_INPUT_ID}
          value={reason}
          required
          aria-invalid={error !== null}
          autoComplete="off"
          onChange={(e) => setReason(e.target.value)}
        />
      </Field>
      {serverError !== "" && (
        <p className="thong-bao-loi m-0" role="alert">
          {serverError}
        </p>
      )}
    </ConfirmDialog>
  );
}
