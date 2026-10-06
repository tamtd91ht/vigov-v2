import { Maximize2 } from "lucide-react";

import { PendingButton } from "@/components/ui/pending-feature";

import { DashboardExportActions } from "./export-actions";
import type { ExportSource } from "./export-actions";
import { pendingPart } from "./labels";

export type { ExportSource };

/**
 * The PageHeader buttons of spec §2 after the period picker: `[PDF][XLSX][PPTX]` and `[⤢ Trình chiếu]`.
 *
 * THE EXPORT IS BUILT (user decision 06/10/2026, overriding ADR 0053 B4 / ADR 0068 §14 "phase 2" for
 * Tổng quan only): files made in the browser from the figures on screen (`export-actions.tsx`).
 * `exportSource` absent = the gate is closed, the session still being read, or no commune
 * configuration was passed — the three buttons are drawn disabled, which reveals nothing.
 *
 * "Trình chiếu" stays Phase 2: disabled with its "?" (`labels.ts`).
 *
 * NOT HERE, ON PURPOSE: `⟳ Tính lại ngay` — the owner decided not to build it (ADR 0053), and a
 * placeholder for it would announce a feature the authority refused.
 *
 * `/bao-cao`'s export is NOT this component: it is still unbuilt there and has its own pending row
 * (`features/report/export-pending-actions.tsx`).
 */
export function DashboardHeaderActions({ exportSource }: { exportSource?: ExportSource }) {
  return (
    <>
      <DashboardExportActions source={exportSource} />
      <PendingButton
        info={pendingPart("Chế độ trình chiếu phòng họp")}
        phase2
        size="sm"
        icon={<Maximize2 aria-hidden="true" focusable="false" />}
      >
        Trình chiếu
      </PendingButton>
    </>
  );
}
