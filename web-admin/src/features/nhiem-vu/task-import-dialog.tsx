"use client";

import { Download } from "lucide-react";
import { useEffect, useRef, useState, type Ref } from "react";

import {
  TASK_TEMPLATE_FILE_NAME,
  downloadTaskImportTemplate,
  submitTaskImport,
  type TaskImportOutcome,
} from "@/lib/api/task-import";

import { saveFile } from "./save-file";
import {
  IMPORT_CHECK_BUTTON,
  IMPORT_CHECK_NOTE,
  IMPORT_CHECKING,
  IMPORT_CLOSE_BUTTON,
  IMPORT_DEADLINE_NOTE,
  IMPORT_DESCRIPTION,
  IMPORT_DROP_HINT,
  IMPORT_FILE_LABEL,
  IMPORT_NOT_COMMITTED,
  IMPORT_SENDING,
  IMPORT_SUBMIT_BUTTON,
  IMPORT_TEMPLATE_BUTTON,
  IMPORT_TITLE,
  checkedOkText,
  importErrorLine,
  importFileProblem,
  importKeyFor,
  importedText,
  refusedHeading,
  replayedText,
  sortedImportErrors,
} from "./task-import";
import { Glyph } from "./task-ui";

/** The last answer on screen: which mode sent it, and what came back (or the refusal sentence). */
export type ImportAnswer =
  | { readonly dryRun: boolean; readonly ok: true; readonly outcome: TaskImportOutcome }
  | { readonly dryRun: boolean; readonly ok: false; readonly message: string };

/**
 * `⬆ Nhập từ Excel` (§8) — "Modal tương tự mẫu chung: mô tả → `⬇ Tải mẫu nhiệm vụ` → vùng kéo thả
 * `Chọn tệp .xlsx` → `Đóng` / `Nhập`".
 *
 * AN IN-PAGE DIALOG (`role="dialog"`), the repo's precedent (`features/thu-chi/dot-thu-chi.tsx`), not a
 * focus-trapping modal: focus moves into it on open and back to the opening button on `Đóng`.
 *
 * Only a `task.create` holder gets the button (the page gates it); the route checks again.
 */
export function TaskImportDialog({
  onClose,
  onImported,
}: {
  onClose: () => void;
  /** 201 (or a replayed 201): the register must be read again — the new rows are not spliced in. */
  onImported: () => void;
}) {
  const [file, setFile] = useState<File | null>(null);
  const [fileProblem, setFileProblem] = useState<string | null>(null);
  // The REAL import's key — kept across retries of THIS file, dropped on a new file or a success.
  const [importKey, setImportKey] = useState<string | null>(null);
  const [sending, setSending] = useState<"check" | "import" | null>(null);
  const [answer, setAnswer] = useState<ImportAnswer | null>(null);
  const [templateBusy, setTemplateBusy] = useState(false);
  const [templateError, setTemplateError] = useState<string | null>(null);
  const titleRef = useRef<HTMLHeadingElement>(null);

  useEffect(() => {
    titleRef.current?.focus();
  }, []);

  function choose(f: File | null) {
    setAnswer(null);
    setImportKey(null);
    setFile(f);
    setFileProblem(f === null ? null : importFileProblem(f));
  }

  function send(dryRun: boolean) {
    if (file === null || fileProblem !== null || sending !== null) return;
    const key = dryRun ? crypto.randomUUID() : importKeyFor(importKey, () => crypto.randomUUID());
    if (!dryRun) setImportKey(key);
    setSending(dryRun ? "check" : "import");
    setAnswer(null);
    submitTaskImport(file, file.name, key, dryRun).then((r) => {
      setSending(null);
      if (!r.ok) {
        setAnswer({ dryRun, ok: false, message: r.thongBao });
        return;
      }
      setAnswer({ dryRun, ok: true, outcome: r.duLieu });
      if (r.duLieu.kind === "imported" || r.duLieu.kind === "replayed") {
        // Done with this file: a later "Nhập" of it would be a NEW import under a new key.
        setImportKey(null);
        setFile(null);
        onImported();
      }
    });
  }

  function template() {
    setTemplateBusy(true);
    setTemplateError(null);
    downloadTaskImportTemplate().then((r) => {
      setTemplateBusy(false);
      if (!r.ok) {
        setTemplateError(r.thongBao);
        return;
      }
      saveFile(r.duLieu, TASK_TEMPLATE_FILE_NAME);
    });
  }

  return (
    <TaskImportView
      titleRef={titleRef}
      fileName={file?.name ?? null}
      fileProblem={fileProblem}
      sending={sending}
      answer={answer}
      templateBusy={templateBusy}
      templateError={templateError}
      onTemplate={template}
      onChoose={choose}
      onCheck={() => send(true)}
      onImport={() => send(false)}
      onClose={onClose}
    />
  );
}

