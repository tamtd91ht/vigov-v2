"use client";

import { Download, LoaderCircle } from "lucide-react";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import { saveFile } from "@/features/nhiem-vu/save-file";

import { EXPORT_FORMATS } from "./export-model";
import type { DashboardExport, ExportFormat } from "./export-model";

/**
 * What the header's export buttons need: why they are blocked (or `null` = ready), and how to take the
 * file content at the instant of the click. `view.tsx` `exportSourceOf` builds it.
 */
export type ExportSource = {
  readonly blockedReason: string | null;
  readonly build: (generatedAt: number) => DashboardExport;
};

const FORMAT_LABEL: Readonly<Record<ExportFormat, string>> = { pdf: "PDF", xlsx: "XLSX", pptx: "PPTX" };

/** Shown under the buttons when a file could not be made. States what failed and what to do. */
export const EXPORT_FAILED =
  "Chưa tạo được tệp. Số liệu trên màn hình không bị ảnh hưởng; hãy thử lại, hoặc tải lại trang nếu lỗi lặp lại.";

/** Shown while there is no source at all — gate closed or the session still being read. */
export const EXPORT_UNAVAILABLE = "Xuất tệp khi số liệu của trang đã hiện.";

/**
 * `[PDF][XLSX][PPTX]` — Tổng quan's export, BUILT IN THE BROWSER (user decision 06/10/2026).
 *
 * ENABLED ONLY when the source says ready: the account holds `report.read` (the page gate — no source
 * without it), holds `report.export` (the permission the authority grants for "Xuất báo cáo",
 * `lib/quyen.ts`), and every visible block has answered. Disabling is convenience: the files are made
 * from figures the server already chose to send this account, and no new data is fetched.
 *
 * ONE FILE AT A TIME: the pressed button shows it is working (`aria-busy`, spinner), the other two wait.
 * The click reads the clock — the files' "Xuất lúc …" is the act, never a render.
 *
 * NOT AUDITED: no server route records a staff "export" event (BACKEND DEPENDENCY, reported with
 * TASK-19). Rule 3 invariant 4 asks for one; it cannot be met from the browser alone.
 */
export function DashboardExportActions({ source }: { source?: ExportSource }) {
  const [busy, setBusy] = useState<ExportFormat | null>(null);
  const [failed, setFailed] = useState(false);

  const blocked = source === undefined ? EXPORT_UNAVAILABLE : source.blockedReason;

  async function run(format: ExportFormat) {
    if (source === undefined || source.blockedReason !== null || busy !== null) return;
    setBusy(format);
    setFailed(false);
    try {
      const doc = source.build(new Date().getTime());
      // The builders, and the libraries behind them, load on the first click — not with the page.
      const { EXPORT_BUILDERS } = await import("./export-files");
      const blob = await EXPORT_BUILDERS[format](doc);
      saveFile(blob, `${doc.fileStem}.${format}`);
    } catch {
      // No detail on screen: the cause is technical (a library, a font fetch), and the sentence says
      // what the reader can do. Nothing personal could be in it — the files hold aggregates only.
      setFailed(true);
    } finally {
      setBusy(null);
    }
  }

  return (
    <span className="inline-flex flex-col items-end gap-1">
      <span
        role="group"
        aria-label="Xuất báo cáo"
        title={blocked === null ? undefined : blocked}
        className="inline-flex flex-wrap gap-2"
      >
        {EXPORT_FORMATS.map((format) => {
          const working = busy === format;
          return (
            <Button
              key={format}
              type="button"
              variant="secondary"
              size="sm"
              disabled={blocked !== null || busy !== null}
              aria-busy={working ? true : undefined}
              data-export-format={format}
              onClick={() => void run(format)}
              icon={
                working ? (
                  <LoaderCircle aria-hidden="true" focusable="false" className="animate-spin motion-reduce:animate-none" />
                ) : (
                  <Download aria-hidden="true" focusable="false" />
                )
              }
            >
              {FORMAT_LABEL[format]}
            </Button>
          );
        })}
      </span>
      {blocked !== null && <span className="an-thi-giac">{blocked}</span>}
      {busy !== null && (
        <span role="status" className="an-thi-giac">
          {`Đang tạo tệp ${FORMAT_LABEL[busy]}…`}
        </span>
      )}
      {failed && (
        <span role="alert" className="max-w-xs text-right text-xs leading-snug text-danger-600">
          {EXPORT_FAILED}
        </span>
      )}
    </span>
  );
}
