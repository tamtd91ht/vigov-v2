"use client";

import { CircleCheck, Download, Loader2, TriangleAlert, Upload } from "lucide-react";
import { useReducer, useState, type DragEvent } from "react";

import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";
import { saveFile } from "@/features/nhiem-vu/save-file";
import { commitImport, downloadImportTemplate, previewImport } from "@/lib/api/excel-import";
import type { ImportPreview, ImportResult } from "@/lib/api/excel-import";
import { LOI_KHONG_RO } from "@/lib/api/goi";
import type { KetQua } from "@/lib/api/goi"; // vi-name-ok: existing type of goi.ts, imported not declared (rule 12 inv 3)
import { cn } from "@/lib/cn";

import {
  CHOOSE_XLSX_BUTTON,
  CLOSE_BUTTON,
  EMPTY_ATTEMPT,
  IMPORT_DIALOG_DESCRIPTION,
  IMPORT_SENDING,
  IMPORT_SUBMIT,
  NOTHING_TO_CREATE,
  PREVIEW_SENDING,
  TEMPLATE_BUTTON,
  canImport,
  errorRows,
  keyForAttempt,
  nextAttempt,
} from "./excel-import-flow";
import type { AttemptEvent, ErrorRow, ImportAttempt, ImportTarget } from "./excel-import-flow";

/** `check` = the preview that runs first when `Nhập` is pressed; `import` = the write. */
export type ImportDialogBusy = "" | "template" | "check" | "import";

const titleIdOf = (targetId: string) => `tieu-de-nhap-${targetId}`;
const fileInputIdOf = (targetId: string) => `o-tep-nhap-${targetId}`;

/**
 * "Nhập … từ Excel" — the prototype's shared `ExcelImportDialog` (spec §1, user 09/10/2026), behind every
 * import of Sơ đồ tổ chức, Thôn / Tổ dân phố, Danh mục and Người dùng: 672px, the prototype's one-line
 * description, the `Tải tệp mẫu` link, the dashed box with `Chọn tệp .xlsx`, then `Đóng` · `Nhập`.
 * Generalised from the staff dialog over `ImportTarget<R, C>`; `StaffImportDialog` is now a thin wrapper.
 *
 * `Nhập` RUNS THE TWO REQUESTS EVERY IMPORT OF THIS APP RUNS (ADR 0059), in one press:
 *
 *   1. preview — writes nothing. Row errors → the list by row and column, nothing written, `Nhập` stays
 *      offered for the fixed file. A refusal of the FILE (not xlsx, too big, 403) → the server's sentence.
 *   2. clean → the import, under ONE `Idempotency-Key` per attempt (`nextAttempt` / `keyForAttempt`).
 *      If that send got NO ANSWER (`LOI_KHONG_RO`), the key is kept and the next `Nhập` re-sends the SAME
 *      attempt WITHOUT a new preview: a new preview drops the key (`previewed`), and a write that did
 *      reach the server would then run twice. A refusal the server DID answer (400 rows, 409 "changed
 *      since the check") wrote nothing, so the next `Nhập` checks the file again from the start. A new
 *      file is a new attempt.
 *
 * A target with a `resultView` (the staff import's one-time passwords) owns closing after a success: Esc
 * and the ✕ do nothing then, and nothing while the write is in flight. Every other target ends on its
 * `importedSentence` and a lone `Đóng`.
 *
 * The long per-target guidance (`target.explanation`) is NOT drawn here, as decided: the prototype's one
 * description stands for every target.
 *
 * Opened only for an account holding the target's write key (the caller gates); the three routes check
 * the key again (rule 5).
 */