/** The drawing, without state — rendered by the tests with `renderToStaticMarkup`. */
export function TaskImportView({
  titleRef,
  fileName,
  fileProblem,
  sending,
  answer,
  templateBusy,
  templateError,
  onTemplate,
  onChoose,
  onCheck,
  onImport,
  onClose,
}: {
  titleRef?: Ref<HTMLHeadingElement>;
  fileName: string | null;
  fileProblem: string | null;
  sending: "check" | "import" | null;
  answer: ImportAnswer | null;
  templateBusy: boolean;
  templateError: string | null;
  onTemplate: () => void;
  onChoose: (f: File | null) => void;
  onCheck: () => void;
  onImport: () => void;
  onClose: () => void;
}) {
  const canSend = fileName !== null && fileProblem === null && sending === null;

  return (
    <section className="khoi-chi-tiet" role="dialog" aria-labelledby="tieu-de-nhap-excel">
      <div className="dau-khoi-chi-tiet">
        <h3 id="tieu-de-nhap-excel" ref={titleRef} tabIndex={-1}>
          {IMPORT_TITLE}
        </h3>
      </div>
      <p>{IMPORT_DESCRIPTION}</p>
      <p className="ghi-chu">{IMPORT_DEADLINE_NOTE}</p>

      <div className="cum-nut">
        <button type="button" className="nut-phu" disabled={templateBusy} onClick={onTemplate}>
          <Glyph icon={Download} className="size-[18px]" />
          {IMPORT_TEMPLATE_BUTTON}
        </button>
      </div>
      {templateError !== null && (
        <p className="thong-bao-loi" role="alert">
          {templateError}
        </p>
      )}

      {/* THE DROP ZONE IS THE LABEL OF THE FILE INPUT: a click, the keyboard and a drop all reach the
          same `onChoose`. Dragging is an extra, never the only way (a11y). */}
      <label
        htmlFor="tep-nhap-nhiem-vu"
        className="o-nhap"
        onDragOver={(e) => e.preventDefault()}
        onDrop={(e) => {
          e.preventDefault();
          onChoose(e.dataTransfer.files.item(0));
        }}
      >
        {IMPORT_FILE_LABEL} <span className="dong-phu">{IMPORT_DROP_HINT}</span>
        <input
          id="tep-nhap-nhiem-vu"
          name="tep-nhap-nhiem-vu"
          type="file"
          accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
          disabled={sending !== null}
          onChange={(e) => onChoose(e.target.files?.item(0) ?? null)}
        />
      </label>
      {fileName !== null && <p className="ghi-chu">Tệp đã chọn: {fileName}</p>}
      {fileProblem !== null && (
        <p className="thong-bao-loi" role="alert">
          {fileProblem}
        </p>
      )}
      <p className="ghi-chu">{IMPORT_CHECK_NOTE}</p>

      {/* Always in the DOM while the dialog is: a live region inserted later is not always announced. */}
      <p role="status">
        {sending === "check" ? IMPORT_CHECKING : sending === "import" ? IMPORT_SENDING : ""}
      </p>
      {answer !== null && sending === null && <ImportAnswerView answer={answer} />}

      <div className="cum-nut">
        <button type="button" className="nut-phu" onClick={onClose} disabled={sending !== null}>
          {IMPORT_CLOSE_BUTTON}
        </button>
        <button type="button" className="nut-phu" disabled={!canSend} onClick={onCheck}>
          {IMPORT_CHECK_BUTTON}
        </button>
        <button type="button" className="nut-chinh" disabled={!canSend} onClick={onImport}>
          {IMPORT_SUBMIT_BUTTON}
        </button>
      </div>
    </section>
  );
}

/** The report: refusal sentence, row refusals, a clean check, the issued codes, or a replay. */
export function ImportAnswerView({ answer }: { answer: ImportAnswer }) {
  if (!answer.ok) {
    return (
      <p className="thong-bao-loi" role="alert">
        {answer.message}
      </p>
    );
  }
  const o = answer.outcome;
  if (o.kind === "replayed") return <p role="alert">{replayedText(o.firstCode)}</p>;
  if (o.kind === "imported") return <p role="alert">{importedText(o.report.codes)}</p>;
  if (o.report.errors.length === 0) {
    // A REAL send answered 200 with no refusal: nothing was written, and "bấm Nhập" would be wrong.
    return <p role="alert">{answer.dryRun ? checkedOkText(o.report.total_rows) : IMPORT_NOT_COMMITTED}</p>;
  }
  return (
    <div role="alert">
      <p className="thong-bao-loi">{refusedHeading(o.report.errors.length, answer.dryRun)}</p>
      <ul>
        {sortedImportErrors(o.report.errors).map((e, i) => (
          <li key={`${e.row}-${e.column}-${i}`}>{importErrorLine(e)}</li>
        ))}
      </ul>
    </div>
  );
}
