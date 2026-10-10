"use client";

import { Download, Loader2, Paperclip, Trash2 } from "lucide-react";
import { useEffect, useRef, useState, type FormEvent } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";
import { cn } from "@/lib/cn";
import { removeTaskAttachment } from "@/lib/api/nhiem-vu";

import { attachmentDownloadLink, uploadTaskAttachment } from "@/lib/api/task-attachments";
import type { petitions_taskAttachmentOut } from "@/lib/api/schema.gen";
import type { UploadProgress, UploadResult } from "@/lib/api/upload";

import {
  ATTACH_ACCEPT,
  ATTACH_BUTTON,
  ATTACH_INPUT_LABEL,
  ATTACH_NOTE,
  ATTACH_REMOVE_BUTTON,
  ATTACH_RETRY_BUTTON,
  DOWNLOAD_OPENING,
  DOWNLOAD_REFUSED,
  afterUpload,
  attachmentStateText,
  attachmentTypeLabel,
  declaredType,
  downloadLabel,
  formatBytes,
  type AttachmentItem,
  type AttachmentState,
} from "./task-attachments";
import { INPUT_CLASS, LABEL_CLASS } from "./task-spec";
import { Glyph } from "./task-ui";

/** One file's upload to one record's route: one multipart request (`lib/api/upload.ts`). */
export type SendAttachment = (
  file: File,
  contentType: string,
  idempotencyKey: string,
  progress: UploadProgress,
) => Promise<UploadResult<petitions_taskAttachmentOut>>;

/**
 * The files of ONE log entry being written, each sent as ONE request (ADR 0052 §Sửa đổi 09/10/2026).
 * The entry form owns the list; it sends `storedIds(items)` with the entry and clears the list after a
 * 201. Used by the task log (`useAttachmentUploads`) and the petition log (`usePetitionLogAttachments`)
 * — only the route differs.
 *
 * EACH FILE ON ITS OWN: one refused file does not stop the others, and the list says per file where it
 * stands. Removing a file that is not sent yet only drops it from the list — its id is simply never
 * sent (a stored-but-unsent upload is never attached to anything). The chosen `File` stays in `picked`
 * (browser memory, this form only) so `retry` can send the same bytes again after a refusal of the
 * moment; `retry` acts only on a key still in the list, and `clear` starts a fresh map.
 */
export function useAttachmentUploadList(send: SendAttachment, keyPrefix: string) {
  const [items, setItems] = useState<readonly AttachmentItem[]>([]);
  const seq = useRef(0);
  const picked = useRef(new Map<string, File>());

  const set = (key: string, state: AttachmentState) =>
    setItems((list) => list.map((i) => (i.key === key ? { ...i, state } : i)));

  async function start(key: string, file: File): Promise<void> {
    const t = declaredType(file);
    if (!t.ok) {
      set(key, { kind: "refused", message: t.message });
      return;
    }
    set(key, { kind: "uploading", percent: 0 });
    // One Idempotency-Key per attempt: a retry is a new attempt.
    const r = await send(file, t.contentType, crypto.randomUUID(), {
      onProgress: (percent) => set(key, { kind: "uploading", percent }),
      onSent: () => set(key, { kind: "checking" }),
    });
    // The limits refusal, 422, 403, 503 "chưa cấu hình kho lưu tệp" — the server's sentence verbatim.
    set(key, afterUpload(r));
  }

  function add(chosen: readonly File[]): void {
    for (const f of chosen) {
      seq.current += 1;
      const key = `${keyPrefix}-${seq.current}`;
      picked.current.set(key, f);
      setItems((list) => [...list, { key, name: f.name, size: f.size, state: { kind: "uploading", percent: 0 } }]);
      void start(key, f);
    }
  }

  return {
    items,
    add,
    /** A refusal of the moment: send the same file again — nothing was stored the first time. */
    retry: (key: string) => {
      const it = items.find((i) => i.key === key);
      const f = picked.current.get(key);
      if (it !== undefined && it.state.kind === "retry" && f !== undefined) void start(key, f);
    },
    remove: (key: string) => setItems((list) => list.filter((i) => i.key !== key)),
    clear: () => {
      picked.current = new Map();
      setItems([]);
    },
  };
}

/** The task log's files: `POST /api/v1/tasks/{ma}/attachments`. */
export function useAttachmentUploads(taskCode: string) {
  return useAttachmentUploadList(
    (file, contentType, key, progress) => uploadTaskAttachment(taskCode, file, contentType, key, progress),
    "tep",
  );
}