export function ExcelImportDialog<R, C = R>({
  target,
  onClose,
  onImported,
}: {
  target: ImportTarget<R, C>;
  onClose: () => void;
  /** A 201 (or a replayed 201): the caller reads its rows again — nothing is spliced in. */
  onImported: () => void;
}) {
  const [attempt, dispatch] = useReducer(
    (s: ImportAttempt<R, C>, e: AttemptEvent<R, C>) => nextAttempt(s, e),
    EMPTY_ATTEMPT as ImportAttempt<R, C>,
  );
  const [busy, setBusy] = useState<ImportDialogBusy>("");
  const [templateError, setTemplateError] = useState("");
  const routes = target.routes;

  async function template() {
    setBusy("template");
    setTemplateError("");
    const r = await downloadImportTemplate(routes);
    setBusy("");
    if (!r.ok) {
      setTemplateError(r.thongBao);
      return;
    }
    saveFile(r.duLieu, target.templateFileName);
  }

  async function write(file: NonNullable<ImportAttempt<R, C>["file"]>, current: string | null) {
    const key = keyForAttempt(current, () => crypto.randomUUID());
    dispatch({ type: "importStarted", key });
    setBusy("import");
    const r = await commitImport<C>(routes, file.blob, file.name, key);
    setBusy("");
    dispatch({ type: "imported", key, result: r });
    if (r.ok) onImported();
  }

  async function submit() {
    const { file, preview, key, result } = attempt;
    if (file === null || busy !== "") return;
    // A write of THIS file that got no answer: retry the same attempt with its key (see the header).
    const unanswered = result !== null && !result.ok && result.message === LOI_KHONG_RO;
    if (unanswered && key !== null && preview !== null && preview.ok && canImport(preview.duLieu)) {
      await write(file, key);
      return;
    }
    setBusy("check");
    dispatch({ type: "previewStarted" });
    const p = await previewImport<R>(routes, target.rowsField, file.blob, file.name);
    setBusy("");
    dispatch({ type: "previewed", preview: p });
    if (!p.ok || !canImport(p.duLieu)) return;
    await write(file, null);
  }

  function close() {
    dispatch({ type: "closed" });
    onClose();
  }

  const done = attempt.result !== null && attempt.result.ok;
  const resultOwnsClosing = done && target.resultView !== undefined;
  const titleId = titleIdOf(target.id);

  return (
    <ModalDialog
      titleId={titleId}
      className="max-w-[672px]"
      closeDisabled={busy === "import" || resultOwnsClosing}
      onDismiss={() => {
        if (busy === "import" || resultOwnsClosing) return;
        close();
      }}
    >
      <ModalDialogHeader titleId={titleId} title={target.title} description={IMPORT_DIALOG_DESCRIPTION} />
      <ExcelImportDialogView
        target={target}
        fileName={attempt.file?.name ?? null}
        preview={attempt.preview}
        result={attempt.result}
        busy={busy}
        templateError={templateError}
        onTemplate={() => void template()}
        onChoose={(f) => dispatch({ type: "chosen", file: f === null ? null : { blob: f, name: f.name } })}
        onImport={() => void submit()}
        onClose={close}
      />
    </ModalDialog>
  );
}

