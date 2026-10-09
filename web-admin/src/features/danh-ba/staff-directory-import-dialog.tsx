"use client";

import { CircleCheck, Download, LoaderCircle, TriangleAlert, Upload } from "lucide-react";
import { useReducer, useState, type ReactNode } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";
import {
  EMPTY_ATTEMPT,
  canImport,
  errorRows,
  keyForAttempt,
  nextAttempt,
} from "@/features/cau-hinh/excel-import-flow";
import type { AttemptEvent, ImportAttempt } from "@/features/cau-hinh/excel-import-flow";
import { STAFF_IMPORT_TARGET } from "@/features/cau-hinh/excel-import-targets";
import {
  StaffImportResult,
  issuedCredentials,
  issuedWithoutPassword,
} from "@/features/cau-hinh/staff-import-result";
import { commitImport, downloadImportTemplate, previewImport } from "@/lib/api/excel-import";
import type { ImportError, ImportPreview, ImportResult } from "@/lib/api/excel-import";
import type { KetQua } from "@/lib/api/goi"; // vi-name-ok: existing type of goi.ts, imported not declared (rule 12 inv 3)
import type { StaffImportCreatedRow, StaffImportPlannedRow } from "@/lib/api/staff-import";

/**
 * "Nhập từ Excel" of the Danh bạ tab, in the prototype's presentation (`ExcelImportDialog.tsx:104-230`):
 * no icon in the title, a ghost template link, a dashed drop box, the check runs as soon as a file is
 * chosen, one result box (green / red + errors), and a footer of exactly two buttons.
 *
 * THE LOGIC IS THE STAFF IMPORT OF `/nguoi-dung`, UNCHANGED (ADR 0059): the same three routes
 * (`STAFF_IMPORT_TARGET.routes`, `admin.user` on each), the same attempt reducer (`nextAttempt` — ONE
 * place holds what the server answered, and `closed` empties it), the same one-key-per-attempt rule.
 * Only the presentation is this tab's.
 *
 * WHERE THE PROTOTYPE CLOSES AND THIS DOES NOT: a staff row with a work address is issued an account,
 * and the 201 carries its temporary password ONCE. Closing on success would destroy values nobody can
 * show again, so when the 201 holds any (or is a replay, whose sentence says how to get new ones) the
 * box stays open on `StaffImportResult`, which only closes on its explicit "Tôi đã lưu, đóng".
 */

export const DIRECTORY_IMPORT_TITLE = "Nhập danh bạ cán bộ từ Excel";
export const DIRECTORY_IMPORT_DESCRIPTION =
  "Tệp được kiểm trước và chưa ghi gì. Còn một dòng sai thì không dòng nào được nhận — sửa tệp rồi nhập lại.";
export const DIRECTORY_TEMPLATE_BUTTON = "Tải mẫu danh bạ";
export const DIRECTORY_TEMPLATE_FILE = "mau-danh-ba-can-bo.xlsx";
export const CHOOSE_FILE_BUTTON = "Chọn tệp .xlsx";
export const CHECKING_FILE = "Đang kiểm tệp…";
export const DIRECTORY_CLOSE_BUTTON = "Đóng";
export const ERRORS_ISSUE_HEADER = "Vấn đề";

/** The prototype's error toasts, verbatim (`ExcelImportDialog.tsx:69, 86, 89, 121`). */
export const FILE_UNREADABLE_TOAST = "Không đọc được tệp. Kiểm tra lại định dạng .xlsx.";
export const ROWS_REFUSED_TOAST = "Tệp còn dòng sai, chưa ghi dòng nào.";
export const IMPORT_FAILED_TOAST = "Không nhập được. Vui lòng thử lại.";
export const TEMPLATE_FAILED_TOAST = "Không tải được tệp mẫu.";

export function validSentence(n: number): string {
  return `${n} dòng hợp lệ, sẵn sàng nhập`;
}

export function invalidSentence(wrong: number, total: number): string {
  return `${wrong} dòng sai trên tổng số ${total} dòng`;
}

/** The footer's confirm button: "Nhập {n} cán bộ" once a check has counted the rows. */
export function importButtonText(n: number | null): string {
  return n === null ? "Nhập" : `Nhập ${n} cán bộ`;
}

