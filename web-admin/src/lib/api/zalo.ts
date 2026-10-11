/**
 * Zalo Bot, the commune side (ADR 0074 §"Tài nguyên URL", ADR 0079 Q1 #5):
 *   - the signed-in staff member's OWN link — `zalo-links/current` (+ `/pairing-codes`,
 *     `/test-messages`), `AnyAuthenticated`, filtered server-side by the SESSION's staff code: no staff
 *     code, no id, no commune travels from here (rule 1 forbidden #2, rule 4 inv. 2);
 *   - the commune's channel settings and who is linked — `zalo-channel-settings`, `zalo-links`,
 *     `admin.lookup`;
 *   - the commune's OWN bot — `zalo-bots/current` (+ `/check`, `/webhook`; DELETE = back to the shared
 *     bot), `admin.lookup` (`service-comms/internal/http/zalo_commune_bot.go`);
 *   - the commune's Zalo GROUPS (ADR 0092) — `zalo-groups` (+ `/{id}`) and `zalo-group-pairing-codes`,
 *     `admin.lookup` (`service-comms/internal/http/zalo_groups.go`).
 *
 * EVERY SHAPE IS `schema.gen.ts`'s (agent rule 6). The names below are ALIASES kept for the callers
 * that already import them (`features/zalo`, `dev-preview`), never a second copy of a shape; each path
 * carries its `satisfies …["duongDan"]`, so a renamed route is a red `tsc` here.
 *
 * SECRETS, BOTH DIRECTIONS:
 *   - `bot_token` goes UP only, and only when staff typed one (absent = keep the live bot's token);
 *     no reply carries it;
 *   - the webhook `secret` comes DOWN once, in the reply of the call that generated it (ADR 0079
 *     Q1 #3). This file hands it to the caller and keeps nothing.
 * Nothing here logs, caches or echoes a body (rule 3, rule 8). NO `chat_id` ANYWHERE: comms never
 * returns it (ADR 0074), and no generated type has a field that could carry it.
 *
 * Enum VALUES are the stored Vietnamese-without-diacritics strings (ADR 0011).
 */

import { docJSON, docThanLoiGoi, goiGhi } from "./goi";
import type { KetQua } from "./goi";
import type {
  comms_communeZaloBotCurrentOut,
  comms_communeZaloBotIn,
  comms_communeZaloBotRetiredOut,
  comms_communeZaloBotSavedOut,
  comms_delete_zalo_bots_current,
  comms_delete_zalo_links_current,
  comms_get_zalo_bots_current,
  comms_get_zalo_channel_settings,
  comms_get_zalo_links,
  comms_get_zalo_links_current,
  comms_pairingCodeOut,
  comms_post_zalo_bots_current_check,
  comms_post_zalo_bots_current_webhook,
  comms_post_zalo_links_current_pairing_codes,
  comms_post_zalo_links_current_test_messages,
  comms_put_zalo_bots_current,
  comms_put_zalo_channel_settings,
  comms_zaloBotCheckResultOut,
  comms_zaloBotWebhookOut,
  comms_zaloChannelSettingsIn,
  comms_zaloChannelSettingsOut,
  comms_zaloLinkCurrentOut,
  comms_zaloLinkedStaffList,
  comms_zaloLinkedStaffOut,
} from "./schema.gen";

/** GET /api/v1/zalo-links/current — the signed-in person's own link, never anybody else's. */
export type ZaloLinkCurrent = comms_zaloLinkCurrentOut;

/** POST …/pairing-codes — a one-time 8-character code, alive 10 minutes (ADR 0074 business rules). */
export type ZaloPairingCode = comms_pairingCodeOut;

/**
 * GET · PUT /api/v1/zalo-channel-settings. `kinds` and `supported_events` are per-domain kinds
 * (`nhiem-vu.qua-han`, … — comms migration 0021); `platform_ready` is sent by the GET only.
 */
export type ZaloChannelSettings = comms_zaloChannelSettingsOut & DueSoonDaysUntilRegen;

export type ZaloChannelSettingsChange = comms_zaloChannelSettingsIn & Required<DueSoonDaysUntilRegen>;

/**
 * Until contract regen: `due_soon_days` (comms 3c9f14fd, `service-comms/internal/http/zalo_links.go:112,133`)
 * is not in the COMMITTED `schema.gen.ts` — regenerating it here would mix in other sessions'
 * uncommitted routes. Same stopgap as `optionalColorIn` in `danh-muc.ts`. It is harmless once the
 * regenerated file carries the field (the intersection then adds nothing); delete this and the two
 * intersections above in the commit that lands that regen.
 *
 * Wire meaning: 1–14 = the Zalo copy of a due-soon digest keeps only records whose STORED deadline is
 * within that many days; null = no narrowing (the bell's set, as is). The bell and the "Sắp đến hạn"
 * list never read it (ADR 0079 lô 5 Q13). REQUIRED on the PUT side: the PUT replaces the whole row, so
 * a body without it CLEARS the commune's choice.
 */
