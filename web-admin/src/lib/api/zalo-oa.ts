/**
 * The commune's Zalo OA (ADR 0098, khối A): the OA the commune AUTHORISED to the platform's shared Zalo
 * app, and the ZBS templates loaded from it. Everything under `zalo-oas/current`, `admin.zalo`
 * (`service-comms/internal/http/zalo_commune_oa.go`, being built in parallel — TASK-A4/A5).
 *
 * SECRETS, ONE DIRECTION ONLY — DOWN, AND NEVER HERE:
 *   - no reply carries an access token, a refresh token or a code verifier (ADR 0098 #5: write-only);
 *   - the AUTHORISATION LINK (`POST …/authorization-links`) carries a one-time `state` — handed to the
 *     caller as is, to be OPENED, never stored, logged, cached or echoed into a `title`/`aria-label`.
 * Nothing here logs or caches a body (rule 3, rule 8). No tenant, no staff code in any request: the
 * commune comes from `Host`, the actor from the session (rule 1 forbidden #2).
 *
 * Enum VALUES: outcome classes are the stored Vietnamese-without-diacritics strings (ADR 0011);
 * `zalo_status` / `zalo_quality` are Zalo's own upper-case words, stored as Zalo sends them (ADR 0098 #8).
 */

import { docJSON, docThanLoiGoi, goiGhi } from "./goi";
import type { KetQua } from "./goi";

// ---- UNTIL REGEN: the commune's Zalo OA (ADR 0098, comms TASK-A4/A5) ---------------------------------
//
// `schema.gen.ts` is NOT regenerated yet: the comms routes are being written in a parallel session and
// `make kb` runs in the main one; regenerating here would mix in other sessions' uncommitted routes (the
// same stopgap as the Zalo-groups block in `zalo.ts`). Every shape below is copied from the TASK-A6
// contract (ADR 0098 #7–#11), named as the generator will name them.
//
// DELETE THIS WHOLE BLOCK in the commit that lands the regen: import the `comms_*` names from
// `schema.gen` instead, and turn each `…Route` below into the generated route type in the `satisfies`.
// A shape that drifted from the server is then a red `tsc`, which is the point of rule 6.

/** Why the OA must be authorised again (ADR 0098 #6). */
export type ZaloOAReauthorizationReason = "refresh-token-bi-tu-choi" | "refresh-token-het-han";

/**
 * The live OA row, as the screen may see it: names, ids, instants, outcome classes. NO token field exists
 * on this shape, by contract. `last_check.result` is one of the bot's seven outcome classes
 * (`service-comms/internal/domain/zalo_bot.go` ZaloCall*).
 */
export type comms_communeZaloOAOut = {
  oa_id: string;
  oa_name: string;
  authorized_at: string;
  authorized_by: string;
  authorized_by_name: string;
  access_expires_at: string;
  token_refreshed_at: string;
  needs_reauthorization: null | { since: string; reason: ZaloOAReauthorizationReason };
  last_check: null | { at: string; result: string };
};

/** GET zalo-oas/current. `configured`: the platform has declared its shared Zalo app (ZALO_OA_* set). */
export type comms_communeZaloOACurrentOut = {
  configured: boolean;
  has_oa: boolean;
  oa: comms_communeZaloOAOut | null;
  template_count: number;
  attached_count: number;
};

/** POST …/authorization-links, 201. `url` carries a one-time `state`: open it, keep nothing. */
export type comms_zaloOAAuthorizationLinkOut = { url: string; expires_at: string };

/** POST …/check, 200. `result` is an outcome class, never Zalo's words; `oa_name` on success only. */
export type comms_zaloOACheckResultOut = { result: string; checked_at: string; oa_name?: string };

/** DELETE zalo-oas/current — rule 7: a soft delete carries its reason (≤ 200 characters). */
export type comms_communeZaloOADisconnectIn = { reason: string };

