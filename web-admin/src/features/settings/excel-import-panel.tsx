"use client";

import { useReducer, useState } from "react";

import { commitImport, downloadImportTemplate, previewImport } from "@/lib/api/excel-import";
import type { ImportPreview, ImportResult } from "@/lib/api/excel-import";
import type { KetQua } from "@/lib/api/request";

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
}: {
  target: ImportTarget<R, C>;
  onImported: () => void;
  onClose: () => void;
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

  return (
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
    />
  );
}

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
}: {
  target: ImportTarget<R, C>;
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
  const titleId = `tieu-de-nhap-${target.id}`;
  const fileId = `o-tep-nhap-${target.id}`;
  return (
    <section className="form-danh-muc" aria-labelledby={titleId}>
      <h4 id={titleId}>{target.title}</h4>
      <p className="ghi-chu">{target.explanation}</p>

      <p>
        <button type="button" className="nut-phu" onClick={onDownloadTemplate} disabled={busy !== ""}>
          {TEMPLATE_BUTTON}
        </button>
      </p>
      {templateError !== "" && (
        <p className="thong-bao-loi" role="alert">
          {templateError}
        </p>
      )}

      {!done && (
        <div className="o-nhap">
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
        <p>
          <button type="button" className="nut-phu" onClick={onPreview} disabled={!fileChosen || busy !== ""}>
            {busy === "preview" ? PREVIEW_SENDING : PREVIEW_BUTTON}
          </button>
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
        <div className="thong-bao-loi" role="alert">
          <p>{result.message}</p>
          {result.errors.length > 0 && <ErrorsTable rows={errorRows(result.errors)} />}
        </div>
      )}

      {result !== null && result.ok && (
        <p role="status">{target.importedSentence(result.created === null ? null : result.created.length)}</p>
      )}

      {/* A target with its own result view owns closing once the import is in (`resultView`). */}
      {result !== null && result.ok && target.resultView !== undefined ? (
        target.resultView(result.created, onClose)
      ) : (
        <p>
          <button type="button" className="nut-phu" onClick={onClose} disabled={busy === "import"}>
            {CLOSE_BUTTON}
          </button>
        </p>
      )}
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
      <div className="thong-bao-loi" role="alert">
        <p>{target.errorsHeading}</p>
        <ErrorsTable rows={errorRows(preview.errors)} />
      </div>
    );
  }
  if (!canImport(preview)) return <p className="trang-thai-rong">{NOTHING_TO_CREATE}</p>;
  return (
    <div>
      <p>{target.previewLead(preview.rows.length)}</p>
      <div className="bang-cuon" role="region" aria-label={target.rowsLabel} tabIndex={0}>
        <table className="bang-danh-muc">
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
      </div>
      <p>
        <button type="button" className="nut-chinh" onClick={onImport} disabled={disabled}>
          {importing ? IMPORT_SENDING : target.confirmButton}
        </button>
      </p>
    </div>
  );
}

/** Row · column · message. A table, not a list: staff go back to the spreadsheet cell by cell. */
export function ErrorsTable({ rows }: { rows: readonly ErrorRow[] }) {
  return (
    <div className="bang-cuon" role="region" aria-label="Lỗi trong tệp" tabIndex={0}>
      <table className="bang-danh-muc">
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
    </div>
  );
}
