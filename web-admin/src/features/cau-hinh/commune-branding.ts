/**
 * Cấu hình › Nhận diện xã (ADR 0069) — the words, the pre-check, the one-upload state and the flow,
 * kept out of the `.tsx` so each refusal path has a test that does not need a browser.
 *
 * THE SERVER'S POLICY DECIDES. The pre-check spares an upload the server would refuse, with the
 * limits ADR 0069 #4–#5 fixes and platform's `upload_policy` rows hold today (`tenant-logo`,
 * `tenant-banner`: 2 097 152 bytes, PNG/WebP/JPEG, migration 0016). An operator can edit those rows;
 * whatever the server answers is shown word for word, and a file passing here can still be refused
 * there (sniffed type, malware scan, pixel limits — `service-platform/internal/http/branding.go`).
 *
 * 2 MB IS 2 × 1024 × 1024 BYTES — exactly the server's `max_bytes`, so this check never refuses a file
 * the policy accepts.
 */

import { formatDateTime } from "@/features/dashboard/period";
import type { platform_brandingFileOut } from "@/lib/api/schema.gen";
import {
  completeBrandingUpload,
  requestBrandingUpload,
  type BrandingImageKind,
} from "@/lib/api/commune-branding";
import { uploadToStorage, type CallResult } from "@/lib/api/task-attachments";

export const BRANDING_MAX_BYTES = 2 * 1024 * 1024;

/** `accept` of the input AND the check — one list. No HEIC (ADR 0069 #4), no GIF, no SVG. */
export const BRANDING_TYPES: Readonly<Record<string, string>> = {
  "image/png": "PNG",
  "image/webp": "WebP",
  "image/jpeg": "JPEG",
};
export const BRANDING_ACCEPT = ".png,.webp,.jpg,.jpeg,image/png,image/webp,image/jpeg";

const BY_EXTENSION: Readonly<Record<string, string>> = {
  png: "image/png",
  webp: "image/webp",
  jpg: "image/jpeg",
  jpeg: "image/jpeg",
};

/** Words of each card. Vietnamese: the officer reads them. */
export const BRANDING_TEXT: Readonly<
  Record<
    BrandingImageKind,
    {
      readonly title: string;
      readonly description: string;
      readonly hint: string;
      readonly empty: string;
      readonly previewLabel: string;
      readonly removeButton: string;
      readonly confirmTitle: string;
      readonly confirmConsequence: string;
      readonly removed: string;
      readonly saved: string;
    }
  >
> = {
  logo: {
    title: "Logo xã",
    description: "Hiện ở góc trái thanh bên, màn đăng nhập và Zalo Mini App của xã.",
    hint: "PNG nền trong, ảnh vuông, nên từ 512 × 512 px trở lên. PNG, WebP hoặc JPEG — tối đa 2 MB.",
    empty: "Xã chưa có logo — thanh bên và màn đăng nhập đang hiện biểu tượng toà nhà.",
    previewLabel: "Logo hiện tại của xã",
    removeButton: "Gỡ logo",
    confirmTitle: "Gỡ logo của xã?",
    confirmConsequence:
      "Thanh bên, màn đăng nhập và Zalo Mini App sẽ quay về biểu tượng toà nhà cho tới khi xã tải logo mới.",
    removed: "Đã gỡ logo. Thanh bên và màn đăng nhập quay về biểu tượng toà nhà.",
    saved: "Đã đặt logo mới — thanh bên và màn đăng nhập hiện logo này ngay.",
  },
  banner: {
    title: "Banner trang quản trị",
    description: "Dải ảnh ngang dưới thanh trên cùng ở mọi trang quản trị của xã. Không hiện trên Mini App.",
    hint: "Ảnh ngang, nên rộng từ 1600 px trở lên. PNG, WebP hoặc JPEG — tối đa 2 MB.",
    empty: "Chưa có banner — các trang quản trị không có dải ảnh dưới thanh trên cùng.",
    previewLabel: "Banner hiện tại của trang quản trị",
    removeButton: "Gỡ banner",
    confirmTitle: "Gỡ banner trang quản trị?",
    confirmConsequence: "Dải ảnh dưới thanh trên cùng sẽ không còn ở mọi trang quản trị của xã.",
    removed: "Đã gỡ banner. Các trang quản trị không còn dải ảnh.",
    saved: "Đã đặt banner mới — các trang quản trị hiện banner này ngay.",
  },
};

export const BRANDING_PICK_BUTTON = "Tải ảnh lên";
export const BRANDING_REPLACE_BUTTON = "Tải ảnh khác";
export const BRANDING_RETRY_BUTTON = "Hoàn tất lại";
export const BRANDING_CANCEL_BUTTON = "Huỷ";
export const BRANDING_EMPTY_FILE = "Tệp rỗng — không có ảnh nào để tải lên.";
export const BRANDING_TYPE_REFUSED = "Chỉ nhận ảnh PNG, WebP hoặc JPEG (không nhận HEIC).";
export const BRANDING_TOO_LARGE = "Ảnh lớn hơn 2 MB — hãy chọn ảnh nhỏ hơn.";
export const BRANDING_STORAGE_FAILED = "Chưa tải được ảnh lên kho lưu tệp. Hãy chọn lại ảnh.";
export const BRANDING_NOT_READY = "Ảnh chưa được đặt. Hãy chọn lại ảnh và tải lên lại.";

/**
 * The content type to DECLARE, or a refusal. The browser's `type` first; empty (some systems report
 * none) → the extension. The declaration is a claim: the server sniffs the bytes at completion.
 */
