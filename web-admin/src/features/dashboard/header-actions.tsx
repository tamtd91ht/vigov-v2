import { FileDown, Maximize2 } from "lucide-react";

import { Button } from "@/components/ui/button";
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
 * Rendered by `app/tong-quan/page.tsx` in the header, outside the `report.read` gate like the title:
 * a disabled control with no data behind it reveals nothing an account may not read.
 */
export function DashboardHeaderActions() {
  return (
    <>
      <PendingFeature info={pendingPart("Xuất báo cáo PDF, XLSX, PPTX")} phase2>
        <span role="group" aria-label="Xuất báo cáo" className="inline-flex gap-1">
          {(["PDF", "XLSX", "PPTX"] as const).map((format) => (
            <Button
              key={format}
              type="button"
              variant="secondary"
              size="md"
              disabled
              icon={format === "PDF" ? <FileDown aria-hidden="true" focusable="false" /> : undefined}
            >
              {format}
            </Button>
          ))}
        </span>
      </PendingFeature>
      <PendingButton
        info={pendingPart("Chế độ trình chiếu phòng họp")}
        phase2
        icon={<Maximize2 aria-hidden="true" focusable="false" />}
      >
        Trình chiếu
      </PendingButton>
    </>
  );
}
