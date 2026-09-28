"use client";

import { useState } from "react";

import type { KetQua } from "@/lib/api/goi";
import { downloadOrgUnitTemplate, importOrgUnits, previewOrgUnitImport } from "@/lib/api/org-unit-import";
import type { OrgUnitImportResult } from "@/lib/api/org-unit-import";
import type { identity_orgUnitImportPreviewOut } from "@/lib/api/schema.gen";

import {
  CLOSE_BUTTON,
  CONFIRM_IMPORT_BUTTON,
  ERRORS_HEADING,
  FILE_LABEL,
  IMPORT_EXPLANATION,
  IMPORT_SENDING,
  NOTHING_TO_CREATE,
  PREVIEW_BUTTON,
  PREVIEW_SENDING,
  TEMPLATE_BUTTON,
  TEMPLATE_FILE_NAME,
  canImport,
  errorRows,
  importedSentence,
  keyForAttempt,
  parentText,
  previewLead,
} from "./org-unit-import-flow";
import type { ErrorRow } from "./org-unit-import-flow";

type ChosenFile = { readonly blob: Blob; readonly name: string };

/**
 * The import panel, opened by "⬆ Nhập từ Excel" (only drawn for `admin.org`; the server checks the
 * same key on all three routes). `onImported` makes the tab read the tree again: the created units'
 * staff counts exist only on the server.
 */
export function OrgUnitImportPanel({ onImported, onClose }: { onImported: () => void; onClose: () => void }) {
  const [file, setFile] = useState<ChosenFile | null>(null);
  const [preview, setPreview] = useState<KetQua<identity_orgUnitImportPreviewOut> | null>(null);
  const [importKey, setImportKey] = useState<string | null>(null);
  const [result, setResult] = useState<OrgUnitImportResult | null>(null);
  const [busy, setBusy] = useState<"" | "template" | "preview" | "import">("");
  const [templateError, setTemplateError] = useState("");

  function choose(f: File | null) {
    // A NEW FILE IS A NEW ATTEMPT: its preview, its key and its result all start over.
    setFile(f === null ? null : { blob: f, name: f.name });
    setPreview(null);
    setImportKey(null);
    setResult(null);
  }

  async function downloadTemplate() {
    setBusy("template");
    setTemplateError("");
    const r = await downloadOrgUnitTemplate();
    setBusy("");
    if (!r.ok) {
      setTemplateError(r.thongBao);
      return;
    }
    const url = URL.createObjectURL(r.duLieu);
    const a = document.createElement("a");
    a.href = url;
    a.download = TEMPLATE_FILE_NAME;
    a.click();
    URL.revokeObjectURL(url);
  }

  async function check() {
    if (file === null) return;
    setBusy("preview");
    setResult(null);
    const p = await previewOrgUnitImport(file.blob, file.name);
    setBusy("");
    setPreview(p);
    setImportKey(null);
  }

  async function commit() {
    if (file === null || preview === null || !preview.ok || !canImport(preview.duLieu)) return;
    const key = keyForAttempt(importKey, () => crypto.randomUUID());
    setImportKey(key);
    setBusy("import");
    const r = await importOrgUnits(file.blob, file.name, key);
    setBusy("");
    setResult(r);
    if (r.ok) {
      setImportKey(null);
      onImported();
    }
  }

  return (
    <OrgUnitImportView
      fileChosen={file !== null}
      preview={preview}
      result={result}
      busy={busy}
      templateError={templateError}
      onDownloadTemplate={() => void downloadTemplate()}
      onChooseFile={choose}
      onPreview={() => void check()}
      onImport={() => void commit()}
      onClose={onClose}
    />
  );
}

/**
 * Pure rendering — exported so the errors table and the preview list have tests without a DOM.
 */
export function OrgUnitImportView({
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
  fileChosen: boolean;
  preview: KetQua<identity_orgUnitImportPreviewOut> | null;
  result: OrgUnitImportResult | null;
  busy: "" | "template" | "preview" | "import";
  templateError: string;
  onDownloadTemplate: () => void;
  onChooseFile: (f: File | null) => void;
  onPreview: () => void;
  onImport: () => void;
  onClose: () => void;
}) {
  const done = result !== null && result.ok;
  return (
    <section className="form-danh-muc" aria-labelledby="tieu-de-nhap-so-do">
      <h4 id="tieu-de-nhap-so-do">Nhập sơ đồ tổ chức từ Excel</h4>
      <p className="ghi-chu">{IMPORT_EXPLANATION}</p>

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
          <label htmlFor="o-tep-nhap-so-do">{FILE_LABEL}</label>
          <input
            id="o-tep-nhap-so-do"
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
        <p role="status">{importedSentence(result.created === null ? null : result.created.length)}</p>
      )}

      <p>
        <button type="button" className="nut-phu" onClick={onClose} disabled={busy === "import"}>
          {CLOSE_BUTTON}
        </button>
      </p>
    </section>
  );
}

function PreviewBody({
  preview,
  importing,
  disabled,
  onImport,
}: {
  preview: identity_orgUnitImportPreviewOut;
  importing: boolean;
  disabled: boolean;
  onImport: () => void;
}) {
  if (!preview.valid) {
    return (
      <div className="thong-bao-loi" role="alert">
        <p>{ERRORS_HEADING}</p>
        <ErrorsTable rows={errorRows(preview.errors)} />
      </div>
    );
  }
  if (!canImport(preview)) return <p className="trang-thai-rong">{NOTHING_TO_CREATE}</p>;
  return (
    <div>
      <p>{previewLead(preview.units.length)}</p>
      <div className="bang-cuon" role="region" aria-label="Các bộ phận sẽ tạo" tabIndex={0}>
        <table className="bang-danh-muc">
          <thead>
            <tr>
              <th scope="col">Dòng</th>
              <th scope="col">Tên bộ phận</th>
              <th scope="col">Mã</th>
              <th scope="col">Bộ phận cha</th>
              <th scope="col">Thứ tự</th>
            </tr>
          </thead>
          <tbody>
            {preview.units.map((u) => (
              <tr key={u.row}>
                <td>{u.row}</td>
                <td>{u.name}</td>
                <td className="ma-muc">{u.code}</td>
                <td>{parentText(u)}</td>
                <td>{u.order}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <p>
        <button type="button" className="nut-chinh" onClick={onImport} disabled={disabled}>
          {importing ? IMPORT_SENDING : CONFIRM_IMPORT_BUTTON}
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