type DueSoonDaysUntilRegen = { due_soon_days?: number | null };

// ---- UNTIL REGEN: Zalo groups (ADR 0092, comms cb0cab90) ----------------------------------------
//
// `schema.gen.ts` is NOT regenerated yet: `make kb` has not run in the shared tree, and regenerating it
// here would mix in other sessions' uncommitted routes (the same stopgap as `DueSoonDaysUntilRegen`
// above). Every shape below is copied from `service-comms/internal/http/zalo_groups.go` (groupPairingCodeIn
// · groupPairingCodeOut · zaloGroupOut · zaloGroupList · zaloGroupPatchIn · zaloGroupUnlinkIn) and
// `zalo_commune_bot.go` (live_group_count · ended_group_count), named as the generator will name them.
//
// DELETE THIS WHOLE BLOCK in the commit that lands the regen: import the `comms_*` names from
// `schema.gen` instead, and turn each `…Route` below into the generated route type in the `satisfies`.
// A shape that drifted from the server is then a red `tsc`, which is the point of rule 6.

/** POST zalo-group-pairing-codes. `org_unit_id` null = the whole commune; `kinds` non-empty. */
export type comms_groupPairingCodeIn = { org_unit_id: string | null; kinds: string[] };
/** 201 — the code is shown ONCE (`Cache-Control: no-store`); the server keeps only its hash. */
export type comms_groupPairingCodeOut = { code: string; expires_at: string };
/**
 * One live group. `org_unit_id` null = the whole commune; `org_unit_name` "" when identity holds no name;
 * `org_unit_removed`: the unit left the org chart. `linked_by` is the code ISSUER's business code. NO chat id.
 */
export type comms_zaloGroupOut = {
  id: string;
  org_unit_id: string | null;
  org_unit_name: string;
  org_unit_removed: boolean;
  kinds: string[];
  linked_at: string;
  linked_by: string;
  linked_by_name: string;
};
/** GET zalo-groups. `supported_kinds`: the kinds a group may ever tick, in comms' display order. */
export type comms_zaloGroupList = { items: comms_zaloGroupOut[]; supported_kinds: string[] };
/** PATCH zalo-groups/{id}: an ABSENT (or null) key keeps its value; `org_unit_id: ""` = the whole commune. */
export type comms_zaloGroupPatchIn = { org_unit_id?: string; kinds?: string[] };
/** DELETE zalo-groups/{id} — rule 7: a soft delete carries its reason. */
export type comms_zaloGroupUnlinkIn = { reason: string };

type PairingCodesRoute = { duongDan: "/api/v1/zalo-group-pairing-codes" };
type GroupsRoute = { duongDan: "/api/v1/zalo-groups" };
type GroupByIdRoute = { duongDan: "/api/v1/zalo-groups/{id}" };

/** The two counts comms added to the bot routes for groups (zalo_commune_bot.go:90,110,144). */
type BotGroupCountsUntilRegen = { live_group_count?: number };
type BotEndedGroupsUntilRegen = { ended_group_count?: number };
export type CommuneZaloBotCurrent = comms_communeZaloBotCurrentOut & BotGroupCountsUntilRegen;
export type CommuneZaloBotSaved = comms_communeZaloBotSavedOut & BotEndedGroupsUntilRegen;
export type CommuneZaloBotRetired = comms_communeZaloBotRetiredOut & BotEndedGroupsUntilRegen;

// ---- end UNTIL REGEN ---------------------------------------------------------------------------

/** GET /api/v1/zalo-links — staff of THIS commune with a live link. Names, codes, a date; no chat id. */
export type ZaloLinkedStaff = comms_zaloLinkedStaffOut;

const CURRENT_PATH = "/api/v1/zalo-links/current" satisfies comms_get_zalo_links_current["duongDan"] &
  comms_delete_zalo_links_current["duongDan"];
const PAIRING_PATH =
  "/api/v1/zalo-links/current/pairing-codes" satisfies comms_post_zalo_links_current_pairing_codes["duongDan"];
const TEST_PATH =
  "/api/v1/zalo-links/current/test-messages" satisfies comms_post_zalo_links_current_test_messages["duongDan"];
const LINKS_PATH = "/api/v1/zalo-links" satisfies comms_get_zalo_links["duongDan"];
const SETTINGS_PATH = "/api/v1/zalo-channel-settings" satisfies comms_get_zalo_channel_settings["duongDan"] &
  comms_put_zalo_channel_settings["duongDan"];
