"use client";

import { Loader2, Paperclip, Trash2 } from "lucide-react";
import { useId, useState, type FormEvent } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";
import {
  attachmentTypeLabel,
  DOWNLOAD_OPENING,
  DOWNLOAD_REFUSED,
  downloadLabel,
  formatBytes,
} from "@/features/nhiem-vu/task-attachments";
import { useAttachmentUploadList, type SendAttachment } from "@/features/nhiem-vu/task-attachments-ui";
import type { KetQua } from "@/lib/api/goi";
import { logAttachmentDownloadLink, removeLogAttachment, uploadLogAttachment } from "@/lib/api/phieu-phan-anh";
import type { petitions_taskAttachmentDownloadOut, petitions_taskAttachmentOut } from "@/lib/api/schema.gen";
import type { CallResult } from "@/lib/api/task-attachments";

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
 * `petition-log-attachment` policy), the per-file state, `afterUpload`, `storedIds`, the words — the
 * per-file hook (`useAttachmentUploadList`) and the presentational picker (`AttachmentPicker`) come from
 * `features/nhiem-vu/task-attachments*`. What is HERE is only what is bound to a route: the upload
 * call (one multipart request, ADR 0052 §Sửa đổi 09/10/2026) and the download button's one call.
 *
 * THE DOWNLOAD IS AUDITED on the server (a file on a petition's log is evidence about one citizen's
 * case): the link is asked for AT THE CLICK, never prefetched, used at once, never kept.
 */

/** Injected only by tests; the screen always calls the contract route. */
export type LogAttachmentDeps = {
  upload: (lookupCode: string, ...rest: Parameters<SendAttachment>) => ReturnType<SendAttachment>;
};

const DEFAULT_DEPS: LogAttachmentDeps = { upload: uploadLogAttachment };

/**
 * The files of ONE log entry being written. The entry form owns the list: it sends `storedIds(items)`
 * with the entry and clears the list after a 201. Each file on its own — one refusal does not stop the
 * others; removing an unsent file only drops it (its id is simply never sent). Whether THIS officer may
 * attach is the server's note rule; its 403 comes back verbatim.
 */
export function usePetitionLogAttachments(lookupCode: string, deps: LogAttachmentDeps = DEFAULT_DEPS) {
  return useAttachmentUploadList(
    (file, contentType, key, progress) => deps.upload(lookupCode, file, contentType, key, progress),
    "tep-phieu",
  );
}

/** A photo the browser can draw itself: its name opens the in-page viewer instead of a new tab. */
export function isViewableImage(mimeType: string): boolean {
  return mimeType === "image/jpeg" || mimeType === "image/png";
}

/**
 * The files of one timeline row: name, size, type, and a download button. A blank tab is opened IN the
 * click (a tab opened after an `await` is a popup the browser blocks), detached (`opener = null`), then
 * sent to the link. Blocked anyway → this tab goes to the link.
 *
 * A PHOTO'S NAME OPENS IT IN THE PAGE (customer bug sheet row 61c): a viewer over the drawer, the whole
 * image fitted to the window. The link is asked for AT THE CLICK, exactly as for `Tải về` — every view
 * is an audited read on the server, so nothing is prefetched — and it lives only while the viewer is
 * open (`<img src>` needs it; closing drops it). The store serves media inline, so `src` shows it. A PDF
 * keeps the plain name and `Tải về`.
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
  const [viewing, setViewing] = useState<{ file: petitions_taskAttachmentOut; url: string } | null>(null);
  if (attachments.length === 0) return null;

  function view(a: petitions_taskAttachmentOut) {
    setBusy(a.id);
    setRefusal(null);
    download(lookupCode, a.id).then((r) => {
      setBusy(null);
      if (!r.ok) {
        setRefusal({ id: a.id, message: r.message });
        return;
      }
      setViewing({ file: a, url: r.data.url });
    });
  }

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
    <>
      <ul className="mt-1.5 mb-0 flex list-none flex-col gap-1 p-0 text-[13px]" aria-label="Tệp đính kèm">
        {attachments.map((a) => (
          <li key={a.id} className="flex flex-wrap items-center gap-x-2 gap-y-1">
            <span className="min-w-0 break-all">
              <Glyph icon={Paperclip} className="mr-1 inline size-3.5 align-[-2px]" />
              {isViewableImage(a.mime_type) ? (
                <button
                  type="button"
                  className="cursor-pointer border-0 bg-transparent p-0 text-left text-[13px] text-navy [font-family:inherit] break-all hover:underline focus-visible:outline-2 focus-visible:outline-brand-500"
                  aria-haspopup="dialog"
                  aria-label={viewImageLabel(a.file_name)}
                  disabled={busy !== null}
                  onClick={() => view(a)}
                >
                  {a.file_name}
                </button>
              ) : (
                a.file_name
              )}{" "}
              · {formatBytes(a.size_bytes)} · {attachmentTypeLabel(a.mime_type)}
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
      {viewing !== null && (
        <ImageViewer fileName={viewing.file.file_name} url={viewing.url} onClose={() => setViewing(null)} />
      )}
    </>
  );
}

/** Accessible name of a photo's name button. */
export function viewImageLabel(fileName: string): string {
  return `Xem ảnh ${fileName}`;
}

export const IMAGE_VIEW_BROKEN = "Không hiển thị được ảnh này. Hãy đóng và mở lại, hoặc bấm Tải về.";

/**
 * The in-page photo viewer: the system's centred dialog, widened to the window, the image fitted inside
 * it (`object-contain`, never cropped). `no-referrer` and no `loading="lazy"`, as every signed link in
 * this app (`scene-photos.tsx`). A link that fails to draw (expired while open) says so — the remedy is
 * to open it again, which asks for a fresh link.
 */
function ImageViewer({ fileName, url, onClose }: { fileName: string; url: string; onClose: () => void }) {
  const titleId = useId();
  const [broken, setBroken] = useState(false);
  return (
    <ModalDialog titleId={titleId} onDismiss={onClose} size="lg" className="max-w-[min(1200px,calc(100vw-2rem))]!">
      <ModalDialogHeader titleId={titleId} title={<span className="pr-8 break-all">{fileName}</span>} />
      <div className="flex min-h-0 flex-1 items-center justify-center overflow-auto">
        {broken ? (
          <p className="thong-bao-loi m-0" role="alert">
            {IMAGE_VIEW_BROKEN}
          </p>
        ) : (
          // eslint-disable-next-line @next/next/no-img-element -- a short-lived signed link, see scene-photos.tsx
          <img
            src={url}
            alt={fileName}
            referrerPolicy="no-referrer"
            decoding="async"
            onError={() => setBroken(true)}
            className="block h-auto max-h-[calc(90dvh-6rem)] w-auto max-w-full rounded-lg object-contain"
          />
        )}
      </div>
    </ModalDialog>
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