/** DELETE zalo-oas/current, 200. `revoke_notice`: disconnecting here revokes nothing at Zalo (ADR 0098 #11). */
export type comms_communeZaloOADisconnectedOut = {
  disconnected: boolean;
  detached_template_count: number;
  revoke_notice: string;
};

export type ZaloTemplateStatus = "ENABLE" | "PENDING_REVIEW" | "REJECT" | "DISABLE" | "DELETE";
export type ZaloTemplateQuality = "HIGH" | "MEDIUM" | "LOW" | "UNDEFINED";

/** One parameter a template declares on Zalo's portal, as `template/info` returns it. */
export type comms_zaloOATemplateParam = { name: string; required: boolean; type: string; max_length: number };

/**
 * One template loaded from the OA. `params_match`: the three required names are all declared
 * (ADR 0098 #9); `missing_params`: which are not. `milestone`: the ADR 0041 milestone it is attached to,
 * or null; `milestone_set_by` is the attaching officer's business code.
 */
export type comms_zaloOATemplateOut = {
  id: string;
  template_id: string;
  template_name: string;
  zalo_status: ZaloTemplateStatus;
  zalo_quality: ZaloTemplateQuality;
  zalo_tag: string;
  params: comms_zaloOATemplateParam[];
  params_match: boolean;
  missing_params: string[];
  milestone: string | null;
  milestone_set_at: string | null;
  milestone_set_by: string | null;
  milestone_set_by_name: string | null;
};

/** GET …/templates. `synced_at` null = never loaded from the OA. */
export type comms_zaloOATemplateList = { items: comms_zaloOATemplateOut[]; synced_at: string | null };

/** POST …/templates/sync, 200. */
export type comms_zaloOASyncResultOut = {
  loaded: number;
  enabled: number;
  matching: number;
  changed: number;
  synced_at: string;
};

/** PATCH …/templates/{id}: `milestone` is ALWAYS a key — null = detach. */
export type comms_zaloOATemplatePatchIn = { milestone: string | null };

type OACurrentRoute = { duongDan: "/api/v1/zalo-oas/current" };
type OAAuthorizationLinksRoute = { duongDan: "/api/v1/zalo-oas/current/authorization-links" };
type OACheckRoute = { duongDan: "/api/v1/zalo-oas/current/check" };
type OATemplatesRoute = { duongDan: "/api/v1/zalo-oas/current/templates" };
type OATemplatesSyncRoute = { duongDan: "/api/v1/zalo-oas/current/templates/sync" };
type OATemplateByIdRoute = { duongDan: "/api/v1/zalo-oas/current/templates/{id}" };

// ---- end UNTIL REGEN ---------------------------------------------------------------------------

const OA_PATH = "/api/v1/zalo-oas/current" satisfies OACurrentRoute["duongDan"];
const OA_AUTHORIZATION_LINKS_PATH =
  "/api/v1/zalo-oas/current/authorization-links" satisfies OAAuthorizationLinksRoute["duongDan"];
const OA_CHECK_PATH = "/api/v1/zalo-oas/current/check" satisfies OACheckRoute["duongDan"];
const OA_TEMPLATES_PATH = "/api/v1/zalo-oas/current/templates" satisfies OATemplatesRoute["duongDan"];
const OA_TEMPLATES_SYNC_PATH = "/api/v1/zalo-oas/current/templates/sync" satisfies OATemplatesSyncRoute["duongDan"];
const OA_TEMPLATE_TEMPLATE = "/api/v1/zalo-oas/current/templates/{id}" satisfies OATemplateByIdRoute["duongDan"];

/** `encodeURIComponent`: the id goes into the PATH, not the body. */
function templatePath(id: string): string {
  return OA_TEMPLATE_TEMPLATE.replace("{id}", encodeURIComponent(id));
}

