"use client";

import { Upload } from "lucide-react";

import { PendingButton } from "@/components/ui/pending-feature";

import { PHAN_CHUA_DUNG } from "./nhan-thu-chi";

/**
 * PageHeader actions of Thu - Chi, spec §2 (`Nút phải: [⬆ Nạp từ Excel] [🗑 Gỡ]`).
 *
 * `⬆ Nạp từ Excel` is a disabled "?" placeholder (ADR 0068 §14): no route takes a file. Its
 * description is the `PHAN_CHUA_DUNG` entry, looked up by name — never a second copy of the
 * sentence. Nothing here fetches or stores anything.
 *
 * `🗑 Gỡ` is NOT here: it is built, as `Gỡ bảng` in the tree's toolbar, behind `budget.confirm` and
 * only once a sheet exists. Moving it is a behaviour change, out of a presentation pass (§1).
 */
export function BudgetSheetHeaderActions() {
  const info = PHAN_CHUA_DUNG.find((p) => p.ten === IMPORT_EXCEL);
  if (info === undefined) throw new Error(`PHAN_CHUA_DUNG has no entry "${IMPORT_EXCEL}"`);
  return (
    <PendingButton info={info} side="bottom" icon={<Upload aria-hidden="true" />}>
      Nạp từ Excel
    </PendingButton>
  );
}

/** Exact `ten` of the entry — the description shown behind the "?". */
export const IMPORT_EXCEL = "Nạp từ Excel";
