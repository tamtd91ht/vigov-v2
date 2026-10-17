/**
 * Petition ↔ citizen-directory link (ADR 0102) — two `service-petitions` routes, both READS:
 *
 *   GET /api/v1/citizen-reports/{lookupCode}/directory-match           feedback.read + citizen.read
 *       200 `{status, contact_id, phone_unverified}` · 503 = identity unreachable
 *   GET /api/v1/citizen-report-lists/by-contact/{contactId}?page=&page_size=   citizen.read + feedback.read
 *       200 `{items:[{lookup_code, title, status, received_at}], total, page, page_size}`
 *
 * THE LINK IS DERIVED, NEVER STORED (ADR 0102 #1): petitions asks identity on every call; nothing here
 * caches an answer (`CHUNG` is `no-store`). Neither route carries a phone number in either direction —
 * the person is named by the directory ROW id, the petition by its lookup code (#8, rule 3 forbidden #4).
 * The match answers an id and a flag, never the number; the list rows carry no reporter at all.
 *
 * 503 OR ANY OTHER REFUSAL IS NOT "NOT FOUND": a screen that read a failed match as `not_found` would offer
 * to add somebody who is already in the directory. `readDirectoryMatch` folds every failure — and any
 * status it does not know — into `error`, and the screen says it could not check (fail closed).
 *
 * NO `tenant_id`, RELATIVE PATHS, `credentials: same-origin`: the three rules of `goi.ts`.
 */

import {
  docJSON, // vi-name-ok: existing export of goi.ts, imported not declared (rule 12 inv 3)
  thamSoTheoHopDong, // vi-name-ok: existing export of goi.ts, imported not declared (rule 12 inv 3)
} from "./goi";
import type { KetQua } from "./goi"; // vi-name-ok: existing type of goi.ts, imported not declared (rule 12 inv 3)

// ---- UNTIL REGEN: petition ↔ directory link (ADR 0102, backend TASK-04) --------------------------
//
// `schema.gen.ts` does not carry these two routes yet: the backend half is being built in parallel and
// `make kb` has not regenerated the contract in the shared tree (regenerating here would also mix in other
// sessions' uncommitted routes — the stopgap `zalo.ts` documents). The shapes below are the REST contract
// fixed by the task card that TASK-04 implements, named as the generator names petitions shapes.
//
// DELETE THIS WHOLE BLOCK in the commit that lands the regen: import the `petitions_*` names from
// `schema.gen` instead and turn each `…Route` below into the generated route type in the `satisfies`.
// A shape that drifted from the server is then a red `tsc`, which is the point of agent rule 6.

/** 200 of the match. `contact_id` is set ONLY for `found`; `phone_unverified`: the number was self-declared (ADR 0080). */
export type petitions_directoryMatchOut = {
  status: "found" | "not_found" | "not_applicable";
  contact_id: string | null;
  phone_unverified: boolean;
};
/** One petition of a person: lookup code, title, status, received time. No reporter field (ADR 0102 #8). */
export type petitions_contactPetitionOut = {
  lookup_code: string;
  title: string;
  status: string;
  received_at: string;
};
/** One page of a person's petitions. `page` is 1-based. */
export type petitions_contactPetitionPageOut = {
  items: petitions_contactPetitionOut[];
  total: number;
  page: number;
  page_size: number;
};
type DirectoryMatchRoute = { duongDan: "/api/v1/citizen-reports/{lookupCode}/directory-match" };
type ContactPetitionsRoute = {
  duongDan: "/api/v1/citizen-report-lists/by-contact/{contactId}";
  truyVan: { page?: number; page_size?: number };
};

// ---- end UNTIL REGEN ------------------------------------------------------------------------------

const MATCH_PATH = "/api/v1/citizen-reports/{lookupCode}/directory-match" satisfies DirectoryMatchRoute["duongDan"];
const BY_CONTACT_PATH = "/api/v1/citizen-report-lists/by-contact/{contactId}" satisfies ContactPetitionsRoute["duongDan"];

/** Rows per page of "Xem phản ánh" — the task card's figure (ADR 0102 request: "phân trang"). */
export const CONTACT_PETITION_PAGE_SIZE = 10;

/**
 * The match as the screen acts on it. `error` covers a 503 (identity unreachable), any refusal, an
 * unreadable body, an unknown `status`, and a `found` without an id — none of them is evidence of absence.
 */
export type DirectoryMatch =
  | { kind: "found"; contactId: string; phoneUnverified: boolean }
  | { kind: "not_found"; phoneUnverified: boolean }
  | { kind: "not_applicable" }
  | { kind: "error" };

/** The match route's path for one petition. The code is percent-encoded: a raw value could walk to another route. */
export function directoryMatchPath(lookupCode: string): string {
  return MATCH_PATH.replace("{lookupCode}", encodeURIComponent(lookupCode));
}

/** `GET …/citizen-reports/{lookupCode}/directory-match` — is this petition's sender in the directory. */
export async function readDirectoryMatch(lookupCode: string): Promise<DirectoryMatch> {
  const kq = await docJSON<petitions_directoryMatchOut>(directoryMatchPath(lookupCode));
  if (!kq.ok) return { kind: "error" };
  const m = kq.duLieu;
  switch (m?.status) {
    case "found":
      return typeof m.contact_id === "string" && m.contact_id !== ""
        ? { kind: "found", contactId: m.contact_id, phoneUnverified: m.phone_unverified === true }
        : { kind: "error" };
    case "not_found":
      return { kind: "not_found", phoneUnverified: m.phone_unverified === true };
    case "not_applicable":
      return { kind: "not_applicable" };
    default:
      return { kind: "error" };
  }
}

/**
 * The list route's path: the directory ROW id in the path, `page` (1-based) and `page_size` in the query.
 * Never a phone number — the server resolves the number from the id, in memory (ADR 0102 #2).
 */
export function contactPetitionsPath(contactId: string, page: number): string {
  const query = new URLSearchParams();
  const set = thamSoTheoHopDong<ContactPetitionsRoute["truyVan"]>(query);
  set("page", page);
  set("page_size", CONTACT_PETITION_PAGE_SIZE);
  return `${BY_CONTACT_PATH.replace("{contactId}", encodeURIComponent(contactId))}?${query.toString()}`;
}

/** `GET …/citizen-report-lists/by-contact/{contactId}` — one page of that person's petitions, or the server's sentence. */
export function listContactPetitions(contactId: string, page: number): Promise<KetQua<petitions_contactPetitionPageOut>> {
  return docJSON<petitions_contactPetitionPageOut>(contactPetitionsPath(contactId, page));
}
