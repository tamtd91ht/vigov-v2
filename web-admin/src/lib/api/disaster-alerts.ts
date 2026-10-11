/**
 * Disaster alerts — `cảnh báo thiên tai` (ADR 0091 §2), the staff half in `service-comms`:
 *
 *   GET    /api/v1/disaster-alerts                 content.read    a cursor page, newest start first
 *   GET    /api/v1/disaster-alerts/{id}            content.read    one alert, full lists
 *   POST   /api/v1/disaster-alerts                 content.update  + Idempotency-Key REQUIRED → 201
 *   PATCH  /api/v1/disaster-alerts/{id}            content.update  absent = leave alone → 200
 *   POST   /api/v1/disaster-alerts/{id}/ending     content.update  end early → 200 (409 `alert_ended`)
 *   DELETE /api/v1/disaster-alerts/{id}            content.update  soft delete, `{reason}` in the body → 204
 *
 * Every shape is the GENERATED one (`schema.gen.ts`); nothing here retypes the contract.
 *
 * THE CONTACT NAMES AND PHONES IN THE TWO LISTS MAY BE AN OFFICER'S OWN (rule 3): nothing here logs a
 * body, a reply or a failure. Relative paths, no `tenant_id`, `credentials: same-origin` — `goi.ts`.
 */

import { CHUNG, docJSON, docThanLoiGoi, goiGhi, LOI_KHONG_RO, thamSoTheoHopDong, thongBaoLoi } from "./goi";
import type { KetQua } from "./goi";
import type {
  comms_createDisasterAlertIn,
  comms_delete_disaster_alerts_by_id,
  comms_deleteDisasterAlertIn,
  comms_disasterAlertOut,
  comms_get_disaster_alerts,
  comms_get_disaster_alerts_by_id,
  comms_patch_disaster_alerts_by_id,
  comms_post_disaster_alerts,
  comms_post_disaster_alerts_by_id_ending,
  comms_updateDisasterAlertIn,
  page_Result_comms_disasterAlertOut,
} from "./schema.gen";
import type { CallResult } from "./task-attachments";

export type DisasterAlert = comms_disasterAlertOut;
export type DisasterAlertPage = page_Result_comms_disasterAlertOut;
export type CreateDisasterAlertBody = comms_createDisasterAlertIn;
export type UpdateDisasterAlertBody = comms_updateDisasterAlertIn;

const LIST_PATH = "/api/v1/disaster-alerts" satisfies comms_get_disaster_alerts["duongDan"] &
  comms_post_disaster_alerts["duongDan"];
const ONE_TEMPLATE = "/api/v1/disaster-alerts/{id}" satisfies comms_get_disaster_alerts_by_id["duongDan"] &
  comms_patch_disaster_alerts_by_id["duongDan"] &
  comms_delete_disaster_alerts_by_id["duongDan"];
const ENDING_TEMPLATE =
  "/api/v1/disaster-alerts/{id}/ending" satisfies comms_post_disaster_alerts_by_id_ending["duongDan"];

/** `{id}` encoded: the id comes from the server, but a raw id in a path is one typed string from a leak. */
export function disasterAlertPath(id: string): string {
  return ONE_TEMPLATE.replace("{id}", encodeURIComponent(id));
}

/** A page of the list. `cursor` null is the first page (an empty `cursor=` is a 400). */
export function disasterAlertsPath(cursor: string | null, limit?: number): string {
  const q = new URLSearchParams();
  const set = thamSoTheoHopDong<comms_get_disaster_alerts["truyVan"]>(q);
  set("limit", limit);
  set("cursor", cursor);
  const s = q.toString();
  return s === "" ? LIST_PATH : `${LIST_PATH}?${s}`;
}

export async function listDisasterAlerts(cursor: string | null, limit?: number): Promise<KetQua<DisasterAlertPage>> {
  const r = await docJSON<DisasterAlertPage>(disasterAlertsPath(cursor, limit));
  if (!r.ok) return r;
  return {
    ok: true,
    duLieu: {
      items: Array.isArray(r.duLieu.items) ? r.duLieu.items : [],
      next_cursor: typeof r.duLieu.next_cursor === "string" ? r.duLieu.next_cursor : "",
      has_more: r.duLieu.has_more === true,
    },
  };
}

/** One alert with its three lists — what the edit dialog opens on (the list rows carry them too). */
export function getDisasterAlert(id: string): Promise<KetQua<DisasterAlert>> {
  return docJSON<DisasterAlert>(disasterAlertPath(id));
}

/**
 * POST — post one alert. `idempotencyKey` is HELD BY THE FORM for the life of one opening: a retry after
 * a network error must reuse it, since the first attempt may already have put a warning on every
 * resident's Mini App (the route is `idem.Required`; with no unique key under it, the key is the only
 * layer against a double post).
 *
 * The body is rebuilt field by field, never spread: a row read back carries `in_force`, `ended_at`,
 * `created_by` — fields a request must never carry.
 */
export function createDisasterAlert(
  body: CreateDisasterAlertBody,
  idempotencyKey: string,
): Promise<KetQua<DisasterAlert>> {
  const sent: CreateDisasterAlertBody = {
    kind: body.kind,
    title: body.title,
    detail: body.detail,
    effective_from: body.effective_from,
  };
  if (body.level !== undefined) sent.level = body.level;
  if (body.headline !== undefined) sent.headline = body.headline;
  if (body.effective_to !== undefined) sent.effective_to = body.effective_to;
  if (body.danger_zone !== undefined) sent.danger_zone = body.danger_zone;
  if (body.evacuation_points !== undefined) sent.evacuation_points = body.evacuation_points;
  if (body.emergency_contacts !== undefined) sent.emergency_contacts = body.emergency_contacts;
  return docThanLoiGoi<DisasterAlert>(goiGhi(LIST_PATH, "POST", sent, 201, { "Idempotency-Key": idempotencyKey }));
}

/** PATCH — only the fields that changed (`alertPatch` builds it). 409 once the alert was ended. */
export function updateDisasterAlert(id: string, body: UpdateDisasterAlertBody): Promise<KetQua<DisasterAlert>> {
  return docThanLoiGoi<DisasterAlert>(goiGhi(disasterAlertPath(id), "PATCH", body, 200));
}

/** POST …/ending — no body: the server records who ended it and when, from the session. */
export function endDisasterAlert(id: string): Promise<KetQua<DisasterAlert>> {
  return docThanLoiGoi<DisasterAlert>(
    goiGhi(ENDING_TEMPLATE.replace("{id}", encodeURIComponent(id)), "POST", undefined, 200),
  );
}

/**
 * DELETE with `{reason}` IN THE BODY (never the query string an access log keeps). 204, no body.
 * `CallResult` for the STATUS: a 404 (deleted by someone else, or never this commune's — one answer)
 * closes the dialog; any other refusal keeps it open with the typed reason (`deleteContentItem`'s rule).
 */
export async function deleteDisasterAlert(id: string, reason: string): Promise<CallResult<null>> {
  const sent: comms_deleteDisasterAlertIn = { reason };
  try {
    const res = await fetch(disasterAlertPath(id), {
      ...CHUNG,
      method: "DELETE",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(sent),
    });
    if (res.status !== 204) return { ok: false, status: res.status, message: await thongBaoLoi(res) };
    return { ok: true, data: null };
  } catch {
    return { ok: false, status: 0, message: LOI_KHONG_RO };
  }
}
