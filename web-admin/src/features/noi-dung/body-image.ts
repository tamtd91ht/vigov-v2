/**
 * Images INSIDE the article body (ADR 0067 §Sửa đổi 03/10/2026, H1–H10, K1–K8): the two ways in, the
 * words for every refusal, and the preview `src`.
 *
 *   Tải ảnh từ máy        declare → presigned POST straight to the store → completion (the cover's flow,
 *                         `cover-image.ts`, same pre-check and the same `uploadToStorage`)
 *   Dán liên kết ảnh      the SERVER downloads the https link (SSRF wall, sniff, scan, 1280 px copy, K6)
 *
 * Both end in a READY file with a server-signed preview; only then does the editor insert
 * `<figure><img data-file-id="…">` at the cursor (`rich-text.ts`). The body never holds a URL (K2).
 *
 * ONE ARTICLE ID FOR EVERY FILE OF AN UNSAVED ARTICLE. The first body image of a new article reserves
 * the article's id at the server (`content_item_id` in the reply); the form keeps it and sends it on
 * every later body image AND on the cover — otherwise the save finds files of two different drafts and
 * answers 422. The edit form starts with the saved article's id.
 *
 * ⚠ CONTRACT GAP (reported, not patched here): the COVER's reply carries no `content_item_id`. A cover
 * uploaded FIRST on a new article reserves an id this screen never learns, so a body image after it would
 * reserve a second one. Until the cover's reply carries it, that order is held off with a sentence
 * (`BODY_IMAGE_AFTER_COVER`) instead of failing at Lưu.
 */

import {
  completeBodyImageUpload,
  fetchBodyImageFromUrl,
  requestBodyImageUpload,
  type CodedCallResult,
} from "@/lib/api/noi-dung";
import type { comms_bodyImageFileOut, comms_bodyImageOut } from "@/lib/api/schema.gen";
import { uploadToStorage } from "@/lib/api/task-attachments";

import { COVER_STORAGE_FAILED, declaredCoverType, signedPreviewSrc } from "./cover-image";
import { isHttpsLink } from "./rich-text";

/**
 * The owner's 03/10/2026 cap (H7): at most 20 images in one body. CONVENIENCE ONLY — the platform's
 * `content-body-image` policy is the real limit and its 409 `body_image_limit` is still worded below.
 */
export const BODY_IMAGE_MAX = 20;

export const BODY_IMAGE_LIMIT_REACHED = `Bài đã có ${BODY_IMAGE_MAX} ảnh trong thân bài — gỡ bớt một ảnh để chèn ảnh khác.`;
export const BODY_IMAGE_AFTER_COVER =
  "Ảnh bìa của bài mới đã tải lên trước, nên chưa chèn được ảnh vào thân bài: hãy bấm Lưu rồi mở lại bài để chèn ảnh.";
export const BODY_IMAGE_URL_INVALID =
  "Liên kết ảnh phải bắt đầu bằng https:// và có tên miền, không có dấu cách.";
export const BODY_IMAGE_NO_PREVIEW = "Ảnh chưa xem trước được — lưu rồi mở lại để xem";
export const BODY_IMAGE_NOT_READY = "Ảnh chưa sẵn sàng để chèn vào bài. Hãy chọn lại ảnh.";

/**
 * One sentence per refusal code of the three routes. The SAME code means the same thing on every route
 * except `not_found` (see `bodyImageErrorText`), so one table.
 */