export function declaredBrandingType(file: { readonly name: string; readonly type: string; readonly size: number }):
  | { readonly ok: true; readonly contentType: string }
  | { readonly ok: false; readonly message: string } {
  if (file.size <= 0) return { ok: false, message: BRANDING_EMPTY_FILE };
  let type: string | undefined;
  if (file.type !== "") {
    type = BRANDING_TYPES[file.type] !== undefined ? file.type : undefined;
  } else {
    type = BY_EXTENSION[file.name.toLowerCase().split(".").pop() ?? ""];
  }
  if (type === undefined) return { ok: false, message: BRANDING_TYPE_REFUSED };
  if (file.size > BRANDING_MAX_BYTES) return { ok: false, message: BRANDING_TOO_LARGE };
  return { ok: true, contentType: type };
}

/** Where one card's upload stands. `idle` = nothing moving, nothing to report. */
export type BrandingUploadState =
  | { readonly kind: "idle" }
  | { readonly kind: "requesting" }
  | { readonly kind: "uploading"; readonly id: string; readonly percent: number }
  | { readonly kind: "checking"; readonly id: string }
  /** Published and set as current: the screen re-reads the settings and refreshes the shell. */
  | { readonly kind: "done" }
  /** 503 / 409 / no answer at completion: the bytes are stored — retry the COMPLETION, never re-upload. */
  | { readonly kind: "retry"; readonly id: string; readonly message: string }
  /** For good: the pre-check, the declaration, the store, or the scan / decode (422). */
  | { readonly kind: "refused"; readonly message: string };

export function brandingInFlight(s: BrandingUploadState): boolean {
  return s.kind === "requesting" || s.kind === "uploading" || s.kind === "checking";
}

/**
 * A completion answer → state. 422 is final. 503 (`image_processing_busy`, scanner or store not
 * reachable), 409 (not received yet / changed) and no answer: retry the completion — the server's
 * own sentence says "bấm hoàn tất lại" for the busy case. Some 409s are final (`upload_expired`) and
 * their sentence says "chọn ảnh và tải lên lại"; the screen offers both buttons, and retrying a final
 * one only repeats the same sentence. NEVER branched on `code` (`lib/api/goi.ts`).
 */
export function afterBrandingCompletion(id: string, r: CallResult<platform_brandingFileOut>): BrandingUploadState {
  if (r.ok) return r.data.status === "ready" ? { kind: "done" } : { kind: "refused", message: BRANDING_NOT_READY };
  if (r.status === 503 || r.status === 409 || r.status === 0) return { kind: "retry", id, message: r.message };
  return { kind: "refused", message: r.message };
}

/** The line under the card's buttons — `role="status"` while it moves, `role="alert"` when refused. */
export function brandingStateText(s: BrandingUploadState): string {
  switch (s.kind) {
    case "idle":
    case "done":
      return "";
    case "requesting":
      return "Đang tải lên…";
    case "uploading":
      return `Đang tải lên… ${s.percent}%`;
    case "checking":
      return "Đang kiểm tra ảnh (dò kiểu, quét mã độc, chuẩn hoá)…";
    case "retry":
      return s.message;
    case "refused":
      return s.message;
  }
}

/**
 * The flow a → b → c for ONE file, reporting each state. Returns the last state. `newKey` mints the
 * Idempotency-Key — `crypto.randomUUID()` at the call site, never `Math.random` (rule 13).
 */
export async function runBrandingUpload(
  kind: BrandingImageKind,
  file: File,
  newKey: () => string,
  onState: (s: BrandingUploadState) => void,
): Promise<BrandingUploadState> {
  const report = (s: BrandingUploadState) => {
    onState(s);
    return s;
  };
  const t = declaredBrandingType(file);
  if (!t.ok) return report({ kind: "refused", message: t.message });

  report({ kind: "requesting" });
  const req = await requestBrandingUpload(kind, { file_name: file.name, content_type: t.contentType, size: file.size }, newKey());
  // Policy refusal, 503 "not configured", 403, 409 profile deleted — the server's sentence verbatim.
  if (!req.ok) return report({ kind: "refused", message: req.message });

  const id = req.data.file.id;
  report({ kind: "uploading", id, percent: 0 });
  // The form (`req.data.upload`) is used here and dropped.
  const up = await uploadToStorage(req.data.upload, file, file.name, (percent) =>
    onState({ kind: "uploading", id, percent }),
  );
  // The store's refusal is its XML, not a sentence for an officer: one sentence of our own.
  if (!up.ok) return report({ kind: "refused", message: BRANDING_STORAGE_FAILED });

  return retryBrandingCompletion(kind, id, onState);
}

/** c. alone — the `Hoàn tất lại` button: the bytes are already in the store, never re-upload. */
export async function retryBrandingCompletion(
  kind: BrandingImageKind,
  id: string,
  onState: (s: BrandingUploadState) => void,
): Promise<BrandingUploadState> {
  onState({ kind: "checking", id });
  const s = afterBrandingCompletion(id, await completeBrandingUpload(kind, id));
  onState(s);
  return s;
}

/**
 * "Cập nhật lần cuối … bởi CB-…" — the server's `updated_at` / `updated_by` (the CB- business code,
 * rule 6 inv. 8). "" when the commune has no display profile yet (both absent).
 */
export function brandingUpdatedText(updatedAt: string | null | undefined, updatedBy: string | undefined): string {
  if (updatedAt === undefined || updatedAt === null || updatedAt === "") return "";
  const t = Date.parse(updatedAt);
  if (Number.isNaN(t)) return "";
  const when = formatDateTime(t);
  return updatedBy !== undefined && updatedBy !== ""
    ? `Cập nhật lần cuối: ${when}, bởi ${updatedBy}`
    : `Cập nhật lần cuối: ${when}`;
}
