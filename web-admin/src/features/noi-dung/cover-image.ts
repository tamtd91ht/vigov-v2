/**
 * §7 `Ảnh đại diện` — the cover upload: the pre-check, the one-file state, the words, the flow.
 *
 * THE SERVER'S POLICY DECIDES (platform's `content-image` limits, ADR 0052 §10). The pre-check here
 * is the user's decision of 01/10/2026 (ADR 0047 §6) written as a CONVENIENCE: JPG, PNG or WebP, at
 * most 50 MB, NO HEIC — it only spares an upload the server would refuse. Whatever the server
 * answers is shown word for word, and a file that passes here can still be refused there.
 *
 * 50 MB IS COUNTED AS 50 × 1024 × 1024 BYTES. The spec writes "50MB" without saying which; the
 * larger reading is chosen so this check can never refuse a file the server's policy would accept —
 * if the policy counts 50 000 000, the server's own sentence refuses the gap.
 *
 * The flow (`runCoverUpload`) is ONE multipart request (ADR 0052 §Sửa đổi 09/10/2026,
 * `uploadCoverImage` → `lib/api/upload.ts`).
 */

import { uploadCoverImage } from "@/lib/api/noi-dung";
import { isTransientUpload, type UploadResult } from "@/lib/api/upload";
import type { comms_coverFileOut, comms_coverImageOut } from "@/lib/api/schema.gen";

/** §7 and the prototype's `FilePicker` (`ContentItemForm.tsx:338-350`, `:421-425`), verbatim. */
export const COVER_LABEL = "Ảnh đại diện";
export const COVER_BANNER_LABEL = "Ảnh banner";
export const COVER_PICK_BUTTON = "Chọn tệp từ máy";
export const COVER_HINT = "JPG, PNG hoặc WebP — tối đa 50MB";
export const COVER_BANNER_HINT = "JPG, PNG hoặc WebP — nên dùng ảnh ngang, tỷ lệ khoảng 16:9";
/** The slot's text once the article has a cover (saved or just uploaded) and no new file is chosen. */
export const COVER_REPLACE_BUTTON = "Đã có tệp — chọn tệp mới để thay";

/** `2.4MB` — the chosen file's size, as the prototype prints it next to the slot. */
export function coverSizeLabel(bytes: number): string {
  return `${(bytes / 1024 / 1024).toFixed(1)}MB`;
}
export const COVER_REMOVE_BUTTON = "Gỡ ảnh";
export const COVER_RETRY_BUTTON = "Gửi lại";

export const COVER_MAX_BYTES = 50 * 1024 * 1024;

/** `accept` of the input AND the check — one list. No HEIC, no GIF, no SVG. */
export const COVER_TYPES: Readonly<Record<string, string>> = {
  "image/jpeg": "JPG",
  "image/png": "PNG",
  "image/webp": "WebP",
};
export const COVER_ACCEPT = ".jpg,.jpeg,.png,.webp,image/jpeg,image/png,image/webp";

const BY_EXTENSION: Readonly<Record<string, string>> = {
  jpg: "image/jpeg",
  jpeg: "image/jpeg",
  png: "image/png",
  webp: "image/webp",
};

export const COVER_TYPE_REFUSED = "Chỉ nhận ảnh JPG, PNG hoặc WebP (không nhận HEIC).";
export const COVER_TOO_LARGE = "Ảnh lớn hơn 50MB — hãy chọn ảnh nhỏ hơn.";
export const COVER_EMPTY = "Tệp rỗng — không có ảnh nào để tải lên.";
export const COVER_NOT_READY = "Ảnh chưa sẵn sàng để gắn vào bài. Hãy chọn lại ảnh.";
export const COVER_WAIT_NOTE = "Chờ ảnh tải lên và kiểm tra xong rồi mới lưu.";

/**
 * The content type to DECLARE, or a refusal. The browser's `type` first; empty (some systems report
 * none) → the extension. The declaration is a claim: the server sniffs the bytes in the same request.
 */
export function declaredCoverType(file: { readonly name: string; readonly type: string; readonly size: number }):
  | { readonly ok: true; readonly contentType: string }
  | { readonly ok: false; readonly message: string } {
  if (file.size <= 0) return { ok: false, message: COVER_EMPTY };
  let type: string | undefined;
  if (file.type !== "") {
    type = COVER_TYPES[file.type] !== undefined ? file.type : undefined;
  } else {
    type = BY_EXTENSION[file.name.toLowerCase().split(".").pop() ?? ""];
  }
  if (type === undefined) return { ok: false, message: COVER_TYPE_REFUSED };
  if (file.size > COVER_MAX_BYTES) return { ok: false, message: COVER_TOO_LARGE };
  return { ok: true, contentType: type };
}

/** Where the one upload of the form stands. `idle` = nothing moving, nothing to report. */
export type CoverUploadState =
  | { readonly kind: "idle" }
  | { readonly kind: "uploading"; readonly percent: number }
  /** Every byte sent: the server is sniffing, scanning, deriving the 1280 px copy. */
  | { readonly kind: "checking" }
  /** `ready` — its id is now the form's `cover_image_file_id`. */
  | { readonly kind: "ready"; readonly id: string }
  /** A refusal of the moment (`isTransientUpload`): nothing stored — `Gửi lại` sends `file` again. */
  | { readonly kind: "retry"; readonly file: File; readonly message: string }
  /** For good: the pre-check, the limits, the type, or the scan (422). */
  | { readonly kind: "refused"; readonly message: string };

