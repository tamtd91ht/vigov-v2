/**
 * `📎 Đính kèm` on the task timeline (§5.9) — ADR 0052 §Sửa đổi 09/10/2026. Two calls:
 *
 *   a. POST /api/v1/tasks/{ma}/attachments              ONE multipart request: size, content_type,
 *                                                        file_name, then the file + Idempotency-Key
 *                                                        → 201 the stored file | 4xx | 503
 *   b. GET  /api/v1/tasks/{ma}/attachments/{id}/download     a presigned GET → {url, expires_at}
 *
 * then the entry itself carries `attachments: [id…]` (`addTaskLogEntry`).
 *
 * The presigned POST to the object store and the completion call are GONE (the server change of
 * 09/10/2026): the bytes go to this commune's own origin, and the service streams, sniffs and scans
 * them inside that one request.
 *
 * ⚠ The download `url` IS A BEARER CREDENTIAL: used in the one request it signs and nowhere else —
 * never logged, never stored, never put in the address bar of this app, never cached (`no-store`).
 *
 * WHY THE CALLS RETURN THE HTTP STATUS: the answer decides what the screen offers next — a refusal of
 * the file (4xx) is final, a refusal of the moment (`isTransientUpload`: no answer, 408, 502–504) may
 * be sent again. The sentence is always the server's when it wrote one; the status only picks the button.
 */

import { CHUNG, LOI_KHONG_RO, thongBaoLoi } from "./goi"; // vi-name-ok: existing exports of goi.ts (rule 12 inv 3)
import type {
  petitions_get_tasks_by_ma_attachments_by_id_download,
  petitions_post_tasks_by_ma_attachments,
  petitions_taskAttachmentDownloadOut,
  petitions_taskAttachmentOut,
} from "./schema.gen";
import { sendUpload, type UploadProgress, type UploadResult } from "./upload";

/** A call's answer with its status — see the header for why the status matters here. */
export type CallResult<T> =
  | { readonly ok: true; readonly data: T }
  | { readonly ok: false; readonly status: number; readonly message: string };

/** Status 0 = no answer at all (network). */
const NO_ANSWER = 0;

function taskPath(template: string, code: string, id?: string): string {
  let p = template.replace("{ma}", encodeURIComponent(code));
  if (id !== undefined) p = p.replace("{id}", encodeURIComponent(id));
  return p;
}

/** Also used by the commune-branding calls (`commune-branding.ts`), which share this answer shape. */
export async function readJSON<T>(res: Response, want: number): Promise<CallResult<T>> {
  if (res.status !== want) return { ok: false, status: res.status, message: await thongBaoLoi(res) };
  try {
    return { ok: true, data: (await res.json()) as T };
  } catch {
    return { ok: false, status: res.status, message: LOI_KHONG_RO };
  }
}

/**
 * Upload ONE file for the next log entry: one multipart POST (`lib/api/upload.ts`), parts in the
 * contract's order. 201 is the file, already sniffed, scanned and stored — its id goes on the entry.
 * The limits refusal, 422 `attachment_rejected`, 503 `upload_busy` / `storage_not_configured` … come
 * back as the server's sentence with their status. `file_name` can name a person (rule 3): sent as a
 * part, never logged.
 */
export function uploadTaskAttachment(
  code: string,
  file: File,
  contentType: string,
  idempotencyKey: string,
  progress: UploadProgress = {},
): Promise<UploadResult<petitions_taskAttachmentOut>> {
  const template: TaskUpload["duongDan"] = "/api/v1/tasks/{ma}/attachments";
  return sendUpload<TaskUpload, petitions_taskAttachmentOut>({
    path: taskPath(template, code),
    parts: TASK_UPLOAD_PARTS,
    fields: { content_type: contentType, file_name: file.name },
    file,
    fileName: file.name,
    idempotencyKey,
    ...progress,
  });
}

type TaskUpload = petitions_post_tasks_by_ma_attachments;
/** The contract's order — `tsc` turns red here the day it changes. */
const TASK_UPLOAD_PARTS: TaskUpload["multipartParts"] = ["size", "content_type", "file_name", "file"];

/** b. A short-lived download link. Asked for at the click, used at once, never kept. */
export async function attachmentDownloadLink(
  code: string,
  id: string,
): Promise<CallResult<petitions_taskAttachmentDownloadOut>> {
  const template: petitions_get_tasks_by_ma_attachments_by_id_download["duongDan"] =
    "/api/v1/tasks/{ma}/attachments/{id}/download";
  try {
    const res = await fetch(taskPath(template, code, id), { ...CHUNG, method: "GET" });
    return await readJSON<petitions_taskAttachmentDownloadOut>(res, 200);
  } catch {
    return { ok: false, status: NO_ANSWER, message: LOI_KHONG_RO };
  }
}
