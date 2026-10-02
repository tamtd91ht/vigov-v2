/**
 * `📎 Đính kèm` on the task timeline (§5.9) — ADR 0052 §1, backend b37ec2d. Four calls, in order:
 *
 *   a. POST /api/v1/tasks/{ma}/attachments              declare {file_name, content_type, size}
 *                                                        + Idempotency-Key → 201 {attachment, upload}
 *   b. POST <upload.url>                                 the bytes, STRAIGHT to the object store
 *   c. POST /api/v1/tasks/{ma}/attachments/{id}/completion   sniff, scan, store → 200 | 422 | 409 | 503
 *   d. GET  /api/v1/tasks/{ma}/attachments/{id}/download     a presigned GET → {url, expires_at}
 *
 * then the entry itself carries `attachments: [id…]` (`addTaskLogEntry`).
 *
 * ⚠ `upload.url` + `upload.fields` AND the download `url` ARE BEARER CREDENTIALS. They go into the
 * one request they sign and nowhere else: never logged, never stored, never put in the address bar of
 * this app, never cached (`no-store`). The object store is on ANOTHER origin (ADR 0052 §4), so the
 * upload carries NO cookie — the policy in `fields` is the whole authorisation.
 *
 * WHY THE CALLS RETURN THE HTTP STATUS: the completion's answer decides what the screen offers next —
 * 422 the file is refused for good, 503 "not now" (retry the completion, never re-upload), 409 the
 * upload is not there yet / expired / changed (the sentence says which; retrying the completion is
 * safe, the server re-reads). The sentence is always the server's, verbatim; the status only picks
 * the button.
 */

import { CHUNG, LOI_KHONG_RO, thongBaoLoi } from "./goi"; // vi-name-ok: existing exports of goi.ts (rule 12 inv 3)
import type {
  petitions_get_tasks_by_ma_attachments_by_id_download,
  petitions_post_tasks_by_ma_attachments,
  petitions_post_tasks_by_ma_attachments_by_id_completion,
  petitions_presignedUploadOut,
  petitions_taskAttachmentDownloadOut,
  petitions_taskAttachmentOut,
  petitions_taskAttachmentUploadIn,
  petitions_taskAttachmentUploadOut,
} from "./schema.gen";

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
 * a. Declare the file. The server checks the declaration against the platform's limits (type, size,
 * count) and answers a refusal in one sentence — the screen hard-codes no size limit. 503
 * `storage_not_configured` ("Chưa cấu hình kho lưu tệp…") comes back verbatim.
 *
 * `idempotencyKey`: the route declares it. ONE KEY PER ATTEMPT (the caller mints it): a replayed answer
 * cannot carry the signed form again, so a retry of the declaration is a new declaration; an abandoned
 * pending slot expires with the temp bucket's lifecycle (ADR 0052 §2).
 */
export async function requestAttachmentUpload(
  code: string,
  body: petitions_taskAttachmentUploadIn,
  idempotencyKey: string,
): Promise<CallResult<petitions_taskAttachmentUploadOut>> {
  const template: petitions_post_tasks_by_ma_attachments["duongDan"] = "/api/v1/tasks/{ma}/attachments";
  // Field by field — never `...body`.
  const sent: petitions_taskAttachmentUploadIn = {
    file_name: body.file_name,
    content_type: body.content_type,
    size: body.size,
  };
  try {
    const res = await fetch(taskPath(template, code), {
      ...CHUNG,
      method: "POST",
      headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey },
      body: JSON.stringify(sent),
    });
    const r = await readJSON<petitions_taskAttachmentUploadOut>(res, 201);
    // A replayed 201 has no form to upload with (`core/idem` stores a code, never a body).
    if (r.ok && (r.data?.upload?.url === undefined || r.data.upload.fields === undefined)) {
      return { ok: false, status: 201, message: UPLOAD_FORM_MISSING };
    }
    return r;
  } catch {
    return { ok: false, status: NO_ANSWER, message: LOI_KHONG_RO };
  }
}

export const UPLOAD_FORM_MISSING =
  "Máy chủ không gửi kèm biểu mẫu tải lên cho tệp này. Hãy bỏ tệp khỏi danh sách rồi chọn lại.";
export const UPLOAD_TO_STORAGE_FAILED =
  "Chưa tải được tệp lên kho lưu tệp. Hãy bỏ tệp khỏi danh sách rồi chọn lại.";

/**
 * The multipart body of b: EVERY `fields` entry first, in the order given, then the file as the LAST
 * field, named `file` (`core/storage` `PresignedPost`). S3/MinIO ignores fields after the file, so a
 * policy field placed after it is a field the store never sees — and the upload is refused.
 */
export function uploadForm(post: Pick<petitions_presignedUploadOut, "fields">, file: Blob, fileName: string): FormData {
  const form = new FormData();
  for (const [k, v] of Object.entries(post.fields)) form.append(k, v);
  form.append("file", file, fileName);
  return form;
}

/**
 * b. The bytes to the object store, with upload progress — `XMLHttpRequest`, because `fetch` reports
 * no upload progress. `withCredentials` stays FALSE: another origin, no cookie (ADR 0052 §4). No
 * header is set: the browser writes the multipart boundary. 2xx is success (MinIO answers 204).
 *
 * A refusal here is the STORE's XML, not the service's sentence — so the screen says one sentence of
 * its own, and nothing of the response is read or kept.
 */
export function uploadToStorage(
  post: Pick<petitions_presignedUploadOut, "url" | "fields">,
  file: Blob,
  fileName: string,
  onProgress: (percent: number) => void,
): Promise<CallResult<null>> {
  return new Promise((resolve) => {
    const xhr = new XMLHttpRequest();
    xhr.open("POST", post.url);
    xhr.withCredentials = false;
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable && e.total > 0) onProgress(Math.min(100, Math.round((e.loaded / e.total) * 100)));
    };
    xhr.onload = () =>
      resolve(
        xhr.status >= 200 && xhr.status < 300
          ? { ok: true, data: null }
          : { ok: false, status: xhr.status, message: UPLOAD_TO_STORAGE_FAILED },
      );
    xhr.onerror = () => resolve({ ok: false, status: NO_ANSWER, message: UPLOAD_TO_STORAGE_FAILED });
    xhr.send(uploadForm(post, file, fileName));
  });
}

/** c. Complete: sniff the bytes, scan, store. Safe to repeat — a stored file answers itself again. */
export async function completeAttachment(
  code: string,
  id: string,
): Promise<CallResult<petitions_taskAttachmentOut>> {
  const template: petitions_post_tasks_by_ma_attachments_by_id_completion["duongDan"] =
    "/api/v1/tasks/{ma}/attachments/{id}/completion";
  try {
    const res = await fetch(taskPath(template, code, id), { ...CHUNG, method: "POST" });
    return await readJSON<petitions_taskAttachmentOut>(res, 200);
  } catch {
    return { ok: false, status: NO_ANSWER, message: LOI_KHONG_RO };
  }
}

/** d. A short-lived download link. Asked for at the click, used at once, never kept. */
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
