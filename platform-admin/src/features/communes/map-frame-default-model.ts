import type { ApiError, MapFrameDefault, MapFrameDefaultChange } from "@/lib/api";

/**
 * The pure half of "Khung bản đồ mặc định" (ADR 0072 §"Sửa đổi 04/10/2026 (lần 2)", K1–K2): parsing the
 * form, the deviation warning, the PUT body, and the server's refusals as sentences. No React, so the
 * rules the operator relies on are tested on their own.
 *
 * THE HINTS ARE THE SERVER'S. Recommended radius, usual band and ceiling come from the GET answer
 * (`service-platform/internal/domain/map_frame_default.go`), never restated here: the band is a
 * proposal the owner may still adjust, and a second copy in this bundle would keep warning on the old
 * one. The mainland box for the centre IS restated, because the GET does not carry it; it is the
 * fixed H3 box (ADR 0072 K2 "Tâm … không đổi") and the server refuses anything outside it anyway.
 *
 * Every check here is UX: the server repeats all of them, the confirmation included (rule 5 forbidden #1).
 */

export type MapFrameDefaultField = "lat" | "lng" | "radius" | "acknowledge" | "reason" | "form";

export type MapFrameDefaultHints = Pick<MapFrameDefault, "recommended_radius_km" | "usual_radius_km" | "max_radius_km">;

/** What the edit dialog holds. Numbers stay text until submit, so "10," mid-typing is not lost. */
export type MapFrameDefaultForm = {
  lat: string;
  lng: string;
  radius: string;
  reason: string;
  /** The "Tôi xác nhận đặt bán kính này" box. Cleared whenever the radius text changes. */
  acknowledged: boolean;
  /**
   * The server answered `radius_unusual_unconfirmed` for this radius: show the warning even if this
   * console's copy of the band (from the GET) says the radius is usual — the server's band is the one
   * that decides. Cleared with the radius.
   */
  serverAskedConfirmation: boolean;
};

/** The fixed mainland box for the centre (service-platform domain `MapFrameMin/Max…`). */
export const MAINLAND = { minLat: 8.4, maxLat: 23.4, minLng: 102.1, maxLng: 109.5 } as const;

export const CENTER_OUTSIDE_MAINLAND =
  "Tâm khung phải nằm trong khung đất liền Việt Nam (vĩ độ 8,4–23,4; kinh độ 102,1–109,5).";

export const DEFAULT_NOTE = "Đây là giá trị mặc định. Xã tự đặt khung ở web quản trị thì giá trị của xã được dùng.";

export const NOT_CONFIGURED =
  "Chưa đặt — xã chưa có khung mặc định; xã vẫn tự đặt được ở web quản trị của xã.";

export const CONFIRM_LABEL = "Tôi xác nhận đặt bán kính này";

/** A km figure the Vietnamese way: 2.9 → "2,9". */
export function formatKm(n: number): string {
  return String(n).replace(".", ",");
}

/** "Đề xuất: 10 km (thường 3–20 km); tối đa 50 km." — from the server's hints. */
export function radiusHint(h: MapFrameDefaultHints): string {
  const [lo, hi] = h.usual_radius_km;
  return `Đề xuất: ${formatKm(h.recommended_radius_km)} km (thường ${formatKm(lo)}–${formatKm(hi)} km); tối đa ${formatKm(h.max_radius_km)} km.`;
}

export function deviationWarning(radiusKm: number, h: MapFrameDefaultHints): string {
  const [lo, hi] = h.usual_radius_km;
  return (
    `Bán kính ${formatKm(radiusKm)} km lệch nhiều so với đề xuất (${formatKm(h.recommended_radius_km)} km, thường ` +
    `${formatKm(lo)}–${formatKm(hi)} km). Khung quá nhỏ có thể không bao hết địa bàn xã; khung quá lớn có thể ` +
    "trùm địa bàn ngoài xã. Người đặt chịu trách nhiệm về giá trị này."
  );
}

/** A coordinate: dot or comma as the decimal mark, at most 6 decimals (the stored scale). */
export function parseCoordinate(text: string): number | null {
  const t = text.trim().replace(",", ".");
  if (!/^-?\d{1,3}(\.\d{1,6})?$/.test(t)) return null;
  return Number(t);
}

/** A radius: at most 1 decimal (numeric(4,1), step 0.1 km). Range is checked by the caller. */
export function parseRadius(text: string): number | null {
  const t = text.trim().replace(",", ".");
  if (!/^\d{1,3}(\.\d)?$/.test(t)) return null;
  return Number(t);
}

/** Outside the usual band — the server's `IsUnusualRadius`, with the band the server sent. */
export function isUnusualRadius(radiusKm: number, h: MapFrameDefaultHints): boolean {
  const [lo, hi] = h.usual_radius_km;
  return radiusKm < lo || radiusKm > hi;
}