/** The prototype's success toast (`ExcelImportDialog.tsx:81`, unit "cán bộ"). */
export function importedToast(n: number): string {
  return `Đã nhập ${n} cán bộ.`;
}

/**
 * Rows counted for the red sentence. The contract has no "total rows" field, so the total is the union
 * of the rows the server planned and the rows it refused — correct whether or not a refused row is also
 * echoed in the plan. `row: 0` is an error of the whole file, not a row: never counted.
 */
export function rowCounts(
  planned: readonly { readonly row: number }[],
  errors: readonly ImportError[],
): { readonly wrong: number; readonly total: number } {
  const wrongRows = new Set(errors.filter((e) => e.row > 0).map((e) => e.row));
  const all = new Set([...planned.map((p) => p.row), ...wrongRows]);
  return { wrong: wrongRows.size, total: all.size };
}

/**
 * Whether a successful 201 must stay on screen: it carries temporary passwords, an issued account whose
 * password did not arrive, or it is a replay (`null`). Otherwise the toast says it all and the box closes.
 */
export function mustShowResult(created: readonly StaffImportCreatedRow[] | null): boolean {
  if (created === null) return true;
  return issuedCredentials(created).length > 0 || issuedWithoutPassword(created).length > 0;
}

type Busy = "" | "template" | "preview" | "import";

const FILE_INPUT_ID = "o-tep-nhap-danh-ba";
const TITLE_ID = "tieu-de-nhap-danh-ba";

/** Mounted = open. `onImported` re-reads the list and the counts; `onClose` unmounts. */
export function StaffDirectoryImportDialog({
  onImported,
  onClose,
}: {
  onImported: () => void;
  onClose: () => void;
}) {
  const [attempt, dispatch] = useReducer(
    (s: ImportAttempt<StaffImportPlannedRow, StaffImportCreatedRow>, e: AttemptEvent<StaffImportPlannedRow, StaffImportCreatedRow>) =>
      nextAttempt(s, e),
    EMPTY_ATTEMPT as ImportAttempt<StaffImportPlannedRow, StaffImportCreatedRow>,
  );
  const [busy, setBusy] = useState<Busy>("");
  const { file, preview, key: importKey, result } = attempt;
  const showingResult = result !== null && result.ok;

  function close() {
    dispatch({ type: "closed" });
    onClose();
  }

  async function downloadTemplate() {
    setBusy("template");
    const r = await downloadImportTemplate(STAFF_IMPORT_TARGET.routes);
    setBusy("");
    if (!r.ok) {
      toast.error(TEMPLATE_FAILED_TOAST);
      return;
    }
    const url = URL.createObjectURL(r.duLieu);
    const a = document.createElement("a");
    a.href = url;
    a.download = DIRECTORY_TEMPLATE_FILE;
    a.click();
    URL.revokeObjectURL(url);
  }

  /** Choosing a file IS the check (the prototype's `check`): a new attempt, previewed at once. */
  async function choose(chosen: File) {
    dispatch({ type: "chosen", file: { blob: chosen, name: chosen.name } });
    setBusy("preview");
    dispatch({ type: "previewStarted" });
    const p = await previewImport<StaffImportPlannedRow>(
      STAFF_IMPORT_TARGET.routes,
      STAFF_IMPORT_TARGET.rowsField,
      chosen,
      chosen.name,
    );
    setBusy("");
    dispatch({ type: "previewed", preview: p });
    // The file itself was refused (not an xlsx, too big, the server unreachable): the prototype's toast,
    // no box — there is nothing in the file to point at.
    if (!p.ok) toast.error(FILE_UNREADABLE_TOAST);
  }

  async function commit() {
    if (file === null || preview === null || !preview.ok || !canImport(preview.duLieu)) return;
    const key = keyForAttempt(importKey, () => crypto.randomUUID());
    dispatch({ type: "importStarted", key });
    setBusy("import");
    const r = await commitImport<StaffImportCreatedRow>(STAFF_IMPORT_TARGET.routes, file.blob, file.name, key);
    setBusy("");
    dispatch({ type: "imported", key, result: r });
    if (!r.ok) {
      // Row errors (400 `import_invalid`, the file changed under the check): the preview turns red with
      // them. Anything else: the toast alone, the green preview and the SAME key stay — pressing again
      // retries this attempt, never a second import (`keyAfterAttempt`).
      toast.error(r.errors.length > 0 ? ROWS_REFUSED_TOAST : IMPORT_FAILED_TOAST);
      return;
    }
    onImported();
    toast.success(r.created === null ? STAFF_IMPORT_TARGET.importedSentence(null) : importedToast(r.created.length));
    if (!mustShowResult(r.created)) close();
  }

  return (
    // Esc / ✕ do nothing while the import is in flight, nor while one-time passwords are on screen:
    // only the result's explicit button closes then.
    <ModalDialog
      titleId={TITLE_ID}
      onDismiss={() => {
        if (busy === "import" || showingResult) return;
        close();
      }}
      closeDisabled={busy === "import" || showingResult}
      size="lg"
    >
      <StaffDirectoryImportView
        fileName={file === null ? null : file.name}
        preview={preview}
        result={result}
        busy={busy}
        onDownloadTemplate={() => void downloadTemplate()}
        onChooseFile={(f) => void choose(f)}
        onImport={() => void commit()}
        onClose={close}
      />
    </ModalDialog>
  );
}

