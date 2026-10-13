/**
 * Cấu hình › Kênh Zalo › "Mã QR mở Mini App của xã" (ADR 0070 §Sửa đổi 13/10/2026 #1–#2) — platform's
 * `GET /api/v1/commune-launch-link`, `admin.lookup`, the commune taken from `Host` by the server. No
 * parameter: nothing here names a commune.
 *
 * ONLY `url` LEAVES THIS FILE. The answer also carries `source` ("rieng" | "chung"), and the owner's
 * decision is that this screen says nothing about which app the code opens — dropping the field here
 * means no component below can render it by accident.
 *
 * EVERY 409 IS ONE NEUTRAL SENTENCE. The refusals (`commune_inactive`, `shared_mini_app_*`,
 * `own_mini_app_ambiguous`, `commune_launch_code_invalid`) are the operator's to fix, not the commune
 * staff's, and the server's own wording names the shared app — which this screen must not show.
 */

import { CHUNG, LOI_KHONG_RO } from "./goi"; // vi-name-ok: existing exports of goi.ts (rule 12 inv 3)
import type { platform_communeLaunchLinkOut, platform_get_commune_launch_link } from "./schema.gen";
import { readJSON, type CallResult } from "./task-attachments";

const NO_ANSWER = 0;

/** What the staff member reads when the server could not build the link (any 409, or a malformed 200). */
export const LAUNCH_QR_UNAVAILABLE = "Chưa tạo được mã QR cho xã. Vui lòng liên hệ đơn vị vận hành.";

export type CommuneLaunchLink = Pick<platform_communeLaunchLinkOut, "url">;

export async function getCommuneLaunchLink(): Promise<CallResult<CommuneLaunchLink>> {
  const path: platform_get_commune_launch_link["duongDan"] = "/api/v1/commune-launch-link";
  try {
    const res = await fetch(path, { ...CHUNG, method: "GET" });
    if (res.status === 409) return { ok: false, status: 409, message: LAUNCH_QR_UNAVAILABLE };
    const r = await readJSON<platform_communeLaunchLinkOut>(res, 200);
    if (!r.ok) return r;
    // Fail closed: a 200 without an https link is not something to print and post in public.
    const url = r.data?.url;
    if (typeof url !== "string" || !url.startsWith("https://")) {
      return { ok: false, status: res.status, message: LAUNCH_QR_UNAVAILABLE };
    }
    return { ok: true, data: { url } };
  } catch {
    return { ok: false, status: NO_ANSWER, message: LOI_KHONG_RO };
  }
}
