/**
 * §7 `Truyền thanh` — the broadcast's audio file: the pre-check, the typed duration, the one-file state,
 * the words, the flow (ADR 0067 §4, backend 026ae398).
 *
 * THE SERVER'S POLICY DECIDES (platform's `content-audio`: `audio/mpeg` · `audio/mp4`, 31 457 280 bytes,
 * one file per item). The pre-check here only spares an upload the server would refuse; whatever the
 * server answers is shown word for word.
 *
 * 30 MB IS 30 × 1024 × 1024 BYTES — exactly the policy's 31 457 280 (service-platform migration 0013).
 *
 * THE DURATION IS TYPED, NEVER MEASURED (ADR 0067 §4.1, ADR 0047 G7): the server stores what the officer
 * says. The check here mirrors `domain.CheckAudioDuration` (1 .. 21 600 s, i.e. 6 hours).
 *
 * THE DECLARATION NAMES A SAVED ITEM: `content_item_id` of a saved `truyen-thanh` item, and the completion
 * attaches the file to it at once. The form therefore HOLDS the chosen file (`HeldAudio`) — on create and
 * on edit alike — and runs the three steps with the item's id once the save returns (prototype
 * `ContentItemForm.tsx:150-180`: one press for the officer, the file after the item).
 */

import { completeAudioUpload, requestAudioUpload, type AudioCallResult } from "@/lib/api/noi-dung";
import { uploadToStorage } from "@/lib/api/task-attachments";
import type { comms_audioFileOut, comms_suaNoiDungVao } from "@/lib/api/schema.gen";

/* ── Words ──────────────────────────────────────────────────────────────────────────────────── */

export const AUDIO_LABEL = "Tệp âm thanh";
/** The empty slot's words, verbatim prototype `ContentItemForm.tsx:425`. */
export const AUDIO_PICK_BUTTON = "Chọn tệp từ máy";
/** The slot of a saved broadcast that already has its file (prototype `ContentItemForm.tsx:424`). */
export const AUDIO_HAS_FILE = "Đã có tệp — chọn tệp mới để thay";
/** The policy's limits (ADR 0067 §4.1: MP3/M4A, 30MB) in the prototype's shape (`ContentItemForm.tsx:328`). */
export const AUDIO_HINT = "MP3 hoặc M4A — tối đa 30MB";
export const AUDIO_RETRY_BUTTON = "Hoàn tất lại";

/**
 * The item was saved but its held file did not get attached — the prototype's sentence, verbatim
 * (`ContentItemForm.tsx:174`). The dialog stays open on the saved item, with the upload's own reason.
 */
export const AUDIO_SAVED_NOT_UPLOADED = "Đã lưu nội dung nhưng chưa tải được tệp lên.";

/** Verbatim prototype `ContentItemForm.tsx:314`. */
export const AUDIO_DURATION_LABEL = "Thời lượng (giây)";
export const AUDIO_DURATION_FORMAT = "Thời lượng phải là một số giây nguyên, ví dụ 750.";
export const AUDIO_DURATION_RANGE = "Thời lượng phải từ 1 đến 21600 giây (6 giờ).";

export const AUDIO_TYPE_REFUSED = "Chỉ nhận tệp âm thanh MP3 hoặc M4A.";
export const AUDIO_TOO_LARGE = "Tệp lớn hơn 30MB — hãy chọn tệp nhỏ hơn.";
export const AUDIO_EMPTY = "Tệp rỗng — không có âm thanh nào để tải lên.";
export const AUDIO_STORAGE_FAILED = "Chưa tải được tệp lên kho lưu tệp. Hãy chọn lại tệp.";
export const AUDIO_NOT_READY = "Tệp âm thanh chưa sẵn sàng để gắn vào mục. Hãy chọn lại tệp.";
export const AUDIO_WAIT_NOTE = "Chờ tệp âm thanh tải lên và kiểm tra xong rồi mới lưu.";

/* ── The pre-check ──────────────────────────────────────────────────────────────────────────── */

export const AUDIO_MAX_BYTES = 30 * 1024 * 1024;

/** `accept` of the input. Extensions first: browsers disagree on the type of an `.m4a`. */
export const AUDIO_ACCEPT = ".mp3,.m4a,audio/mpeg,audio/mp4,audio/x-m4a";

/**
 * Browser-reported type → the type to DECLARE. Only the two the policy lists are declared; the aliases
 * are what real systems report for the same files (`audio/x-m4a` on Windows/Chrome, `audio/mp3`).
 */
const BY_BROWSER_TYPE: Readonly<Record<string, string>> = {
  "audio/mpeg": "audio/mpeg",
  "audio/mp3": "audio/mpeg",
  "audio/mp4": "audio/mp4",
  "audio/x-m4a": "audio/mp4",
  "audio/m4a": "audio/mp4",
};

