/**
 * The commune's Zalo OA (ADR 0098, khối A): the OA the commune AUTHORISED to the platform's shared Zalo
 * app, and the ZBS templates loaded from it. Everything under `zalo-oas/current`, `admin.zalo`
 * (`service-comms/internal/http/zalo_oa.go`).
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
import type {
  comms_communeZaloOACurrentOut as gen_communeZaloOACurrentOut,
  comms_communeZaloOADisconnectedOut,
  comms_communeZaloOADisconnectIn,
  comms_communeZaloOAOut as gen_communeZaloOAOut,
  comms_delete_zalo_oas_current,
  comms_get_zalo_oas_current,
  comms_get_zalo_oas_current_templates,
  comms_patch_zalo_oas_current_templates_by_id,
  comms_post_zalo_oas_current_authorization_links,
  comms_post_zalo_oas_current_check,
  comms_post_zalo_oas_current_templates_sync,
  comms_zaloOAAuthorizationLinkOut,
  comms_zaloOACheckResultOut,
  comms_zaloOASyncResultOut,
  comms_zaloOATemplateListOut,
  comms_zaloOATemplateOut as gen_zaloOATemplateOut,
  comms_zaloOATemplateParamOut,
  comms_zaloOATemplatePatchIn,
} from "./schema.gen";

// Every shape is `schema.gen.ts`'s (agent rule 6), generated from service-comms/internal/http/zalo_oa.go.
// The ONLY additions here narrow three server strings to the closed sets the screen labels: Zalo's own
// template status and quality words (ADR 0098 #8, migration 0031 CHECKs) and the two re-authorization
// reasons (ADR 0098 #6). A value outside them is a red `tsc` at the label tables, not a blank cell.

/** Why the OA must be authorised again (ADR 0098 #6; 0031 zalo_commune_oa_reauthorization_reason_known). */
export type ZaloOAReauthorizationReason =
  "refresh-token-bi-tu-choi" | "refresh-token-het-han";
export type ZaloTemplateStatus =
  "ENABLE" | "PENDING_REVIEW" | "REJECT" | "DISABLE" | "DELETE";
export type ZaloTemplateQuality = "HIGH" | "MEDIUM" | "LOW" | "UNDEFINED";

/** The live OA row. NO token field exists on this shape, by contract. */
export type comms_communeZaloOAOut = Omit<
  gen_communeZaloOAOut,
  "needs_reauthorization"
> & {
  needs_reauthorization: null | {
    since: string;
    reason: ZaloOAReauthorizationReason;
  };
};

/** GET zalo-oas/current. `configured`: the platform has declared its shared Zalo app (ZALO_OA_* set). */
export type comms_communeZaloOACurrentOut = Omit<
  gen_communeZaloOACurrentOut,
  "oa"
> & {
  oa: comms_communeZaloOAOut | null;
};

/** One template loaded from the OA (ADR 0098 #8–#10). */
export type comms_zaloOATemplateOut = Omit<
  gen_zaloOATemplateOut,
  "zalo_status" | "zalo_quality"
> & {
  zalo_status: ZaloTemplateStatus;
  zalo_quality: ZaloTemplateQuality;
};

/** GET …/templates. `synced_at` null = never loaded from the OA. */
export type comms_zaloOATemplateList = Omit<
  comms_zaloOATemplateListOut,
  "items"
> & { items: comms_zaloOATemplateOut[] };

/** One parameter a template declares on Zalo's portal, as `template/info` returns it. */
export type comms_zaloOATemplateParam = comms_zaloOATemplateParamOut;

export type {
  comms_communeZaloOADisconnectedOut,
  comms_communeZaloOADisconnectIn,
  comms_zaloOAAuthorizationLinkOut,
  comms_zaloOACheckResultOut,
  comms_zaloOASyncResultOut,
  comms_zaloOATemplatePatchIn,
};

const OA_PATH =
  "/api/v1/zalo-oas/current" satisfies comms_get_zalo_oas_current["duongDan"] &
    comms_delete_zalo_oas_current["duongDan"];
const OA_AUTHORIZATION_LINKS_PATH =
  "/api/v1/zalo-oas/current/authorization-links" satisfies comms_post_zalo_oas_current_authorization_links["duongDan"];
