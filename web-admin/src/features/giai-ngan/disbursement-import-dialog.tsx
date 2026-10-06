"use client";

import { CircleCheck, Download, FileSpreadsheet, TriangleAlert, Upload, X } from "lucide-react";
import { useRef, useState, type DragEvent } from "react";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba"; // vi-name-ok: existing idempotency-key helper (rule 12 invariant 3)
import { Button } from "@/components/ui/button";
import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";
import { CLOSE_BUTTON, errorRows, keyAfterAttempt, keyForAttempt } from "@/features/cau-hinh/excel-import-flow";
import { ErrorsTable } from "@/features/cau-hinh/excel-import-panel";
import { BusyLabel } from "@/features/danh-ba/busy-label";
import {
  commitDisbursementImport,
  DISBURSEMENT_TEMPLATE_FILE_NAME,
  downloadDisbursementImportTemplate,
  previewDisbursementImport,
} from "@/lib/api/disbursement-import";
import type { DisbursementImportCreatedRow, DisbursementImportPreview } from "@/lib/api/disbursement-import";
import type { ImportResult } from "@/lib/api/excel-import";
import type { KetQua } from "@/lib/api/goi"; // vi-name-ok: existing result type of goi.ts (rule 12 invariant 3)
import { cn } from "@/lib/cn";

import { nhanTien } from "./nhan-du-an";

/**
 * `[Nhập giải ngân]` of the Giải ngân header and its §10 modal — prototype `BudgetWorkspace.tsx:424-431`
 * + `shared/ExcelImportDialog.tsx`: template link, `Chọn tệp .xlsx`, automatic check, `Đóng` · `Nhập`.
 *
 * TWO REQUESTS, AS EVERY IMPORT OF THIS APP (ADR 0059): the preview writes nothing; `Nhập` is offered
 * only once THE SAME FILE came back valid with at least one voucher. All-or-nothing is the SERVER's
 * rule (`service-finance/internal/http/disbursement_import.go`); the browser only shows its answer.
 *
 * ONE `Idempotency-Key` PER ATTEMPT AT ONE CHOSEN FILE: minted at the first press of `Nhập`, reused by a
 * retry of that file (the first send may have reached the server — a new key would write every voucher
 * twice), dropped when another file is chosen or the attempt succeeds. Same rule and same functions as
 * the Cấu hình imports (`excel-import-flow.ts`).
 *
 * WHAT THE KEY CANNOT DO: tell a second, deliberate import of the same file from a new batch. Each
 * successful import is a new batch, so the modal says so in one line before the button.
 *
 * Shown only with `budget.update` (the caller's `canRecord`), as the prototype does. UX only — the
 * three routes check `budget.read` / `budget.update` themselves (rule 5).
 */
export function DisbursementImportButton({
  canImport,
  onImported,
}: {
  /** `budget.update` held. `false` → nothing is drawn. */
  canImport: boolean;
  /** After a 201: the page re-reads the register, the year summary and the per-source progress. */
  onImported: () => void;
}) {
  const [open, setOpen] = useState(false);
  if (!canImport) return null;
  return (
    <>
      <Button
        type="button"
        variant="secondary"
        icon={<Upload aria-hidden="true" />}
        aria-haspopup="dialog"
        onClick={() => setOpen(true)}
      >
        Nhập giải ngân
      </Button>
      {open && <DisbursementImportDialog onClose={() => setOpen(false)} onImported={onImported} />}
    </>
  );
}

export const IMPORT_TITLE = "Nhập lần giải ngân từ Excel";
export const IMPORT_DESCRIPTION =
  "Tệp được kiểm trước và chưa ghi gì. Còn một dòng sai thì không dòng nào được nhận — sửa tệp rồi nhập lại.";
export const TEMPLATE_LINK = "Tải mẫu giải ngân";
export const DROP_ZONE_LABEL = "Chọn tệp .xlsx";
export const CHECKING = "Đang kiểm tệp…";
export const IMPORT_CONFIRM = "Nhập";
export const DUPLICATE_CAUTION = "Mỗi lần nhập là một đợt mới — nhập lại cùng tệp sẽ ghi trùng chứng từ.";
export const INVALID_HEADING = "Tệp còn dòng sai nên chưa có chứng từ nào được nhận. Sửa các dòng dưới đây rồi chọn lại tệp.";
export const NO_VOUCHER = "Tệp không có dòng chứng từ nào để nhập.";

