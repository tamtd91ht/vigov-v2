/**
 * THE one upload path of this app (ADR 0052 §Sửa đổi 09/10/2026): ONE multipart `POST` to the service
 * that owns the record, on this commune's own origin (`/api/v1/…`, through the web gateway
 * `lib/may-chu/chuyen-tiep.ts`). No presigned form, no request to any storage host, no `/completion`:
 * the service streams the file into storage, sniffs and scans it, and answers 201 with the stored file
 * — or a refusal — in that same request.
 *
 * WHY `XMLHttpRequest` AND NOT `fetch`: `fetch` reports no upload progress, and a 50 MB file on a
 * commune's uplink takes long enough that a screen without a percentage looks frozen.
 *
 * THE PARTS GO IN THE CONTRACT'S ORDER (`multipartParts`, generated from `kb/20-contracts/openapi.json`):
 * text parts first, `size` among them, `file` LAST. The server reads `size` before the file (multipart
 * carries no part length) and refuses any part after it (`core/httpx/upload.go`). `size` is always
 * `file.size` — never a caller's number, which could disagree with the bytes and earn a 400.
 *
 * NOTHING HERE IS LOGGED: a file name is often a person's name (rule 3), and the reply carries ids of
 * evidence about a citizen's case.
 *
 * Every refusal the SERVER wrote (`httpx.Error`) comes back as its sentence, verbatim, with its `code`
 * and status — the screen picks a button by status/code, never by wording. A refusal with no such body
 * (a proxy's HTML page: the ingress's 413, a 502, the network gone) gets one FIXED sentence per status
 * below, so the officer never reads "Không kết nối được máy chủ" for a file that was simply too big.
 */

import { LOI_KHONG_RO, stripTechnicalPrefix } from "./goi"; // vi-name-ok: existing exports of goi.ts (rule 12 inv 3)

/** An upload's answer: the stored file, or a refusal with the server's `code` (`""` when none). */
export type UploadResult<T> =
  | { readonly ok: true; readonly data: T }
  | { readonly ok: false; readonly status: number; readonly code: string; readonly message: string };

/** The shape every generated multipart route has (`scripts/gen-api-types.mjs`, `dichMultipart`). */
export type MultipartRoute = {
  multipart: { size: string; file: Blob };
  multipartParts: readonly string[];
};

/** The text parts a caller may set — everything but `size` (always `file.size`) and `file`. */
export type UploadFields<R extends MultipartRoute> = Omit<R["multipart"], "size" | "file">;

/** Status 0 = no answer at all. */
export const NO_ANSWER = 0;

export const UPLOAD_NETWORK_FAILED =
  "Mất kết nối khi đang gửi tệp nên tệp CHƯA được nhận. Hãy kiểm tra mạng rồi thử lại.";
export const UPLOAD_TOO_LARGE = "Tệp quá lớn để gửi lên hệ thống. Hãy chọn tệp nhỏ hơn.";
export const UPLOAD_TOO_SLOW = "Gửi tệp quá lâu nên hệ thống chưa nhận được. Vui lòng thử lại.";
export const UPLOAD_UNAVAILABLE = "Hệ thống tạm thời chưa nhận được tệp. Vui lòng thử lại sau ít phút.";
export const UPLOAD_REPLAYED =
  "Tệp này đã được gửi ở một lần trước nên hệ thống không trả lại thông tin tệp. Hãy tải lại trang để xem tệp.";

/**
 * The sentence for a refusal whose body is NOT the server's `httpx.Error`, by status. Not a mapping of
 * the server's codes: when the server wrote a sentence, that sentence wins (`readRefusal`).
 */
export function uploadFallbackSentence(status: number): string {
  switch (status) {
    case NO_ANSWER:
      return UPLOAD_NETWORK_FAILED;
    case 413:
      return UPLOAD_TOO_LARGE;
    case 408:
    case 504:
      return UPLOAD_TOO_SLOW;
    case 502:
    case 503:
      return UPLOAD_UNAVAILABLE;
    default:
      return LOI_KHONG_RO;
  }
}

/**
 * Worth sending the SAME file again without the officer choosing anything: nothing was stored, and the
 * cause is the moment, not the file — no answer, 408 too slow, 502/504 from a gateway, 503 (`upload_busy`
 * with Retry-After, scanner / store / limits not reachable). Every other refusal is about the file or the
 * record and repeats itself.
 */
