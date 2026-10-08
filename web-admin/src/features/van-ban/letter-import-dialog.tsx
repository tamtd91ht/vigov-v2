"use client";

import { useRef, useState } from "react";
import { toast } from "sonner";

import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";
import { saveFile } from "@/features/nhiem-vu/save-file";
import {
  IMPORT_DESCRIPTION,
  IMPORT_ROWS_STILL_WRONG,
  canCommitImport,
  importFileProblem,
  importKeyFor,
} from "@/features/nhiem-vu/task-import";
import { TaskImportView, type ImportViewWords } from "@/features/nhiem-vu/task-import-dialog";
import {
  LETTER_IMPORT_ROUTES,
  LETTER_ROWS_FIELD,
  LETTER_TEMPLATE_FILE_NAME,
  type LetterImportRow,
} from "@/lib/api/citizen-letter-import";
import { commitImport, downloadImportTemplate, previewImport } from "@/lib/api/excel-import";

import {
  LETTER_IMPORT_FILE_INPUT_ID,
  LETTER_IMPORT_TEMPLATE_BUTTON,
  LETTER_IMPORT_TITLE,
  letterImportButtonLabel,
  letterImportedToast,
  letterPreviewHeading,
  previewReport,
  type PreviewReport,
} from "./letter-import";

export const LETTER_IMPORT_TITLE_ID = "tieu-de-nhap-don-thu";

const WORDS: ImportViewWords = {
  templateButton: LETTER_IMPORT_TEMPLATE_BUTTON,
  submitLabel: letterImportButtonLabel,
  heading: letterPreviewHeading,
  fileInputId: LETTER_IMPORT_FILE_INPUT_ID,
};

/**
 * "Nhập sổ đơn thư từ Excel" — the prototype's shared `ExcelImportDialog` (`components/shared/
 * ExcelImportDialog.tsx`) as the Nhiệm vụ screen already draws it (`TaskImportView`, same markup, the
 * letter register's words). The flow is that dialog's: template link, choose a file, the check runs AT
 * ONCE (`import-previews`, writes nothing), the result box with `Dòng | Cột | Vấn đề`, then
 * `[Đóng] [Nhập N đơn thư]`, enabled only for a clean check of THIS file.
 *
 * ONE KEY PER ATTEMPT: minted at the first "Nhập", kept across retries of the same file (a retry after a
 * lost answer replays; numbers issued are never reissued, rule 7), dropped when another file is chosen.
 *
 * Only a `petition.create` holder gets the button (the register's header gates it); all three routes
 * check again (rule 5). The file and the preview rows are never logged (rule 3).
 */
export function LetterImportDialog({
  onClose,
  onImported,
}: {
  onClose: () => void;
  /** 201 (or a replay): the register and its tab count must be read again. */
  onImported: () => void;
}) {
  const [fileName, setFileName] = useState<string | null>(null);
  const [file, setFile] = useState<File | null>(null);
  const [problem, setProblem] = useState<string | null>(null);
  const [report, setReport] = useState<PreviewReport | null>(null);
  const [importKey, setImportKey] = useState<string | null>(null);
  const [busy, setBusy] = useState<"" | "template" | "check" | "import">("");
  // A check answering after ANOTHER file was chosen is dropped (see `TaskImportDialog`).
  const choice = useRef(0);

  async function choose(f: File | null) {
    if (f === null || busy === "import") return;
    const mine = ++choice.current;
    setFile(f);
    setFileName(f.name);
    setReport(null);
    setImportKey(null);
    const local = importFileProblem(f);
    setProblem(local);
    if (local !== null) return;
    setBusy("check");
    const r = await previewImport<LetterImportRow>(LETTER_IMPORT_ROUTES, LETTER_ROWS_FIELD, f, f.name);
    if (choice.current !== mine) return;
    setBusy("");
    if (!r.ok) {
      setProblem(r.thongBao);
      return;
    }
    const read = previewReport(r.duLieu);
    if ("problem" in read) setProblem(read.problem);
    else setReport(read.report);
  }

  async function commit() {
    if (file === null || !canCommitImport(report) || busy !== "") return;
    const key = importKeyFor(importKey, () => crypto.randomUUID());
    setImportKey(key);
    setBusy("import");
    const r = await commitImport<LetterImportRow>(LETTER_IMPORT_ROUTES, file, file.name, key);
    setBusy("");
    if (r.ok) {
      setImportKey(null);
      toast.success(letterImportedToast(r.created));
      onImported();
      onClose();
      return;
    }
    if (r.errors.length > 0) {
      // 400 import_invalid: nothing written. The rows replace the clean check, which no longer holds.
      setReport({ total_rows: 0, created: 0, committed: false, errors: [...r.errors], codes: [] });
      toast.error(IMPORT_ROWS_STILL_WRONG);
      return;
    }
    // Any other refusal (409 numbering, 503, network): the server's sentence. The key is KEPT — the
    // send may have reached the server, and a retry must be recognised as the same attempt.
    toast.error(r.message);
  }

  async function template() {
    setBusy("template");
    const r = await downloadImportTemplate(LETTER_IMPORT_ROUTES);
    setBusy("");
    if (!r.ok) {
      toast.error(r.thongBao);
      return;
    }
    saveFile(r.duLieu, LETTER_TEMPLATE_FILE_NAME);
  }

  return (
    <ModalDialog
      titleId={LETTER_IMPORT_TITLE_ID}
      className="max-w-[42rem]"
      closeDisabled={busy === "import"}
      onDismiss={() => busy !== "import" && onClose()}
    >
      <ModalDialogHeader titleId={LETTER_IMPORT_TITLE_ID} title={LETTER_IMPORT_TITLE} description={IMPORT_DESCRIPTION} />
      <TaskImportView
        words={WORDS}
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