/** Success statuses, as the contract publishes them. */
export const ZALO_OA_STATUS = {
  linkCreated: 201,
  checked: 200,
  disconnected: 200,
  synced: 200,
  templatePatched: 200,
} as const;

/**
 * The refusal code of `POST …/authorization-links` when the platform has not declared its shared app
 * (503). The section then disables "Kết nối OA" with the server's sentence — a remedy that lives with
 * ViHAT, not on this screen — so this is a documented reader of `KetQua.code` (see `goi.ts`).
 */
export const ZALO_OA_NOT_CONFIGURED = "zalo_oa_not_configured";

/** GET zalo-oas/current: whether the platform is ready, whether the commune has an OA, and its row. Never a token. */
export function getCommuneZaloOA(): Promise<KetQua<comms_communeZaloOACurrentOut>> {
  return docJSON<comms_communeZaloOACurrentOut>(OA_PATH);
}

/**
 * POST …/authorization-links, no body (the commune from `Host`, the officer from the session). The reply
 * holds a one-time link with a `state`: returned to the caller as is, to be opened in a new tab and
 * dropped — never stored, logged or cached here.
 */
export function createZaloOAAuthorizationLink(): Promise<KetQua<comms_zaloOAAuthorizationLinkOut>> {
  return docThanLoiGoi<comms_zaloOAAuthorizationLinkOut>(
    goiGhi(OA_AUTHORIZATION_LINKS_PATH, "POST", undefined, ZALO_OA_STATUS.linkCreated, undefined, { withCode: true }),
  );
}

/** POST …/check — `getoa` with the STORED token; `result` is an outcome class, never Zalo's words. */
export function checkCommuneZaloOA(): Promise<KetQua<comms_zaloOACheckResultOut>> {
  return docThanLoiGoi<comms_zaloOACheckResultOut>(goiGhi(OA_CHECK_PATH, "POST", undefined, ZALO_OA_STATUS.checked));
}

/**
 * DELETE zalo-oas/current with `{reason}` — soft, trailed (rule 7); every attached milestone is detached
 * in the same transaction (ADR 0098 #11). The caller confirms that BEFORE calling.
 */
export function disconnectCommuneZaloOA(reason: string): Promise<KetQua<comms_communeZaloOADisconnectedOut>> {
  const sent: comms_communeZaloOADisconnectIn = { reason };
  return docThanLoiGoi<comms_communeZaloOADisconnectedOut>(goiGhi(OA_PATH, "DELETE", sent, ZALO_OA_STATUS.disconnected));
}

// ---- ZBS templates of the OA (ADR 0098 #8–#10) -----------------------------------------------------

/** GET …/templates — what was loaded from the OA at the last sync, with each milestone attachment. */
export function listZaloOATemplates(): Promise<KetQua<comms_zaloOATemplateList>> {
  return docJSON<comms_zaloOATemplateList>(OA_TEMPLATES_PATH);
}

/** POST …/templates/sync, no body — `template/all` then `template/info` per template, on the server. */
export function syncZaloOATemplates(): Promise<KetQua<comms_zaloOASyncResultOut>> {
  return docThanLoiGoi<comms_zaloOASyncResultOut>(goiGhi(OA_TEMPLATES_SYNC_PATH, "POST", undefined, ZALO_OA_STATUS.synced));
}

/**
 * PATCH …/templates/{id} with `{milestone}` — ALWAYS a key, null = detach (`JSON.stringify` would drop
 * `undefined`, and an absent key is not "detach"). 409 `milestone_taken` / 422 `template_not_attachable`
 * come back as the server's sentence, verbatim.
 */
export function attachZaloOATemplate(id: string, milestone: string | null): Promise<KetQua<comms_zaloOATemplateOut>> {
  const sent: comms_zaloOATemplatePatchIn = { milestone };
  return docThanLoiGoi<comms_zaloOATemplateOut>(goiGhi(templatePath(id), "PATCH", sent, ZALO_OA_STATUS.templatePatched));
}