const BOT_PATH = "/api/v1/zalo-bots/current" satisfies comms_get_zalo_bots_current["duongDan"] &
  comms_put_zalo_bots_current["duongDan"] &
  comms_delete_zalo_bots_current["duongDan"];
const BOT_CHECK_PATH = "/api/v1/zalo-bots/current/check" satisfies comms_post_zalo_bots_current_check["duongDan"];
const BOT_WEBHOOK_PATH =
  "/api/v1/zalo-bots/current/webhook" satisfies comms_post_zalo_bots_current_webhook["duongDan"];
const GROUP_PAIRING_PATH = "/api/v1/zalo-group-pairing-codes" satisfies PairingCodesRoute["duongDan"];
const GROUPS_PATH = "/api/v1/zalo-groups" satisfies GroupsRoute["duongDan"];
const GROUP_TEMPLATE = "/api/v1/zalo-groups/{id}" satisfies GroupByIdRoute["duongDan"];

/** `encodeURIComponent`: the id goes into the PATH, not the body. */
function groupPath(id: string): string {
  return GROUP_TEMPLATE.replace("{id}", encodeURIComponent(id));
}

/** Success statuses, as the contract publishes them (`phanHoi` of each route in `schema.gen.ts`). */
export const ZALO_STATUS = {
  pairingCreated: 201,
  testSent: 200,
  unlinked: 204,
  settingsSaved: 200,
  botSaved: 200,
  botChecked: 200,
  webhookRegistered: 200,
  botRetired: 200,
  groupCodeCreated: 201,
  groupEdited: 204,
  groupUnlinked: 204,
} as const;

export function getCurrentZaloLink(): Promise<KetQua<ZaloLinkCurrent>> {
  return docJSON<ZaloLinkCurrent>(CURRENT_PATH);
}

/** No body: whose code it is comes from the session, never from the request. */
export function createPairingCode(): Promise<KetQua<ZaloPairingCode>> {
  return docThanLoiGoi<ZaloPairingCode>(goiGhi(PAIRING_PATH, "POST", undefined, ZALO_STATUS.pairingCreated));
}

/** Soft delete server-side, trailed (rule 7). No body. */
export async function unlinkCurrentZalo(): Promise<KetQua<null>> {
  const r = await goiGhi(CURRENT_PATH, "DELETE", undefined, ZALO_STATUS.unlinked);
  return r.ok ? { ok: true, duLieu: null } : r;
}

/** One fixed test message to the person's own linked chat. */
export async function sendZaloTestMessage(): Promise<KetQua<null>> {
  const r = await goiGhi(TEST_PATH, "POST", undefined, ZALO_STATUS.testSent);
  return r.ok ? { ok: true, duLieu: null } : r;
}

export function getZaloChannelSettings(): Promise<KetQua<ZaloChannelSettings>> {
  return docJSON<ZaloChannelSettings>(SETTINGS_PATH);
}

/**
 * Built key by key: a `...body` is how a stray field — `updated_by`, `supported_events` — goes up.
 * `due_soon_days` is ALWAYS a key, null included (`JSON.stringify` drops `undefined`, and an absent key
 * clears the stored lead).
 *
 * `withCode`: the second documented reader of `KetQua.code` — `due_soon_days_out_of_range` is drawn
 * under its own select instead of a toast; the sentence is still the server's, verbatim.
 */
export function saveZaloChannelSettings(change: ZaloChannelSettingsChange): Promise<KetQua<ZaloChannelSettings>> {
  const sent: ZaloChannelSettingsChange = {
    is_enabled: change.is_enabled,
    kinds: [...change.kinds],
    quiet_start: change.quiet_start,
    quiet_end: change.quiet_end,
    overdue_start_after_days: change.overdue_start_after_days,
    overdue_repeat_every_days: change.overdue_repeat_every_days,
    due_soon_days: change.due_soon_days ?? null,
  };
  return docThanLoiGoi<ZaloChannelSettings>(
    goiGhi(SETTINGS_PATH, "PUT", sent, ZALO_STATUS.settingsSaved, undefined, { withCode: true }),
  );
}

export async function listZaloLinkedStaff(): Promise<KetQua<ZaloLinkedStaff[]>> {
  const r = await docJSON<comms_zaloLinkedStaffList>(LINKS_PATH);
  return r.ok ? { ok: true, duLieu: r.duLieu.items } : r;
}

// ---- the commune's own bot ---------------------------------------------------------------------

/** GET zalo-bots/current: own bot or not, and `live_link_count` — never the token, never the secret. */
export function getCommuneZaloBot(): Promise<KetQua<CommuneZaloBotCurrent>> {
  return docJSON<CommuneZaloBotCurrent>(BOT_PATH);
}

