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
import type { StaffImportCreatedRow, StaffImportPlannedRow } from "@/lib/api/staff-import";
import { cn } from "@/lib/cn";

import {
  CLOSE_BUTTON,
  EMPTY_ATTEMPT,
  IMPORT_SENDING,
  NOTHING_TO_CREATE,
  PREVIEW_SENDING,
  TEMPLATE_BUTTON,
  canImport,
  errorRows,
  keyForAttempt,
  nextAttempt,
} from "./excel-import-flow";
import type { AttemptEvent, ErrorRow, ImportAttempt } from "./excel-import-flow";
import { STAFF_IMPORT_TARGET } from "./excel-import-targets";
import {
  STAFF_IMPORT_CHOOSE_FILE,
  STAFF_IMPORT_DESCRIPTION,
  STAFF_IMPORT_ERRORS_HEADING,
  STAFF_IMPORT_SUBMIT,
  STAFF_IMPORT_TITLE,
} from "./nhan-can-bo";
import { StaffImportResult } from "./staff-import-result";

type Attempt = ImportAttempt<StaffImportPlannedRow, StaffImportCreatedRow>;
type StaffImportEvent = AttemptEvent<StaffImportPlannedRow, StaffImportCreatedRow>;

/** `check` = the preview that runs first when `Nhập` is pressed; `import` = the write. */
export type StaffImportBusy = "" | "template" | "check" | "import";

export const STAFF_IMPORT_TITLE_ID = "tieu-de-nhap-nguoi-dung";
const FILE_INPUT_ID = "tep-nhap-nguoi-dung";

/**
 * `Nhập người dùng từ Excel` — the prototype's `ExcelImportDialog` for staff (user 09/10/2026): 672px,
 * the `Tải tệp mẫu` link, the dashed box with `Chọn tệp .xlsx`, then `Đóng` · `Nhập`.
 *
 * A STAFF DIALOG OF ITS OWN, not `TaskImportView` made generic: that view is typed on the task import's
 * report (`petitions_taskImportResultOut`) and checks on CHOOSE, while this flow checks on `Nhập` and
 * ends in the one-time password table. Its look is copied, its behaviour is not; the shared
 * `ExcelImportPanel` (org units, residential units, asset types) is untouched.
 *
 * `Nhập` RUNS THE TWO REQUESTS EVERY IMPORT OF THIS APP RUNS (ADR 0059), in one press:
 *
 *   1. preview — writes nothing. Row errors → the list by row, nothing written, `Nhập` stays offered for
 *      the fixed file. A refusal of the FILE (not xlsx, too big, 403 `role_permission_required` for a
 *      Vai trò column without `admin.role`) → the server's sentence.
 *   2. clean → the import, under ONE `Idempotency-Key` per attempt (`nextAttempt` / `keyForAttempt`).
 *      If that send got NO ANSWER (`LOI_KHONG_RO`), the key is kept and the next `Nhập` re-sends the SAME
 *      attempt WITHOUT a new preview: a new preview drops the key (`previewed`), and a write that did
 *      reach the server would then run twice. A refusal the server DID answer (400 rows, 409
 *      `staff_changed`) wrote nothing, so the next `Nhập` checks the file again from the start. A new
 *      file is a new attempt.
 *
 * The 201 carries the temporary passwords ONCE: `StaffImportResult` draws them and owns closing; Esc and
 * the ✕ do nothing then, and nothing while the write is in flight.
 *
 * The long guidance the old panel printed (template choices, passwords, Vai trò needing Phân quyền) is no
 * longer in the dialog, as decided. Moving it into the template's guidance sheet is a change of the
 * server's template, not made here.
 *
 * Opened only by `admin.user` holders (the screen's gate); the three routes check the key again (rule 5).
 */
export function StaffImportDialog({
  onClose,
  onImported,
}: {
  onClose: () => void;
  /** A 201 (or a replayed 201): the list must be read again — nothing is spliced in. */
  onImported: () => void;
}) {
  const [attempt, dispatch] = useReducer((s: Attempt, e: StaffImportEvent) => nextAttempt(s, e), EMPTY_ATTEMPT as Attempt);
  const [busy, setBusy] = useState<StaffImportBusy>("");
  const [templateError, setTemplateError] = useState("");
  const routes = STAFF_IMPORT_TARGET.routes;

  async function template() {
    setBusy("template");
    setTemplateError("");
    const r = await downloadImportTemplate(routes);
    setBusy("");
    if (!r.ok) {
      setTemplateError(r.thongBao);
      return;
    }
    saveFile(r.duLieu, STAFF_IMPORT_TARGET.templateFileName);
  }

  async function write(file: NonNullable<Attempt["file"]>, current: string | null) {
    const key = keyForAttempt(current, () => crypto.randomUUID());
    dispatch({ type: "importStarted", key });
    setBusy("import");
    const r = await commitImport<StaffImportCreatedRow>(routes, file.blob, file.name, key);
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
    const p = await previewImport<StaffImportPlannedRow>(routes, STAFF_IMPORT_TARGET.rowsField, file.blob, file.name);
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

  return (
    <ModalDialog
      titleId={STAFF_IMPORT_TITLE_ID}
      className="max-w-[672px]"
      closeDisabled={busy === "import" || done}
      onDismiss={() => {
        if (busy === "import" || done) return;
        close();
      }}
    >
      <ModalDialogHeader titleId={STAFF_IMPORT_TITLE_ID} title={STAFF_IMPORT_TITLE} description={STAFF_IMPORT_DESCRIPTION} />
      <StaffImportView
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
export function StaffImportView({
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
  fileName: string | null;
  preview: KetQua<ImportPreview<StaffImportPlannedRow>> | null;
  result: ImportResult<StaffImportCreatedRow> | null;
  busy: StaffImportBusy;
  templateError: string;
  onTemplate: () => void;
  onChoose: (f: File | null) => void;
  onImport: () => void;
  onClose: () => void;
}) {
  const done = result !== null && result.ok;
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
            id={FILE_INPUT_ID}
            name={FILE_INPUT_ID}
            type="file"
            accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
            className="hidden"
            aria-label={STAFF_IMPORT_CHOOSE_FILE}
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
            onClick={() => document.getElementById(FILE_INPUT_ID)?.click()}
          >
            {STAFF_IMPORT_CHOOSE_FILE}
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
        <ErrorBox heading={STAFF_IMPORT_ERRORS_HEADING} rows={errorRows(preview.duLieu.errors)} />
      )}

      {preview !== null && preview.ok && preview.duLieu.valid && preview.duLieu.rows.length === 0 && (
        <EmptyState icon={Upload} title={NOTHING_TO_CREATE} />
      )}

      {result !== null && !result.ok && (
        <ErrorBox heading={result.message} rows={errorRows(result.errors)} />
      )}

      {done && (
        <>
          <p role="status" className="m-0 flex items-center gap-2 font-medium text-success-600">
            <CircleCheck aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-[18px] shrink-0" />
            {STAFF_IMPORT_TARGET.importedSentence(result.created === null ? null : result.created.length)}
          </p>
          {/* Owns closing from here on: N one-time passwords are closed by an explicit act only. */}
          <StaffImportResult created={result.created} onClose={onClose} />
        </>
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
            {STAFF_IMPORT_SUBMIT}
          </Button>
        </div>
      )}
    </div>
  );
}

/** The red box: one heading, then `Dòng | Cột | Vấn đề` (task import's look). Row 0 reads "Cả tệp". */
function ErrorBox({ heading, rows }: { heading: string; rows: readonly ErrorRow[] }) {
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
