"use client";

import { Loader2, Paperclip, Trash2 } from "lucide-react";
import { useRef, useState, type FormEvent } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";
import {
  afterCompletion,
  attachmentTypeLabel,
  declaredType,
  DOWNLOAD_OPENING,
  DOWNLOAD_REFUSED,
  downloadLabel,
  formatBytes,
  type AttachmentItem,
  type AttachmentState,
} from "@/features/nhiem-vu/task-attachments";
import type { KetQua } from "@/lib/api/goi";
import {
  completeLogAttachment,
  logAttachmentDownloadLink,
  removeLogAttachment,
  requestLogAttachmentUpload,
} from "@/lib/api/phieu-phan-anh";
import type {
  petitions_taskAttachmentDownloadOut,
  petitions_taskAttachmentOut,
  petitions_taskAttachmentUploadIn,
  petitions_taskAttachmentUploadOut,
} from "@/lib/api/schema.gen";
import { uploadToStorage, type CallResult } from "@/lib/api/task-attachments";

import {
  LOG_FILE_REMOVE_NOTE,
  LOG_FILE_REMOVE_REASON_LABEL,
  LOG_FILE_REMOVED_TOAST,
  logFileRemoveLabel,
  logFileRemoveTitle,
} from "./nhan-phieu";
import { buttonClass, Glyph, LABEL_CLASS } from "./petition-ui";

/**
 * `📎 Đính kèm` on the PETITION log (§8.7) — the task log's attachment flow (ADR 0052), on the
 * petition's own routes (`…/citizen-reports/{maTraCuu}/log-attachments…`).
 *
 * REUSED, NOT COPIED: the pure half — the pre-check (`declaredType`, PDF / JPG / PNG, the platform's
 * `petition-log-attachment` policy), the per-file state, `afterCompletion`, `storedIds`, the words —
 * and the presentational picker (`AttachmentPicker`) come from `features/nhiem-vu/task-attachments*`;
 * the upload to the store is `uploadToStorage`. What is HERE is only what is bound to a route: the
 * hook's three calls and the download button's one call. The task versions take a task code and call
 * the task routes, so they cannot be pointed at a petition without changing that feature.
 *
 * THE DOWNLOAD IS AUDITED on the server (a file on a petition's log is evidence about one citizen's
 * case): the link is asked for AT THE CLICK, never prefetched, used at once, never kept.
 */

export type LogAttachmentDeps = {
  request: (
    lookupCode: string,
    body: petitions_taskAttachmentUploadIn,
    key: string,
  ) => Promise<CallResult<petitions_taskAttachmentUploadOut>>;
  upload: typeof uploadToStorage;
  complete: (lookupCode: string, id: string) => Promise<CallResult<petitions_taskAttachmentOut>>;
};

const DEFAULT_DEPS: LogAttachmentDeps = {
  request: requestLogAttachmentUpload,
  upload: uploadToStorage,
  complete: completeLogAttachment,
};

/**
 * The files of ONE log entry being written. The entry form owns the list: it sends `storedIds(items)`
 * with the entry and clears the list after a 201. Each file on its own — one refusal does not stop the
 * others; removing an unsent file only drops it (its id is simply never sent).
 */