/** The picker and the per-file list under the entry's text field. Presentational. */
export function AttachmentPicker({
  fieldId,
  items,
  disabled,
  onAdd,
  onRetry,
  onRemove,
}: {
  fieldId: string;
  items: readonly AttachmentItem[];
  /** The entry is being sent: the list is frozen. */
  disabled: boolean;
  onAdd: (files: File[]) => void;
  onRetry: (key: string) => void;
  onRemove: (key: string) => void;
}) {
  const inputId = `${fieldId}-dinh-kem`;
  return (
    <div className="task-attachments">
      {/* The input comes FIRST, visually hidden but in the tab order; its label right after it is the
          visible `📎 Đính kèm` control, ringed while the input has focus (`input:focus-visible + label`).
          Enter/Space on the focused input open the picker. */}
      <input
        id={inputId}
        name={inputId}
        type="file"
        multiple
        accept={ATTACH_ACCEPT}
        className="an-thi-giac"
        disabled={disabled}
        onChange={(e) => {
          const files = e.target.files === null ? [] : Array.from(e.target.files);
          e.target.value = ""; // the same file can be chosen again after a removal
          if (files.length > 0) onAdd(files);
        }}
      />
      <label htmlFor={inputId} className="nut-phu">
        <Glyph icon={Paperclip} className="size-[18px]" />
        {ATTACH_BUTTON}
        <span className="an-thi-giac"> — {ATTACH_INPUT_LABEL}</span>
      </label>
      <p className="ghi-chu">{ATTACH_NOTE}</p>
      {items.length > 0 && (
        <ul aria-label="Tệp đính kèm của dòng nhật ký này">
          {items.map((i) => {
            const refused = i.state.kind === "refused" || i.state.kind === "retry";
            return (
              <li key={i.key}>
                <span>
                  {i.name} · {formatBytes(i.size)}
                </span>
                <span role={refused ? "alert" : "status"}>{attachmentStateText(i.state)}</span>
                {i.state.kind === "retry" && (
                  <button type="button" className="nut-phu" disabled={disabled} onClick={() => onRetry(i.key)}>
                    {ATTACH_RETRY_BUTTON}
                  </button>
                )}
                <button
                  type="button"
                  className="nut-phu"
                  disabled={disabled}
                  aria-label={`${ATTACH_REMOVE_BUTTON} ${i.name}`}
                  onClick={() => onRemove(i.key)}
                >
                  {ATTACH_REMOVE_BUTTON}
                </button>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}

/**
 * The files picked for ONE log entry of the Nhiệm vụ panel — spec 08 (prototype
 * `TaskActivityPanel.tsx:191-215`): small bordered chips under the text box, each with its name and a
 * Trash2 that takes it out of the list. The ADR 0052 states (uploading, scanning, refused, retry) stay
 * as words on the chip — a file must never look attached while it is still being checked.
 *
 * Presentational, like `AttachmentPicker` (which Phản ánh keeps using unchanged): the entry form owns
 * the list and the hidden file input.
 */
export function PickedFileChips({
  items,
  disabled,
  onRetry,
  onRemove,
}: {
  items: readonly AttachmentItem[];
  /** The entry is being sent: the list is frozen. */
  disabled: boolean;
  onRetry: (key: string) => void;
  onRemove: (key: string) => void;
}) {
  if (items.length === 0) return null;
  return (
    <ul className="m-0 mt-1.5 list-none space-y-1 p-0" aria-label="Tệp đính kèm của dòng nhật ký này">
      {items.map((i) => {
        const refused = i.state.kind === "refused" || i.state.kind === "retry";
        const stored = i.state.kind === "stored";
        return (
          <li
            key={i.key}
            className="border-line text-ink-muted flex flex-wrap items-center gap-1.5 rounded-[6px] border bg-white px-2 py-1 text-[11.5px]"
          >
            <Paperclip aria-hidden="true" focusable="false" className="size-3 shrink-0" />
            <span className="min-w-0 flex-1 truncate">{i.name}</span>
            {/* Always present: `status` while it moves, `alert` when refused; visually hidden once stored. */}
            <span role={refused ? "alert" : "status"} className={cn("w-full", refused && "text-danger", stored && "an-thi-giac")}>
              {attachmentStateText(i.state)}
            </span>
            {i.state.kind === "retry" && (
              <button
                type="button"
                className="text-brand cursor-pointer border-0 bg-transparent p-0 text-[11.5px] [font-family:inherit] hover:underline"
                disabled={disabled}
                onClick={() => onRetry(i.key)}
              >
                {ATTACH_RETRY_BUTTON}
              </button>
            )}
            <button
              type="button"
              className="hover:text-danger text-ink-muted cursor-pointer border-0 bg-transparent p-0"
              disabled={disabled}
              aria-label={`Bỏ tệp ${i.name}`}
              onClick={() => onRemove(i.key)}
            >
              <Trash2 aria-hidden="true" focusable="false" className="size-3" />
            </button>
          </li>
        );
      })}
    </ul>
  );
}

/**
 * The files of one timeline row — spec 08 `FileRow` (compact: the row already says who and when):
 * Paperclip, the name, its size, a Download icon, and — for an account that may write in this log
 * (`onRemove` given) — a Trash2 that asks for the MANDATORY reason before
 * `DELETE /api/v1/tasks/{ma}/attachments/{id}` (ADR 0076 #4b, rule 7). The server decides again on
 * the row (uploader or `task.update`) and its refusal is shown verbatim.
 *
 * THE LINK IS ASKED FOR AT THE CLICK AND NEVER KEPT (a bearer credential, ≤ 15 minutes). A blank tab
 * is opened IN the click (a tab opened after an `await` is a popup the browser blocks), detached from
 * this page (`opener = null`), then sent to the link. Blocked anyway → this tab goes to the link.
 */
export function TimelineAttachments({
  taskCode,
  attachments,
  onRemove,
}: {
  taskCode: string;
  attachments: readonly petitions_taskAttachmentOut[];
  /** Ask to remove a file (opens the reason dialog). Absent = no remove control. */
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
    attachmentDownloadLink(taskCode, a.id).then((r) => {
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
    <ul className="m-0 mt-1.5 list-none space-y-1 p-0" aria-label="Tệp đính kèm">
      {attachments.map((a) => (
        <li key={a.id}>
          <div className="border-line flex items-center gap-2 rounded-[8px] border bg-white px-2.5 py-1.5">
            <Paperclip aria-hidden="true" focusable="false" className="text-ink-muted size-3.5 shrink-0" />
            <div className="min-w-0 flex-1">
              <button
                type="button"
                className="text-navy block max-w-full cursor-pointer truncate border-0 bg-transparent p-0 text-left text-[12px] [font-family:inherit] hover:underline"
                title={attachmentTypeLabel(a.mime_type)}
                disabled={busy !== null}
                onClick={() => open(a)}
              >
                {a.file_name}
              </button>
            </div>
            <span className="text-ink-muted shrink-0 text-[10.5px] tabular-nums">{formatBytes(a.size_bytes)}</span>
            <button
              type="button"
              className="text-ink-muted hover:text-navy shrink-0 cursor-pointer border-0 bg-transparent p-0"
              aria-label={downloadLabel(a.file_name)}
              disabled={busy !== null}
              onClick={() => open(a)}
            >
              <Download aria-hidden="true" focusable="false" className="size-3.5" />
            </button>
            {onRemove !== undefined && (
              <button
                type="button"
                className="text-ink-muted hover:text-danger shrink-0 cursor-pointer border-0 bg-transparent p-0"
                aria-label={removeFileLabel(a.file_name)}
                aria-haspopup="dialog"
                disabled={busy !== null}
                onClick={() => onRemove(a)}
              >
                <Trash2 aria-hidden="true" focusable="false" className="size-3.5" />
              </button>
            )}
          </div>
          {busy === a.id && (
            <span role="status" className="text-ink-muted text-[11px]">
              {DOWNLOAD_OPENING}
            </span>
          )}
          {refusal?.id === a.id && (
            <span className="thong-bao-loi text-[11px]" role="alert">
              {DOWNLOAD_REFUSED} {refusal.message}
            </span>
          )}
        </li>
      ))}
    </ul>
  );
}

/** Accessible name of a file's remove control (prototype `TaskActivityPanel.tsx:432`). */
export function removeFileLabel(fileName: string): string {
  return `Gỡ tệp ${fileName}`;
}

export const ATTACHMENT_REMOVED = "Đã gỡ tệp.";
const REMOVE_TITLE_ID = "task-attachment-remove-title";
export const REMOVE_REASON_ID = "ly-do-go-tep";

/**
 * The reason box of a file removal (ADR 0076 #4b): the reason is REQUIRED (rule 7 — the file stays,
 * soft-removed, with who and why). A refusal stays IN the dialog, verbatim, the reason kept; success
 * toasts and tells the caller to read the log again (the row's file list is the server's).
 */
export function AttachmentRemoveDialog({
  taskCode,
  file,
  onClose,
  onRemoved,
}: {
  taskCode: string;
  file: petitions_taskAttachmentOut;
  onClose: () => void;
  onRemoved: () => void;
}) {
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    document.getElementById(REMOVE_REASON_ID)?.focus();
  }, []);

  function submit(e: FormEvent<HTMLFormElement>): void {
    e.preventDefault();
    const text = reason.trim();
    if (text === "" || busy) return;
    setBusy(true);
    setError(null);
    removeTaskAttachment(taskCode, file.id, text).then((r) => {
      setBusy(false);
      if (!r.ok) {
        setError(r.thongBao);
        return;
      }
      toast.success(ATTACHMENT_REMOVED);
      onRemoved();
      onClose();
    });
  }

  return (
    <ModalDialog titleId={REMOVE_TITLE_ID} closeDisabled={busy} onDismiss={() => !busy && onClose()}>
      <ModalDialogHeader
        titleId={REMOVE_TITLE_ID}
        title={`Gỡ tệp “${file.file_name}”?`}
        description="Tệp được gỡ khỏi nhật ký nhưng vẫn lưu lại cùng người gỡ và lý do."
      />
      <form className="m-0 flex flex-col gap-4" onSubmit={submit}>
        <div>
          <label htmlFor={REMOVE_REASON_ID} className={LABEL_CLASS}>
            Lý do gỡ (bắt buộc)
          </label>
          <input
            id={REMOVE_REASON_ID}
            name={REMOVE_REASON_ID}
            value={reason}
            autoComplete="off"
            required
            className={INPUT_CLASS}
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
            className="bg-danger hover:not-disabled:bg-danger/90 text-white"
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
