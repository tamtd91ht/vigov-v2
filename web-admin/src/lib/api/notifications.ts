/**
 * The header bell (`docs/ui-ux/08-thong-bao.md §8`, ADR 0058 §3) — the signed-in staff member's OWN
 * inbox in `service-comms`. Four routes, all `AnyAuthenticated` (any staff account has a bell):
 *
 *   GET   /api/v1/notifications                a cursor page, newest first
 *   GET   /api/v1/notifications/unread-count   { unread } — the red badge
 *   PATCH /api/v1/notifications/{id}           { read: true } → 200 the notice
 *   PATCH /api/v1/notifications                { read: true } → 200 { marked } — "Đọc hết"
 *
 * WHOSE INBOX IS DECIDED BY THE SERVER, FROM THE SESSION. No function here takes a recipient, and none
 * may: the server filters every read and write by (commune, staff code of the session), and another
 * person's notice answers 404 like one that does not exist. The browser only renders what it is sent.
 *
 * `{ read: true }` ONLY: the server refuses `false` (read is one-way), so no function offers it.
 *
 * NO `tenant_id`, RELATIVE PATHS, `credentials: same-origin`: the three rules of `goi.ts`.
 */

import { docJSON, docThanLoiGoi, goiGhi, thamSoTheoHopDong } from "./request";
import type { KetQua } from "./request";
import type {
  comms_get_notifications,
  comms_get_notifications_unread_count,
  comms_markAllReadOut,
  comms_markReadIn,
  comms_notificationOut,
  comms_patch_notifications,
  comms_patch_notifications_by_id,
  comms_unreadCountOut,
  page_Result_comms_notificationOut,
} from "./schema.gen";

export type StaffNotification = comms_notificationOut;
export type NotificationPage = page_Result_comms_notificationOut;

const LIST_PATH = "/api/v1/notifications" satisfies comms_get_notifications["duongDan"] &
  comms_patch_notifications["duongDan"];
const COUNT_PATH = "/api/v1/notifications/unread-count" satisfies comms_get_notifications_unread_count["duongDan"];
const ONE_TEMPLATE = "/api/v1/notifications/{id}" satisfies comms_patch_notifications_by_id["duongDan"];

/** The one body both PATCH routes accept. */
const MARK_READ: comms_markReadIn = { read: true };

/** A page of the bell. `cursor` null / "" is the first page (an empty `cursor=` is a 400). */
export function notificationsPath(cursor: string | null, limit?: number): string {
  const q = new URLSearchParams();
  const set = thamSoTheoHopDong<comms_get_notifications["truyVan"]>(q);
  set("limit", limit);
  set("cursor", cursor);
  const s = q.toString();
  return s === "" ? LIST_PATH : `${LIST_PATH}?${s}`;
}

export async function listNotifications(
  cursor: string | null,
  limit?: number,
): Promise<KetQua<NotificationPage>> {
  const r = await docJSON<NotificationPage>(notificationsPath(cursor, limit));
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

export async function unreadCount(): Promise<KetQua<number>> {
  const r = await docJSON<comms_unreadCountOut>(COUNT_PATH);
  if (!r.ok) return r;
  return typeof r.duLieu.unread === "number" && r.duLieu.unread >= 0
    ? { ok: true, duLieu: r.duLieu.unread }
    : { ok: false, thongBao: "Không đọc được số thông báo chưa đọc." };
}

export function markNotificationRead(id: string): Promise<KetQua<StaffNotification>> {
  const path = ONE_TEMPLATE.replace("{id}", encodeURIComponent(id));
  return docThanLoiGoi<StaffNotification>(goiGhi(path, "PATCH", MARK_READ, 200));
}

/** "Đọc hết". Answers how many notices moved. */
export function markAllNotificationsRead(): Promise<KetQua<comms_markAllReadOut>> {
  return docThanLoiGoi<comms_markAllReadOut>(goiGhi(LIST_PATH, "PATCH", MARK_READ, 200));
}