const SENTENCES: Readonly<Record<string, string>> = {
  invalid_request: "Ảnh này không đúng loại hoặc kích thước được phép — chỉ nhận JPG, PNG hoặc WebP.",
  invalid_image_url:
    "Liên kết ảnh không hợp lệ: cần là địa chỉ https có tên miền, không kèm cổng khác hay thông tin đăng nhập.",
  body_image_limit:
    "Bài đã có đủ số ảnh tối đa trong thân bài. Hãy gỡ bớt ảnh, bấm Lưu, rồi chèn ảnh mới.",
  body_image_state: "Lượt tải ảnh này đã bị từ chối hoặc đã hết hạn. Hãy chọn ảnh và tải lên lại.",
  upload_not_received: "Kho lưu tệp chưa nhận được ảnh. Hãy chọn ảnh và tải lên lại.",
  upload_expired: "Lượt tải ảnh đã quá 15 phút và hết hạn. Hãy chọn ảnh và tải lên lại.",
  upload_changed: "Tệp trên kho lưu đã bị thay đổi sau khi tải lên. Hãy chọn ảnh và tải lên lại.",
  body_image_rejected:
    "Ảnh bị từ chối (không đúng định dạng, hỏng, hoặc không qua được kiểm tra mã độc) nên không được chèn.",
  image_fetch_failed:
    "Không tải được ảnh từ liên kết này. Hãy kiểm tra liên kết, hoặc tải ảnh về máy rồi chọn “Tải ảnh từ máy”.",
  storage_not_configured:
    "Hệ thống chưa cấu hình kho lưu tệp nên chưa chèn được ảnh. Hãy báo quản trị hệ thống.",
  upload_limits_unavailable: "Chưa đọc được giới hạn tải ảnh của hệ thống. Hãy thử lại sau ít phút.",
  malware_scan_unavailable: "Máy quét mã độc đang tạm ngưng nên chưa kiểm tra được ảnh. Hãy thử lại sau ít phút.",
};

const NOT_FOUND_ARTICLE = "Không tìm thấy bài để gắn ảnh nữa — hãy đóng biểu mẫu, mở lại bài rồi chèn ảnh.";
const NOT_FOUND_FILE = "Không tìm thấy lượt tải ảnh này nữa. Hãy chọn ảnh và tải lên lại.";

/**
 * The officer's sentence for a refused call. `not_found` names the ARTICLE on the two routes that name
 * one, and the FILE at completion. A code this table does not know (403 included) keeps the server's own
 * sentence, which is Vietnamese — a raw code never reaches the screen.
 */
export function bodyImageErrorText(
  step: "request" | "complete" | "from-url",
  r: { readonly code: string; readonly message: string },
): string {
  if (r.code === "not_found") return step === "complete" ? NOT_FOUND_FILE : NOT_FOUND_ARTICLE;
  return SENTENCES[r.code] ?? r.message;
}

/** What a ready image gives the form: the id for the body, the article it belongs to, its preview. */
export type BodyImageReady = {
  readonly fileId: string;
  readonly contentItemId: string;
  readonly previewUrl: string | undefined;
};

/** Where the one insert in progress stands. */
export type BodyImageState =
  | { readonly kind: "idle" }
  | { readonly kind: "requesting" }
  | { readonly kind: "uploading"; readonly percent: number }
  | { readonly kind: "checking"; readonly id: string }
  | { readonly kind: "fetching" }
  | { readonly kind: "ready"; readonly image: BodyImageReady }
  /** 503 / no answer at completion: the bytes are in the store — retry the COMPLETION, not the upload. */
  | { readonly kind: "retry"; readonly id: string; readonly message: string }
  | { readonly kind: "refused"; readonly message: string };

export function bodyImageInFlight(s: BodyImageState): boolean {
  return s.kind === "requesting" || s.kind === "uploading" || s.kind === "checking" || s.kind === "fetching";
}

/** The line under the insert panel. */
export function bodyImageStateText(s: BodyImageState): string {
  switch (s.kind) {
    case "idle":
    case "ready":
      return "";
    case "requesting":
      return "Đang xin tải ảnh lên…";
    case "uploading":
      return `Đang tải lên ${s.percent}%`;
    case "checking":
      return "Đang kiểm tra ảnh (dò kiểu, quét mã độc)…";
    case "fetching":
      return "Máy chủ đang tải ảnh từ liên kết và kiểm tra…";
    case "retry":
      return `Chưa kiểm tra xong: ${s.message}`;
    case "refused":
      return s.message;
  }
}

