import { Download } from "lucide-react";

import { Button } from "@/components/ui/button";
import type { ButtonSize } from "@/components/ui/button";
import { PendingFeature } from "@/components/ui/pending-feature";

import { reportPendingPart } from "./labels";

/**
 * `/bao-cao`'s `[Xuất PDF][Xuất XLSX][Xuất PPTX]` row — NOT BUILT (ADR 0053 amendment 04/10/2026, B4),
 * so disabled with one "?" (ADR 0068 §14). The three formats share one "?": one feature in three file
 * types.
 *
 * WHY THIS IS NOT TỔNG QUAN'S COMPONENT ANY MORE: Tổng quan's export was built on 06/10/2026 (user
 * decision) and Báo cáo's was not. One shared component would have had to switch on the page — a
 * second page silently turning live the day someone flips it. Two components, two truths.
 */
export function ExportPendingActions({
  size = "md",
  labelPrefix = "Xuất ",
}: {
  size?: ButtonSize;
  labelPrefix?: string;
}) {
  return (
    <PendingFeature info={reportPendingPart("Xuất báo cáo PDF, XLSX, PPTX")} phase2>
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
