"use client";

import { useId } from "react";

import { Button } from "@/components/ui/button";
import { controlClass } from "@/components/ui/field";
import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";
import { BUSY_DELETING, BusyLabel } from "@/features/danh-ba/busy-label";
import type { identity_boPhanRa, identity_orgUnitHoldingsOut } from "@/lib/api/schema.gen";
import { cn } from "@/lib/cn";

import {
  DELETE_CANCEL_BUTTON,
  DELETE_CONFIRM_BUTTON,
  DELETE_EXPLANATION,
  DELETE_REASON_HINT,
  DELETE_REASON_LABEL,
  HOLDINGS_HEADING,
  deleteFormTitle,
  holdingLines,
} from "./org-unit-delete";
import { DIALOG_FOOTER_CLASS, DIALOG_LABEL_CLASS } from "./org-unit-dialog-classes";

/** `id` of the reason box — focus goes there when the form opens. */
export const DELETE_REASON_ID = "o-ly-do-xoa-bo-phan";

/** The server's last refusal: its sentence, and the counts when it was a 409 `org_unit_in_use`. */
export type DeleteRefusal = {
  readonly message: string;
  readonly holdings: identity_orgUnitHoldingsOut | null;
};

/**
 * The delete dialog of a unit, opened from the card's Trash2. PURE RENDERING: every value in by
 * props, every change out by callbacks — exported so the 409 rendering has a test
 * (`org-unit-delete.test.tsx`).
 *
 * SPEC 03 DELETES WITHOUT ASKING; THIS ASKS FOR A REASON. ADR 0079 "Giữ bất kể spec": the server
 * requires `reason` (rule 7, ADR 0056), so the step stays and only its look follows the spec's dialog
 * (title, description, `DialogFooter`). The reason is checked before sending (`kiemLyDoXoa`) because
 * a round-trip to be told it is blank is a worse sentence.
 *
 * The refusal (409 holdings / 503) stays IN the dialog, as the server's sentence verbatim plus the
 * non-zero counts: once what the unit holds is moved, the same click is the retry.
 */
export function OrgUnitDeleteForm({
  unit,
  reason,
  setReason,
  localError,
  refusal,
  sending,
  onSubmit,
  onCancel,
}: {
  unit: identity_boPhanRa;
  reason: string;
  setReason: (s: string) => void;
  localError: string;
  refusal: DeleteRefusal | null;
  sending: boolean;
  onSubmit: () => void;
  onCancel: () => void;
}) {
  const titleId = useId();
  const title = deleteFormTitle(unit.name);
  const lines = refusal?.holdings ? holdingLines(refusal.holdings) : [];
  return (
    <ModalDialog
      titleId={titleId}
      onDismiss={() => {
        if (!sending) onCancel();
      }}
      closeDisabled={sending}
      className="sm:max-w-lg"
    >
      <ModalDialogHeader titleId={titleId} title={title} description={DELETE_EXPLANATION} />
      <form
        className="m-0 flex min-h-0 min-w-0 flex-col gap-4"
        aria-label={title}
        onSubmit={(e) => {
          e.preventDefault();
          onSubmit();
        }}
      >
        <div className="min-h-0 space-y-4 overflow-y-auto">
          <div className="block">
            <label htmlFor={DELETE_REASON_ID} className={DIALOG_LABEL_CLASS}>
              {DELETE_REASON_LABEL}
            </label>
            <textarea
              id={DELETE_REASON_ID}
              name="reason"
              value={reason}
              required
              rows={3}
              onChange={(e) => setReason(e.target.value)}
              aria-invalid={localError !== ""}
              aria-describedby="giai-thich-ly-do-xoa-bo-phan"
              className={cn(controlClass, "mt-1.5 h-auto min-h-16 resize-y py-2")}
            />
            <p id="giai-thich-ly-do-xoa-bo-phan" className="text-ink-muted m-0 mt-1.5 text-[12px]">
              {DELETE_REASON_HINT}
            </p>
          </div>

          {localError !== "" && (
            <p role="alert" className="text-danger m-0 text-[12px] font-medium">
              {localError}
            </p>
          )}

          {refusal !== null && (
            <div role="alert" className="text-danger m-0 text-[12px] font-medium [&_p]:m-0 [&_ul]:my-1 [&_ul]:pl-5">
              {/* The server's sentence first, verbatim — it already says "chuyển trước khi xoá". */}
              <p>{refusal.message}</p>
              {lines.length > 0 && (
                <>
                  <p>{HOLDINGS_HEADING}</p>
                  <ul>
                    {lines.map((l) => (
                      <li key={l}>{l}</li>
                    ))}
                  </ul>
                </>
              )}
            </div>
          )}
        </div>

        <div className={DIALOG_FOOTER_CLASS}>
          <Button type="button" variant="outline" onClick={onCancel} disabled={sending}>
            {DELETE_CANCEL_BUTTON}
          </Button>
          <Button type="submit" variant="danger" disabled={sending} aria-busy={sending}>
            <BusyLabel busy={sending} label={DELETE_CONFIRM_BUTTON} busyText={BUSY_DELETING} />
          </Button>
        </div>
      </form>
    </ModalDialog>
  );
}
