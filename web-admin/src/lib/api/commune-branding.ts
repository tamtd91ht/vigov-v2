/**
 * Cấu hình › Nhận diện xã (ADR 0069) — the commune's logo and web-admin banner, on platform's
 * `/api/v1/commune-branding` routes (all `admin.org`; the server checks the key on every call).
 *
 * The upload is ONE multipart request (ADR 0052 §Sửa đổi 09/10/2026, `lib/api/upload.ts`):
 *
 *   POST …/{logo|banner}-uploads   size, file_name, then the file + Idempotency-Key
 *                                  → 201 the file: sniffed, scanned, normalised, published and SET AS
 *                                  CURRENT | 4xx | 503
 *
 * The presigned POST to the object store and `…/{id}/completion` are gone. Removing is
 * `DELETE …/logo` / `…/banner` → 204.
 *
 * `file_name` can name a person (rule 3): sent as a part, never logged. Every refusal is the server's
 * sentence verbatim; the status is kept so the screen can offer "Gửi lại" for a refusal of the moment
 * (`isTransientUpload`).
 */

import { CHUNG, LOI_KHONG_RO, thongBaoLoi } from "./goi"; // vi-name-ok: existing exports of goi.ts (rule 12 inv 3)
import type {
  platform_brandingFileOut,
  platform_brandingSettingsOut,
  platform_delete_commune_branding_banner,
  platform_delete_commune_branding_logo,
  platform_get_commune_branding,
  platform_post_commune_branding_banner_uploads,
  platform_post_commune_branding_logo_uploads,
} from "./schema.gen";
import { readJSON, type CallResult } from "./task-attachments";
import { sendUpload, type UploadProgress, type UploadResult } from "./upload";

/** Which of the two identity images a call is about. */
export type BrandingImageKind = "logo" | "banner";

const NO_ANSWER = 0;

type LogoUpload = platform_post_commune_branding_logo_uploads;
type BannerUpload = platform_post_commune_branding_banner_uploads;

/** The route paths, typed from the contract so a renamed route turns `tsc` red here. */
const UPLOAD_PATH: Record<BrandingImageKind, LogoUpload["duongDan"] | BannerUpload["duongDan"]> = {
  logo: "/api/v1/commune-branding/logo-uploads",
  banner: "/api/v1/commune-branding/banner-uploads",
};

/** The contract's order, identical for both routes — `tsc` turns red here the day either changes. */
const BRANDING_PARTS: LogoUpload["multipartParts"] & BannerUpload["multipartParts"] = ["size", "file_name", "file"];

const REMOVE_PATH: Record<
  BrandingImageKind,
  platform_delete_commune_branding_logo["duongDan"] | platform_delete_commune_branding_banner["duongDan"]
> = {
  logo: "/api/v1/commune-branding/logo",
  banner: "/api/v1/commune-branding/banner",
};

/** The tab's read: both current images and who last changed them (CB- code). */
export async function getCommuneBranding(): Promise<CallResult<platform_brandingSettingsOut>> {
  const path: platform_get_commune_branding["duongDan"] = "/api/v1/commune-branding";
  try {
    const res = await fetch(path, { ...CHUNG, method: "GET" });
    return await readJSON<platform_brandingSettingsOut>(res, 200);
  } catch {
    return { ok: false, status: NO_ANSWER, message: LOI_KHONG_RO };
  }
}

/**
 * Upload one image of `kind`. `idempotencyKey`: the route requires one, ONE KEY PER ATTEMPT (the caller
 * mints it with `crypto.randomUUID()`).
 */
export function uploadBrandingImage(
  kind: BrandingImageKind,
  file: File,
  idempotencyKey: string,
  progress: UploadProgress = {},
): Promise<UploadResult<platform_brandingFileOut>> {
  return sendUpload<LogoUpload, platform_brandingFileOut>({
    path: UPLOAD_PATH[kind],
    parts: BRANDING_PARTS,
    fields: { file_name: file.name },
    file,
    fileName: file.name,
    idempotencyKey,
    ...progress,
  });
}

/** Remove the current image → 204. The sidebar falls back to the building icon / the strip disappears. */
export async function removeBrandingImage(kind: BrandingImageKind): Promise<CallResult<null>> {
  try {
    const res = await fetch(REMOVE_PATH[kind], { ...CHUNG, method: "DELETE" });
    if (res.status === 204) return { ok: true, data: null };
    return { ok: false, status: res.status, message: await thongBaoLoi(res) };
  } catch {
    return { ok: false, status: NO_ANSWER, message: LOI_KHONG_RO };
  }
}
