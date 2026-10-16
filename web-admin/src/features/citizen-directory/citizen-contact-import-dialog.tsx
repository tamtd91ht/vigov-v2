"use client";

import { useRef, useState } from "react";
import { toast } from "sonner";

import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";
import { saveFile } from "@/features/nhiem-vu/save-file";
import { IMPORT_DESCRIPTION, IMPORT_ROWS_STILL_WRONG, canCommitImport, importFileProblem, importKeyFor } from "@/features/nhiem-vu/task-import";
import { TaskImportView, type ImportViewWords } from "@/features/nhiem-vu/task-import-dialog";
import {
  CITIZEN_CONTACT_TEMPLATE_FILE_NAME,
  commitCitizenContactImport,
  downloadCitizenContactImportTemplate,
  previewCitizenContactImport,
} from "@/lib/api/citizen-contact-import";

import {
  IMPORT_FILE_INPUT_ID,
  IMPORT_TEMPLATE_BUTTON,
  IMPORT_TITLE,
  IMPORT_TITLE_ID,
  importButtonLabel,
  importPreviewHeading,
  importPreviewReport,
  importedToast,
  type ImportReport,
} from "./citizen-directory-words";

const WORDS: ImportViewWords = {
  templateButton: IMPORT_TEMPLATE_BUTTON,
  submitLabel: importButtonLabel,
  heading: importPreviewHeading,
  fileInputId: IMPORT_FILE_INPUT_ID,
};

/**
 * "Nhập danh bạ người dân từ Excel" (chapter 16 §7, ADR 0097 D3/D13) — the shared import dialog as the
 * Nhiệm vụ and Đơn thư screens draw it (`TaskImportView`): template link, choose a file, the check runs AT
 * ONCE (`import-previews`, writes nothing), the result box with `Dòng | Cột | Vấn đề`, then
 * `[Đóng] [Nhập N người dân]`, enabled only for a clean check of THIS file. ALL OR NOTHING is the
 * server's rule (§7); the dialog shows its answer.
 *
 * ONE KEY PER ATTEMPT: minted at the first "Nhập", kept across retries of the same file (a retry after a
 * lost answer replays), dropped when another file is chosen. Only a `citizen.update` holder gets the
 * button (the header gates it); all three routes check again (rule 5). The file and the errors are never
 * logged (rule 3) — an error names a row and a column, never a number.
 */
export function CitizenContactImportDialog({
  onClose,
  onImported,
}: {
  onClose: () => void;
  /** 201 (or a replay): the list, its count, the tags and the groups' member counts must be read again. */
  onImported: () => void;
}) {
  const [fileName, setFileName] = useState<string | null>(null);
  const [file, setFile] = useState<File | null>(null);
  const [problem, setProblem] = useState<string | null>(null);
  const [report, setReport] = useState<ImportReport | null>(null);
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
    const r = await previewCitizenContactImport(f, f.name);
    if (choice.current !== mine) return;
    setBusy("");
    if (!r.ok) {
      setProblem(r.thongBao);
      return;
    }
    const read = importPreviewReport(r.duLieu);
    if ("problem" in read) setProblem(read.problem);
    else setReport(read.report);
  }

  async function commit() {
    if (file === null || !canCommitImport(report) || busy !== "") return;
    const key = importKeyFor(importKey, () => crypto.randomUUID());
    setImportKey(key);
    setBusy("import");
    const r = await commitCitizenContactImport(file, file.name, key);
    setBusy("");
    if (r.ok) {
      setImportKey(null);
      toast.success(importedToast(r.rowCount));
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
    // Any other refusal (409 the directory changed, 503, network): the server's sentence. The key is
    // KEPT — the send may have reached the server, and a retry must be recognised as the same attempt.
    toast.error(r.message);
  }

  async function template() {
    setBusy("template");
    const r = await downloadCitizenContactImportTemplate();
    setBusy("");
    if (!r.ok) {
      toast.error(r.thongBao);
      return;
    }
    saveFile(r.duLieu, CITIZEN_CONTACT_TEMPLATE_FILE_NAME);
  }

  return (
    <ModalDialog titleId={IMPORT_TITLE_ID} className="max-w-[42rem]" closeDisabled={busy === "import"} onDismiss={() => busy !== "import" && onClose()}>
      <ModalDialogHeader titleId={IMPORT_TITLE_ID} title={IMPORT_TITLE} description={IMPORT_DESCRIPTION} />
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
