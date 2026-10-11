"use client";

import { useId } from "react";

import { Button } from "@/components/ui/button";
import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";
import { BUSY_DELETING, BusyLabel } from "@/features/danh-ba/busy-label";
import type { identity_boPhanRa, identity_orgUnitHoldingsOut } from "@/lib/api/schema.gen";

import {
  DELETE_CANCEL_BUTTON,
  DELETE_CONFIRM_BUTTON,
  DELETE_EXPLANATION,
  deleteFormTitle,
  holdingSentences,
} from "./org-unit-delete";
import { DIALOG_FOOTER_CLASS } from "./org-unit-dialog-classes";

/** `id` of the Huỷ button — focus goes there when the dialog opens: a stray Enter must not delete. */
export const DELETE_CANCEL_ID = "nut-huy-xoa-bo-phan";

/** The server's last refusal: its sentence, and the counts when it was a 409 `org_unit_in_use`. */
export type DeleteRefusal = {
  readonly message: string;
  readonly holdings: identity_orgUnitHoldingsOut | null;
};

/**
 * The delete dialog of a unit, opened from the card's Trash2. PURE RENDERING: every value in by
 * props, every change out by callbacks — exported so each refusal has a test
 * (`org-unit-delete.test.tsx`).
 *
 * A CONFIRM, NO REASON BOX (bug sheet row 30, owner decision 10/10/2026): the question, the rule,
 * Huỷ and the red Xoá. The request carries an empty reason; the server records its fixed sentence.
 *
 * EVERY REFUSAL STAYS IN THE DIALOG AS ITS OWN SENTENCE: a 409 with readable `holdings` becomes one
 * sentence per kind held ("Bộ phận còn 3 cán bộ trực thuộc…"); any other refusal — the 409 without
 * counts, 503 `org_unit_delete_not_configured` / `org_unit_delete_unavailable`, 404, 403 — is the
 * server's sentence verbatim. Once what the unit holds is moved, the same click is the retry.
 */
export function OrgUnitDeleteForm({
  unit,
  refusal,
  sending,
  onSubmit,
  onCancel,
}: {
  unit: identity_boPhanRa;
  refusal: DeleteRefusal | null;
  sending: boolean;
  onSubmit: () => void;
  onCancel: () => void;
}) {
  const titleId = useId();
  const title = deleteFormTitle(unit.name);
  const held = refusal?.holdings ? holdingSentences(refusal.holdings) : [];
  // A 409 whose counts are all zero (or unreadable) still has the server's sentence to say.
  const sentences = refusal === null ? [] : held.length > 0 ? held : [refusal.message];
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
        {sentences.length > 0 && (
          <div role="alert" className="text-danger m-0 space-y-1 text-[12px] font-medium [&_p]:m-0">
            {sentences.map((s) => (
              <p key={s}>{s}</p>
            ))}
          </div>
        )}

        <div className={DIALOG_FOOTER_CLASS}>
          <Button type="button" variant="outline" id={DELETE_CANCEL_ID} onClick={onCancel} disabled={sending}>
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