/** Pure rendering, no hooks — the test reads it as an element tree and as static markup. */
export function StaffDirectoryImportView({
  fileName,
  preview,
  result,
  busy,
  onDownloadTemplate,
  onChooseFile,
  onImport,
  onClose,
}: {
  fileName: string | null;
  preview: KetQua<ImportPreview<StaffImportPlannedRow>> | null;
  result: ImportResult<StaffImportCreatedRow> | null;
  busy: Busy;
  onDownloadTemplate: () => void;
  onChooseFile: (f: File) => void;
  onImport: () => void;
  onClose: () => void;
}) {
  const header = <ModalDialogHeader titleId={TITLE_ID} title={DIRECTORY_IMPORT_TITLE} description={DIRECTORY_IMPORT_DESCRIPTION} />;

  if (result !== null && result.ok) {
    return (
      <>
        {header}
        <div className="mt-4 min-h-0 min-w-0 overflow-y-auto">
          <StaffImportResult created={result.created} onClose={onClose} />
        </div>
      </>
    );
  }

  const previewed = preview !== null && preview.ok ? preview.duLieu : null;
  // An import refused for ROW errors replaces the check's verdict with the server's (prototype
  // `setPreview(result)`, `ExcelImportDialog.tsx:85`): red, and Nhập held until a new file is chosen.
  const checked =
    previewed !== null && result !== null && !result.ok && result.errors.length > 0
      ? { ...previewed, valid: false, errors: result.errors }
      : previewed;
  const clean = checked !== null && canImport(checked);

  return (
    <>
      {header}
      <div className="mt-4 min-h-0 min-w-0 space-y-4 overflow-y-auto">
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="text-brand h-auto px-0"
          disabled={busy === "template"}
          icon={
            busy === "template" ? (
              <LoaderCircle aria-hidden="true" className="size-4 animate-spin" />
            ) : (
              <Download aria-hidden="true" className="size-4" />
            )
          }
          onClick={onDownloadTemplate}
        >
          {DIRECTORY_TEMPLATE_BUTTON}
        </Button>

        <div className="border-line rounded-[10px] border border-dashed p-5 text-center">
          <Upload aria-hidden="true" className="text-ink-muted mx-auto mb-2 size-6" />
          <input
            id={FILE_INPUT_ID}
            type="file"
            accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
            className="hidden"
            aria-label={CHOOSE_FILE_BUTTON}
            onChange={(event) => {
              const chosen = event.target.files?.[0];
              // Emptied so choosing the same file again (re-saved after a fix) checks it again.
              event.target.value = "";
              if (chosen) onChooseFile(chosen);
            }}
          />
          <Button
            type="button"
            variant="outline"
            disabled={busy !== ""}
            onClick={() => document.getElementById(FILE_INPUT_ID)?.click()}
          >
            {CHOOSE_FILE_BUTTON}
          </Button>
          {fileName !== null && <p className="text-ink-muted m-0 mt-2 text-[12px]">{fileName}</p>}
        </div>

        {busy === "preview" && (
          <p className="text-ink-muted m-0 flex items-center gap-2 text-[12.5px]" role="status">
            <LoaderCircle aria-hidden="true" className="size-4 animate-spin" />
            {CHECKING_FILE}
          </p>
        )}

        {/* A refused file or a refused import is said by a toast (`StaffDirectoryImportDialog`); only a
            completed check draws a box. */}
        {checked !== null && busy !== "preview" && <CheckResult preview={checked} />}

        <div className="flex justify-end gap-2">
          <Button type="button" variant="outline" onClick={onClose} disabled={busy === "import"}>
            {DIRECTORY_CLOSE_BUTTON}
          </Button>
          <Button
            type="button"
            variant="primary"
            onClick={onImport}
            disabled={!clean || busy !== ""}
            aria-busy={busy === "import"}
          >
            {importButtonText(checked === null ? null : checked.valid ? checked.rows.length : rowCounts(checked.rows, checked.errors).total)}
          </Button>
        </div>
      </div>
    </>
  );
}