const BY_EXTENSION: Readonly<Record<string, string>> = {
  mp3: "audio/mpeg",
  m4a: "audio/mp4",
};

/**
 * The content type to declare, or a refusal. The browser's `type` first; empty → the extension. The
 * declaration is a claim: the server sniffs the bytes at completion (a renamed video is refused there).
 */
export function declaredAudioType(file: { readonly name: string; readonly type: string; readonly size: number }):
  | { readonly ok: true; readonly contentType: string }
  | { readonly ok: false; readonly message: string } {
  if (file.size <= 0) return { ok: false, message: AUDIO_EMPTY };
  const type =
    file.type !== ""
      ? BY_BROWSER_TYPE[file.type.toLowerCase()]
      : BY_EXTENSION[file.name.toLowerCase().split(".").pop() ?? ""];
  if (type === undefined) return { ok: false, message: AUDIO_TYPE_REFUSED };
  if (file.size > AUDIO_MAX_BYTES) return { ok: false, message: AUDIO_TOO_LARGE };
  return { ok: true, contentType: type };
}

/* ── The typed duration ─────────────────────────────────────────────────────────────────────── */

/** `domain.CheckAudioDuration`'s bounds: 1 second .. 6 hours. */
export const AUDIO_DURATION_MIN_SECONDS = 1;
export const AUDIO_DURATION_MAX_SECONDS = 6 * 60 * 60;

/**
 * Whole seconds (prototype `ContentItemForm.tsx:314-320`: `Thời lượng (giây)`, a number box) → seconds.
 * Only digits: a decimal or a sign is refused rather than rounded, since the figure is shown to residents
 * as typed (the server never measures it).
 */
export function parseAudioDuration(raw: string):
  | { readonly ok: true; readonly seconds: number }
  | { readonly ok: false; readonly message: string } {
  const s = raw.trim();
  if (!/^\d{1,6}$/.test(s)) return { ok: false, message: AUDIO_DURATION_FORMAT };
  const seconds = Number(s);
  if (seconds < AUDIO_DURATION_MIN_SECONDS || seconds > AUDIO_DURATION_MAX_SECONDS) {
    return { ok: false, message: AUDIO_DURATION_RANGE };
  }
  return { ok: true, seconds };
}

/** Bytes → `12,5 MB` (Vietnamese decimal comma). */
export function formatAudioSize(bytes: number | undefined): string {
  if (bytes === undefined || bytes <= 0) return "không rõ dung lượng";
  return `${(bytes / (1024 * 1024)).toFixed(1).replace(".", ",")} MB`;
}

/* ── The held file and the saved broadcast's audio ──────────────────────────────────────────── */

/**
 * A picked file with the duration typed for it. Never sent until the save returns: the declaration needs
 * the id of a SAVED `truyen-thanh` item.
 */
export type HeldAudio = { readonly file: File; readonly duration: string };

/**
 * Why the save cannot go, or `null`. A picked file needs a valid duration, and so does a duration EDITED
 * on a broadcast that already has its file (`durationEdited`). A broadcast saved with no file is allowed,
 * as in the prototype.
 */
export function heldAudioProblem(file: File | null, duration: string, durationEdited = false): string | null {
  if (file === null && !durationEdited) return null;
  const d = parseAudioDuration(duration);
  if (d.ok) return null;
  return duration.trim() === "" ? AUDIO_DURATION_NEEDED_TO_SAVE : d.message;
}

export const AUDIO_DURATION_NEEDED_TO_SAVE = "Nhập thời lượng của tệp âm thanh (số giây) rồi mới lưu.";

/** The audio half of the edit form's PATCH — the contract's two audio fields, nothing else. */
export type AudioPatch = Pick<comms_suaNoiDungVao, "audio_file_id" | "audio_duration_seconds">;

/**
 * What the edit form's PATCH carries for the audio of a broadcast that ALREADY has its file:
 *
 *   - a new file picked → `audio_file_id: ""`. The item holds ONE live file (platform `content-audio`,
 *     `max_files_per_subject` = 1) and a second declaration is 409 `audio_limit`, so a replacement is
 *     "remove, then upload" (service-comms `app/content_audio.go`). The removal rides in the same PATCH as
 *     the other boxes; the upload follows with the saved id. If that upload then fails, the item is left
 *     WITHOUT audio — the dialog says so and the officer picks the file again.
 *   - only the duration edited → `audio_duration_seconds` (the PATCH's "correct the typed duration").
 *   - nothing → `{}`.
 *
 * No file attached → `{}`: a picked file is uploaded after the save, and a duration alone means nothing
 * (422 `audio_all_or_none`).
 */
export function savedAudioPatch(
  attachedDuration: string | null,
  file: File | null,
  duration: string,
): AudioPatch {
  if (attachedDuration === null) return {};
  if (file !== null) return { audio_file_id: "" };
  if (duration.trim() === attachedDuration) return {};
  const d = parseAudioDuration(duration);
  return d.ok ? { audio_duration_seconds: d.seconds } : {};
}

