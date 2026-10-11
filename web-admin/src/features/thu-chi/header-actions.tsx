"use client";

import { BudgetImportButton } from "./budget-import-button";
import type { BudgetTemplate } from "./budget-template";

/**
 * PageHeader actions of Thu - Chi, spec §2 (`Nút phải: [⬆ Nạp từ Excel] [🗑 Gỡ]`).
 *
 * `⬆ Nạp từ Excel` is LIVE (ADR 0081 #6): a popup with `Tải file mẫu` and `Chọn tệp`; a picked file is
 * loaded at once, as in the prototype (`budget-import-button.tsx`). The caller draws it only with
 * `budget.update`; the route checks the key anyway (rule 5).
 *
 * `🗑 Gỡ` is NOT here: it is built, as `Gỡ` in the selection bar, behind `budget.confirm` and only once
 * a sheet exists — an imported sheet included, since it is read back through the same routes.
 */
export function BudgetSheetHeaderActions({
  year,
  template,
  busy,
  onImported,
}: {
  year: number;
  template: BudgetTemplate;
  busy: boolean;
  onImported: (firstKind: "thu" | "chi" | null) => void;
}) {
  return <BudgetImportButton year={year} template={template} disabled={busy} onImported={onImported} />;
}
