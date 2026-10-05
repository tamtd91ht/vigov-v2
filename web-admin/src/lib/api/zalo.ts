/**
 * Zalo Bot, the commune side (ADR 0074 §"Tài nguyên URL"):
 *   - the signed-in staff member's OWN link — `zalo-links/current` (+ `/pairing-codes`,
 *     `/test-messages`), `AnyAuthenticated`, filtered server-side by the SESSION's staff code: no staff
 *     code, no id, no commune travels from here (rule 1 forbidden #2, rule 4 inv. 2);
 *   - the commune's channel settings and who is linked — `zalo-channel-settings`, `zalo-links`,
 *     `admin.lookup`.
 *
 * TYPES ARE HAND-WRITTEN, TEMPORARILY, AND THAT IS A KNOWN DEBT (agent rule 6). The comms routes were
 * built in parallel with this screen and were not yet in `kb/20-contracts/openapi.json`, so
 * `schema.gen.ts` has no shape to import. When the routes are published and `npm run gen:api` emits
 * them, the types below are DELETED in favour of `schema.gen.ts` — never kept beside it — and each
 * path gets its `satisfies …["duongDan"]` like `mail-settings.ts`. The gateway route table
 * (`dinh-tuyen.gen.ts`) must gain `/api/v1/zalo-links` and `/api/v1/zalo-channel-settings` → comms
 * the same way; until then the gateway refuses these paths (fail closed).
 *
 * NO `chat_id` ANYWHERE: comms never returns it (ADR 0074, "không trả ra giao diện"), and no type
 * here has a field that could carry it.
 *
 * Enum VALUES are the stored Vietnamese-without-diacritics strings (ADR 0011).
 */

import { docJSON, docThanLoiGoi, goiGhi } from "./goi";
import type { KetQua } from "./goi";

/** GET /api/v1/zalo-links/current — the signed-in person's own link, never anybody else's. */
export type ZaloLinkCurrent = {
  linked: boolean;
  linked_at?: string;
  bot_name?: string;
  chat_url?: string;
  /** The commune's switch (`zalo-channel-settings.is_enabled`). A commune with no row is off. */
  channel_enabled: boolean;
};

/** POST …/pairing-codes — a one-time 8-character code, alive 10 minutes (ADR 0074 business rules). */
export type ZaloPairingCode = { code: string; expires_at: string; chat_url: string };

/** The four reminder kinds of wave 1 (`comms.proto` DUE_SOON / OVERDUE / ESCALATION / WEEKLY_DIGEST). */
export type ZaloReminderKind = "sap-den-han" | "qua-han" | "leo-thang" | "ban-tin-tuan";

export const ZALO_REMINDER_KINDS: readonly ZaloReminderKind[] = ["sap-den-han", "qua-han", "leo-thang", "ban-tin-tuan"];

/**
 * GET · PUT /api/v1/zalo-channel-settings. The two overdue cadences are null unless `qua-han` is
 * chosen, and REQUIRED when it is (the server refuses otherwise). Quiet hours are "HH:MM", Vietnam time.
 */
export type ZaloChannelSettings = {
  is_enabled: boolean;
  kinds: ZaloReminderKind[];
  quiet_start: string;
  quiet_end: string;
  overdue_start_after_days: number | null;
  overdue_repeat_every_days: number | null;
  updated_at?: string;
  /** Staff BUSINESS code (rule 6 inv. 8). */
  updated_by?: string;
};

export type ZaloChannelSettingsChange = Omit<ZaloChannelSettings, "updated_at" | "updated_by">;

/** GET /api/v1/zalo-links — staff of THIS commune with a live link. Names, codes, a date; no chat id. */
export type ZaloLinkedStaff = { staff_code: string; staff_name: string; linked_at: string };

const CURRENT_PATH = "/api/v1/zalo-links/current";
const PAIRING_PATH = "/api/v1/zalo-links/current/pairing-codes";
const TEST_PATH = "/api/v1/zalo-links/current/test-messages";
const LINKS_PATH = "/api/v1/zalo-links";
const SETTINGS_PATH = "/api/v1/zalo-channel-settings";

/**
 * Success statuses ASSUMED for the write routes (the contract was not published when this was
 * written): a new pairing code is a created resource (201), the test send and the PUT answer 200, the
 * unlink 204. One place to correct when the contract lands.
 */
export const ZALO_STATUS = { pairingCreated: 201, testSent: 200, unlinked: 204, settingsSaved: 200 } as const;

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

/** Built key by key: a `...body` is how a stray field — `updated_by`, say — goes up. */
export function saveZaloChannelSettings(change: ZaloChannelSettingsChange): Promise<KetQua<ZaloChannelSettings>> {
  const sent: ZaloChannelSettingsChange = {
    is_enabled: change.is_enabled,
    kinds: [...change.kinds],
    quiet_start: change.quiet_start,
    quiet_end: change.quiet_end,
    overdue_start_after_days: change.overdue_start_after_days,
    overdue_repeat_every_days: change.overdue_repeat_every_days,
  };
  return docThanLoiGoi<ZaloChannelSettings>(goiGhi(SETTINGS_PATH, "PUT", sent, ZALO_STATUS.settingsSaved));
}

export async function listZaloLinkedStaff(): Promise<KetQua<ZaloLinkedStaff[]>> {
  const r = await docJSON<{ items: ZaloLinkedStaff[] }>(LINKS_PATH);
  return r.ok ? { ok: true, duLieu: r.duLieu.items } : r;
}