export function coverInFlight(s: CoverUploadState): boolean {
  return s.kind === "uploading" || s.kind === "checking";
}

/** An upload's answer → state. A refusal of the moment may be sent again; every other one is final. */
export function afterCoverUpload(file: File, r: UploadResult<comms_coverFileOut>): CoverUploadState {
  if (r.ok) return r.data.status === "ready" ? { kind: "ready", id: r.data.id } : { kind: "refused", message: COVER_NOT_READY };
  if (isTransientUpload(r)) return { kind: "retry", file, message: r.message };
  return { kind: "refused", message: r.message };
}

/** The line under the picker — `role="status"` while it moves, `role="alert"` when refused. */
export function coverStateText(s: CoverUploadState): string {
  switch (s.kind) {
    case "idle":
      return "";
    case "uploading":
      return `Đang tải lên ${s.percent}%`;
    case "checking":
      return "Đang kiểm tra ảnh (dò kiểu, quét mã độc)…";
    case "ready":
      return "Ảnh đã tải lên và kiểm tra xong — sẽ gắn vào bài khi bấm Lưu.";
    case "retry":
      return `Chưa gửi được: ${s.message}`;
    case "refused":
      return `Bị từ chối: ${s.message}`;
  }
}

/**
 * ONE file, one request, reporting each state. Returns the last state. `Gửi lại` calls this again with
 * the `retry` state's file.
 *
 * `contentItemId`: the form's article id — the saved article's on the EDIT form, the one a body image
 * already reserved on a new article, `undefined` before any file (the server reserves one). One
 * Idempotency-Key per attempt.
 *
 * `onArticle` hears the article id the reply names — the same contract as `runBodyImageUpload` — so a
 * body image inserted after a cover uploaded first on a new article names the SAME reserved article.
 * The reply is the only place it is known now: the whole upload is in flight until then, which is why
 * the form holds the body image back for the cover's WHOLE flight on a new article (`coverInFlight`).
 */
export async function runCoverUpload(
  file: File,
  contentItemId: string | undefined,
  onState: (s: CoverUploadState) => void,
  onArticle: (id: string) => void = () => {},
): Promise<CoverUploadState> {
  const report = (s: CoverUploadState) => {
    onState(s);
    return s;
  };
  const t = declaredCoverType(file);
  if (!t.ok) return report({ kind: "refused", message: t.message });

  report({ kind: "uploading", percent: 0 });
  const r = await uploadCoverImage({ file, contentType: t.contentType, contentItemId }, crypto.randomUUID(), {
    onProgress: (percent) => onState({ kind: "uploading", percent }),
    onSent: () => onState({ kind: "checking" }),
  });
  // The policy refusal, 503, 403, 404, 422 — the server's sentence verbatim.
  if (r.ok && r.data.content_item_id) onArticle(r.data.content_item_id);
  return report(afterCoverUpload(file, r));
}

/**
 * THE ONLY `src` THIS SCREEN BUILDS: the server's signed preview of the cover's DERIVATIVE (1280 px
 * JPEG, no EXIF), from the detail route's `cover_image.preview_url`.
 *
 * It reads `preview_url` and nothing else — never `image_url`, never `source_url`, never anything an
 * officer typed. And even the server's string is parsed: only an absolute `http(s)` URL becomes a
 * `src`. `ranh-gioi-html.test.ts` holds every `src={…}` of the screen to this function.
 */
export function coverPreviewSrc(cover: comms_coverImageOut | null | undefined): string | null {
  return signedPreviewSrc(cover?.preview_url);
}

/**
 * The parse behind `coverPreviewSrc` and `bodyImagePreviewSrc` (`body-image.ts`): a server-signed
 * `preview_url` → a `src`, only when it is an absolute http(s) URL. One rule for both previews.
 */
export function signedPreviewSrc(raw: string | undefined | null): string | null {
  if (raw === undefined || raw === null || raw === "") return null;
  let u: URL;
  try {
    u = new URL(raw);
  } catch {
    return null;
  }
  return u.protocol === "https:" || u.protocol === "http:" ? u.href : null;
}

/** The status line of the cover already on a saved article. */
export function savedCoverText(cover: comms_coverImageOut | null | undefined): string {
  if (cover === undefined || cover === null) return "Bài đã có ảnh bìa.";
  if (cover.status !== "ready") {
    return `Ảnh bìa chưa sẵn sàng (trạng thái: ${cover.status === "" ? "không đọc được" : cover.status}).`;
  }
  return cover.public
    ? "Ảnh bìa hiện tại — bà con đang thấy ảnh này trên Mini App."
    : "Ảnh bìa hiện tại — chưa công khai cho bà con.";
}

export const COVER_PREVIEW_ALT = "Ảnh bìa hiện tại của bài";
export const COVER_PREVIEW_MISSING = "Chưa lấy được ảnh xem trước — mở lại bài để thử lại.";
export const COVER_WILL_DETACH = "Ảnh bìa sẽ được gỡ khỏi bài khi bấm Lưu.";
export const COVER_NONE = "Bài chưa có ảnh bìa.";
