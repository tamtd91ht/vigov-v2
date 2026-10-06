"use client";

import { CircleCheck, Download, FileSearch, FileSpreadsheet, Upload, X } from "lucide-react";
import { useReducer, useState } from "react";

import { Button } from "@/components/ui/button";
import { CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { ModalDialog } from "@/components/ui/modal-dialog";
import { cn } from "@/lib/cn";
import { DATA_TABLE_CLASS, TableScroll } from "@/components/ui/data-table";
import { EmptyState } from "@/components/ui/empty-state";
import { BusyLabel } from "@/features/danh-ba/busy-label";

import { commitImport, downloadImportTemplate, previewImport } from "@/lib/api/excel-import";
import type { ImportPreview, ImportResult } from "@/lib/api/excel-import";
import type { KetQua } from "@/lib/api/goi";

import {
  CLOSE_BUTTON,
  FILE_LABEL,
  IMPORT_SENDING,
  NOTHING_TO_CREATE,
  PREVIEW_BUTTON,
  PREVIEW_SENDING,
  TEMPLATE_BUTTON,
  EMPTY_ATTEMPT,
  canImport,
  errorRows,
  keyForAttempt,
  nextAttempt,
} from "./excel-import-flow";
import type { AttemptEvent, ErrorRow, ImportAttempt, ImportTarget } from "./excel-import-flow";

export type ImportBusy = "" | "template" | "preview" | "import";

/**
 * The import panel shared by every "⬆ Nhập từ Excel" of the configuration screen: template → file →
 * preview (rows or row/column errors) → import with ONE `Idempotency-Key` per attempt.
 *
 * Opened only for an account holding the target's key (the caller decides); the server checks the same
 * key on all three routes. `onImported` makes the caller read its rows again: what was created, and
 * anything derived from it, exists only on the server.
 */
export function ExcelImportPanel<R, C = R>({
  target,
  onImported,
  onClose,
  asDialog = false,
}: {
  target: ImportTarget<R, C>;
  onImported: () => void;
  onClose: () => void;
  /**
   * Open as a centred dialog — the prototype's `ExcelImportDialog` behind every "Nhập từ Excel" of the
   * Cấu hình screen (ADR 0068 lần 5). Same view, same flow; only the frame changes.
   */
  asDialog?: boolean;
}) {
  // ONE reducer for everything the server answered (`nextAttempt`): the 201 of the staff import carries
  // temporary passwords, and "which event clears them" must be one tested function, not four setters.
  const [attempt, dispatch] = useReducer(
    (s: ImportAttempt<R, C>, e: AttemptEvent<R, C>) => nextAttempt(s, e),
    EMPTY_ATTEMPT as ImportAttempt<R, C>,
  );
  const [busy, setBusy] = useState<ImportBusy>("");
  const [templateError, setTemplateError] = useState("");
  const { file, preview, key: importKey, result } = attempt;

  function choose(f: File | null) {
    dispatch({ type: "chosen", file: f === null ? null : { blob: f, name: f.name } });
  }

  async function downloadTemplate() {
    setBusy("template");
    setTemplateError("");
    const r = await downloadImportTemplate(target.routes);
    setBusy("");
    if (!r.ok) {
      setTemplateError(r.thongBao);
      return;
    }
    const url = URL.createObjectURL(r.duLieu);
    const a = document.createElement("a");
    a.href = url;
    a.download = target.templateFileName;
    a.click();
    URL.revokeObjectURL(url);
  }

  async function check() {
    if (file === null) return;
    setBusy("preview");
    dispatch({ type: "previewStarted" });
    const p = await previewImport<R>(target.routes, target.rowsField, file.blob, file.name);
    setBusy("");
    dispatch({ type: "previewed", preview: p });
  }

  async function commit() {
    if (file === null || preview === null || !preview.ok || !canImport(preview.duLieu)) return;
    const key = keyForAttempt(importKey, () => crypto.randomUUID());
    dispatch({ type: "importStarted", key });
    setBusy("import");
    const r = await commitImport<C>(target.routes, file.blob, file.name, key);
    setBusy("");
    dispatch({ type: "imported", key, result: r });
    if (r.ok) onImported();
  }

  function close() {
    dispatch({ type: "closed" });
    onClose();
  }

  const view = (
    <ExcelImportView
      target={target}
      fileChosen={file !== null}
      preview={preview}
      result={result}
      busy={busy}
      templateError={templateError}
      onDownloadTemplate={() => void downloadTemplate()}
      onChooseFile={choose}
      onPreview={() => void check()}
      onImport={() => void commit()}
      onClose={close}
      framed={!asDialog}
    />
  );
  if (!asDialog) return view;
  return (
    // Esc while the import is in flight does nothing: the box must stay until the server has answered,
    // or the "Đã nhập…" sentence (and a staff import's temporary passwords) would have nowhere to show.
    // Nor after a success whose result owns its own closing (`resultView` — the staff import's one-time
    // passwords): a reflex Esc must not destroy values nobody can show again; its explicit button closes.
    <ModalDialog
      titleId={importTitleId(target.id)}
      onDismiss={() => {
        if (busy === "import") return;
        if (target.resultView !== undefined && result !== null && result.ok) return;
        close();
      }}
      size="lg"
      className="p-0"
    >
      <div className="min-h-0 min-w-0 overflow-y-auto">{view}</div>
    </ModalDialog>
  );
}

const importTitleId = (targetId: string) => `tieu-de-nhap-${targetId}`;

/** Pure rendering — exported so the errors table and the preview list have tests without a DOM. */
export function ExcelImportView<R, C = R>({
  target,
  fileChosen,
  preview,
  result,
  busy,
  templateError,
  onDownloadTemplate,
  onChooseFile,
  onPreview,
  onImport,
  onClose,
  framed = true,
}: {
  target: ImportTarget<R, C>;
  /** `false` inside a dialog: the dialog already draws the frame. */
  framed?: boolean;
  fileChosen: boolean;
  preview: KetQua<ImportPreview<R>> | null;
  result: ImportResult<C> | null;
  busy: ImportBusy;
  templateError: string;
  onDownloadTemplate: () => void;
  onChooseFile: (f: File | null) => void;
  onPreview: () => void;
  onImport: () => void;
  onClose: () => void;
}) {
  const done = result !== null && result.ok;
  const titleId = importTitleId(target.id);
  const fileId = `o-tep-nhap-${target.id}`;
  return (
    // A card of its own (spec v2 §7); NO HOOKS anywhere in this view — the staff-import test renders
    // it under a mocked React that only knows useState/useReducer inside its own render loop.
    <section
      className={cn(
        "form-danh-muc m-0 min-w-0 overflow-hidden bg-surface p-0",
        framed ? "rounded-card border border-line shadow-sm" : "border-0",
      )}
      aria-labelledby={titleId}
    >
      <CardHeader className="m-0 justify-between">
        <div className="min-w-0 flex-1 basis-64">
          <CardTitle as="h4" id={titleId} className="flex items-center gap-2">
            <FileSpreadsheet aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-[18px] shrink-0 text-brand-600" />
            {target.title}
          </CardTitle>
          <p className="ghi-chu m-0 mt-1 text-[13px] text-ink-500">{target.explanation}</p>
        </div>
        <Button
          type="button"
          variant="secondary"
          icon={<Download aria-hidden="true" focusable="false" strokeWidth={1.8} />}
          onClick={onDownloadTemplate}
          disabled={busy !== ""}
        >
          {TEMPLATE_BUTTON}
        </Button>
      </CardHeader>
      <CardContent className="flex min-w-0 flex-col gap-3 [&>*]:my-0">
      {templateError !== "" && (
        <p className="thong-bao-loi" role="alert">
          {templateError}
        </p>
      )}

      {!done && (
        <div className="o-nhap m-0">
          <label htmlFor={fileId}>{FILE_LABEL}</label>
          <input
            id={fileId}
            type="file"
            accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
            onChange={(e) => onChooseFile(e.target.files?.[0] ?? null)}
            disabled={busy !== ""}
          />
        </div>
      )}

      {!done && (
        <p className="m-0">
          <Button
            type="button"
            variant="secondary"
            icon={busy === "preview" ? undefined : <FileSearch aria-hidden="true" focusable="false" strokeWidth={1.8} />}
            onClick={onPreview}
            disabled={!fileChosen || busy !== ""}
            aria-busy={busy === "preview"}
          >
            <BusyLabel busy={busy === "preview"} label={PREVIEW_BUTTON} busyText={PREVIEW_SENDING} />
          </Button>
        </p>
      )}

      {/* A refusal of the FILE (not xlsx, too big): one sentence, the server's. */}
      {preview !== null && !preview.ok && (
        <p className="thong-bao-loi" role="alert">
          {preview.thongBao}
        </p>
      )}

      {preview !== null && preview.ok && !done && (
        <PreviewBody
          target={target}
          preview={preview.duLieu}
          importing={busy === "import"}
          disabled={busy !== ""}
          onImport={onImport}
        />
      )}

      {result !== null && !result.ok && (
        <div className="thong-bao-loi [&_p]:m-0 [&>p]:mb-2" role="alert">
          <p>{result.message}</p>
          {result.errors.length > 0 && <ErrorsTable rows={errorRows(result.errors)} />}
        </div>
      )}

      {result !== null && result.ok && (
        <p role="status" className="flex items-center gap-2 font-medium text-success-600">
          <CircleCheck aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-[18px] shrink-0" />
          {target.importedSentence(result.created === null ? null : result.created.length)}
        </p>
      )}

      {/* A target with its own result view owns closing once the import is in (`resultView`). */}
      {result !== null && result.ok && target.resultView !== undefined ? (
        target.resultView(result.created, onClose)
      ) : (
        <p className="m-0 flex justify-end">
          <Button
            type="button"
            variant="ghost"
            icon={<X aria-hidden="true" focusable="false" strokeWidth={1.8} />}
            onClick={onClose}
            disabled={busy === "import"}
          >
            {CLOSE_BUTTON}
          </Button>
        </p>
      )}
      </CardContent>
    </section>
  );
}

function PreviewBody<R, C>({
  target,
  preview,
  importing,
  disabled,
  onImport,
}: {
  target: ImportTarget<R, C>;
  preview: ImportPreview<R>;
  importing: boolean;
  disabled: boolean;
  onImport: () => void;
}) {
  if (!preview.valid) {
    return (
      <div className="thong-bao-loi [&_p]:m-0 [&>p]:mb-2" role="alert">
        <p>{target.errorsHeading}</p>
        <ErrorsTable rows={errorRows(preview.errors)} />
      </div>
    );
  }
  if (!canImport(preview)) return <EmptyState icon={FileSpreadsheet} title={NOTHING_TO_CREATE} />;
  return (
    <div className="flex min-w-0 flex-col gap-3 [&>*]:my-0">
      <p>{target.previewLead(preview.rows.length)}</p>
      <TableScroll sticky aria-label={target.rowsLabel} className="m-0">
        <table className={`bang-danh-muc ${DATA_TABLE_CLASS}`}>
          <thead>
            <tr>
              {target.columns.map((c) => (
                <th key={c.header} scope="col">
                  {c.header}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {preview.rows.map((r) => (
              <tr key={target.rowKey(r)}>
                {target.columns.map((c) => (
                  <td key={c.header} className={c.mono === true ? "ma-muc" : undefined}>
                    {c.cell(r)}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </TableScroll>
      <p className="m-0 flex justify-end">
        <Button
          type="button"
          variant="primary"
          icon={importing ? undefined : <Upload aria-hidden="true" focusable="false" strokeWidth={1.8} />}
          onClick={onImport}
          disabled={disabled}
          aria-busy={importing}
        >
          <BusyLabel busy={importing} label={target.confirmButton} busyText={IMPORT_SENDING} />
        </Button>
      </p>
    </div>
  );
}

/** Row · column · message. A table, not a list: staff go back to the spreadsheet cell by cell. */
export function ErrorsTable({ rows }: { rows: readonly ErrorRow[] }) {
  return (
    <TableScroll sticky aria-label="Lỗi trong tệp" className="m-0">
      <table className={`bang-danh-muc ${DATA_TABLE_CLASS}`}>
        <thead>
          <tr>
            <th scope="col">Dòng</th>
            <th scope="col">Cột</th>
            <th scope="col">Lỗi</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r, i) => (
            // Two errors may share a row and a column — the index keeps keys distinct.
            <tr key={`${r.row}-${r.column}-${i}`}>
              <td>{r.row}</td>
              <td>{r.column}</td>
              <td>{r.message}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </TableScroll>
  );
}
