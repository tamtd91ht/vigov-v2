/**
 * The commune's Zalo OAs (ADR 0098 khối A; SEVERAL per commune since ADR 0099 #1): every OA the commune
 * AUTHORISED to the platform's shared Zalo app, and the ZBS templates loaded from each. `admin.zalo` on
 * every route (`service-comms/internal/http/zalo_oa.go`). There is no "current" OA any more: every act on
 * one OA names it by its row `id` (the `id` of `GET zalo-oas`), and an id that is not a live OA of this
 * commune is `404 not_found` — one sentence for unknown, retired and another commune's.
 *
 * SECRETS, ONE DIRECTION ONLY — DOWN, AND NEVER HERE:
 *   - no reply carries an access token, a refresh token or a code verifier (ADR 0098 #5: write-only);
 *   - the AUTHORISATION LINK (`POST …/authorization-links`) carries a one-time `state` — handed to the
 *     caller as is, to be OPENED, never stored, logged, cached or echoed into a `title`/`aria-label`.
 * Nothing here logs or caches a body (rule 3, rule 8). No tenant, no staff code in any request: the
 * commune comes from `Host`, the actor from the session (rule 1 forbidden #2). The OA row id is the one
 * client-supplied value, and comms resolves it inside the commune's scoped read.
 *
 * Enum VALUES: outcome classes are the stored Vietnamese-without-diacritics strings (ADR 0011);
 * `zalo_status` / `zalo_quality` are Zalo's own upper-case words, stored as Zalo sends them (ADR 0098 #8).
 */

import { docJSON, docThanLoiGoi, goiGhi } from "./goi";
import type { KetQua } from "./goi";
import type {
  comms_communeZaloOADisconnectedOut,
  comms_communeZaloOADisconnectIn,
  comms_zaloOAAuthorizationLinkOut,
  comms_zaloOACheckResultOut,
  comms_zaloOALastCheckOut,
  comms_zaloOASyncResultOut,
  comms_zaloOATemplateParamOut,
  comms_zaloOATemplatePatchIn,
} from "./schema.gen";

// The shapes imported above are `schema.gen.ts`'s and did not change with ADR 0099 (agent rule 6). The
// ONLY additions here narrow three server strings to the closed sets the screen labels: Zalo's own
// template status and quality words (ADR 0098 #8, migration 0031 CHECKs) and the two re-authorization
// reasons (ADR 0098 #6). A value outside them is a red `tsc` at the label tables, not a blank cell.

/** Why the OA must be authorised again (ADR 0098 #6; 0031 zalo_commune_oa_reauthorization_reason_known). */
export type ZaloOAReauthorizationReason =
  "refresh-token-bi-tu-choi" | "refresh-token-het-han";
export type ZaloTemplateStatus =
  "ENABLE" | "PENDING_REVIEW" | "REJECT" | "DISABLE" | "DELETE";
export type ZaloTemplateQuality = "HIGH" | "MEDIUM" | "LOW" | "UNDEFINED";

// ---- UNTIL REGEN: several OAs per commune (ADR 0099 #1 #10, comms 2ed395e8) --------------------------
//
// `schema.gen.ts` is NOT regenerated yet: `make kb` has not run in the shared tree, and regenerating it
// here would mix in other sessions' uncommitted routes (the same stopgap as the UNTIL REGEN block of
// `zalo.ts`). It still describes the GONE one-OA routes under `zalo-oas/` + `current`. Every shape below is copied from
// `service-comms/internal/http/zalo_oa.go` (communeZaloOAOut · communeZaloOAListOut · zaloOATemplateOut ·
// zaloOATemplateListOut), named as the generator will name them, and every route type is the path
// registered there.
//
// DELETE THIS WHOLE BLOCK in the commit that lands the regen: build the three narrowed types below with
// `Omit<gen_…>` from `schema.gen` as before, and turn each `…Route` into the generated route type in the
// `satisfies`. A shape that drifted from the server is then a red `tsc`, which is the point of rule 6.

/** One live OA. `id` is the row id every other route names. NO token field exists on this shape, by contract. */
export type comms_communeZaloOAOut = {
  id: string;
  oa_id: string;
  oa_name: string;
  authorized_at: string;
  authorized_by: string;
  authorized_by_name: string;
  access_expires_at: string;
  /** The INSTANT the token row was last written — never a token value (ADR 0098 §Hai cột thời điểm). */
  token_refreshed_at: string;
  needs_reauthorization: null | { since: string; reason: ZaloOAReauthorizationReason };
  last_check: comms_zaloOALastCheckOut | null;
  template_count: number;
  attached_count: number;
};

/** GET zalo-oas: every live OA, oldest first; `items` is [] when none. `configured`: ZALO_OA_* set on the platform. */
export type comms_communeZaloOAListOut = {
  configured: boolean;
  items: comms_communeZaloOAOut[];
};

/** One template loaded from an OA (ADR 0098 #8–#10). `oa_row_id` is the `id` of the OA it belongs to. */
export type comms_zaloOATemplateOut = {
  id: string;
  oa_row_id: string;
  template_id: string;
  template_name: string;
  zalo_status: ZaloTemplateStatus;
  zalo_quality: ZaloTemplateQuality;
  zalo_tag: string;
  params: comms_zaloOATemplateParamOut[];
  params_match: boolean;
  missing_params: string[];
  milestone: string | null;
  milestone_set_at: string | null;
  milestone_set_by: string | null;
  milestone_set_by_name: string | null;
};

/** GET zalo-oas/{id}/templates. `synced_at` null = never loaded from that OA. */
export type comms_zaloOATemplateList = {
  items: comms_zaloOATemplateOut[];
  synced_at: string | null;
};

