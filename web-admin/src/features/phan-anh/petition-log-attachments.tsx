"use client";

import { Paperclip } from "lucide-react";
import { useRef, useState } from "react";

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
import {
  completeLogAttachment,
  logAttachmentDownloadLink,
  requestLogAttachmentUpload,
} from "@/lib/api/phieu-phan-anh";
import type {
  petitions_taskAttachmentDownloadOut,
  petitions_taskAttachmentOut,
  petitions_taskAttachmentUploadIn,
  petitions_taskAttachmentUploadOut,
} from "@/lib/api/schema.gen";
import { uploadToStorage, type CallResult } from "@/lib/api/task-attachments";

import { buttonClass, Glyph } from "./petition-ui";

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
}: {
  lookupCode: string;
  attachments: readonly petitions_taskAttachmentOut[];
  /** Injected only by tests. */
  download?: (lookupCode: string, id: string) => Promise<CallResult<petitions_taskAttachmentDownloadOut>>;
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
