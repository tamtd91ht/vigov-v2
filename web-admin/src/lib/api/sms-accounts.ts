/**
 * The commune's eSMS accounts (ADR 0099 #2 #3) — Cấu hình › Kênh SMS. Seven routes, all `admin.sms`
 * (`service-comms/internal/http/sms_accounts.go`):
 *
 *   GET    /api/v1/sms-accounts                       the live accounts, the default first
 *   POST   /api/v1/sms-accounts                       add one — Idempotency-Key REQUIRED; eSMS checks the keys first
 *   PATCH  /api/v1/sms-accounts/{id}                  edit — an absent field, or an empty key, keeps the stored value
 *   DELETE /api/v1/sms-accounts/{id}                  "Ngừng dùng" — soft delete with `{reason}` (rule 7)
 *   POST   /api/v1/sms-accounts/{id}/check            "Kiểm tra kết nối" — a failing check is still a 200
 *   GET    /api/v1/sms-accounts/{id}/templates        the CSKH templates, one per text
 *   POST   /api/v1/sms-accounts/{id}/templates/sync   "Nạp mẫu từ eSMS"
 *
 * SECRETS GO UP ONLY. `api_key` / `secret_key` are WRITE-ONLY: no reply type below has a field for them,
 * and they travel only when staff typed one. Nothing here logs, caches or echoes a body (rule 3, rule 8).
 * No tenant and no staff code in any request: the commune is the `Host`'s, the actor the session's
 * (rule 1 forbidden #2).
 *
 * `withCode` on every write: the tab draws `sms_label_taken` under "Tên gợi nhớ", `sms_key_rejected` under
 * the keys, `sms_brandname_inactive` under "Brandname", and disables "Lưu" on `encryption_not_configured`.
 * The sentence shown is still the server's, verbatim.
 */

import { docJSON, docThanLoiGoi, goiGhi } from "./goi";
import type { KetQua } from "./goi";

import type {
  comms_delete_sms_accounts_by_id,
  comms_get_sms_accounts,
  comms_get_sms_accounts_by_id_templates,
  comms_post_sms_accounts_by_id_check,
  comms_post_sms_accounts_by_id_templates_sync,
  comms_smsAccountCreateIn,
  comms_smsAccountListOut,
  comms_smsAccountOut,
  comms_smsAccountRetiredOut,
  comms_smsAccountRetireIn,
  comms_smsAccountUpdateIn,
  comms_smsCheckOut,
  comms_smsCheckResultOut,
  comms_smsTemplateListOut,
  comms_smsTemplateNetworkOut,
  comms_smsTemplateOut,
  comms_smsTemplateParamOut,
  comms_smsTemplateSyncOut,
} from "./schema.gen";

// Every shape is `schema.gen.ts`'s (agent rule 6), generated from service-comms/internal/http/sms_accounts.go;
// each path below carries its generated route's `satisfies …["duongDan"]`, so a renamed route is a red `tsc`.
export type {
  comms_smsAccountCreateIn,
  comms_smsAccountListOut,
  comms_smsAccountOut,
  comms_smsAccountRetiredOut,
  comms_smsAccountRetireIn,
  comms_smsAccountUpdateIn,
  comms_smsCheckOut,
  comms_smsCheckResultOut,
  comms_smsTemplateListOut,
  comms_smsTemplateNetworkOut,
  comms_smsTemplateOut,
  comms_smsTemplateParamOut,
  comms_smsTemplateSyncOut,
};

/**
 * The class comms records for a call to eSMS (migration 0032). `balance` only with `thanh-cong`.
 * Kept as `string` on the wire: a class this build does not know must still be shown (as a failure).
 */
export type SmsCheckResult =
  | "thanh-cong"
  | "sai-khoa"
  | "brandname-chua-kich-hoat"
  | "khong-kha-dung"
  | "bi-tu-choi"
  | "phan-hoi-sai-dang";

const ACCOUNTS_PATH = "/api/v1/sms-accounts" satisfies comms_get_sms_accounts["duongDan"];
const ACCOUNT_TEMPLATE = "/api/v1/sms-accounts/{id}" satisfies comms_delete_sms_accounts_by_id["duongDan"];
const CHECK_TEMPLATE = "/api/v1/sms-accounts/{id}/check" satisfies comms_post_sms_accounts_by_id_check["duongDan"];
const TEMPLATES_TEMPLATE = "/api/v1/sms-accounts/{id}/templates" satisfies comms_get_sms_accounts_by_id_templates["duongDan"];
const SYNC_TEMPLATE = "/api/v1/sms-accounts/{id}/templates/sync" satisfies comms_post_sms_accounts_by_id_templates_sync["duongDan"];

/** `encodeURIComponent`: the id goes into the PATH, not the body. */
function withId(template: string, id: string): string {
  return template.replace("{id}", encodeURIComponent(id));
}

