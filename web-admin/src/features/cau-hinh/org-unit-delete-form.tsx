"use client";

import { Trash2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { BUSY_DELETING, BusyLabel } from "@/features/danh-ba/busy-label";
import type { identity_boPhanRa, identity_orgUnitHoldingsOut } from "@/lib/api/schema.gen";

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

/** `id` of the reason box — focus goes there when the form opens. */
export const DELETE_REASON_ID = "o-ly-do-xoa-bo-phan";

/** The server's last refusal: its sentence, and the counts when it was a 409 `org_unit_in_use`. */
export type DeleteRefusal = {
  readonly message: string;
  readonly holdings: identity_orgUnitHoldingsOut | null;
};

/**
 * The delete form of a unit, opened in a dialog from the card's 🗑. PURE RENDERING: every value in by props, every change
 * out by callbacks — exported so the 409 rendering has a test (`org-unit-delete.test.tsx`).
 *
 * THE REASON IS REQUIRED and checked before sending (`kiemLyDoXoa`), because the server refuses a
 * blank one anyway and a round-trip to be told so is a worse sentence.
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
  const title = deleteFormTitle(unit.name);
  const lines = refusal?.holdings ? holdingLines(refusal.holdings) : [];
  return (
    // The existing confirm form, framed as the shared ConfirmDialog (spec v2 §7): same element,
    // same `onSubmit`/`onKeyDown`, the specific title, a red confirm button that names the action.
    <ConfirmDialog
      as="form"
      // Shown inside the tab's dialog (ADR 0068 lần 5): the dialog is the box, so no second padding/shadow.
      className="form-danh-muc form-bo-phan m-0 p-0 shadow-none"
      aria-label={title}
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit();
      }}
      onKeyDown={(e) => {
        if (e.key === "Escape" && !sending) onCancel();
      }}
      title={title}
      titleAs="h4"
      tone="danger"
      icon={Trash2}
      actions={
        <>
          <Button type="submit" variant="danger" disabled={sending} aria-busy={sending}>
            <BusyLabel busy={sending} label={DELETE_CONFIRM_BUTTON} busyText={BUSY_DELETING} />
          </Button>
          <Button type="button" variant="secondary" onClick={onCancel} disabled={sending}>
            {DELETE_CANCEL_BUTTON}
          </Button>
        </>
      }
    >
      <p className="ghi-chu m-0">{DELETE_EXPLANATION}</p>

      <div className="o-nhap m-0">
        <label htmlFor={DELETE_REASON_ID}>{DELETE_REASON_LABEL}</label>
        <textarea
          id={DELETE_REASON_ID}
          name="reason"
          value={reason}
          required
          onChange={(e) => setReason(e.target.value)}
          aria-invalid={localError !== ""}
          aria-describedby="giai-thich-ly-do-xoa-bo-phan"
        />
        <p className="ghi-chu" id="giai-thich-ly-do-xoa-bo-phan">
          {DELETE_REASON_HINT}
        </p>
      </div>

      {localError !== "" && (
        <p className="thong-bao-loi m-0" role="alert">
          {localError}
        </p>
      )}

      {refusal !== null && (
        <div className="thong-bao-loi m-0 [&_p]:m-0 [&_ul]:my-1" role="alert">
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

    </ConfirmDialog>
  );
}
