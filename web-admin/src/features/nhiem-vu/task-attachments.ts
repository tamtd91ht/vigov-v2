/**
 * `📎 Đính kèm` (§5.9, A4) — pure half: the pre-check, the per-file state, the words.
 *
 * THE SERVER'S POLICY DECIDES — type, size, count (ADR 0052 §10, platform's limits). The pre-check
 * here only spares an upload the server would certainly refuse: a type outside pdf / jpeg / png. NO
 * SIZE LIMIT IS WRITTEN HERE: the limit is platform configuration, and a copied number would refuse
 * a file the commune allows, or let through one it refuses, with nothing to say which is right.
 */

import type { petitions_taskAttachmentOut } from "@/lib/api/schema.gen";
import { isTransientUpload, type UploadResult } from "@/lib/api/upload";

/** The spec's `📎` is drawn as a lucide `Paperclip` beside the word (ADR 0068). */
export const ATTACH_BUTTON = "Đính kèm";
export const ATTACH_INPUT_LABEL = "Chọn tệp đính kèm (PDF, JPG, PNG)";
export const ATTACH_NOTE =
  "Mỗi tệp được tải lên, kiểm tra kiểu và quét mã độc trước khi gắn vào dòng nhật ký. Chỉ tệp đã " +
  "lưu mới được gửi kèm; dung lượng và số tệp tối đa do hệ thống quy định.";

/** The pre-check's list. `accept` of the input AND the check — one list. */
export const ATTACH_TYPES: Readonly<Record<string, string>> = {
  "application/pdf": "PDF",
  "image/jpeg": "JPG",
  "image/png": "PNG",
};
export const ATTACH_ACCEPT = ".pdf,.jpg,.jpeg,.png,application/pdf,image/jpeg,image/png";

const BY_EXTENSION: Readonly<Record<string, string>> = {
  pdf: "application/pdf",
  jpg: "image/jpeg",
  jpeg: "image/jpeg",
  png: "image/png",
};

export const ATTACH_TYPE_REFUSED = "Chỉ đính kèm được tệp PDF, JPG hoặc PNG.";
export const ATTACH_EMPTY_REFUSED = "Tệp rỗng — không có gì để đính kèm.";

/**
 * The content type to DECLARE, or a refusal. The browser's `type` first; when it is empty (some
 * systems report none) the extension. The declaration is a claim: the server sniffs the bytes in the
 * same request and refuses a mismatch (ADR 0052 §1c).
 */
export function declaredType(file: { readonly name: string; readonly type: string; readonly size: number }):
  | { readonly ok: true; readonly contentType: string }
  | { readonly ok: false; readonly message: string } {
  if (file.size <= 0) return { ok: false, message: ATTACH_EMPTY_REFUSED };
  if (file.type !== "") {
    return ATTACH_TYPES[file.type] !== undefined
      ? { ok: true, contentType: file.type }
      : { ok: false, message: ATTACH_TYPE_REFUSED };
  }
  const ext = file.name.toLowerCase().split(".").pop() ?? "";
  const t = BY_EXTENSION[ext];
  return t === undefined ? { ok: false, message: ATTACH_TYPE_REFUSED } : { ok: true, contentType: t };
}

/**
 * One file in the form's list, and where it stands. ONE request per file (ADR 0052 §Sửa đổi 09/10/2026):
 * the bytes leave (`uploading`), the server sniffs and scans (`checking`), then it answers.
 */
export type AttachmentState =
  | { readonly kind: "uploading"; readonly percent: number }
  | { readonly kind: "checking" }
  | { readonly kind: "stored"; readonly id: string }
  /** A refusal of the MOMENT (`isTransientUpload`): nothing was stored — send the same file again. */
  | { readonly kind: "retry"; readonly message: string }
  /** For good: refused by the pre-check, the limits, the type, or the scan (422). */
  | { readonly kind: "refused"; readonly message: string };

export type AttachmentItem = {
  /** Local key for the list — not a server id (a refused file never gets one). */
  readonly key: string;
  readonly name: string;
  readonly size: number;
  readonly state: AttachmentState;
};

/** Ids to send with the entry — ONLY stored files, in the order chosen. */
export function storedIds(items: readonly AttachmentItem[]): string[] {
  return items.flatMap((i) => (i.state.kind === "stored" ? [i.state.id] : []));
}

/** Something still moving: the entry waits for it (sending now would drop a file the clerk chose). */
export function anyInFlight(items: readonly AttachmentItem[]): boolean {
  return items.some((i) => i.state.kind === "uploading" || i.state.kind === "checking");
}

/**
 * What an upload's answer turns into: the stored file; a refusal of the moment → `retry` (send the same
 * file again); anything else refused for good. The sentence is the server's.
 */
export function afterUpload(
  r: UploadResult<petitions_taskAttachmentOut>,
): AttachmentState {
  if (r.ok) return r.data.status === "stored" || r.data.status === "ready"
    ? { kind: "stored", id: r.data.id }
    : { kind: "refused", message: ATTACH_NOT_STORED };
  if (isTransientUpload(r)) return { kind: "retry", message: r.message };
  return { kind: "refused", message: r.message };
}

export const ATTACH_NOT_STORED = "Tệp chưa được lưu. Hãy bỏ tệp khỏi danh sách rồi chọn lại.";
export const ATTACH_RETRY_BUTTON = "Gửi lại";
export const ATTACH_REMOVE_BUTTON = "Bỏ";
export const ATTACH_WAIT_NOTE = "Chờ các tệp tải lên và kiểm tra xong rồi mới ghi nhật ký.";

/** The line under a file — `role="status"` while it moves, `role="alert"` when refused. */
export function attachmentStateText(s: AttachmentState): string {
  switch (s.kind) {
    case "uploading":
      return `Đang tải ${s.percent}%`;
    case "checking":
      return "Đang kiểm tra (dò kiểu, quét mã độc)…";
    case "stored":
      return "Đã lưu — sẽ gửi kèm dòng nhật ký.";
    case "retry":
      return `Chưa gửi được: ${s.message}`;
    case "refused":
      return `Bị từ chối: ${s.message}`;
  }
}

/** `1,2 MB` · `340 KB` · `12 B` — the size shown beside a name, Vietnamese decimal comma. */
export function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  const fmt = (v: number) => new Intl.NumberFormat("vi-VN", { maximumFractionDigits: 1 }).format(v);
  if (n < 1024 * 1024) return `${fmt(n / 1024)} KB`;
  return `${fmt(n / (1024 * 1024))} MB`;
}

/** The type label of a stored file (sniffed `mime_type`), the raw type when unknown, never blank. */
export function attachmentTypeLabel(mime: string): string {
  return ATTACH_TYPES[mime] ?? (mime === "" ? "—" : mime);
}

/** Accessible name of a download button: which file. */
export function downloadLabel(fileName: string): string {
  return `Tải về ${fileName}`;
}

export const DOWNLOAD_OPENING = "Đang mở tệp…";
export const DOWNLOAD_REFUSED = "Chưa mở được tệp:";