/* ── The upload state and flow ──────────────────────────────────────────────────────────────── */

export type AudioUploadState =
  | { readonly kind: "idle" }
  | { readonly kind: "requesting" }
  | { readonly kind: "uploading"; readonly id: string; readonly percent: number }
  | { readonly kind: "checking"; readonly id: string }
  /** Attached to the item — the server already wrote it. */
  | { readonly kind: "ready"; readonly id: string; readonly file: comms_audioFileOut }
  /** The bytes are in the store: complete AGAIN (503 / 409 / no answer / a duration the server refused). */
  | { readonly kind: "retry"; readonly id: string; readonly message: string }
  /** For good: the pre-check, the declaration, the store, or the bytes (422 `audio_rejected`). */
  | { readonly kind: "refused"; readonly message: string };

export function audioInFlight(s: AudioUploadState): boolean {
  return s.kind === "requesting" || s.kind === "uploading" || s.kind === "checking";
}

/**
 * A completion answer → state. 422 `invalid_audio_duration` is NOT final: the duration is checked before
 * the file is touched, so it is still pending and completing again with a corrected duration works — and
 * a new upload would meet 409 `audio_limit` while that pending file holds the item's one slot.
 */
export function afterAudioCompletion(id: string, r: AudioCallResult<comms_audioFileOut>): AudioUploadState {
  if (r.ok) {
    return r.data.status === "ready" ? { kind: "ready", id, file: r.data } : { kind: "refused", message: AUDIO_NOT_READY };
  }
  if (r.status === 503 || r.status === 409 || r.status === 0 || r.code === "invalid_audio_duration") {
    return { kind: "retry", id, message: r.message };
  }
  return { kind: "refused", message: r.message };
}

export function audioStateText(s: AudioUploadState): string {
  switch (s.kind) {
    case "idle":
      return "";
    case "requesting":
      return "Đang xin tải tệp âm thanh lên…";
    case "uploading":
      return `Đang tải lên ${s.percent}%`;
    case "checking":
      return "Đang kiểm tra tệp (dò kiểu, kiểm đúng là âm thanh, quét mã độc)…";
    case "ready":
      return "Tệp âm thanh đã tải lên, kiểm tra xong và đã gắn vào mục.";
    case "retry":
      return `Chưa hoàn tất: ${s.message}`;
    case "refused":
      return `Bị từ chối: ${s.message}`;
  }
}

/**
 * a → b → c for ONE file of a SAVED broadcast, reporting each state. The duration is parsed first: a file
 * is never declared without a duration the server will accept.
 */
export async function runAudioUpload(
  file: File,
  contentItemId: string,
  durationRaw: string,
  onState: (s: AudioUploadState) => void,
): Promise<AudioUploadState> {
  const report = (s: AudioUploadState) => {
    onState(s);
    return s;
  };
  const d = parseAudioDuration(durationRaw);
  if (!d.ok) return report({ kind: "refused", message: d.message });
  const t = declaredAudioType(file);
  if (!t.ok) return report({ kind: "refused", message: t.message });

  report({ kind: "requesting" });
  const req = await requestAudioUpload(
    { content_item_id: contentItemId, file_name: file.name, content_type: t.contentType, size: file.size },
    crypto.randomUUID(),
  );
  // 400 / 404 / 409 `audio_limit` / 422 `audio_only_for_truyen_thanh` / 503 — the server's sentence verbatim.
  if (!req.ok) return report({ kind: "refused", message: req.message });

  const id = req.data.audio_file.id;
  report({ kind: "uploading", id, percent: 0 });
  // The form (`req.data.upload`) is used here and dropped.
  const up = await uploadToStorage(req.data.upload, file, file.name, (percent) =>
    onState({ kind: "uploading", id, percent }),
  );
  if (!up.ok) return report({ kind: "refused", message: AUDIO_STORAGE_FAILED });

  return completeAudio(id, d.seconds, onState);
}

/** c. alone — `Hoàn tất lại`: the bytes are already in the store, never re-upload. */
export async function retryAudioCompletion(
  id: string,
  durationRaw: string,
  onState: (s: AudioUploadState) => void,
): Promise<AudioUploadState> {
  const d = parseAudioDuration(durationRaw);
  if (!d.ok) {
    const s: AudioUploadState = { kind: "retry", id, message: d.message };
    onState(s);
    return s;
  }
  return completeAudio(id, d.seconds, onState);
}

async function completeAudio(
  id: string,
  seconds: number,
  onState: (s: AudioUploadState) => void,
): Promise<AudioUploadState> {
  onState({ kind: "checking", id });
  const s = afterAudioCompletion(id, await completeAudioUpload(id, seconds));
  onState(s);
  return s;
}