/** The preview's two numbers, in the units the accountant checks them against: vouchers and full đồng. */
export function previewSummary(p: DisbursementImportPreview): string {
  return `${p.rowCount} chứng từ · tổng ${nhanTien(p.totalAmount)}`;
}

/** `null` = the server replayed an earlier success of the same key; the count is then unknown. */
export function importedSentence(created: readonly DisbursementImportCreatedRow[] | null): string {
  return created === null ? "Đã nhập tệp chứng từ." : `Đã nhập ${created.length} chứng từ.`;
}

/** Whether `Nhập` may be pressed: a valid preview of this file with at least one voucher. */
export function canCommit(preview: KetQua<DisbursementImportPreview> | null): boolean {
  return preview !== null && preview.ok && preview.duLieu.valid && preview.duLieu.rowCount > 0;
}

const TITLE_ID = "tieu-de-nhap-giai-ngan";
const FILE_INPUT_ID = "o-tep-nhap-giai-ngan";

type Busy = "" | "template" | "preview" | "import";
type Chosen = { readonly blob: Blob; readonly name: string };

/** Mounted = open (`ModalDialog`). Everything the server answered lives here and goes on close. */
export function DisbursementImportDialog({ onClose, onImported }: { onClose: () => void; onImported: () => void }) {
  const [file, setFile] = useState<Chosen | null>(null);
  const [preview, setPreview] = useState<KetQua<DisbursementImportPreview> | null>(null);
  const [importKey, setImportKey] = useState<string | null>(null);
  const [result, setResult] = useState<ImportResult<DisbursementImportCreatedRow> | null>(null);
  const [busy, setBusy] = useState<Busy>("");
  const [templateError, setTemplateError] = useState("");
  // Bumped on every choice: a preview that answers after ANOTHER file was chosen is dropped, or the
  // first file's "valid" would enable `Nhập` for the second.
  const choice = useRef(0);

  const done = result !== null && result.ok;

  async function choose(f: File | null) {
    if (f === null || busy === "import" || done) return;
    const mine = ++choice.current;
    // A NEW FILE IS A NEW ATTEMPT: preview, key and result start over.
    setFile({ blob: f, name: f.name });
    setPreview(null);
    setImportKey(null);
    setResult(null);
    setBusy("preview");
    const p = await previewDisbursementImport(f, f.name);
    if (choice.current !== mine) return;
    setBusy("");
    setPreview(p);
  }

  async function downloadTemplate() {
    setBusy("template");
    setTemplateError("");
    const r = await downloadDisbursementImportTemplate();
    setBusy("");
    if (!r.ok) {
      setTemplateError(r.thongBao);
      return;
    }
    const url = URL.createObjectURL(r.duLieu);
    const a = document.createElement("a");
    a.href = url;
    a.download = DISBURSEMENT_TEMPLATE_FILE_NAME;
    a.click();
    URL.revokeObjectURL(url);
  }

  async function commit() {
    if (file === null || !canCommit(preview) || busy !== "") return;
    const key = keyForAttempt(importKey, khoaChongTrungMoi);
    setImportKey(key);
    setBusy("import");
    setResult(null);
    const r = await commitDisbursementImport(file.blob, file.name, key);
    setBusy("");
    setImportKey(keyAfterAttempt(key, r.ok));
    setResult(r);
    // Re-read at once, not on close: the vouchers exist on the server whether or not the box stays open.
    if (r.ok) onImported();
  }

  function onDrop(e: DragEvent<HTMLLabelElement>) {
    e.preventDefault();
    void choose(e.dataTransfer.files[0] ?? null);
  }

  const locked = busy === "import" || done;

  return (
    // Esc while the import is in flight does nothing: the answer must have somewhere to show.
    <ModalDialog titleId={TITLE_ID} size="lg" onDismiss={() => busy !== "import" && onClose()}>
      <ModalDialogHeader titleId={TITLE_ID} title={IMPORT_TITLE} description={IMPORT_DESCRIPTION} />

      <div className="flex min-h-0 min-w-0 flex-col gap-3 overflow-y-auto [&>*]:my-0">
        <p className="m-0">
          <Button
            type="button"
            variant="ghost"
            icon={busy === "template" ? undefined : <Download aria-hidden="true" focusable="false" strokeWidth={1.8} />}
            onClick={() => void downloadTemplate()}
            disabled={busy !== ""}
            aria-busy={busy === "template"}
          >
            <BusyLabel busy={busy === "template"} label={TEMPLATE_LINK} busyText="Đang tải mẫu…" />
          </Button>
        </p>
        {templateError !== "" && (
          <p className="thong-bao-loi m-0" role="alert">
            {templateError}
          </p>
        )}

        {!done && (
          <label
            htmlFor={FILE_INPUT_ID}
            onDragOver={(e) => e.preventDefault()}
            onDrop={onDrop}
            className={cn(
              "flex min-w-0 cursor-pointer flex-col items-center gap-2 rounded-card border border-dashed border-line p-5 text-center",
              "focus-within:outline-2 focus-within:outline-brand-600",
              locked && "cursor-not-allowed opacity-60",
            )}
          >
            <Upload aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-6 text-ink-400" />
            <span className="font-medium text-ink-900">{DROP_ZONE_LABEL}</span>
            {file !== null && <span className="max-w-full truncate text-xs text-ink-500">{file.name}</span>}
            <input
              id={FILE_INPUT_ID}
              type="file"
              accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
              className="sr-only"
              disabled={locked}
              onChange={(e) => {
                const f = e.target.files?.[0] ?? null;
                // Cleared so choosing the SAME file again (after fixing it) fires `change` again.
                e.target.value = "";
                void choose(f);
              }}
            />
          </label>
        )}

        {busy === "preview" && (
          <p className="m-0 text-sm text-ink-500" role="status">
            {CHECKING}
          </p>
        )}

        {/* A refusal of the FILE (not xlsx, too big): one sentence, the server's — 413/415 included. */}
        {preview !== null && !preview.ok && (
          <p className="thong-bao-loi m-0" role="alert">
            {preview.thongBao}
          </p>
        )}

        {preview !== null && preview.ok && !done && <PreviewBody preview={preview.duLieu} />}

        {result !== null && !result.ok && (
          <div className="thong-bao-loi [&_p]:m-0 [&>p]:mb-2" role="alert">
            <p>{result.message}</p>
            {result.errors.length > 0 && <ErrorsTable rows={errorRows(result.errors)} />}
          </div>
        )}

        {done && (
          <p role="status" className="m-0 flex items-center gap-2 font-medium text-success-600">
            <CircleCheck aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-[18px] shrink-0" />
            {importedSentence(result.created)}
          </p>
        )}

        {!done && (
          <p className="m-0 flex items-start gap-1.5 text-xs text-ink-500" data-duplicate-caution="">
            <TriangleAlert aria-hidden="true" focusable="false" strokeWidth={1.8} className="mt-px size-3.5 shrink-0" />
            {DUPLICATE_CAUTION}
          </p>
        )}
      </div>

      <div className="flex shrink-0 flex-wrap justify-end gap-2">
        <Button
          type="button"
          variant="ghost"
          icon={<X aria-hidden="true" focusable="false" strokeWidth={1.8} />}
          onClick={onClose}
          disabled={busy === "import"}
        >
          {CLOSE_BUTTON}
        </Button>
        {!done && (
          <Button
            type="button"
            variant="primary"
            icon={busy === "import" ? undefined : <Upload aria-hidden="true" focusable="false" strokeWidth={1.8} />}
            onClick={() => void commit()}
            disabled={!canCommit(preview) || busy !== ""}
            aria-busy={busy === "import"}
          >
            <BusyLabel busy={busy === "import"} label={IMPORT_CONFIRM} busyText="Đang nhập…" />
          </Button>
        )}
      </div>
    </ModalDialog>
  );
}

function PreviewBody({ preview }: { preview: DisbursementImportPreview }) {
  if (preview.valid && preview.rowCount === 0) {
    return (
      <p className="m-0 flex items-center gap-2 text-sm text-ink-500">
        <FileSpreadsheet aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-4 shrink-0" />
        {NO_VOUCHER}
      </p>
    );
  }
  if (preview.valid) {
    return (
      <p className="m-0 flex items-center gap-2 font-medium text-ink-900" data-preview-summary="">
        <CircleCheck aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-[18px] shrink-0 text-success-600" />
        {previewSummary(preview)} — sẵn sàng nhập.
      </p>
    );
  }
  return (
    <div className="thong-bao-loi [&_p]:m-0 [&>p]:mb-2" role="alert">
      <p>{INVALID_HEADING}</p>
      <p data-preview-summary="">Đọc được {previewSummary(preview)}.</p>
      <ErrorsTable rows={errorRows(preview.errors)} />
    </div>
  );
}