export function isTransientUpload(r: { readonly status: number }): boolean {
  return r.status === NO_ANSWER || r.status === 408 || r.status === 502 || r.status === 503 || r.status === 504;
}

/**
 * The multipart body in `parts` order: `size` = `file.size`, text parts that have a non-empty value
 * (absent ≠ `""`: an empty part would be a third spelling of "none"), `file` last.
 */
export function uploadFormData<R extends MultipartRoute>(
  parts: R["multipartParts"],
  fields: UploadFields<R>,
  file: Blob,
  fileName: string,
): FormData {
  const text = fields as Readonly<Record<string, string | undefined>>;
  const form = new FormData();
  for (const name of parts) {
    if (name === "size") form.append("size", String(file.size));
    else if (name === "file") form.append("file", file, fileName);
    else {
      const v = text[name];
      if (v !== undefined && v !== "") form.append(name, v);
    }
  }
  return form;
}

/** The two progress hooks of an upload, passed through from a screen. */
export type UploadProgress = {
  /** 0..100 while the bytes leave the browser. */
  readonly onProgress?: (percent: number) => void;
  /** Every byte is sent: the server is now checking the file (sniff, scan) — the reply comes after. */
  readonly onSent?: () => void;
};

export type UploadRequest<R extends MultipartRoute> = UploadProgress & {
  /** The route's path with its parameters already filled and encoded. Relative: this origin only. */
  readonly path: string;
  /** The route's `multipartParts`, typed from the contract at the call site. */
  readonly parts: R["multipartParts"];
  readonly fields: UploadFields<R>;
  readonly file: Blob;
  readonly fileName: string;
  /** The route requires one. ONE KEY PER ATTEMPT (`crypto.randomUUID()` at the caller). */
  readonly idempotencyKey: string;
};

function parseJSON(text: string): unknown {
  try {
    return JSON.parse(text) as unknown;
  } catch {
    return undefined;
  }
}

function readRefusal(status: number, body: unknown): { ok: false; status: number; code: string; message: string } {
  const b = body as { code?: unknown; message?: unknown } | undefined;
  const code = typeof b?.code === "string" ? b.code : "";
  const message =
    typeof b?.message === "string" && b.message !== "" ? stripTechnicalPrefix(b.message) : uploadFallbackSentence(status);
  return { ok: false, status, code, message };
}

/** Send one file. Resolves, never rejects. 201 is the only success. */
export function sendUpload<R extends MultipartRoute, T>(req: UploadRequest<R>): Promise<UploadResult<T>> {
  return new Promise((resolve) => {
    const xhr = new XMLHttpRequest();
    xhr.open("POST", req.path);
    // Same origin: the session cookie goes with it on its own. Never `withCredentials` — nothing here
    // is ever sent to another origin.
    xhr.setRequestHeader("Idempotency-Key", req.idempotencyKey);
    // NO Content-Type header: the browser writes `multipart/form-data; boundary=…` itself.
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable && e.total > 0) req.onProgress?.(Math.min(100, Math.round((e.loaded / e.total) * 100)));
    };
    xhr.upload.onload = () => {
      req.onProgress?.(100);
      req.onSent?.();
    };
    xhr.onload = () => {
      const body = parseJSON(xhr.responseText);
      if (xhr.status !== 201) {
        resolve(readRefusal(xhr.status, body));
        return;
      }
      if (body === undefined || body === null || typeof body !== "object") {
        resolve({ ok: false, status: 201, code: "", message: LOI_KHONG_RO });
        return;
      }
      // A replayed key answers `{code, replayed: true}` and nothing else (`core/idem` stores a code,
      // never a body) — not the stored file the screen needs.
      if ((body as { replayed?: unknown }).replayed === true) {
        resolve({ ok: false, status: 201, code: "replayed", message: UPLOAD_REPLAYED });
        return;
      }
      resolve({ ok: true, data: body as T });
    };
    xhr.onerror = () => resolve({ ok: false, status: NO_ANSWER, code: "", message: UPLOAD_NETWORK_FAILED });
    xhr.onabort = xhr.onerror;
    xhr.send(uploadFormData<R>(req.parts, req.fields, req.file, req.fileName));
  });
}