function DangerBox({ children }: { children: ReactNode }) {
  return (
    <div className="border-danger/25 bg-danger/8 rounded-[10px] border border-solid p-3" role="alert">
      {children}
    </div>
  );
}

/**
 * The green or red box of a completed check. No table of valid rows: the prototype has none. A valid file
 * with no row is green too — `0 dòng hợp lệ, sẵn sàng nhập`, as the prototype says it — and the footer's
 * Nhập stays held (`canImport`), so nothing empty is ever sent.
 */
function CheckResult({ preview }: { preview: ImportPreview<StaffImportPlannedRow> }) {
  if (preview.valid) {
    return (
      <div className="border-leaf/25 bg-leaf/8 rounded-[10px] border border-solid p-3" role="status">
        <p className="m-0 flex items-center gap-2 text-[12.5px] font-semibold">
          <CircleCheck aria-hidden="true" className="text-leaf size-4" />
          <span className="text-navy">{validSentence(preview.rows.length)}</span>
        </p>
      </div>
    );
  }
  const { wrong, total } = rowCounts(preview.rows, preview.errors);
  return (
    <DangerBox>
      <p className="m-0 flex items-center gap-2 text-[12.5px] font-semibold">
        <TriangleAlert aria-hidden="true" className="text-danger size-4" />
        {/* Only whole-file errors (`row: 0`): no row to count, so the heading says what was not created. */}
        <span className="text-navy">{wrong > 0 ? invalidSentence(wrong, total) : STAFF_IMPORT_TARGET.errorsHeading}</span>
      </p>
      <ErrorsTable errors={preview.errors} />
    </DangerBox>
  );
}

/** Dòng · Cột · Vấn đề — the prototype's table (`ExcelImportDialog.tsx:188-214`). */
function ErrorsTable({ errors }: { errors: readonly ImportError[] }) {
  return (
    <div className="mt-2 max-h-52 overflow-y-auto">
      <table className="w-full border-collapse text-[12px]">
        <thead>
          <tr className="text-ink-muted text-left text-[10.5px] uppercase">
            <th scope="col" className="py-1.5 pr-3 font-semibold">
              Dòng
            </th>
            <th scope="col" className="py-1.5 pr-3 font-semibold">
              Cột
            </th>
            <th scope="col" className="py-1.5 font-semibold">
              {ERRORS_ISSUE_HEADER}
            </th>
          </tr>
        </thead>
        <tbody>
          {errorRows(errors).map((e, i) => (
            // Two errors may share a row and a column — the index keeps keys distinct.
            <tr key={`${e.row}-${e.column}-${i}`} className="border-line border-t border-solid">
              <td className="text-danger py-1.5 pr-3 font-semibold">{e.row}</td>
              <td className="text-ink-muted py-1.5 pr-3">{e.column}</td>
              <td className="py-1.5">{e.message}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
