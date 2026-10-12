/**
 * PHOTOS OF A CONSULTATION REQUEST — the flow, without React (owner 12/10/2026: "Tư vấn và báo giá" may carry up to
 * 3 photos, taken or picked, sent with the request to ViHAT Group's specialists).
 *
 * WHY A PLAIN MODULE: the test suite renders with `react-dom/server` and has no DOM to tap — the same reason as
 * `quick-request.ts`. Every branch a person meets is reachable only if the flow is a function with its steps
 * passed in. `ConsultPhotos.tsx` and `TuVanBaoGiaScreen.tsx` are thin over these.
 *
 * THE ORDER IS THE CONTRACT (`vihat-miniapp` README): the request is created FIRST (JSON, `requestId` back), then
 * each photo goes up ONE AT A TIME to `/requests/{requestId}/photos`. So a photo failure can never fail the
 * request: by the time the first photo byte moves, the request is already recorded.
 *
 * ⚠ NOTHING HERE LOGS, STORES OR PUTS IN A URL a path or a photo (rule 3). The bytes live in React state — memory —
 *   and die with the screen; the policy's "không lưu ảnh trên máy" stands on that.
 */
import {
  guiYeuCau, // vi-name-ok: existing export
  type KetQuaGui, // vi-name-ok: existing type
  type PhotoUploadResult,
  readPickedFile,
  uploadRequestPhoto,
} from "../../api/goi-may-chu";
import {
  REQUEST_PHOTO_MAX,
  REQUEST_PHOTO_MAX_BYTES,
  type RequestPhotoFailure,
  RETRYABLE_PHOTO_FAILURES,
  sniffRequestPhotoType,
  type YeuCauMoi, // vi-name-ok: existing type
} from "../../api/hop-dong-yeu-cau";
import { chooseConsultPhotos, type PhotoPick, takeConsultPhoto } from "../tinh-nang/zalo-api";

/** "Chụp ảnh" (the phone's camera, one photo) or "Chọn ảnh" (the photo picker). */
export type PhotoSource = "camera" | "library";

/** Asks Zalo for photos. `max` is how many the request can still take. Injected so tests need no Zalo. */
export type PhotoPicker = (source: PhotoSource, max: number) => Promise<PhotoPick>;

export const LIVE_PICKER: PhotoPicker = (source, max) =>
  source === "camera" ? takeConsultPhoto() : chooseConsultPhotos(max);

/** A photo ready to go: its bytes, read once at pick time. `key` is a local counter — never the path. */
export type ConsultPhoto = { readonly key: string; readonly path: string; readonly file: Blob };

/** Why a picked photo was kept out, said while the person can still choose another. */
export type PhotoRejection = "file-unreadable" | "too-large" | "unsupported";

/** Why a pick brought nothing: their no, outside Zalo, or Zalo did not answer. A closed picker is no failure. */
export type PickFailure = "refused" | "outside-zalo" | "failed";

export type PickOutcome = {
  readonly photos: readonly ConsultPhoto[];
  readonly rejected: readonly PhotoRejection[];
  readonly failure: PickFailure | null;
};

/**
 * Read a picked file and check it the way the server will, so a refusal is said BEFORE the request is sent.
 *
 *   unreadable path   the open device question (`readPickedFile`) — "chụp lại hoặc chọn ảnh khác"
 *   over 5 MB         `REQUEST_PHOTO_MAX_BYTES`
 *   not JPEG/PNG/WebP  read from the first bytes, as the server reads them (a HEIC from an iPhone is the usual one)
 */
export async function preparePickedPhoto(
  path: string,
  readFile: (path: string) => Promise<Blob | null>,
): Promise<{ file: Blob } | { rejected: PhotoRejection }> {
  const file = await readFile(path);
  if (file === null) return { rejected: "file-unreadable" };
  if (file.size > REQUEST_PHOTO_MAX_BYTES) return { rejected: "too-large" };
  let head: Uint8Array;
  try {
    head = new Uint8Array(await file.slice(0, 12).arrayBuffer());
  } catch {
    return { rejected: "file-unreadable" };
  }
  return sniffRequestPhotoType(head) === null ? { rejected: "unsupported" } : { file };
}

/**
 * One tap → the photos to keep. Never more than `room` (the camera takes ONE), never throws. `nextKey` draws the
 * local keys, so two picks never reuse one.
 */
export async function pickConsultPhotos(
  pick: PhotoPicker,
  source: PhotoSource,
  room: number,
  nextKey: () => string,
  readFile: (path: string) => Promise<Blob | null> = readPickedFile,
): Promise<PickOutcome> {
  const limit = Math.min(source === "camera" ? 1 : room, REQUEST_PHOTO_MAX);
  if (limit <= 0) return { photos: [], rejected: [], failure: null };
  let answer: PhotoPick;
  try {
    answer = await pick(source, limit);
  } catch {
    return { photos: [], rejected: [], failure: "failed" };
  }
  switch (answer.kieu) {
    case "huy":
      return { photos: [], rejected: [], failure: null };
    case "tu-choi":
      return { photos: [], rejected: [], failure: "refused" };
    case "ngoai-zalo":
      return { photos: [], rejected: [], failure: "outside-zalo" };
    case "khong-lay-duoc":
      return { photos: [], rejected: [], failure: "failed" };
    case "xong":
      break;
  }
  const photos: ConsultPhoto[] = [];
  const rejected: PhotoRejection[] = [];
  for (const path of answer.du_lieu.filter((p) => typeof p === "string" && p !== "").slice(0, limit)) {
    const ready = await preparePickedPhoto(path, readFile);
    if ("file" in ready) photos.push({ key: nextKey(), path, file: ready.file });
    else rejected.push(ready.rejected);
  }
  return { photos, rejected, failure: null };
}

