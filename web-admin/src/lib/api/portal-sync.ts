/**
 * The six portal sync routes behind §3 of `docs/ui-ux/11-noi-dung-mini-app.md` (ADR 0067 §2), exactly
 * the set `service-comms/internal/http/routes_portal_sync.go` declares:
 *
 *   GET  /api/v1/portal-sync/settings     content.read
 *   PUT  /api/v1/portal-sync/settings     content.update  (no Idempotency-Key: PUT of the whole form)
 *   GET  /api/v1/portal-sync/categories   content.read    (asks the PORTAL live — 409 / 502 possible)
 *   PUT  /api/v1/portal-sync/categories   content.update
 *   GET  /api/v1/portal-sync/runs         content.read    (newest first, paginated)
 *   POST /api/v1/portal-sync/runs         content.update  + Idempotency-Key REQUIRED → 202
 *
 * THE KEY IS WRITE-ONLY, AND THIS FILE IS THE ONLY PLACE IT TRAVELS. `api_key` is put on the PUT body
 * only when the officer typed one; nothing here reads it back (the response has `api_key_set`, never
 * the key), logs it, or puts it in a URL. A blank key means "keep the stored one" at the server.
 *
 * TYPES ARE THE CONTRACT'S (`schema.gen.ts`, generated from `kb/20-contracts/openapi.json`), never retyped.
 *
 * NO `tenant_id` ANYWHERE — the commune comes from `Host` at the edge (rule 1, forbidden #2).
 *
 * EVERY REFUSAL REACHES THE SCREEN AS THE SERVER'S OWN SENTENCE (`KetQua.thongBao`): 422
 * `api_key_required_for_new_url`, 503 `encryption_not_configured`, 409 `portal_sync_in_progress`, 502
 * `portal_<class>` all carry a Vietnamese sentence written for the administrator. Rewriting them here
 * would be a second copy of the server's rules.
 */

import { docJSON, docThanKetQua, goiGhi, type KetQua } from "./goi"; // vi-name-ok: existing exports of goi.ts (rule 12 inv 3)
import type {
  comms_get_portal_sync_categories,
  comms_get_portal_sync_runs,
  comms_get_portal_sync_settings,
  comms_portalCategoriesIn,
  comms_portalCategoriesOut,
  comms_portalCategoryTreeOut,
  comms_portalSyncSettingsIn,
  comms_portalSyncSettingsOut,
  page_Result_comms_portalRunOut,
} from "./schema.gen";

const SETTINGS_PATH: comms_get_portal_sync_settings["duongDan"] = "/api/v1/portal-sync/settings";
const CATEGORIES_PATH: comms_get_portal_sync_categories["duongDan"] = "/api/v1/portal-sync/categories";
const RUNS_PATH: comms_get_portal_sync_runs["duongDan"] = "/api/v1/portal-sync/runs";

/** GET settings — what the card and the `Cấu hình` form start from. */
export function getPortalSyncSettings(): Promise<KetQua<comms_portalSyncSettingsOut>> {
  return docJSON<comms_portalSyncSettingsOut>(SETTINGS_PATH);
}

/**
 * PUT settings. FIELD BY FIELD, never `...body`: the form state is built from a settings RESPONSE, and a
 * spread is how a field the PUT does not take would ride along one day.
 *
 * `api_key` is copied only when non-empty — omitted means "keep" at the server (portal_sync.go:70).
 */
export function savePortalSyncSettings(
  body: comms_portalSyncSettingsIn,
): Promise<KetQua<comms_portalSyncSettingsOut>> {
  const sent: comms_portalSyncSettingsIn = {
    api_url: body.api_url,
    publish_mode: body.publish_mode,
    interval_hours: body.interval_hours,
    window_days: body.window_days,
    max_items_per_run: body.max_items_per_run,
    keep_source_credit: body.keep_source_credit,
    is_enabled: body.is_enabled,
  };
  if (body.api_key !== undefined && body.api_key !== "") sent.api_key = body.api_key;
  return goiGhi(SETTINGS_PATH, "PUT", sent, 200, undefined).then(
    docThanKetQua<comms_portalSyncSettingsOut>,
  );
}

/**
 * GET categories — the portal's tree ASKED NOW (ADR 0067 §2 "Chế độ đăng" #4: never copied), plus the
 * stored selections the portal no longer lists. Called only when the `Cấu hình` section is open: every
 * call is an outbound request to the commune's portal.
 */
export function getPortalCategories(): Promise<KetQua<comms_portalCategoryTreeOut>> {
  return docJSON<comms_portalCategoryTreeOut>(CATEGORIES_PATH);
}

/** PUT categories — the selection and its mapping. Entries rebuilt field by field. */
export function savePortalCategories(
  body: comms_portalCategoriesIn,
): Promise<KetQua<comms_portalCategoriesOut>> {
  const sent: comms_portalCategoriesIn = {
    categories: body.categories.map((c) => ({
      external_id: c.external_id,
      name: c.name,
      target_kind: c.target_kind,
      is_selected: c.is_selected,
    })),
  };
  return goiGhi(CATEGORIES_PATH, "PUT", sent, 200, undefined).then(
    docThanKetQua<comms_portalCategoriesOut>,
  );
}

/** How many runs the history shows. The newest is the card's `Chạy lần cuối` line. */
export const RUNS_PAGE_SIZE = 10;

/** Path of the run history. No `sort`/`order`: the server's default is newest first. */
export function portalSyncRunsPath(limit: number = RUNS_PAGE_SIZE): string {
  const q = new URLSearchParams();
  q.set("limit", String(limit));
  return `${RUNS_PATH}?${q.toString()}`;
}

/** GET runs — newest first. */
export function listPortalSyncRuns(): Promise<KetQua<page_Result_comms_portalRunOut>> {
  return docJSON<page_Result_comms_portalRunOut>(portalSyncRunsPath());
}

/**
 * POST runs — `⟳ Đồng bộ ngay`. 202 means the run row exists and works in the background.
 *
 * THE BODY OF A 202 IS NOT READ. A replayed 202 (same Idempotency-Key, `core/idem`) carries
 * `{replayed: true}`, not a run — so the screen learns the run's state from the history, never from
 * this answer. The caller holds the key (one per press, reused on a retry after a failure, which
 * `core/idem` releases), so a double click becomes one run.
 */
export function startPortalSyncRun(idempotencyKey: string): Promise<KetQua<null>> {
  return goiGhi(RUNS_PATH, "POST", undefined, 202, { "Idempotency-Key": idempotencyKey }).then(
    (r): KetQua<null> => (r.ok ? { ok: true, duLieu: null } : r),
  );
}
