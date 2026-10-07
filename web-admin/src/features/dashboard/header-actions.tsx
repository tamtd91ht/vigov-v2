import { DashboardExportActions } from "./export-actions";
import type { ExportSource } from "./export-actions";
import { PresentationToggle } from "./presentation";
import type { PresentationControl } from "./presentation";

export type { ExportSource };

/**
 * The PageHeader buttons of spec §2 after the period picker: `[PDF][XLSX][PPTX]` and `[⤢ Trình chiếu]`.
 *
 * THE EXPORT IS BUILT (user decision 06/10/2026, overriding ADR 0053 B4 / ADR 0068 §14 "phase 2" for
 * Tổng quan only): files made in the browser from the figures on screen (`export-actions.tsx`).
 * `exportSource` absent = the gate is closed, the session still being read, or no commune
 * configuration was passed — the three buttons are drawn disabled, which reveals nothing.
 *
 * "TRÌNH CHIẾU" IS BUILT TOO (user decision 07/10/2026, same override): a live toggle
 * (`presentation.tsx`). `presentation` absent = the gate is closed — the toggle is drawn disabled,
 * as the export buttons are, and there is no page behind it to present.
 *
 * NOT HERE, ON PURPOSE: `⟳ Tính lại ngay` — the owner decided not to build it (ADR 0053), and a
 * placeholder for it would announce a feature the authority refused.
 *
 * `/bao-cao`'s export is NOT this component: it is still unbuilt there and has its own pending row
 * (`features/report/export-pending-actions.tsx`); `/bao-cao` has no Trình chiếu (spec 13 §1).
 */
export function DashboardHeaderActions({
  exportSource,
  presentation,
}: {
  exportSource?: ExportSource;
  presentation?: PresentationControl;
}) {
  return (
    <>
      <DashboardExportActions source={exportSource} />
      <PresentationToggle control={presentation} />
    </>
  );
}
