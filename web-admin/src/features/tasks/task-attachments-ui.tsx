"use client";

import { useRef, useState } from "react";

import {
  attachmentDownloadLink,
  completeAttachment,
  requestAttachmentUpload,
  uploadToStorage,
} from "@/lib/api/task-attachments";
import type { petitions_taskAttachmentOut } from "@/lib/api/schema.gen";

import {
  ATTACH_ACCEPT,
  ATTACH_BUTTON,
  ATTACH_INPUT_LABEL,
  ATTACH_NOTE,
  ATTACH_REMOVE_BUTTON,
  ATTACH_RETRY_BUTTON,
  DOWNLOAD_OPENING,
  DOWNLOAD_REFUSED,
  afterCompletion,
  attachmentStateText,
  attachmentTypeLabel,
  declaredType,
  downloadLabel,
  formatBytes,
  type AttachmentItem,
  type AttachmentState,
} from "./task-attachments";

/**
 * The files of ONE log entry being written, and the ADR 0052 flow for each (a → b → c). The entry
 * form owns the list; it sends `storedIds(items)` with the entry and clears the list after a 201.
 *
 * EACH FILE ON ITS OWN: one refused file does not stop the others, and the list says per file where it
 * stands. Removing a file that is not sent yet only drops it from the list — its id is simply never
 * sent (a stored-but-unsent upload is never attached to anything).
 */
export function useAttachmentUploads(taskCode: string) {
  const [items, setItems] = useState<readonly AttachmentItem[]>([]);
  const seq = useRef(0);

  const set = (key: string, state: AttachmentState) =>
    setItems((list) => list.map((i) => (i.key === key ? { ...i, state } : i)));

  async function complete(key: string, id: string): Promise<void> {
    set(key, { kind: "checking", id });
    set(key, afterCompletion(id, await completeAttachment(taskCode, id)));
  }

  async function start(key: string, file: File): Promise<void> {
    const t = declaredType(file);
    if (!t.ok) {
      set(key, { kind: "refused", message: t.message });
      return;
    }
    // a. declare — one key per attempt (see `requestAttachmentUpload`).
    const req = await requestAttachmentUpload(
      taskCode,
      { file_name: file.name, content_type: t.contentType, size: file.size },
      crypto.randomUUID(),
    );
    if (!req.ok) {
      // The limits refusal, 503 "chưa cấu hình kho lưu tệp", 403 — the server's sentence verbatim.
      set(key, { kind: "refused", message: req.message });
      return;
    }
    const id = req.data.attachment.id;
    // b. the bytes, straight to the store. The form (`req.data.upload`) is used here and dropped.
    set(key, { kind: "uploading", id, percent: 0 });
    const up = await uploadToStorage(req.data.upload, file, file.name, (percent) =>
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
      const key = `tep-${seq.current}`;
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
 * The files of one timeline row: name, size, type, and a download button.
 *
 * THE LINK IS ASKED FOR AT THE CLICK AND NEVER KEPT (a bearer credential, ≤ 15 minutes). A blank tab
 * is opened IN the click (a tab opened after an `await` is a popup the browser blocks), detached from
 * this page (`opener = null`), then sent to the link. Blocked anyway → this tab goes to the link.
 */
export function TimelineAttachments({
  taskCode,
  attachments,
}: {
  taskCode: string;
  attachments: readonly petitions_taskAttachmentOut[];
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
    <ul className="task-attachments" aria-label="Tệp đính kèm">
      {attachments.map((a) => (
        <li key={a.id}>
          <span>
            📎 {a.file_name} · {formatBytes(a.size_bytes)} · {attachmentTypeLabel(a.mime_type)}
          </span>
          <button
            type="button"
            className="nut-phu"
            aria-label={downloadLabel(a.file_name)}
            disabled={busy !== null}
            onClick={() => open(a)}
          >
            Tải về
          </button>
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
