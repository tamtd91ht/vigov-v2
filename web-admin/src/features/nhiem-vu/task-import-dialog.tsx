"use client";

import { CircleCheck, Download, Loader2, TriangleAlert, Upload } from "lucide-react";
import { useRef, useState, type DragEvent } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";
import { downloadTaskImportTemplate, submitTaskImport } from "@/lib/api/task-import";
import type { petitions_taskImportResultOut } from "@/lib/api/schema.gen";
import { cn } from "@/lib/cn";

import { saveFile } from "./save-file";
import {
  IMPORT_CHECKING,
  IMPORT_CLOSE_BUTTON,
  IMPORT_DESCRIPTION,
  IMPORT_FILE_LABEL,
  IMPORT_NOT_COMMITTED,
  IMPORT_ROWS_STILL_WRONG,
  IMPORT_TEMPLATE_BUTTON,
  IMPORT_TEMPLATE_SAVE_NAME,
  IMPORT_TITLE,
  canCommitImport,
  importButtonLabel,
  importFileProblem,
  importKeyFor,
  importedToast,
  previewHeading,
  replayedText,
  sortedImportErrors,
} from "./task-import";

/** Id of the import dialog's heading — its accessible name. */
export const IMPORT_TITLE_ID = "tieu-de-nhap-excel";
const FILE_INPUT_ID = "tep-nhap-nhiem-vu";

/**
 * `Nhập từ Excel` (§8) — spec 09: the shared `ExcelImportDialog` (Giải ngân spec 05) for tasks:
 * 42rem, `Tải mẫu giao việc` as a brand-coloured link, the dashed box with `Chọn tệp .xlsx`, the check
 * run AT ONCE on the chosen file, the green / red result box with `Dòng | Cột | Vấn đề`, then
 * `[Đóng] [Nhập N nhiệm vụ]` — enabled only for a clean check.
 *
 * TWO REQUESTS, AS EVERY IMPORT OF THIS APP: the check (`dry_run`) writes nothing, under a fresh key
 * each time; the real import keeps ONE key across retries of this file (a retry after a lost answer
 * replays, never books twice — issued numbers are never reissued, rule 7), dropped when another file
 * is chosen. ALL OR NOTHING is the server's rule; the dialog shows its answer.
 *
 * A success is a toast and closes the dialog (spec 09); a refusal of the import is the server's
 * sentence as a toast — "Tệp còn dòng sai, chưa ghi dòng nào." when it names rows. A refusal of the
 * CHECK (not xlsx, too big, 403) stays in the dialog, verbatim.
 *
 * Only a `task.create` holder gets the button (the page gates it); the routes check again (rule 5).
 */
export function TaskImportDialog({
  onClose,
  onImported,
}: {
  onClose: () => void;
  /** 201 (or a replayed 201): the register must be read again — the new rows are not spliced in. */
  onImported: () => void;
}) {
  const [fileName, setFileName] = useState<string | null>(null);
  const [file, setFile] = useState<File | null>(null);
  const [problem, setProblem] = useState<string | null>(null);
  const [report, setReport] = useState<petitions_taskImportResultOut | null>(null);
  const [importKey, setImportKey] = useState<string | null>(null);
  const [busy, setBusy] = useState<"" | "template" | "check" | "import">("");
  // Bumped on every choice: a check that answers after ANOTHER file was chosen is dropped, or the
  // first file's "valid" would enable `Nhập` for the second.
  const choice = useRef(0);

  async function choose(f: File | null) {
    if (f === null || busy === "import") return;
    const mine = ++choice.current;
    // A NEW FILE IS A NEW ATTEMPT: check, key and result start over.
    setFile(f);
    setFileName(f.name);
    setReport(null);
    setImportKey(null);
    const local = importFileProblem(f);
    setProblem(local);
    if (local !== null) return;
    setBusy("check");
    const r = await submitTaskImport(f, f.name, crypto.randomUUID(), true);
    if (choice.current !== mine) return;
    setBusy("");
    if (!r.ok) {
      setProblem(r.thongBao);
      return;
    }
    if (r.duLieu.kind === "checked" || r.duLieu.kind === "imported") setReport(r.duLieu.report);
  }

  async function commit() {
    if (file === null || !canCommitImport(report) || busy !== "" || report === null) return;
    const key = importKeyFor(importKey, () => crypto.randomUUID());
    setImportKey(key);
    setBusy("import");
    const r = await submitTaskImport(file, file.name, key, false);
    setBusy("");
    if (!r.ok) {
      toast.error(r.thongBao);
      return;
    }
    const o = r.duLieu;
    if (o.kind === "imported") {
      toast.success(importedToast(o.report));
      onImported();
      onClose();
      return;
    }
    if (o.kind === "replayed") {
      // The file is in (an earlier send of this attempt booked it): said, so nobody imports it again.
      toast.success(replayedText(o.firstCode));
      onImported();
      onClose();
      return;
    }
    // 200: nothing was written.
    setReport(o.report);
    toast.error(o.report.errors.length > 0 ? IMPORT_ROWS_STILL_WRONG : IMPORT_NOT_COMMITTED);
  }

  async function template() {
    setBusy("template");
    const r = await downloadTaskImportTemplate();
    setBusy("");
    if (!r.ok) {
      toast.error(r.thongBao);
      return;
    }
    saveFile(r.duLieu, IMPORT_TEMPLATE_SAVE_NAME);
  }

  return (
    // Esc while the import is in flight does nothing: the answer must have somewhere to show.
    <ModalDialog titleId={IMPORT_TITLE_ID} className="max-w-[42rem]" onDismiss={() => busy !== "import" && onClose()}>
      <ModalDialogHeader titleId={IMPORT_TITLE_ID} title={IMPORT_TITLE} description={IMPORT_DESCRIPTION} />
      <TaskImportView
        fileName={fileName}
        problem={problem}
        report={report}
        busy={busy}
        onTemplate={() => void template()}
        onChoose={(f) => void choose(f)}
        onImport={() => void commit()}
        onClose={onClose}
      />
    </ModalDialog>
  );
}

