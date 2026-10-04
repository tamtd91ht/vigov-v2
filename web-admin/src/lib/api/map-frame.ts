/**
 * The commune's map frame — GET · PUT /api/v1/map-frame and POST /api/v1/map-frame/reset
 * (ADR 0072 §Sửa đổi H3, §Sửa đổi lần 2 K3–K4).
 *
 * READ IS `asset.read`, WRITE IS `admin.lookup` (`x-vigov-permission` in the contract). The frame is a
 * per-commune runtime value (rule 1 invariant 10): never a constant in this bundle.
 *
 * The server owns every rule of the frame — centre inside mainland Viet Nam, radius > 0 and ≤ 50 km,
 * the legal notice acknowledged at its CURRENT version — and answers 422 with its own Vietnamese
 * sentence, shown verbatim.
 */

import { CHUNG, LOI_KHONG_RO, docJSON } from "./goi";
import type { KetQua } from "./goi";
import type {
  comms_get_map_frame,
  comms_mapFrameIn,
  comms_mapFrameOut,
  comms_mapFrameResetIn,
  comms_post_map_frame_reset,
  comms_put_map_frame,
  httpx_Error,
} from "./schema.gen";

const FRAME_PATH = "/api/v1/map-frame" satisfies comms_get_map_frame["duongDan"] & comms_put_map_frame["duongDan"];
const RESET_PATH = "/api/v1/map-frame/reset" satisfies comms_post_map_frame_reset["duongDan"];

/**
 * The one 422 code of the two write routes the screen reads: the notice version sent is not the
 * server's current one (the text moved on between GET and the save, ADR 0072 K6 "đổi một chữ = phiên
 * bản mới"). WHY A CODE IS READ when `goi.ts` says never to branch on `code`: the screen's answer is not
 * a sentence but an ACTION — reload the frame and put the notice in front of the officer again. Same
 * shape of exception as `AFTER_PHOTO_REQUIRED_CODE` in `phieu-phan-anh.ts`. The sentence stays the
 * server's, verbatim.
 */
export const NOTICE_NOT_ACKNOWLEDGED_CODE =
  "notice_not_acknowledged" satisfies comms_put_map_frame["errorCodes"][422] & comms_post_map_frame_reset["errorCodes"][422];

/** A write's answer. Assignable to `KetQua<comms_mapFrameOut>`; `noticeStale` is the only extra fact. */
export type FrameWriteResult =
  | { ok: true; duLieu: comms_mapFrameOut }
  | { ok: false; thongBao: string; noticeStale: boolean };

export function getMapFrame(): Promise<KetQua<comms_mapFrameOut>> {
  return docJSON<comms_mapFrameOut>(FRAME_PATH);
}

/**
 * PUT — set or change the commune's own frame. `notice_version` is the version of the K6 text the
 * officer just acknowledged; without it the server refuses (K4: the UI is not the gate). No
 * Idempotency-Key: the contract declares none, and a repeat lands on the same state.
 */
export function putMapFrame(body: comms_mapFrameIn): Promise<FrameWriteResult> {
  const sent: comms_mapFrameIn = {
    center_lat: body.center_lat,
    center_lng: body.center_lng,
    radius_km: body.radius_km,
    notice_version: body.notice_version,
  };
  return write(FRAME_PATH, "PUT", sent);
}

/** POST reset — stop using the commune's own frame ("Về mặc định", K4). Answers the frame now in effect. */
export function resetMapFrame(noticeVersion: string): Promise<FrameWriteResult> {
  const sent: comms_mapFrameResetIn = { notice_version: noticeVersion };
  return write(RESET_PATH, "POST", sent);
}

/** Never logs: the body holds nothing personal, but no write path here gets a debug line (rule 3). */
async function write(path: string, method: "PUT" | "POST", body: unknown): Promise<FrameWriteResult> {
  let res: Response;
  try {
    res = await fetch(path, { ...CHUNG, method, headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
  } catch {
    return { ok: false, thongBao: LOI_KHONG_RO, noticeStale: false };
  }
  if (res.status === 200) {
    try {
      return { ok: true, duLieu: (await res.json()) as comms_mapFrameOut };
    } catch {
      return { ok: false, thongBao: LOI_KHONG_RO, noticeStale: false };
    }
  }
  let message = LOI_KHONG_RO;
  let code = "";
  try {
    const err = (await res.json()) as httpx_Error;
    if (typeof err?.message === "string" && err.message !== "") message = err.message;
    if (typeof err?.code === "string") code = err.code;
  } catch {
    // Not the server's error body (a proxy page): the generic sentence stands.
  }
  return { ok: false, thongBao: message, noticeStale: res.status === 422 && code === NOTICE_NOT_ACKNOWLEDGED_CODE };
}