export function usePetitionLogAttachments(lookupCode: string, deps: LogAttachmentDeps = DEFAULT_DEPS) {
  const [items, setItems] = useState<readonly AttachmentItem[]>([]);
  const seq = useRef(0);

  const set = (key: string, state: AttachmentState) =>
    setItems((list) => list.map((i) => (i.key === key ? { ...i, state } : i)));

  async function complete(key: string, id: string): Promise<void> {
    set(key, { kind: "checking", id });
    set(key, afterCompletion(id, await deps.complete(lookupCode, id)));
  }

  async function start(key: string, file: File): Promise<void> {
    const t = declaredType(file);
    if (!t.ok) {
      set(key, { kind: "refused", message: t.message });
      return;
    }
    // a. declare — ONE key per attempt (a replay cannot carry the form again).
    const req = await deps.request(
      lookupCode,
      { file_name: file.name, content_type: t.contentType, size: file.size },
      crypto.randomUUID(),
    );
    if (!req.ok) {
      // The note rule's 403, the limits, 503 "chưa cấu hình kho lưu tệp" — the server's sentence.
      set(key, { kind: "refused", message: req.message });
      return;
    }
    const id = req.data.attachment.id;
    // b. the bytes, straight to the store. The form is used here and dropped.
    set(key, { kind: "uploading", id, percent: 0 });
    const up = await deps.upload(req.data.upload, file, file.name, (percent) =>
      set(key, { kind: "uploading", id, percent }),
    );
    if (!up.ok) {
      set(key, { kind: "refused", message: up.message });
      return;
    }
    // c. complete.
    await complete(key, id);
  }

  function add(files: readonly File[]): void {
    for (const f of files) {
      seq.current += 1;
      const key = `tep-phieu-${seq.current}`;
      setItems((list) => [...list, { key, name: f.name, size: f.size, state: { kind: "requesting" } }]);
      void start(key, f);
    }
  }

  return {
    items,
    add,
    /** 503 / 409 at completion: ask again — the bytes are already in the store. */
    retry: (key: string) => {
      const it = items.find((i) => i.key === key);
      if (it !== undefined && it.state.kind === "retry") void complete(key, it.state.id);
    },
    remove: (key: string) => setItems((list) => list.filter((i) => i.key !== key)),
    clear: () => setItems([]),
  };
}

/**
 * The files of one timeline row: name, size, type, and a download button. A blank tab is opened IN the
 * click (a tab opened after an `await` is a popup the browser blocks), detached (`opener = null`), then
 * sent to the link. Blocked anyway → this tab goes to the link.
 */
export function PetitionLogAttachmentList({
  lookupCode,
  attachments,
  download = logAttachmentDownloadLink,
  onRemove,
}: {
  lookupCode: string;
  attachments: readonly petitions_taskAttachmentOut[];
  /** Injected only by tests. */
  download?: (lookupCode: string, id: string) => Promise<CallResult<petitions_taskAttachmentDownloadOut>>;
  /**
   * Given only for an account that may remove THIS row's files (the uploader, or `feedback.resolve` —
   * the caller decides, `NhatKyPhieu`). Absent = no remove control at all. UX only: the server decides
   * again on the row and its refusal is shown verbatim (rule 5).
   */
  onRemove?: (file: petitions_taskAttachmentOut) => void;
}) {
  const [busy, setBusy] = useState<string | null>(null);
  const [refusal, setRefusal] = useState<{ id: string; message: string } | null>(null);
  if (attachments.length === 0) return null;

  function open(a: petitions_taskAttachmentOut) {
    const tab = window.open("", "_blank");
    if (tab !== null) tab.opener = null;
    setBusy(a.id);
    setRefusal(null);
    download(lookupCode, a.id).then((r) => {
      setBusy(null);
      if (!r.ok) {
        tab?.close();
        setRefusal({ id: a.id, message: r.message });
        return;
      }
      if (tab !== null) tab.location.href = r.data.url;
      else window.location.assign(r.data.url);
    });
  }

  return (
    <ul className="mt-1.5 mb-0 flex list-none flex-col gap-1 p-0 text-[13px]" aria-label="Tệp đính kèm">
      {attachments.map((a) => (
        <li key={a.id} className="flex flex-wrap items-center gap-x-2 gap-y-1">
          <span className="min-w-0 break-all">
            <Glyph icon={Paperclip} className="mr-1 inline size-3.5 align-[-2px]" />
            {a.file_name} · {formatBytes(a.size_bytes)} · {attachmentTypeLabel(a.mime_type)}
          </span>
          <button
            type="button"
            className={buttonClass("secondary", "sm")}
            aria-label={downloadLabel(a.file_name)}
            disabled={busy !== null}
            onClick={() => open(a)}
          >
            Tải về
          </button>
          {/* The prototype's remove button (`FeedbackActivityPanel.tsx:365-374`). It opens the reason
              dialog (ADR 0088 §2: soft removal, reason required) — never the prototype's bare confirm. */}
          {onRemove !== undefined && (
            <button
              type="button"
              aria-label={logFileRemoveLabel(a.file_name)}
              aria-haspopup="dialog"
              title={logFileRemoveLabel(a.file_name)}
              className="grid size-7 cursor-pointer place-items-center rounded border-0 bg-transparent text-ink-muted hover:text-danger focus-visible:outline-2 focus-visible:outline-brand-500"
              onClick={() => onRemove(a)}
            >
              <Glyph icon={Trash2} className="size-3.5" />
            </button>
          )}
          {busy === a.id && <span role="status">{DOWNLOAD_OPENING}</span>}
          {refusal?.id === a.id && (
            <span className="thong-bao-loi" role="alert">
              {DOWNLOAD_REFUSED} {refusal.message}
            </span>
          )}
        </li>
      ))}
    </ul>
  );
}