/** The body, without state — rendered by the tests with `renderToStaticMarkup`. */
export function TaskImportView({
  fileName,
  problem,
  report,
  busy,
  onTemplate,
  onChoose,
  onImport,
  onClose,
}: {
  fileName: string | null;
  /** The local pre-check's sentence, or the server's refusal of the check — verbatim. */
  problem: string | null;
  /** The last check's report (or the import's, when it named rows). */
  report: petitions_taskImportResultOut | null;
  busy: "" | "template" | "check" | "import";
  onTemplate: () => void;
  onChoose: (f: File | null) => void;
  onImport: () => void;
  onClose: () => void;
}) {
  function onDrop(e: DragEvent<HTMLDivElement>) {
    e.preventDefault();
    onChoose(e.dataTransfer.files.item(0));
  }
  const clean = report !== null && report.errors.length === 0;

  return (
    <div className="flex min-h-0 min-w-0 flex-col gap-4 overflow-y-auto">
      <div>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="text-brand hover:not-disabled:text-brand h-auto px-0 hover:not-disabled:bg-transparent"
          icon={
            busy === "template" ? (
              <Loader2 aria-hidden="true" focusable="false" className="size-4 animate-spin" />
            ) : (
              <Download aria-hidden="true" focusable="false" className="size-4" />
            )
          }
          disabled={busy !== ""}
          aria-busy={busy === "template" || undefined}
          onClick={onTemplate}
        >
          {IMPORT_TEMPLATE_BUTTON}
        </Button>
      </div>

      {/* The dashed box: Upload icon, the outline `Chọn tệp .xlsx`, the file name. Dropping a file on
          the box still works — an extra, never the only way (a11y). */}
      <div
        onDragOver={(e) => e.preventDefault()}
        onDrop={onDrop}
        className="border-line rounded-[10px] border border-dashed p-5 text-center"
      >
        <Upload aria-hidden="true" focusable="false" className="text-ink-muted mx-auto mb-2 block size-6" />
        <input
          id={FILE_INPUT_ID}
          name={FILE_INPUT_ID}
          type="file"
          accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
          className="hidden"
          aria-label={IMPORT_FILE_LABEL}
          disabled={busy === "import"}
          onChange={(e) => {
            const f = e.target.files?.item(0) ?? null;
            // Cleared so choosing the SAME file again (after fixing it) fires `change` again.
            e.target.value = "";
            onChoose(f);
          }}
        />
        <Button
          type="button"
          variant="outline"
          disabled={busy === "import"}
          onClick={() => document.getElementById(FILE_INPUT_ID)?.click()}
        >
          {IMPORT_FILE_LABEL}
        </Button>
        {fileName !== null && <p className="text-ink-muted m-0 mt-2 truncate text-[12px]">{fileName}</p>}
      </div>

      {busy === "check" && (
        <p className="text-ink-muted m-0 flex items-center gap-2 text-[12.5px]" role="status">
          <Loader2 aria-hidden="true" focusable="false" className="size-4 animate-spin" />
          {IMPORT_CHECKING}
        </p>
      )}

      {problem !== null && (
        <p className="thong-bao-loi m-0" role="alert">
          {problem}
        </p>
      )}

      {report !== null && (
        <div
          role={clean ? undefined : "alert"}
          data-preview-summary=""
          className={cn("rounded-[10px] border p-3", clean ? "border-leaf/25 bg-leaf/8" : "border-danger/25 bg-danger/8")}
        >
          <p className="m-0 flex items-center gap-2 text-[12.5px] font-semibold">
            {clean ? (
              <CircleCheck aria-hidden="true" focusable="false" className="text-leaf size-4 shrink-0" />
            ) : (
              <TriangleAlert aria-hidden="true" focusable="false" className="text-danger size-4 shrink-0" />
            )}
            <span className="text-navy">{previewHeading(report)}</span>
          </p>
          {report.errors.length > 0 && (
            <div className="mt-2 max-h-52 overflow-y-auto">
              <table className="w-full border-collapse text-[12px]">
                <thead>
                  <tr className="text-ink-muted text-left text-[10.5px] uppercase">
                    <th scope="col" className="py-1.5 pr-3 text-left font-semibold">
                      Dòng
                    </th>
                    <th scope="col" className="py-1.5 pr-3 text-left font-semibold">
                      Cột
                    </th>
                    <th scope="col" className="py-1.5 text-left font-semibold">
                      Vấn đề
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {sortedImportErrors(report.errors).map((e, i) => (
                    <tr key={`${e.row}-${e.column}-${i}`} className="border-line border-t">
                      <td className="text-danger py-1.5 pr-3 font-semibold">{e.row}</td>
                      <td className="text-ink-muted py-1.5 pr-3">{e.column === "" ? "—" : e.column}</td>
                      <td className="py-1.5">{e.message}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}

      <div className="flex flex-wrap justify-end gap-2">
        <Button type="button" variant="outline" onClick={onClose} disabled={busy === "import"}>
          {IMPORT_CLOSE_BUTTON}
        </Button>
        <Button
          type="button"
          variant="primary"
          disabled={!canCommitImport(report) || busy !== ""}
          aria-busy={busy === "import" || undefined}
          icon={busy === "import" ? <Loader2 aria-hidden="true" focusable="false" className="size-4 animate-spin" /> : undefined}
          onClick={onImport}
        >
          {importButtonLabel(report)}
        </Button>
      </div>
    </div>
  );
}