type OAsRoute = { duongDan: "/api/v1/zalo-oas" };
type OAAuthorizationLinksRoute = { duongDan: "/api/v1/zalo-oas/authorization-links" };
type OAByIdRoute = { duongDan: "/api/v1/zalo-oas/{id}" };
type OACheckRoute = { duongDan: "/api/v1/zalo-oas/{id}/check" };
type OATemplatesRoute = { duongDan: "/api/v1/zalo-oas/{id}/templates" };
type OATemplatesSyncRoute = { duongDan: "/api/v1/zalo-oas/{id}/templates/sync" };
type OATemplateByIdRoute = { duongDan: "/api/v1/zalo-oa-templates/{id}" };

// ---- end UNTIL REGEN ---------------------------------------------------------------------------------

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

const OAS_PATH = "/api/v1/zalo-oas" satisfies OAsRoute["duongDan"];
const OA_AUTHORIZATION_LINKS_PATH =
  "/api/v1/zalo-oas/authorization-links" satisfies OAAuthorizationLinksRoute["duongDan"];
const OA_TEMPLATE = "/api/v1/zalo-oas/{id}" satisfies OAByIdRoute["duongDan"];
const OA_CHECK_TEMPLATE = "/api/v1/zalo-oas/{id}/check" satisfies OACheckRoute["duongDan"];
const OA_TEMPLATES_TEMPLATE = "/api/v1/zalo-oas/{id}/templates" satisfies OATemplatesRoute["duongDan"];
const OA_TEMPLATES_SYNC_TEMPLATE =
  "/api/v1/zalo-oas/{id}/templates/sync" satisfies OATemplatesSyncRoute["duongDan"];
const OA_TEMPLATE_BY_ID_TEMPLATE = "/api/v1/zalo-oa-templates/{id}" satisfies OATemplateByIdRoute["duongDan"];

/** `encodeURIComponent`: the id goes into the PATH, not the body. */
function withId(template: string, id: string): string {
  return template.replace("{id}", encodeURIComponent(id));
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

/** GET zalo-oas: whether the platform is ready, and every live OA of the commune. Never a token. */
export function listCommuneZaloOAs(): Promise<
  KetQua<comms_communeZaloOAListOut>
> {
  return docJSON<comms_communeZaloOAListOut>(OAS_PATH);
}

/**
 * POST zalo-oas/authorization-links, no body (the commune from `Host`, the officer from the session). The
 * OA is the one the OA administrator picks at Zalo: a new one is ADDED beside the others, one already
 * connected here is authorised again (ADR 0099 #1) — so "Kết nối thêm OA" and "Uỷ quyền lại" are the same
 * call. The reply holds a one-time link with a `state`: returned to the caller as is, to be opened in a new
 * tab and dropped — never stored, logged or cached here.
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

/** POST zalo-oas/{id}/check — `getoa` with that OA's STORED token; `result` is an outcome class, never Zalo's words. */
export function checkCommuneZaloOA(
  oaRowId: string,
): Promise<KetQua<comms_zaloOACheckResultOut>> {
  return docThanLoiGoi<comms_zaloOACheckResultOut>(
    goiGhi(withId(OA_CHECK_TEMPLATE, oaRowId), "POST", undefined, ZALO_OA_STATUS.checked),
  );
}

/**
 * DELETE zalo-oas/{id} with `{reason}` — soft, trailed (rule 7); every milestone attached to a template of
 * THAT OA is detached in the same transaction, the commune's other OAs untouched (ADR 0098 #11, ADR 0099
 * #1). The caller confirms that BEFORE calling. A second call is 404.
 */
export function disconnectCommuneZaloOA(
  oaRowId: string,
  reason: string,
): Promise<KetQua<comms_communeZaloOADisconnectedOut>> {
  const sent: comms_communeZaloOADisconnectIn = { reason };
  return docThanLoiGoi<comms_communeZaloOADisconnectedOut>(
    goiGhi(withId(OA_TEMPLATE, oaRowId), "DELETE", sent, ZALO_OA_STATUS.disconnected),
  );
}

// ---- ZBS templates of an OA (ADR 0098 #8–#10) ------------------------------------------------------

/** GET zalo-oas/{id}/templates — what was loaded from that OA at its last sync, with each milestone attachment. */
export function listZaloOATemplates(
  oaRowId: string,
): Promise<KetQua<comms_zaloOATemplateList>> {
  return docJSON<comms_zaloOATemplateList>(withId(OA_TEMPLATES_TEMPLATE, oaRowId));
}

/** POST zalo-oas/{id}/templates/sync, no body — `template/all` then `template/info` per template, on the server. */
export function syncZaloOATemplates(
  oaRowId: string,
): Promise<KetQua<comms_zaloOASyncResultOut>> {
  return docThanLoiGoi<comms_zaloOASyncResultOut>(
    goiGhi(withId(OA_TEMPLATES_SYNC_TEMPLATE, oaRowId), "POST", undefined, ZALO_OA_STATUS.synced),
  );
}

/**
 * PATCH zalo-oa-templates/{id} with `{milestone}` — ALWAYS a key, null = detach (`JSON.stringify` would
 * drop `undefined`, and an absent key is not "detach"). One milestone, one template in the WHOLE commune,
 * whichever OA it is on: 409 `milestone_taken` / 422 `template_not_attachable` come back as the server's
 * sentence, verbatim.
 */
export function attachZaloOATemplate(
  id: string,
  milestone: string | null,
): Promise<KetQua<comms_zaloOATemplateOut>> {
  const sent: comms_zaloOATemplatePatchIn = { milestone };
  return docThanLoiGoi<comms_zaloOATemplateOut>(
    goiGhi(withId(OA_TEMPLATE_BY_ID_TEMPLATE, id), "PATCH", sent, ZALO_OA_STATUS.templatePatched),
  );
}
