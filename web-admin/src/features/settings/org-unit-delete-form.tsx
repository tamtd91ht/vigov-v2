"use client";

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
 * The inline delete form under a unit card. PURE RENDERING: every value in by props, every change
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
    <form
      className="form-danh-muc form-bo-phan"
      aria-label={title}
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit();
      }}
      onKeyDown={(e) => {
        if (e.key === "Escape" && !sending) onCancel();
      }}
    >
      <h4>{title}</h4>
      <p className="ghi-chu">{DELETE_EXPLANATION}</p>

      <div className="o-nhap">
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
        <p className="thong-bao-loi" role="alert">
          {localError}
        </p>
      )}

      {refusal !== null && (
        <div className="thong-bao-loi" role="alert">
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

      <div className="cum-nut">
        <button type="submit" className="nut-chinh nut-xoa" disabled={sending}>
          {DELETE_CONFIRM_BUTTON}
        </button>
        <button type="button" className="nut-phu" onClick={onCancel} disabled={sending}>
          {DELETE_CANCEL_BUTTON}
        </button>
      </div>
    </form>
  );
}
