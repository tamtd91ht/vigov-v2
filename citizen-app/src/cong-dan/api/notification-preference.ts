/**
 * THE CITIZEN'S "NHẬN THÔNG BÁO" SWITCH — contract of `GET` / `PUT /api/v1/my-notification-preferences`
 * (`service-comms/internal/http/citizen_notification_preference.go`, ADR 0091 §4). Path, body and reader only:
 * the call itself goes through `goi-vigov.ts`, the one file of this half allowed to reach the network.
 *
 *   GET  → { zalo_enabled: bool }            the caller's own switch; never touched = `true`
 *   PUT  { zalo_enabled: bool } → same       200 is the value the server now holds
 *
 * WHOSE AND WHICH COMMUNE COME FROM THE SESSION (rule 4 invariant 2, rule 1 forbidden #2): no citizen id, phone or
 * commune in the path, the query or the body — only the bearer `goi-vigov.ts` takes from `layPhienViGov()`. A
 * session without a VERIFIED phone gets 403 `chua_xac_thuc_so`, the same single cause as the petition routes.
 *
 * ⚠ THE SWITCH CHANGES NO MESSAGE TODAY: the ZNS send path is not wired (ADR 0041 open #4, ADR 0091 §Cái giá).
 */
import { diaChiViGov } from "./dia-chi-vigov";

export const MY_NOTIFICATION_PREFERENCES_PATH = "/api/v1/my-notification-preferences";

/** The route on the comms host, or `""` when that host is not configured (the call then does not go). */
export function notificationPreferenceAddress(): string {
  return diaChiViGov("comms", MY_NOTIFICATION_PREFERENCES_PATH);
}

/** The PUT body — THE ONLY PLACE its one key is written. A boolean, never absent: the server refuses a missing key. */
export function notificationPreferenceBody(zaloEnabled: boolean): string {
  return JSON.stringify({ zalo_enabled: zaloEnabled });
}

/** 200 body → the switch, or `null` when malformed (no boolean `zalo_enabled`) — never a guessed default. */
export function readNotificationPreference(body: unknown): boolean | null {
  if (typeof body !== "object" || body === null) return null;
  const v = (body as Record<string, unknown>)["zalo_enabled"];
  return typeof v === "boolean" ? v : null;
}
