"use client";

import type { KeyboardEvent } from "react";

import { Button } from "@/components/ui/button";
import { controlClass } from "@/components/ui/field";
import { BUSY_DELETING, BusyLabel } from "@/features/danh-ba/busy-label";
import { cn } from "@/lib/cn";

// vi-name-ok: imports the existing exports of nhan-thoi-han.ts unchanged (rule 12 invariant 3)
import { NUT_HUY, NUT_XOA, O_LY_DO_XOA } from "./nhan-thoi-han";
import type { InlineRemove } from "./tab-thoi-han-xu-ly";

/**
 * The reason step of one row's removal, inside the action cell: compact (spec 08 draws only a trash
 * button), but never a one-click delete — the server keeps the row and records why (rule 7). Enter
 * confirms, Esc cancels. Refusals show in the row below, like the edit's.
 *
 * Its own file because two tables of tab "Thời hạn xử lý" remove this way — the SLA table and the
 * citizen-letter block — and the block importing it from the tab that renders the block would be a cycle.
 */
export function RemoveStep({
  remove,
  rowName,
}: {
  remove: InlineRemove;
  rowName: string;
}) {
  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      e.preventDefault();
      remove.onConfirm();
    } else if (e.key === "Escape") {
      e.preventDefault();
      remove.onCancel();
    }
  };
  return (
    <>
      <input
        name="reason"
        autoFocus
        aria-label={`${O_LY_DO_XOA} — ${rowName}`}
        aria-invalid={remove.localError !== ""}
        placeholder={O_LY_DO_XOA}
        className={cn(controlClass, "h-8 w-44 text-[12.5px]")}
        value={remove.reason}
        disabled={remove.busy}
        onChange={(e) => remove.setReason(e.currentTarget.value)}
        onKeyDown={onKeyDown}
      />
      <Button
        type="button"
        variant="danger"
        size="sm"
        disabled={remove.busy}
        aria-busy={remove.busy}
        onClick={remove.onConfirm}
      >
        <BusyLabel
          busy={remove.busy}
          label={NUT_XOA}
          busyText={BUSY_DELETING}
        />
      </Button>
      <Button
        type="button"
        variant="outline"
        size="sm"
        disabled={remove.busy}
        onClick={remove.onCancel}
      >
        {NUT_HUY}
      </Button>
    </>
  );
}
