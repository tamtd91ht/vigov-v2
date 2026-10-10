/**
 * Images INSIDE the article body (ADR 0067 §Sửa đổi 03/10/2026, H1–H10, K1–K8): the two ways in, the
 * words for every refusal, and the preview `src`.
 *
 *   Tải ảnh từ máy        ONE multipart request to this origin (ADR 0052 §Sửa đổi 09/10/2026; the
 *                         cover's flow, `cover-image.ts`, same pre-check)
 *   Dán liên kết ảnh      the SERVER downloads the https link (SSRF wall, sniff, scan, 1280 px copy, K6)
 *
 * Both end in a READY file with a server-signed preview; only then does the editor insert
 * `<figure><img data-file-id="…">` at the cursor (`rich-text.ts`). The body never holds a URL (K2).
 *
 * ONE ARTICLE ID FOR EVERY FILE OF AN UNSAVED ARTICLE. The first body image of a new article reserves
 * the article's id at the server (`content_item_id` in the reply); the form keeps it and sends it on
 * every later body image AND on the cover — otherwise the save finds files of two different drafts and
 * answers 422. The edit form starts with the saved article's id. A cover uploaded FIRST reserves it the
 * same way: its upload reply names `content_item_id`, and the form keeps that id.
 *
 * The order still held off is the WHOLE FLIGHT of a first upload on a new article: the id arrives only
 * with the reply, and two uploads in flight with no id would reserve two drafts. `BODY_IMAGE_WAIT_COVER`
 * says so while the cover is in flight; the cover picker is disabled while a body image is.
 */

import { fetchBodyImageFromUrl, uploadBodyImage, type CodedCallResult } from "@/lib/api/noi-dung";
import type { comms_bodyImageFileOut, comms_bodyImageOut } from "@/lib/api/schema.gen";
import { isTransientUpload } from "@/lib/api/upload";

import { declaredCoverType, signedPreviewSrc } from "./cover-image";
import { isHttpsLink } from "./rich-text";

/**
 * The owner's 03/10/2026 cap (H7): at most 20 images in one body. CONVENIENCE ONLY — the platform's
 * `content-body-image` policy is the real limit and its 409 `body_image_limit` is still worded below.
 */
export const BODY_IMAGE_MAX = 20;

export const BODY_IMAGE_LIMIT_REACHED = `Bài đã có ${BODY_IMAGE_MAX} ảnh trong thân bài — gỡ bớt một ảnh để chèn ảnh khác.`;
export const BODY_IMAGE_WAIT_COVER = "Đang tải ảnh bìa lên — chờ tải xong rồi chèn ảnh vào thân bài.";
export const BODY_IMAGE_URL_INVALID =
  "Liên kết ảnh phải bắt đầu bằng https:// và có tên miền, không có dấu cách.";
export const BODY_IMAGE_NO_PREVIEW = "Ảnh chưa xem trước được — lưu rồi mở lại để xem";
export const BODY_IMAGE_NOT_READY = "Ảnh chưa sẵn sàng để chèn vào bài. Hãy chọn lại ảnh.";

/**
 * One sentence per refusal code of the two routes. The SAME code means the same thing on both, so one
 * table. A code it does not know keeps the server's own sentence (`bodyImageErrorText`).
 */
const SENTENCES: Readonly<Record<string, string>> = {
  invalid_request: "Ảnh này không đúng loại hoặc kích thước được phép — chỉ nhận JPG, PNG hoặc WebP.",
  invalid_image_url:
    "Liên kết ảnh không hợp lệ: cần là địa chỉ https có tên miền, không kèm cổng khác hay thông tin đăng nhập.",
  body_image_limit:
    "Bài đã có đủ số ảnh tối đa trong thân bài. Hãy gỡ bớt ảnh, bấm Lưu, rồi chèn ảnh mới.",
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

/**
 * The officer's sentence for a refused call. `not_found` names the ARTICLE on both routes. A code this
 * table does not know (403, the upload envelope's 400/408/413/503 included) keeps the server's own
 * sentence, which is Vietnamese — a raw code never reaches the screen.
 */
export function bodyImageErrorText(r: { readonly code: string; readonly message: string }): string {
  if (r.code === "not_found") return NOT_FOUND_ARTICLE;
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
  | { readonly kind: "uploading"; readonly percent: number }
  /** Every byte sent: the server is sniffing, scanning, deriving the 1280 px copy. */
  | { readonly kind: "checking" }
  | { readonly kind: "fetching" }
  | { readonly kind: "ready"; readonly image: BodyImageReady }
  /** A refusal of the moment on an upload (`isTransientUpload`): `Gửi lại` sends `file` again. */
  | { readonly kind: "retry"; readonly file: File; readonly message: string }
  | { readonly kind: "refused"; readonly message: string };

export function bodyImageInFlight(s: BodyImageState): boolean {
  return s.kind === "uploading" || s.kind === "checking" || s.kind === "fetching";
}

/** The line under the insert panel. */
export function bodyImageStateText(s: BodyImageState): string {
  switch (s.kind) {
    case "idle":
    case "ready":
      return "";
    case "uploading":
      return `Đang tải lên ${s.percent}%`;
    case "checking":
      return "Đang kiểm tra ảnh (dò kiểu, quét mã độc)…";
    case "fetching":
      return "Máy chủ đang tải ảnh từ liên kết và kiểm tra…";
    case "retry":
      return `Chưa gửi được: ${s.message}`;
    case "refused":
      return s.message;
  }
}

/** A file reply → state. `file` is given for an upload only: then a refusal of the moment may be re-sent. */
function fromFileReply(r: CodedCallResult<comms_bodyImageFileOut>, file?: File): BodyImageState {
  if (r.ok) {
    if (r.data.status !== "ready" || !r.data.content_item_id) return { kind: "refused", message: BODY_IMAGE_NOT_READY };
    return {
      kind: "ready",
      image: { fileId: r.data.id, contentItemId: r.data.content_item_id, previewUrl: r.data.preview_url },
    };
  }
  const message = bodyImageErrorText(r);
  if (file !== undefined && isTransientUpload(r)) return { kind: "retry", file, message };
  return { kind: "refused", message };
}

/**
 * Tải ảnh từ máy: ONE request. `articleId` is the form's article id (undefined on a new article before its
 * first file); `onArticle` hears the id the reply names. `Gửi lại` calls this again with the retry's file.
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

  report({ kind: "uploading", percent: 0 });
  const r = await uploadBodyImage({ file, contentType: t.contentType, contentItemId: articleId }, crypto.randomUUID(), {
    onProgress: (percent) => onState({ kind: "uploading", percent }),
    onSent: () => onState({ kind: "checking" }),
  });
  if (r.ok && r.data.content_item_id) onArticle(r.data.content_item_id);
  return report(fromFileReply(r, file));
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
  const s = fromFileReply(await fetchBodyImageFromUrl({ url: link, content_item_id: articleId }, crypto.randomUUID()));
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