const REMOVE_TITLE_ID = "petition-attachment-remove-title";
export const REMOVE_REASON_ID = "ly-do-go-tep-phieu";

/**
 * The reason box of a log-file removal (ADR 0088 §2, the task twin's ADR 0076 §4b shape): the reason is
 * REQUIRED (rule 7 — the file stays, soft-removed, with who and why). A refusal (403, 409 `legal_hold`)
 * stays IN the dialog, verbatim, the reason kept; success toasts and tells the caller to read the log
 * again (a row's file list is the server's). The reason travels in the DELETE body only.
 */
export function LogAttachmentRemoveDialog({
  lookupCode,
  file,
  onClose,
  onRemoved,
  remove = removeLogAttachment,
}: {
  lookupCode: string;
  file: petitions_taskAttachmentOut;
  onClose: () => void;
  onRemoved: () => void;
  /** Injected only by tests. */
  remove?: (lookupCode: string, id: string, reason: string) => Promise<KetQua<void>>;
}) {
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  function submit(e: FormEvent<HTMLFormElement>): void {
    e.preventDefault();
    const text = reason.trim();
    if (text === "" || busy) return;
    setBusy(true);
    setError(null);
    remove(lookupCode, file.id, text).then((r) => {
      setBusy(false);
      if (!r.ok) {
        setError(r.thongBao);
        return;
      }
      toast.success(LOG_FILE_REMOVED_TOAST);
      onRemoved();
      onClose();
    });
  }

  return (
    <ModalDialog
      titleId={REMOVE_TITLE_ID}
      closeDisabled={busy}
      initialFocusId={REMOVE_REASON_ID}
      onDismiss={() => !busy && onClose()}
    >
      <ModalDialogHeader titleId={REMOVE_TITLE_ID} title={logFileRemoveTitle(file.file_name)} description={LOG_FILE_REMOVE_NOTE} />
      <form className="m-0 flex flex-col gap-4" onSubmit={submit}>
        <div>
          <label htmlFor={REMOVE_REASON_ID} className={LABEL_CLASS}>
            {LOG_FILE_REMOVE_REASON_LABEL}
          </label>
          <input
            id={REMOVE_REASON_ID}
            name={REMOVE_REASON_ID}
            value={reason}
            autoComplete="off"
            required
            className="block h-10 w-full min-w-0 rounded-control border border-line-strong bg-surface px-3 [font-family:inherit] text-base text-ink-900 md:text-sm"
            onChange={(e) => setReason(e.target.value)}
          />
        </div>
        {error !== null && (
          <p className="thong-bao-loi m-0" role="alert">
            {error}
          </p>
        )}
        <div className="flex flex-wrap justify-end gap-2">
          <Button type="button" variant="outline" disabled={busy} onClick={onClose}>
            Huỷ
          </Button>
          <Button
            type="submit"
            variant="primary"
            className="bg-danger text-white hover:not-disabled:bg-danger/90"
            icon={
              busy ? (
                <Loader2 aria-hidden="true" focusable="false" className="size-4 animate-spin" />
              ) : (
                <Trash2 aria-hidden="true" focusable="false" className="size-4" />
              )
            }
            disabled={busy || reason.trim() === ""}
          >
            Gỡ tệp
          </Button>
        </div>
      </form>
    </ModalDialog>
  );
}
