/**
 * The commune's mail server (`docs/ui-ux/14-cau-hinh.md §10`): GET · PUT /api/v1/mail-settings and
 * POST /api/v1/mail-settings/test-messages — all three `admin.lookup`.
 *
 * THE PASSWORD IS WRITE-ONLY, IN BOTH DIRECTIONS OF THIS FILE:
 *   - no response carries it; `password_set` is the only thing the server says about it (§12.6);
 *   - `saveMailSettings` sends `password` ONLY when staff typed one. Absent means "keep the stored
 *     password" — except after changing host, port or username, which the server refuses with a
 *     sentence (400 `password_required_for_new_host`): the old password is never sent somewhere it
 *     was not typed for.
 * Nothing here logs, caches or echoes the body (rule 3, rule 8).
 */

import { CHUNG, docJSON, docThanLoiGoi, errorMessageOr, goiGhi, LOI_KHONG_RO } from "./request";
import type { KetQua } from "./request";
import type {
  comms_get_mail_settings,
  comms_mailSettingsIn,
  comms_mailSettingsOut,
  comms_mailTestIn,
  comms_post_mail_settings_test_messages,
  comms_put_mail_settings,
} from "./schema.gen";

const SETTINGS_PATH = "/api/v1/mail-settings" satisfies comms_get_mail_settings["duongDan"] &
  comms_put_mail_settings["duongDan"];
const TEST_PATH =
  "/api/v1/mail-settings/test-messages" satisfies comms_post_mail_settings_test_messages["duongDan"];

export function getMailSettings(): Promise<KetQua<comms_mailSettingsOut>> {
  return docJSON<comms_mailSettingsOut>(SETTINGS_PATH);
}

/**
 * PUT the whole configuration. Built field by field: a `...body` is how a stray field — or a
 * password the caller meant to drop — goes up.
 *
 * `password` blank (or whitespace-free empty) is DROPPED, not sent as `""`. The server reads both as
 * "keep", but an absent field cannot be mistaken for "set the password to empty" by anyone reading
 * the request later.
 */
export function saveMailSettings(body: comms_mailSettingsIn): Promise<KetQua<comms_mailSettingsOut>> {
  const sent: comms_mailSettingsIn = {
    host: body.host,
    port: body.port,
    security: body.security,
    username: body.username,
    from_address: body.from_address,
    from_name: body.from_name,
    is_enabled: body.is_enabled,
  };
  if (body.password !== undefined && body.password !== "") sent.password = body.password;
  return docThanLoiGoi<comms_mailSettingsOut>(goiGhi(SETTINGS_PATH, "PUT", sent, 200));
}

/**
 * 502 whose body is not the server's (a proxy page when service-comms is down) — the service did
 * not answer, which is NOT "the commune's mail server refused".
 */
export const TEST_UNREACHABLE_FALLBACK =
  "Chưa gửi được thư thử vì hệ thống không nhận được trả lời. Vui lòng thử lại sau ít phút.";

/**
 * POST one fixed test message to `recipient` through the SAVED settings (not the form on screen).
 *
 * `idempotencyKey` is the caller's, kept for every retry of the same send and replaced once it
 * succeeds: a retry after a network error must not mail twice. A failure releases the key on the
 * server (`core/idem`), so retrying a 502 with the same key sends again, as staff expect.
 *
 * 502 IS THE COMMUNE'S MAIL SERVER REFUSING (timeout, certificate, login…). The server writes one
 * plain sentence per category saying what to check (`mail_settings.go`, `mailSendFailures`), and it
 * is shown verbatim — mapping `code` to a second set of sentences here would be a copy that drifts.
 * A replayed success answers `{ replayed: true }` without `sent`; a 200 is the fact either way.
 */
export async function sendTestMail(recipient: string, idempotencyKey: string): Promise<KetQua<null>> {
  const body: comms_mailTestIn = { recipient };
  let res: Response;
  try {
    res = await fetch(TEST_PATH, {
      ...CHUNG,
      method: "POST",
      headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey },
      body: JSON.stringify(body),
    });
  } catch {
    return { ok: false, thongBao: LOI_KHONG_RO };
  }
  if (res.status === 200) return { ok: true, duLieu: null };
  const fallback = res.status === 502 || res.status === 503 ? TEST_UNREACHABLE_FALLBACK : LOI_KHONG_RO;
  return { ok: false, thongBao: await errorMessageOr(res, fallback) };
}
