"use client";

import { CircleCheck, Download, FileSpreadsheet, Loader2, TriangleAlert, Upload } from "lucide-react";
import { useRef, useState, type DragEvent } from "react";
import { toast } from "sonner";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba"; // vi-name-ok: existing idempotency-key helper (rule 12 invariant 3)
import { Button } from "@/components/ui/button";
import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";
import { CLOSE_BUTTON, errorRows, keyAfterAttempt, keyForAttempt, type ErrorRow } from "@/features/cau-hinh/excel-import-flow";
import {
  commitDisbursementImport,
  DISBURSEMENT_TEMPLATE_FILE_NAME,
  downloadDisbursementImportTemplate,
  previewDisbursementImport,
} from "@/lib/api/disbursement-import";
import type { DisbursementImportCreatedRow, DisbursementImportPreview } from "@/lib/api/disbursement-import";
import type { ImportError } from "@/lib/api/excel-import";
import type { ImportResult } from "@/lib/api/excel-import";
import type { KetQua } from "@/lib/api/goi"; // vi-name-ok: existing result type of goi.ts (rule 12 invariant 3)
import { cn } from "@/lib/cn";


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
 * successful import is a new batch, so the modal says so in one line before the button (kept against
 * the prototype by the main session's resolution of row E8).
 *
 * LOOK AND WORDS = spec 05 (ADR 0068 lần 6): 42rem, `Tải mẫu giải ngân` as a brand-coloured link, the
 * dashed box with its `Chọn tệp .xlsx` button, the green / red result box, `[Đóng] [Nhập N lần giải
 * ngân]`. A success is a toast and closes the dialog; a refusal is the server's sentence as a toast —
 * except "Tệp còn dòng sai, chưa ghi dòng nào.", the one spec sentence that names an outcome this
 * screen can recognise (the server returned row errors).
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
        variant="outline"
        icon={<Upload aria-hidden="true" className="size-4" />}
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
export const NO_VOUCHER = "Tệp không có dòng chứng từ nào để nhập.";
/** Spec 05 toast when the import comes back with row errors. */
export const ROWS_STILL_WRONG = "Tệp còn dòng sai, chưa ghi dòng nào.";

/** Spec 05: `Nhập {total} lần giải ngân` once a preview is read; `Nhập` before. */
export function importButtonLabel(preview: KetQua<DisbursementImportPreview> | null): string {
  return preview !== null && preview.ok ? `${IMPORT_CONFIRM} ${preview.duLieu.rowCount} lần giải ngân` : IMPORT_CONFIRM;
}

/** Spec 05 result heading: "{n} dòng hợp lệ, sẵn sàng nhập" or "{e} dòng sai trên tổng số {t} dòng". */
export function previewHeading(p: DisbursementImportPreview): string {
  return p.valid ? `${p.rowCount} dòng hợp lệ, sẵn sàng nhập` : `${wrongRowCount(p.errors)} dòng sai trên tổng số ${p.rowCount} dòng`;
}

/** Distinct rows named by the errors: one row with three wrong cells is one wrong row. */
export function wrongRowCount(errors: readonly ImportError[]): number {
  return new Set(errors.map((e) => e.row)).size;
}

/**
 * Spec 05 success toast: `Đã nhập {n} lần giải ngân.` `created` is `null` when the server replayed an
 * earlier success of the same key — the count then comes from the preview of the same file.
 */
export function importedSentence(created: readonly DisbursementImportCreatedRow[] | null, previewed: number): string {
  return `Đã nhập ${created === null ? previewed : created.length} lần giải ngân.`;
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
    const r = await downloadDisbursementImportTemplate();
    setBusy("");
    if (!r.ok) {
      toast.error(r.thongBao);
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
    if (file === null || !canCommit(preview) || busy !== "" || preview === null || !preview.ok) return;
    const previewed = preview.duLieu.rowCount;
    const key = keyForAttempt(importKey, khoaChongTrungMoi);
    setImportKey(key);
    setBusy("import");
    setResult(null);
    const r = await commitDisbursementImport(file.blob, file.name, key);
    setBusy("");
    setImportKey(keyAfterAttempt(key, r.ok));
    if (r.ok) {
      // Re-read at once; the vouchers exist on the server. Then close, as spec 05 does.
      toast.success(importedSentence(r.created, previewed));
      onImported();
      onClose();
      return;
    }
    setResult(r);
    toast.error(r.errors.length > 0 ? ROWS_STILL_WRONG : r.message);
  }

  function onDrop(e: DragEvent<HTMLDivElement>) {
    e.preventDefault();
    void choose(e.dataTransfer.files[0] ?? null);
  }

  const locked = busy === "import";
  const fileInput = useRef<HTMLInputElement>(null);

  return (
    // Esc while the import is in flight does nothing: the answer must have somewhere to show.
    <ModalDialog titleId={TITLE_ID} className="max-w-[42rem]" onDismiss={() => busy !== "import" && onClose()}>
      <ModalDialogHeader titleId={TITLE_ID} title={IMPORT_TITLE} description={IMPORT_DESCRIPTION} />

      <div className="flex min-h-0 min-w-0 flex-col gap-4 overflow-y-auto">
        <div>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="text-brand hover:not-disabled:text-brand h-auto self-start px-0 hover:not-disabled:bg-transparent"
            icon={
              busy === "template" ? (
                <Loader2 aria-hidden="true" className="size-4 animate-spin" />
              ) : (
                <Download aria-hidden="true" className="size-4" />
              )
            }
            onClick={() => void downloadTemplate()}
            disabled={busy !== ""}
            aria-busy={busy === "template"}
          >
            {TEMPLATE_LINK}
          </Button>
        </div>

        {/* Spec 05 drop box: the muted Upload icon, the outline `Chọn tệp .xlsx` button, the file name.
            Dropping a file on the box still works — it changes nothing in the picture. */}
        <div
          onDragOver={(e) => e.preventDefault()}
          onDrop={onDrop}
          className="border-line rounded-[10px] border border-dashed p-5 text-center"
        >
          <Upload aria-hidden="true" focusable="false" className="text-ink-muted mx-auto mb-2 block size-6" />
          <input
            ref={fileInput}
            id={FILE_INPUT_ID}
            type="file"
            accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
            className="hidden"
            aria-label={DROP_ZONE_LABEL}
            disabled={locked}
            onChange={(e) => {
              const f = e.target.files?.[0] ?? null;
              // Cleared so choosing the SAME file again (after fixing it) fires `change` again.
              e.target.value = "";
              void choose(f);
            }}
          />
          <Button type="button" variant="outline" disabled={locked} onClick={() => fileInput.current?.click()}>
            {DROP_ZONE_LABEL}
          </Button>
          {file !== null && <p className="text-ink-muted m-0 mt-2 truncate text-[12px]">{file.name}</p>}
        </div>

        {busy === "preview" && (
          <p className="text-ink-muted m-0 flex items-center gap-2 text-[12.5px]" role="status">
            <Loader2 aria-hidden="true" className="size-4 animate-spin" />
            {CHECKING}
          </p>
        )}

        {/* A refusal of the FILE (not xlsx, too big): one sentence, the server's — 413/415 included. */}
        {preview !== null && !preview.ok && (
          <p className="thong-bao-loi m-0" role="alert">
            {preview.thongBao}
          </p>
        )}

        {result !== null && !result.ok && result.errors.length > 0 ? (
          <ResultBox ok={false} heading={`${wrongRowCount(result.errors)} dòng sai`} rows={errorRows(result.errors)} />
        ) : (
          preview !== null && preview.ok && <PreviewBody preview={preview.duLieu} />
        )}
        {result !== null && !result.ok && result.errors.length === 0 && (
          <p className="thong-bao-loi m-0" role="alert">
            {result.message}
          </p>
        )}

        <p className="text-ink-muted m-0 flex items-start gap-1.5 text-[11.5px]" data-duplicate-caution="">
          <TriangleAlert aria-hidden="true" focusable="false" className="mt-px size-3.5 shrink-0" />
          {DUPLICATE_CAUTION}
        </p>

        <div className="flex flex-wrap justify-end gap-2">
          <Button type="button" variant="outline" onClick={onClose} disabled={busy === "import"}>
            {CLOSE_BUTTON}
          </Button>
          <Button
            type="button"
            variant="primary"
            icon={busy === "import" ? <Loader2 aria-hidden="true" className="size-4 animate-spin" /> : undefined}
            onClick={() => void commit()}
            disabled={!canCommit(preview) || busy !== ""}
            aria-busy={busy === "import"}
          >
            {importButtonLabel(preview)}
          </Button>
        </div>
      </div>
    </ModalDialog>
  );
}

function PreviewBody({ preview }: { preview: DisbursementImportPreview }) {
  if (preview.valid && preview.rowCount === 0) {
    return (
      <p className="text-ink-muted m-0 flex items-center gap-2 text-[12.5px]">
        <FileSpreadsheet aria-hidden="true" focusable="false" className="size-4 shrink-0" />
        {NO_VOUCHER}
      </p>
    );
  }
  return <ResultBox ok={preview.valid} heading={previewHeading(preview)} rows={errorRows(preview.errors)} />;
}

/** Spec 05 result box: green or red, the heading, then `Dòng | Cột | Vấn đề` when there are errors. */
function ResultBox({ ok, heading, rows }: { ok: boolean; heading: string; rows: readonly ErrorRow[] }) {
  return (
    <div
      role={ok ? undefined : "alert"}
      data-preview-summary=""
      className={cn(
        "rounded-[10px] border border-solid p-3",
        ok ? "border-leaf/25 bg-leaf/8" : "border-danger/25 bg-danger/8",
      )}
    >
      <p className="m-0 flex items-center gap-2 text-[12.5px] font-semibold">
        {ok ? (
          <CircleCheck aria-hidden="true" focusable="false" className="text-leaf size-4 shrink-0" />
        ) : (
          <TriangleAlert aria-hidden="true" focusable="false" className="text-danger size-4 shrink-0" />
        )}
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
                <tr key={`${r.row}-${i}`} className="border-line border-t">
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
