import { Download, Maximize2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import type { ButtonSize } from "@/components/ui/button";
import { PendingButton, PendingFeature } from "@/components/ui/pending-feature";

import { pendingPart } from "./labels";

/**
 * The PageHeader buttons of spec §2 that are not built — `[PDF][XLSX][PPTX]` and `[⤢ Trình chiếu]`,
 * both Phase 2 (ADR 0068 §14). Drawn disabled with the "?"; nothing here calls a server or stores
 * anything, and there is no handler to pass.
 *
 * THE THREE EXPORT FORMATS SHARE ONE "?": they are one feature (export the period on screen) in
 * three file types, so three marks would say the same sentence three times in a row.
 *
 * NOT HERE, ON PURPOSE: `⟳ Tính lại ngay` — the owner decided not to build it (ADR 0053), and a
 * placeholder for it would announce a feature the authority refused.
 *
 * Rendered by `DashboardHeader` (`view.tsx`) on the right of the title, after the period buttons —
 * also for an account without `report.read`: a disabled control with no data behind it reveals
 * nothing an account may not read. Small buttons, as the prototype's header (`size="sm"`).
 */
export function DashboardHeaderActions() {
  return (
    <>
      <ExportPendingActions />
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

/**
 * The `[PDF][XLSX][PPTX]` group alone — also drawn by `/bao-cao` (ADR 0053 amendment 04/10/2026, B4:
 * export not built this round), which has no "Trình chiếu". One component, so the two pages cannot
 * describe the same unbuilt export in two ways.
 *
 * The two prototype screens word it differently, and both are kept: Tổng quan's header has small
 * "PDF" buttons, Báo cáo's own row has full-size "Xuất PDF" buttons (`labelPrefix="Xuất "`). Every
 * button carries the download icon, as in both.
 */
export function ExportPendingActions({
  size = "sm",
  labelPrefix = "",
}: {
  size?: ButtonSize;
  labelPrefix?: string;
}) {
  return (
    <PendingFeature info={pendingPart("Xuất báo cáo PDF, XLSX, PPTX")} phase2>
      <span role="group" aria-label="Xuất báo cáo" className="inline-flex flex-wrap gap-2">
        {(["PDF", "XLSX", "PPTX"] as const).map((format) => (
          <Button
            key={format}
            type="button"
            variant="secondary"
            size={size}
            disabled
            icon={<Download aria-hidden="true" focusable="false" />}
          >
            {`${labelPrefix}${format}`}
          </Button>
        ))}
      </span>
    </PendingFeature>
  );
}