/** The body, without state or hooks — rendered by the tests with `renderToStaticMarkup`. */
export function ExcelImportDialogView<R, C = R>({
  target,
  fileName,
  preview,
  result,
  busy,
  templateError,
  onTemplate,
  onChoose,
  onImport,
  onClose,
}: {
  target: ImportTarget<R, C>;
  fileName: string | null;
  preview: KetQua<ImportPreview<R>> | null;
  result: ImportResult<C> | null;
  busy: ImportDialogBusy;
  templateError: string;
  onTemplate: () => void;
  onChoose: (f: File | null) => void;
  onImport: () => void;
  onClose: () => void;
}) {
  const done = result !== null && result.ok;
  const fileInputId = fileInputIdOf(target.id);
  function onDrop(e: DragEvent<HTMLDivElement>) {
    e.preventDefault();
    if (busy === "") onChoose(e.dataTransfer.files.item(0));
  }

  return (
    <div className="flex min-h-0 min-w-0 flex-col gap-4 overflow-y-auto">
      {!done && (
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
            {TEMPLATE_BUTTON}
          </Button>
        </div>
      )}

      {templateError !== "" && (
        <p className="thong-bao-loi m-0" role="alert">
          {templateError}
        </p>
      )}

      {/* The dashed box. Dropping a file on it works too — an extra, never the only way (a11y). */}
      {!done && (
        <div
          onDragOver={(e) => e.preventDefault()}
          onDrop={onDrop}
          className="border-line rounded-[10px] border border-dashed p-5 text-center"
        >
          <Upload aria-hidden="true" focusable="false" className="text-ink-muted mx-auto mb-2 block size-6" />
          <input
            id={fileInputId}
            name={fileInputId}
            type="file"
            accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
            className="hidden"
            aria-label={CHOOSE_XLSX_BUTTON}
            disabled={busy !== ""}
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
            disabled={busy !== ""}
            onClick={() => document.getElementById(fileInputId)?.click()}
          >
            {CHOOSE_XLSX_BUTTON}
          </Button>
          {fileName !== null && <p className="text-ink-muted m-0 mt-2 truncate text-[12px]">{fileName}</p>}
        </div>
      )}

      {(busy === "check" || busy === "import") && (
        <p className="text-ink-muted m-0 flex items-center gap-2 text-[12.5px]" role="status">
          <Loader2 aria-hidden="true" focusable="false" className="size-4 animate-spin" />
          {busy === "check" ? PREVIEW_SENDING : IMPORT_SENDING}
        </p>
      )}

      {/* A refusal of the FILE (not xlsx, too big, 403): one sentence, the server's. */}
      {preview !== null && !preview.ok && (
        <p className="thong-bao-loi m-0" role="alert">
          {preview.thongBao}
        </p>
      )}

      {preview !== null && preview.ok && !preview.duLieu.valid && (
        <ImportErrorBox heading={target.errorsHeading} rows={errorRows(preview.duLieu.errors)} />
      )}

      {preview !== null && preview.ok && preview.duLieu.valid && preview.duLieu.rows.length === 0 && (
        <EmptyState icon={Upload} title={NOTHING_TO_CREATE} />
      )}

      {result !== null && !result.ok && <ImportErrorBox heading={result.message} rows={errorRows(result.errors)} />}

      {done && (
        <p role="status" className="m-0 flex items-center gap-2 font-medium text-success-600">
          <CircleCheck aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-[18px] shrink-0" />
          {target.importedSentence(result.created === null ? null : result.created.length)}
        </p>
      )}

      {/* A target with its own result view owns closing from here on (the staff import's passwords). */}
      {done && target.resultView !== undefined && target.resultView(result.created, onClose)}

      {done && target.resultView === undefined && (
        <div className="flex justify-end">
          <Button type="button" variant="outline" onClick={onClose}>
            {CLOSE_BUTTON}
          </Button>
        </div>
      )}

      {!done && (
        <div className="flex flex-wrap justify-end gap-2">
          <Button type="button" variant="outline" onClick={onClose} disabled={busy === "import"}>
            {CLOSE_BUTTON}
          </Button>
          <Button
            type="button"
            variant="primary"
            disabled={fileName === null || busy !== ""}
            aria-busy={busy === "check" || busy === "import" || undefined}
            icon={
              busy === "check" || busy === "import" ? (
                <Loader2 aria-hidden="true" focusable="false" className="size-4 animate-spin" />
              ) : undefined
            }
            onClick={onImport}
          >
            {IMPORT_SUBMIT}
          </Button>
        </div>
      )}
    </div>
  );
}

/** The red box: one heading, then `Dòng | Cột | Vấn đề` (the prototype's look). Row 0 reads "Cả tệp". */
function ImportErrorBox({ heading, rows }: { heading: string; rows: readonly ErrorRow[] }) {
  return (
    <div role="alert" className={cn("rounded-[10px] border border-solid p-3", "border-danger/25 bg-danger/8")}>
      <p className="m-0 flex items-center gap-2 text-[12.5px] font-semibold">
        <TriangleAlert aria-hidden="true" focusable="false" className="text-danger size-4 shrink-0" />
        <span className="text-navy">{heading}</span>
      </p>
      {rows.length > 0 && (
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
              {rows.map((r, i) => (
                // Two errors may share a row and a column — the index keeps keys distinct.
                <tr key={`${r.row}-${r.column}-${i}`} className="border-line border-t">
                  <td className="text-danger py-1.5 pr-3 font-semibold">{r.row}</td>
                  <td className="text-ink-muted py-1.5 pr-3">{r.column}</td>
                  <td className="py-1.5">{r.message}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