/**
 * PUT zalo-bots/current — "Lưu con bot". Built key by key. `bot_token` is sent ONLY when staff typed
 * one, VERBATIM (not trimmed: a trimmed copy is a different token); absent = keep the live bot's
 * token. A token for another bot account ends every live link in the same transaction (ADR 0079
 * Q1 #4) — the caller confirms that BEFORE calling.
 */
export function saveCommuneZaloBot(body: comms_communeZaloBotIn): Promise<KetQua<CommuneZaloBotSaved>> {
  const sent: comms_communeZaloBotIn = { bot_name: body.bot_name, chat_url: body.chat_url };
  if (body.bot_token !== undefined && body.bot_token !== "") sent.bot_token = body.bot_token;
  return docThanLoiGoi<CommuneZaloBotSaved>(goiGhi(BOT_PATH, "PUT", sent, ZALO_STATUS.botSaved));
}

/** POST …/check — getMe with the STORED token; `result` is an outcome class, never Zalo's words. */
export function checkCommuneZaloBot(): Promise<KetQua<comms_zaloBotCheckResultOut>> {
  return docThanLoiGoi<comms_zaloBotCheckResultOut>(goiGhi(BOT_CHECK_PATH, "POST", undefined, ZALO_STATUS.botChecked));
}

/**
 * POST …/webhook. The reply may carry `secret` — ONCE (ADR 0079 Q1 #3). Returned to the caller as
 * is; the caller shows it and drops it.
 */
export function registerCommuneZaloWebhook(): Promise<KetQua<comms_zaloBotWebhookOut>> {
  return docThanLoiGoi<comms_zaloBotWebhookOut>(
    goiGhi(BOT_WEBHOOK_PATH, "POST", undefined, ZALO_STATUS.webhookRegistered),
  );
}

/** DELETE zalo-bots/current with `{reason}` — back to the shared bot; a soft delete carries its reason (rule 7). */
export function retireCommuneZaloBot(reason: string): Promise<KetQua<CommuneZaloBotRetired>> {
  return docThanLoiGoi<CommuneZaloBotRetired>(goiGhi(BOT_PATH, "DELETE", { reason }, ZALO_STATUS.botRetired));
}

// ---- Zalo groups (ADR 0092) ---------------------------------------------------------------------
//
// `withCode` on every group write: the section draws each refusal under the field it is about (scope,
// kinds, reason) — the fourth documented reader of `KetQua.code` (see `goi.ts`).

/** GET zalo-groups — the commune's live groups and the kinds a group may tick. Never a chat id. */
export function listZaloGroups(): Promise<KetQua<comms_zaloGroupList>> {
  return docJSON<comms_zaloGroupList>(GROUPS_PATH);
}

/**
 * POST zalo-group-pairing-codes. Built key by key; `org_unit_id` is ALWAYS a key (null = the whole
 * commune). The reply holds a live credential: returned to the caller as is, never stored, logged or
 * cached here (the server sends it `no-store`).
 */
export function createZaloGroupPairingCode(body: comms_groupPairingCodeIn): Promise<KetQua<comms_groupPairingCodeOut>> {
  const sent: comms_groupPairingCodeIn = { org_unit_id: body.org_unit_id, kinds: [...body.kinds] };
  return docThanLoiGoi<comms_groupPairingCodeOut>(
    goiGhi(GROUP_PAIRING_PATH, "POST", sent, ZALO_STATUS.groupCodeCreated, undefined, { withCode: true }),
  );
}

/** PATCH zalo-groups/{id}. Only the keys the caller set travel: an absent key keeps the stored value. */
export async function editZaloGroup(id: string, patch: comms_zaloGroupPatchIn): Promise<KetQua<null>> {
  const sent: comms_zaloGroupPatchIn = {};
  if (patch.org_unit_id !== undefined) sent.org_unit_id = patch.org_unit_id;
  if (patch.kinds !== undefined) sent.kinds = [...patch.kinds];
  const r = await goiGhi(groupPath(id), "PATCH", sent, ZALO_STATUS.groupEdited, undefined, { withCode: true });
  return r.ok ? { ok: true, duLieu: null } : r;
}

/** DELETE zalo-groups/{id} with `{reason}` — the ONLY way a group is unlinked (ADR 0092 #8); soft, trailed. */
export async function unlinkZaloGroup(id: string, reason: string): Promise<KetQua<null>> {
  const sent: comms_zaloGroupUnlinkIn = { reason };
  const r = await goiGhi(groupPath(id), "DELETE", sent, ZALO_STATUS.groupUnlinked, undefined, { withCode: true });
  return r.ok ? { ok: true, duLieu: null } : r;
}
