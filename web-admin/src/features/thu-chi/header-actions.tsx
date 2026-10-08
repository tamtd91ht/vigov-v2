"use client";

import { BudgetImportButton } from "./budget-import-button";

/**
 * PageHeader actions of Thu - Chi, spec §2 (`Nút phải: [⬆ Nạp từ Excel] [🗑 Gỡ]`).
 *
 * `⬆ Nạp từ Excel` is LIVE (ADR 0081 #6): pick a file, it is loaded at once, as in the prototype
 * (`budget-import-button.tsx`). The caller draws it only with `budget.update`; the route checks the key
 * anyway (rule 5).
 *
 * `🗑 Gỡ` is NOT here: it is built, as `Gỡ` in the selection bar, behind `budget.confirm` and only once
 * a sheet exists — an imported sheet included, since it is read back through the same routes.
 */
export function BudgetSheetHeaderActions({
  year,
  busy,
  onImported,
}: {
  year: number;
  busy: boolean;
  onImported: (firstKind: "thu" | "chi" | null) => void;
}) {
  return <BudgetImportButton year={year} disabled={busy} onImported={onImported} />;
}