/**
 * The radius the warning is about, or null when no warning shows. A radius that does not parse or is
 * out of (0, max] shows no warning: it is refused under its own field instead.
 */
export function warningRadius(form: MapFrameDefaultForm, h: MapFrameDefaultHints): number | null {
  const r = parseRadius(form.radius);
  if (r === null || r <= 0 || r > h.max_radius_km) return null;
  return isUnusualRadius(r, h) || form.serverAskedConfirmation ? r : null;
}

/** Lưu is disabled while a warning shows and its box is not ticked. */
export function canSave(form: MapFrameDefaultForm, h: MapFrameDefaultHints): boolean {
  return warningRadius(form, h) === null || form.acknowledged;
}

export function formFromView(v: MapFrameDefault): MapFrameDefaultForm {
  const configured = v.configured && v.center_lat !== undefined && v.center_lng !== undefined && v.radius_km !== undefined;
  return {
    lat: configured ? String(v.center_lat) : "",
    lng: configured ? String(v.center_lng) : "",
    radius: configured ? String(v.radius_km) : "",
    reason: "",
    acknowledged: false,
    serverAskedConfirmation: false,
  };
}

/**
 * The PUT body, or the first field to fix. `acknowledged_unusual` is present ONLY when a warning
 * shows and was confirmed — never `false`, never on a usual radius.
 */
export function buildMapFrameDefaultChange(
  form: MapFrameDefaultForm,
  h: MapFrameDefaultHints,
): { ok: true; body: MapFrameDefaultChange } | { ok: false; field: MapFrameDefaultField; text: string } {
  const lat = parseCoordinate(form.lat);
  if (lat === null) return { ok: false, field: "lat", text: "Nhập vĩ độ của tâm là một số, tối đa 6 chữ số thập phân (ví dụ 21.028511)." };
  if (lat < MAINLAND.minLat || lat > MAINLAND.maxLat) return { ok: false, field: "lat", text: CENTER_OUTSIDE_MAINLAND };
  const lng = parseCoordinate(form.lng);
  if (lng === null) return { ok: false, field: "lng", text: "Nhập kinh độ của tâm là một số, tối đa 6 chữ số thập phân (ví dụ 105.804817)." };
  if (lng < MAINLAND.minLng || lng > MAINLAND.maxLng) return { ok: false, field: "lng", text: CENTER_OUTSIDE_MAINLAND };
  const radius = parseRadius(form.radius);
  const outOfRange = `Bán kính phải lớn hơn 0 và không quá ${formatKm(h.max_radius_km)} km, tối đa 1 chữ số thập phân.`;
  if (radius === null || radius <= 0 || radius > h.max_radius_km) return { ok: false, field: "radius", text: outOfRange };
  const warned = warningRadius(form, h) !== null;
  if (warned && !form.acknowledged) {
    return { ok: false, field: "acknowledge", text: `Đánh dấu ô “${CONFIRM_LABEL}” để lưu bán kính lệch xa mức đề xuất.` };
  }
  if (form.reason.trim() === "") return { ok: false, field: "reason", text: "Hãy ghi lý do đặt khung mặc định." };
  const body: MapFrameDefaultChange = { center_lat: lat, center_lng: lng, radius_km: radius, reason: form.reason };
  if (warned) body.acknowledged_unusual = true;
  return { ok: true, body };
}

/**
 * The server's refusals (operator_map_frame_default.go `writeMapFrameError`), each under the input it
 * belongs to. 400 invalid_body, 401, 403, 429 and 503 are the shared guarded handling (`lib/errors.ts`).
 */
const MAP_FRAME_DEFAULT_ERRORS: Record<string, { field: MapFrameDefaultField; text: string }> = {
  commune_not_found: { field: "form", text: "Không tìm thấy xã. Quay lại danh sách xã và chọn lại." },
  commune_inactive: {
    field: "form",
    text: "Xã đang ngừng hoạt động nên không đặt được khung mặc định. Bật hoạt động trở lại trước, nếu việc đó đúng.",
  },
  center_outside_mainland: { field: "form", text: CENTER_OUTSIDE_MAINLAND },
  radius_out_of_range: { field: "radius", text: "Bán kính phải lớn hơn 0 và không quá 50 km." },
  radius_unusual_unconfirmed: {
    field: "acknowledge",
    text: `Bán kính lệch xa mức đề xuất. Đọc cảnh báo, đánh dấu ô “${CONFIRM_LABEL}” rồi lưu lại.`,
  },
  invalid_reason: { field: "reason", text: "Hãy ghi lý do, tối đa 500 ký tự." },
};

export function mapFrameDefaultError(err: ApiError): { field: MapFrameDefaultField; text: string } | null {
  return MAP_FRAME_DEFAULT_ERRORS[err.code] ?? null;
}
