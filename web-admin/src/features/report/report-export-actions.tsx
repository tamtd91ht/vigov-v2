"use client";

import { Download, Loader2 } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import type { ExportSource } from "@/features/dashboard/export-actions";
import { EXPORT_FORMATS } from "@/features/dashboard/export-model";
import type { ExportFormat } from "@/features/dashboard/export-model";
import { saveFile } from "@/features/nhiem-vu/save-file";

/** The prototype's error toast (`ReportWorkspace.tsx`). */
export const REPORT_EXPORT_FAILED = "Không xuất được báo cáo.";

/** Why the row is disabled while there is no source — the commune configuration did not arrive. */
export const REPORT_EXPORT_UNAVAILABLE = "Xuất tệp khi số liệu của trang đã hiện.";

/**
 * `[Xuất PDF] [Xuất XLSX] [Xuất PPTX]` — the prototype's row (`ReportWorkspace.tsx`), BUILT 09/10/2026
 * (owner, ADR 0053 §Sửa đổi 09/10/2026 lần 2, D3) by reusing Tổng quan's browser-side builders
 * (`features/dashboard/export-files.ts`): the file is made from the figures on screen, never fetched.
 *
 * NOT TỔNG QUAN'S COMPONENT: the prototype draws this row differently on its two screens — here
 * outline buttons at the default 36px with "Xuất …" and a toast, there small header buttons with an
 * inline error — and Tổng quan's behaviour does not change with this page.
 *
 * DRAWN ONLY with `report.read` + `report.export` (`reportAccess`, the caller decides); disabled while
 * a visible block is still loading, with the reason on the group's `title` and for screen readers.
 * Hiding is convenience: the files hold only figures the server already sent this account.
 *
 * ONE FILE AT A TIME, as the prototype: while one is made, all three show the spinner and wait. The
 * click reads the clock — the file's "Xuất lúc …" is the act, never a render.
 *
 * NOT AUDITED (the shared "xuất chưa ghi vết" debt, `report-export.ts`): no server route records a
 * staff export event yet.
 */
export function ReportExportActions({ source }: { source?: ExportSource }) {
  const [busy, setBusy] = useState(false);
  const blocked = source === undefined ? REPORT_EXPORT_UNAVAILABLE : source.blockedReason;

  async function run(format: ExportFormat) {
    if (source === undefined || source.blockedReason !== null || busy) return;
    setBusy(true);
    try {
      const doc = source.build(new Date().getTime());
      // The builders, and the libraries behind them, load on the first click — not with the page.
      const { EXPORT_BUILDERS } = await import("@/features/dashboard/export-files");
      const blob = await EXPORT_BUILDERS[format](doc);
      const name = `${doc.fileStem}.${format}`;
      saveFile(blob, name);
      toast.success(`Đã tải ${name}.`);
    } catch {
      // No detail: the cause is technical (a library, a font fetch, no commune name). Nothing personal
      // could be in it — the files hold aggregates only.
      toast.error(REPORT_EXPORT_FAILED);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div
      role="group"
      aria-label="Xuất báo cáo"
      aria-busy={busy ? true : undefined}
      title={blocked ?? undefined}
      className="flex flex-wrap gap-2"
    >
      {EXPORT_FORMATS.map((format) => (
        <Button
          key={format}
          type="button"
          variant="outline"
          size="lg"
          disabled={blocked !== null || busy}
          data-export-format={format}
          onClick={() => void run(format)}
          icon={
            busy ? (
              <Loader2 aria-hidden="true" focusable="false" className="size-4 animate-spin motion-reduce:animate-none" />
            ) : (
              <Download aria-hidden="true" focusable="false" className="size-4" />
            )
          }
        >
          {`Xuất ${format.toUpperCase()}`}
        </Button>
      ))}
      {blocked !== null && <span className="an-thi-giac">{blocked}</span>}
      {busy && (
        <span role="status" className="an-thi-giac">
          Đang tạo tệp…
        </span>
      )}
    </div>
  );
}
