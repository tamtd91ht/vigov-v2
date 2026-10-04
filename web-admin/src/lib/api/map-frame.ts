/**
 * The commune's map frame — GET · PUT /api/v1/map-frame (ADR 0072 §Sửa đổi H3).
 *
 * READ IS `asset.read`, WRITE IS `admin.lookup` (`x-vigov-permission` in the contract). The frame is a
 * per-commune runtime value (rule 1 invariant 10): never a constant in this bundle.
 *
 * The server owns every rule of the frame — centre inside mainland Viet Nam, radius between its named
 * bounds — and answers 422 with its own Vietnamese sentence, shown verbatim (`goi.ts`).
 */

import { docJSON, docThanLoiGoi, goiGhi } from "./goi";
import type { KetQua } from "./goi";
import type { comms_get_map_frame, comms_mapFrameIn, comms_mapFrameOut, comms_put_map_frame } from "./schema.gen";

const FRAME_PATH = "/api/v1/map-frame" satisfies comms_get_map_frame["duongDan"] & comms_put_map_frame["duongDan"];

export function getMapFrame(): Promise<KetQua<comms_mapFrameOut>> {
  return docJSON<comms_mapFrameOut>(FRAME_PATH);
}

/** PUT — set or change the frame. No Idempotency-Key: the contract declares none, and a repeat lands on the same state. */
export function putMapFrame(body: comms_mapFrameIn): Promise<KetQua<comms_mapFrameOut>> {
  const sent: comms_mapFrameIn = {
    center_lat: body.center_lat,
    center_lng: body.center_lng,
    radius_km: body.radius_km,
  };
  return docThanLoiGoi<comms_mapFrameOut>(goiGhi(FRAME_PATH, "PUT", sent, 200));
}
