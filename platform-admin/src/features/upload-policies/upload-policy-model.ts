import type { UploadPolicy, UploadPolicyChange } from "@/lib/api";
import type { UploadPolicyField } from "@/lib/errors";

/**
 * The pure half of the upload-limit screen: unit conversion and building the PUT body. No React, so
 * the conversion the operator relies on is tested on its own (`wave2.test.tsx`).
 *
 * 1 MB HERE = 1 048 576 BYTES (MiB), the unit service-platform's caps are written in
 * (`service-platform/internal/app/upload_policy.go` `mib`): a 50 MB cap shows as exactly 50, not 52.43.
 * The hint under the input says so — an operator typing "10" must know what the server stores.
 */

export const BYTES_PER_MB = 1024 * 1024;

/** Bytes as MB, at most 2 decimals, without trailing zeros: 52428800 → "50", 1572864 → "1.5". */
export function bytesToMegabytes(bytes: number): string {
  const mb = Math.round((bytes / BYTES_PER_MB) * 100) / 100;
  return String(mb);
}

/** For reading: "50 MB", with the Vietnamese decimal comma. */
export function formatMegabytes(bytes: number): string {
  return bytesToMegabytes(bytes).replace(".", ",") + " MB";
}

/**
 * The operator's MB text as bytes. Accepts a dot or a comma as the decimal mark, at most 2 decimals.
 * null = not a positive number. Rounded to whole bytes — the server takes an integer.
 */
export function megabytesToBytes(text: string): number | null {
  const t = text.trim().replace(",", ".");
  if (!/^\d+(\.\d{1,2})?$/.test(t)) return null;
  const bytes = Math.round(Number(t) * BYTES_PER_MB);
  return bytes > 0 && Number.isSafeInteger(bytes) ? bytes : null;
}

/** What the edit dialog holds. `limited` false = "Không giới hạn" — sent as JSON null, never 0. */
export type UploadPolicyForm = {
  megabytes: string;
  mimeTypes: readonly string[];
  limited: boolean;
  maxFiles: string;
  reason: string;
};

export const MAX_FILES_CAP = 100;

export function formFromPolicy(p: UploadPolicy): UploadPolicyForm {
  return {
    megabytes: bytesToMegabytes(p.max_bytes),
    mimeTypes: p.allowed_mime_types,
    limited: p.max_files_per_subject !== null,
    maxFiles: p.max_files_per_subject === null ? "" : String(p.max_files_per_subject),
    reason: "",
  };
}

/**
 * The PUT body, or the first field to fix. Checks only what the screen can see (the cap the server
 * sent, the choices it offered); the server checks everything again.
 */
export function buildUploadPolicyChange(
  form: UploadPolicyForm,
  policy: Pick<UploadPolicy, "max_bytes_cap" | "mime_choices">,
): { ok: true; body: UploadPolicyChange } | { ok: false; field: UploadPolicyField; text: string } {
  const bytes = megabytesToBytes(form.megabytes);
  if (bytes === null) {
    return { ok: false, field: "maxBytes", text: "Nhập dung lượng tối đa bằng MB, là số lớn hơn 0 (tối đa 2 chữ số thập phân)." };
  }
  if (bytes > policy.max_bytes_cap) {
    return {
      ok: false,
      field: "maxBytes",
      text: `Dung lượng tối đa không được vượt mức trần ${formatMegabytes(policy.max_bytes_cap)} của mục đích này.`,
    };
  }
  // In the order the server offered them, so the same choice always sends the same list.
  const mimes = policy.mime_choices.filter((m) => form.mimeTypes.includes(m));
  if (mimes.length === 0) return { ok: false, field: "mimeTypes", text: "Chọn ít nhất một kiểu tệp." };
  let maxFiles: number | null = null;
  if (form.limited) {
    const t = form.maxFiles.trim();
    const n = Number(t);
    if (!/^\d+$/.test(t) || n < 1 || n > MAX_FILES_CAP) {
      return { ok: false, field: "maxFiles", text: `Số tệp tối đa phải là số nguyên từ 1 đến ${MAX_FILES_CAP}.` };
    }
    maxFiles = n;
  }
  if (form.reason.trim() === "") return { ok: false, field: "reason", text: "Hãy ghi lý do sửa." };
  return {
    ok: true,
    body: { max_bytes: bytes, allowed_mime_types: mimes, max_files_per_subject: maxFiles, reason: form.reason },
  };
}

/**
 * core/storage purposes (`core/storage/key.go`) in words. An unknown purpose is shown as its code,
 * never guessed; the code is always shown under the words too, so nobody has to trust the mapping.
 */
const PURPOSE_LABELS: Record<string, string> = {
  "tenant-logo": "Logo của xã",
  "tenant-banner": "Ảnh bìa trang của xã",
  "petition-photo": "Ảnh hiện trường của phản ánh",
  "petition-verification-photo": "Ảnh xác minh phản ánh",
  "petition-log-attachment": "Tệp đính kèm khi xử lý phản ánh",
  "content-image": "Ảnh bìa bài viết Mini App",
  "content-body-image": "Ảnh trong nội dung bài viết",
  "content-video": "Video bài viết",
  "content-audio": "Tệp âm thanh truyền thanh",
  "content-attachment": "Tệp đính kèm bài viết",
  "document-scan": "Bản quét văn bản",
  "task-attachment": "Tệp đính kèm công việc",
  "staff-avatar": "Ảnh đại diện cán bộ",
};

export function purposeLabel(purpose: string): string {
  return PURPOSE_LABELS[purpose] ?? purpose;
}

const MIME_LABELS: Record<string, string> = {
  "image/jpeg": "Ảnh JPEG",
  "image/png": "Ảnh PNG",
  "image/webp": "Ảnh WebP",
  "image/heic": "Ảnh HEIC",
  "application/pdf": "Tài liệu PDF",
  "video/mp4": "Video MP4",
  "video/quicktime": "Video MOV",
  "audio/mpeg": "Âm thanh MP3",
  "audio/mp4": "Âm thanh M4A",
};

export function mimeLabel(mime: string): string {
  return MIME_LABELS[mime] ?? mime;
}