function fromFileReply(
  r: CodedCallResult<comms_bodyImageFileOut>,
  step: "complete" | "from-url",
  id: string,
): BodyImageState {
  if (r.ok) {
    if (r.data.status !== "ready" || !r.data.content_item_id) return { kind: "refused", message: BODY_IMAGE_NOT_READY };
    return {
      kind: "ready",
      image: { fileId: r.data.id, contentItemId: r.data.content_item_id, previewUrl: r.data.preview_url },
    };
  }
  const message = bodyImageErrorText(step, r);
  if (step === "complete" && (r.status === 503 || r.status === 0)) return { kind: "retry", id, message };
  return { kind: "refused", message };
}

/**
 * Tải ảnh từ máy: declare → store → complete. `articleId` is the form's article id (undefined on a new
 * article before its first file); `onArticle` hears the id the declaration answered with AS SOON AS it is
 * known, so a later failure of this image still leaves the form on the same reserved article.
 */
export async function runBodyImageUpload(
  file: File,
  articleId: string | undefined,
  onState: (s: BodyImageState) => void,
  onArticle: (id: string) => void,
): Promise<BodyImageState> {
  const report = (s: BodyImageState) => {
    onState(s);
    return s;
  };
  const t = declaredCoverType(file);
  if (!t.ok) return report({ kind: "refused", message: t.message });

  report({ kind: "requesting" });
  const req = await requestBodyImageUpload(
    { file_name: file.name, content_type: t.contentType, size: file.size, content_item_id: articleId },
    crypto.randomUUID(),
  );
  if (!req.ok) return report({ kind: "refused", message: bodyImageErrorText("request", req) });
  onArticle(req.data.content_item_id);

  const id = req.data.body_image.id;
  report({ kind: "uploading", percent: 0 });
  const up = await uploadToStorage(req.data.upload, file, file.name, (percent) =>
    onState({ kind: "uploading", percent }),
  );
  if (!up.ok) return report({ kind: "refused", message: COVER_STORAGE_FAILED });

  return retryBodyImageCompletion(id, onState);
}

/** The completion alone — `Kiểm tra lại`: the bytes are already in the store, never re-upload. */
export async function retryBodyImageCompletion(
  id: string,
  onState: (s: BodyImageState) => void,
): Promise<BodyImageState> {
  onState({ kind: "checking", id });
  const s = fromFileReply(await completeBodyImageUpload(id), "complete", id);
  onState(s);
  return s;
}

/** Dán liên kết ảnh: the https check here is convenience; the server's wall decides (400 / 502). */
export async function runBodyImageFromUrl(
  url: string,
  articleId: string | undefined,
  onState: (s: BodyImageState) => void,
): Promise<BodyImageState> {
  const link = url.trim();
  if (!isHttpsLink(link)) {
    const s: BodyImageState = { kind: "refused", message: BODY_IMAGE_URL_INVALID };
    onState(s);
    return s;
  }
  onState({ kind: "fetching" });
  const s = fromFileReply(
    await fetchBodyImageFromUrl({ url: link, content_item_id: articleId }, crypto.randomUUID()),
    "from-url",
    "",
  );
  onState(s);
  return s;
}

/**
 * THE `src` OF A BODY IMAGE IN THE EDITOR: the server-signed preview of the 1280 px derivative, from the
 * detail's `body_images` or a completion / from-url reply — parsed by the cover's rule (http(s) only).
 * Never a `blob:`, never the link the officer pasted. `ranh-gioi-html.test.ts` holds the NodeView to it.
 */
export function bodyImagePreviewSrc(raw: string | undefined): string | null {
  return signedPreviewSrc(raw);
}

/** The preview map a saved article opens with: `file_id → preview_url`, only for images that have one. */
export function previewsFromItem(images: readonly comms_bodyImageOut[] | undefined): ReadonlyMap<string, string> {
  const out = new Map<string, string>();
  for (const im of images ?? []) {
    if (im.preview_url) out.set(im.file_id, im.preview_url);
  }
  return out;
}