/* ═══════════════════════════════ AFTER THE REQUEST IS CREATED ═══════════════════════════════ */

/** One photo in the upload list. `index` is its number as the person sees it ("Ảnh 2"). */
export type PhotoUpload = {
  readonly key: string;
  readonly index: number;
  readonly file: Blob;
  readonly status: "waiting" | "sending" | "stored" | "failed";
  readonly failure?: RequestPhotoFailure;
  /** `true` only when a second press can change the answer (`RETRYABLE_PHOTO_FAILURES`). */
  readonly canRetry?: boolean;
};

export type UploadPhoto = (bearer: string, requestId: string, file: Blob) => Promise<PhotoUploadResult>;

/** The upload queue for the photos picked on the form, in the order the person sees them. */
export function queueUploads(photos: readonly ConsultPhoto[]): readonly PhotoUpload[] {
  return photos.slice(0, REQUEST_PHOTO_MAX).map((p, i) => ({ key: p.key, index: i + 1, file: p.file, status: "waiting" }));
}

/**
 * Send every `waiting` photo, ONE AT A TIME, reporting each change. A failure marks that photo and moves to the
 * next one; it never throws. Sequential on purpose: the server counts to 3 under a row lock, the IP bucket is 20
 * calls per 5 minutes, and a rural line does not upload three 5 MB files faster in parallel.
 */
export async function uploadInOrder(
  jobs: readonly PhotoUpload[],
  bearer: string,
  requestId: string,
  upload: UploadPhoto,
  onChange: (jobs: readonly PhotoUpload[]) => void,
): Promise<readonly PhotoUpload[]> {
  let current = jobs;
  const patch = (key: string, change: Partial<PhotoUpload>) => {
    current = current.map((j) => (j.key === key ? { ...j, ...change } : j));
    onChange(current);
  };
  for (const job of jobs) {
    if (job.status !== "waiting") continue;
    patch(job.key, { status: "sending", failure: undefined, canRetry: undefined });
    let out: PhotoUploadResult;
    try {
      out = await upload(bearer, requestId, job.file);
    } catch {
      out = { kind: "failed", failure: "server" };
    }
    if (out.kind === "stored") patch(job.key, { status: "stored" });
    else patch(job.key, { status: "failed", failure: out.failure, canRetry: RETRYABLE_PHOTO_FAILURES.has(out.failure) });
  }
  return current;
}

/** "Tải lại" on one failed photo: back to `waiting`, only when a retry can help. PURE. */
export function markForRetry(jobs: readonly PhotoUpload[], key: string): readonly PhotoUpload[] {
  return jobs.map((j) =>
    j.key === key && j.status === "failed" && j.canRetry === true
      ? { ...j, status: "waiting", failure: undefined, canRetry: undefined }
      : j,
  );
}

export type ConsultSteps = {
  send: (bearer: string, request: YeuCauMoi) => Promise<KetQuaGui>; // vi-name-ok: existing types
  upload: UploadPhoto;
};

export const LIVE_CONSULT_STEPS: ConsultSteps = {
  send: (bearer, request) => guiYeuCau(bearer, request), // vi-name-ok: existing export
  upload: (bearer, requestId, file) => uploadRequestPhoto(bearer, requestId, file),
};

/**
 * Create the request, THEN upload its photos — the whole send, in the order the contract requires.
 *
 *   `onSent` runs as soon as the request answers, so the success screen (with the request code) is up while the
 *   photos are still going; `onPhotos` reports every photo's change after that.
 *   Photos go only with a `consult` request that was created (`xong`). A callback carries none — the server
 *   answers 409 `photo_not_allowed` — and the form says so before the tap (`CONSULT_PHOTOS.callback_no_photos`).
 */
export async function sendConsultation(
  steps: ConsultSteps,
  bearer: string,
  request: YeuCauMoi, // vi-name-ok: existing type
  photos: readonly ConsultPhoto[],
  onSent: (result: KetQuaGui) => void, // vi-name-ok: existing type
  onPhotos: (jobs: readonly PhotoUpload[]) => void,
): Promise<{ result: KetQuaGui; photos: readonly PhotoUpload[] }> { // vi-name-ok: existing type
  const result = await steps.send(bearer, request);
  onSent(result);
  if (result.kieu !== "xong" || request.loai !== "consult" || photos.length === 0) return { result, photos: [] };
  const queued = queueUploads(photos);
  onPhotos(queued);
  return { result, photos: await uploadInOrder(queued, bearer, result.ma_yeu_cau, steps.upload, onPhotos) };
}