const OA_CHECK_PATH =
  "/api/v1/zalo-oas/current/check" satisfies comms_post_zalo_oas_current_check["duongDan"];
const OA_TEMPLATES_PATH =
  "/api/v1/zalo-oas/current/templates" satisfies comms_get_zalo_oas_current_templates["duongDan"];
const OA_TEMPLATES_SYNC_PATH =
  "/api/v1/zalo-oas/current/templates/sync" satisfies comms_post_zalo_oas_current_templates_sync["duongDan"];
const OA_TEMPLATE_TEMPLATE =
  "/api/v1/zalo-oas/current/templates/{id}" satisfies comms_patch_zalo_oas_current_templates_by_id["duongDan"];

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
export function getCommuneZaloOA(): Promise<
  KetQua<comms_communeZaloOACurrentOut>
> {
  return docJSON<comms_communeZaloOACurrentOut>(OA_PATH);
}

/**
 * POST …/authorization-links, no body (the commune from `Host`, the officer from the session). The reply
 * holds a one-time link with a `state`: returned to the caller as is, to be opened in a new tab and
 * dropped — never stored, logged or cached here.
 */
export function createZaloOAAuthorizationLink(): Promise<
  KetQua<comms_zaloOAAuthorizationLinkOut>
> {
  return docThanLoiGoi<comms_zaloOAAuthorizationLinkOut>(
    goiGhi(
      OA_AUTHORIZATION_LINKS_PATH,
      "POST",
      undefined,
      ZALO_OA_STATUS.linkCreated,
      undefined,
      { withCode: true },
    ),
  );
}

/** POST …/check — `getoa` with the STORED token; `result` is an outcome class, never Zalo's words. */
export function checkCommuneZaloOA(): Promise<
  KetQua<comms_zaloOACheckResultOut>
> {
  return docThanLoiGoi<comms_zaloOACheckResultOut>(
    goiGhi(OA_CHECK_PATH, "POST", undefined, ZALO_OA_STATUS.checked),
  );
}

/**
 * DELETE zalo-oas/current with `{reason}` — soft, trailed (rule 7); every attached milestone is detached
 * in the same transaction (ADR 0098 #11). The caller confirms that BEFORE calling.
 */
export function disconnectCommuneZaloOA(
  reason: string,
): Promise<KetQua<comms_communeZaloOADisconnectedOut>> {
  const sent: comms_communeZaloOADisconnectIn = { reason };
  return docThanLoiGoi<comms_communeZaloOADisconnectedOut>(
    goiGhi(OA_PATH, "DELETE", sent, ZALO_OA_STATUS.disconnected),
  );
}

// ---- ZBS templates of the OA (ADR 0098 #8–#10) -----------------------------------------------------

/** GET …/templates — what was loaded from the OA at the last sync, with each milestone attachment. */
export function listZaloOATemplates(): Promise<
  KetQua<comms_zaloOATemplateList>
> {
  return docJSON<comms_zaloOATemplateList>(OA_TEMPLATES_PATH);
}

/** POST …/templates/sync, no body — `template/all` then `template/info` per template, on the server. */
export function syncZaloOATemplates(): Promise<
  KetQua<comms_zaloOASyncResultOut>
> {
  return docThanLoiGoi<comms_zaloOASyncResultOut>(
    goiGhi(OA_TEMPLATES_SYNC_PATH, "POST", undefined, ZALO_OA_STATUS.synced),
  );
}

/**
 * PATCH …/templates/{id} with `{milestone}` — ALWAYS a key, null = detach (`JSON.stringify` would drop
 * `undefined`, and an absent key is not "detach"). 409 `milestone_taken` / 422 `template_not_attachable`
 * come back as the server's sentence, verbatim.
 */
export function attachZaloOATemplate(
  id: string,
  milestone: string | null,
): Promise<KetQua<comms_zaloOATemplateOut>> {
  const sent: comms_zaloOATemplatePatchIn = { milestone };
  return docThanLoiGoi<comms_zaloOATemplateOut>(
    goiGhi(templatePath(id), "PATCH", sent, ZALO_OA_STATUS.templatePatched),
  );
}