/** Success statuses, as `sms_accounts.go` declares them (`@reply`). */
export const SMS_STATUS = {
  created: 201,
  updated: 200,
  retired: 200,
  checked: 200,
  synced: 200,
} as const;

/** The machine codes the tab reads (`KetQua.code`) — all from `sms_accounts.go` `fail`. */
export const SMS_ERROR = {
  invalid: "sms_account_invalid",
  notFound: "not_found",
  labelTaken: "sms_label_taken",
  changed: "sms_account_changed",
  keyRejected: "sms_key_rejected",
  brandnameInactive: "sms_brandname_inactive",
  rejected: "sms_rejected",
  unavailable: "sms_unavailable",
  encryptionMissing: "encryption_not_configured",
} as const;

/** GET sms-accounts — the commune's live accounts, the default first. Never a key. */
export async function listSmsAccounts(): Promise<KetQua<comms_smsAccountOut[]>> {
  const r = await docJSON<comms_smsAccountListOut>(ACCOUNTS_PATH);
  return r.ok ? { ok: true, duLieu: r.duLieu.items } : r;
}

/**
 * POST sms-accounts. Built key by key (a `...body` is how a stray field goes up). The keys are sent
 * VERBATIM — a trimmed copy is a different key, and comms refuses a key with spaces in its own words.
 *
 * `idempotencyKey` REQUIRED and a PARAMETER, created when the dialog opens and kept across a retry of the
 * same dialog: a new key per click would let a retried POST ask eSMS twice and add a second account.
 */
export function createSmsAccount(
  body: comms_smsAccountCreateIn,
  idempotencyKey: string,
): Promise<KetQua<comms_smsAccountOut>> {
  const sent: comms_smsAccountCreateIn = {
    label: body.label,
    api_key: body.api_key,
    secret_key: body.secret_key,
    brandname: body.brandname,
    is_default: body.is_default,
  };
  return docThanLoiGoi<comms_smsAccountOut>(
    goiGhi(ACCOUNTS_PATH, "POST", sent, SMS_STATUS.created, { "Idempotency-Key": idempotencyKey }, { withCode: true }),
  );
}

/**
 * PATCH sms-accounts/{id}. Only the keys the caller set travel; an EMPTY key is not sent at all (comms
 * reads "" as "keep" too, but an absent field cannot be misread).
 */
export function updateSmsAccount(id: string, patch: comms_smsAccountUpdateIn): Promise<KetQua<comms_smsAccountOut>> {
  const sent: comms_smsAccountUpdateIn = {};
  if (patch.label !== undefined) sent.label = patch.label;
  if (patch.brandname !== undefined) sent.brandname = patch.brandname;
  if (patch.is_default !== undefined) sent.is_default = patch.is_default;
  if (patch.api_key !== undefined && patch.api_key !== "") sent.api_key = patch.api_key;
  if (patch.secret_key !== undefined && patch.secret_key !== "") sent.secret_key = patch.secret_key;
  return docThanLoiGoi<comms_smsAccountOut>(
    goiGhi(withId(ACCOUNT_TEMPLATE, id), "PATCH", sent, SMS_STATUS.updated, undefined, { withCode: true }),
  );
}

/** DELETE sms-accounts/{id} with `{reason}` — "Ngừng dùng", a soft delete that carries its reason (rule 7). */
export function retireSmsAccount(id: string, reason: string): Promise<KetQua<comms_smsAccountRetiredOut>> {
  const sent: comms_smsAccountRetireIn = { reason };
  return docThanLoiGoi<comms_smsAccountRetiredOut>(
    goiGhi(withId(ACCOUNT_TEMPLATE, id), "DELETE", sent, SMS_STATUS.retired, undefined, { withCode: true }),
  );
}

/** POST …/check — asks eSMS with the STORED keys; `result` is a class, never eSMS's words. No body. */
export function checkSmsAccount(id: string): Promise<KetQua<comms_smsCheckResultOut>> {
  return docThanLoiGoi<comms_smsCheckResultOut>(
    goiGhi(withId(CHECK_TEMPLATE, id), "POST", undefined, SMS_STATUS.checked, undefined, { withCode: true }),
  );
}

/** GET …/templates — the account's CSKH templates, one entry per text. */
export async function listSmsTemplates(id: string): Promise<KetQua<comms_smsTemplateOut[]>> {
  const r = await docJSON<comms_smsTemplateListOut>(withId(TEMPLATES_TEMPLATE, id));
  return r.ok ? { ok: true, duLieu: r.duLieu.items } : r;
}

/** POST …/templates/sync — "Nạp mẫu từ eSMS". No body. */
export function syncSmsTemplates(id: string): Promise<KetQua<comms_smsTemplateSyncOut>> {
  return docThanLoiGoi<comms_smsTemplateSyncOut>(
    goiGhi(withId(SYNC_TEMPLATE, id), "POST", undefined, SMS_STATUS.synced, undefined, { withCode: true }),
  );
}
