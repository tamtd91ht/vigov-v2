/**
 * Cấu hình › Nhận diện xã (ADR 0069) — the commune's logo and web-admin banner, on platform's
 * `/api/v1/commune-branding` routes (all `admin.org`; the server checks the key on every call).
 *
 * The upload is the same three steps as a task attachment or a content cover (ADR 0052):
 *
 *   a. POST …/{logo|banner}-uploads                   declare {file_name, content_type, size}
 *                                                      + Idempotency-Key → 201 {file, upload}
 *   b. POST <upload.url>                               the bytes, straight to the object store
 *                                                      (`uploadToStorage`, reused)
 *   c. POST …/{logo|banner}-uploads/{id}/completion    sniff, scan, normalise, publish, set as
 *                                                      current → 200 | 409 | 422 | 503
 *
 * Removing is `DELETE …/logo` / `…/banner` → 204.
 *
 * ⚠ `upload.url` + `upload.fields` ARE A BEARER CREDENTIAL (the server's own warning,
 * `service-platform/internal/http/branding.go:6`): used in the one request they sign, never logged,
 * never stored. `file_name` can name a person (rule 3): sent, never logged.
 *
 * Every refusal is the server's sentence verbatim (`thongBaoLoi`); the status is kept only so the
 * screen can offer "Hoàn tất lại" for a 503 / 409 at completion (the bytes are already stored).
 */

import { CHUNG, LOI_KHONG_RO, thongBaoLoi } from "./goi"; // vi-name-ok: existing exports of goi.ts (rule 12 inv 3)
import type {
  platform_brandingFileOut,
  platform_brandingSettingsOut,
  platform_brandingUploadIn,
  platform_brandingUploadOut,
  platform_delete_commune_branding_banner,
  platform_delete_commune_branding_logo,
  platform_get_commune_branding,
  platform_post_commune_branding_banner_uploads,
  platform_post_commune_branding_banner_uploads_by_id_completion,
  platform_post_commune_branding_logo_uploads,
  platform_post_commune_branding_logo_uploads_by_id_completion,
} from "./schema.gen";
import { readJSON, UPLOAD_FORM_MISSING, type CallResult } from "./task-attachments";

/** Which of the two identity images a call is about. */
export type BrandingImageKind = "logo" | "banner";

const NO_ANSWER = 0;

/** The route paths, typed from the contract so a renamed route turns `tsc` red here. */
const UPLOAD_PATH: Record<
  BrandingImageKind,
  | platform_post_commune_branding_logo_uploads["duongDan"]
  | platform_post_commune_branding_banner_uploads["duongDan"]
> = {
  logo: "/api/v1/commune-branding/logo-uploads",
  banner: "/api/v1/commune-branding/banner-uploads",
};

const COMPLETION_PATH: Record<
  BrandingImageKind,
  | platform_post_commune_branding_logo_uploads_by_id_completion["duongDan"]
  | platform_post_commune_branding_banner_uploads_by_id_completion["duongDan"]
> = {
  logo: "/api/v1/commune-branding/logo-uploads/{id}/completion",
  banner: "/api/v1/commune-branding/banner-uploads/{id}/completion",
};

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
 * a. Declare the file. `idempotencyKey`: the route requires one, ONE KEY PER ATTEMPT (the caller mints
 * it with `crypto.randomUUID()`): a replayed answer cannot carry the signed form again
 * (`core/idem` stores a code, never a body), so a retry is a new declaration.
 */
export async function requestBrandingUpload(
  kind: BrandingImageKind,
  body: platform_brandingUploadIn,
  idempotencyKey: string,
): Promise<CallResult<platform_brandingUploadOut>> {
  // Field by field — never `...body`.
  const sent: platform_brandingUploadIn = {
    file_name: body.file_name,
    content_type: body.content_type,
    size: body.size,
  };
  try {
    const res = await fetch(UPLOAD_PATH[kind], {
      ...CHUNG,
      method: "POST",
      headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey },
      body: JSON.stringify(sent),
    });
    const r = await readJSON<platform_brandingUploadOut>(res, 201);
    if (r.ok && (r.data?.upload?.url === undefined || r.data.upload.fields === undefined || r.data.file?.id === undefined)) {
      return { ok: false, status: 201, message: UPLOAD_FORM_MISSING };
    }
    return r;
  } catch {
    return { ok: false, status: NO_ANSWER, message: LOI_KHONG_RO };
  }
}

/** c. Complete. Safe to repeat — the server re-reads the stored bytes; a file already ready answers itself. */
export async function completeBrandingUpload(
  kind: BrandingImageKind,
  id: string,
): Promise<CallResult<platform_brandingFileOut>> {
  try {
    const res = await fetch(COMPLETION_PATH[kind].replace("{id}", encodeURIComponent(id)), {
      ...CHUNG,
      method: "POST",
    });
    return await readJSON<platform_brandingFileOut>(res, 200);
  } catch {
    return { ok: false, status: NO_ANSWER, message: LOI_KHONG_RO };
  }
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
