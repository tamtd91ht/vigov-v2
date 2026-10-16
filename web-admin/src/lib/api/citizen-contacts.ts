/**
 * Danh bạ người dân — the CONTACT half of `service-identity`'s sixteen directory routes
 * (`internal/http/citizen_directory.go`, ADR 0097, `docs/ui-ux/16-danh-ba-nguoi-dan.md`). Groups are
 * `citizen-groups.ts`; the Excel import is `citizen-contact-import.ts`.
 *
 *   GET   /api/v1/citizen-contacts                citizen.read    one page, NO filter — numbers masked
 *   POST  /api/v1/citizen-contact-searches        citizen.read    one page under a filter; keyword IN THE BODY (D11)
 *   POST  /api/v1/citizen-contact-count-queries   citizen.read    `{count}` of the same filter — the tab's "(n)" (D8)
 *   GET   /api/v1/citizen-contact-tags            citizen.read    the tag strip, `{tag, count}` each
 *   POST  /api/v1/citizen-contacts                citizen.update  201 `{id}` · 409 `phone_taken` · Idempotency-Key
 *   GET   /api/v1/citizen-contacts/{id}           citizen.read    the edit dialog's detail — address and note, number STILL masked
 *   PATCH /api/v1/citizen-contacts/{id}           citizen.update  204 · 409 `phone_taken`
 *   POST  /api/v1/citizen-contacts/{id}/deletion  citizen.update  204 · `{reason}` MANDATORY (D7)
 *   GET   /api/v1/citizen-contacts/{id}/phone     citizen.update  the ONE route that answers a full number — one audit entry per call (D2)
 *
 * THE NUMBER IS MASKED ON EVERY ROUTE BUT ONE. `phone_masked` is the only phone field of the list and
 * of the detail; `revealCitizenContactPhone` is the single exit, and it runs on a click, never on
 * render (rule 6, invariant 7: every call is an audited read). Nothing here logs a body, a filter or a
 * reply — the keyword is usually a name or part of a number (rule 3).
 *
 * KIỂU LẤY TỪ HỢP ĐỒNG, KHÔNG GÕ TAY (rule 9): every shape below is `schema.gen.ts`'s. The paths are
 * `satisfies <route>["duongDan"]`, so a renamed route is a `tsc` error here, not a 404 in a commune.
 *
 * NO `tenant_id`, RELATIVE PATHS, `credentials: same-origin`: the three rules of `goi.ts`.
 */

import {
  chuanHoaTuKhoaTim, // vi-name-ok: existing export of can-bo.ts, imported not declared (rule 12 inv 3)
  type TuKhoaTim, // vi-name-ok: existing type of can-bo.ts, imported not declared (rule 12 inv 3)
} from "./can-bo";
import {
  CHUNG, // vi-name-ok: existing export of goi.ts, imported not declared (rule 12 inv 3)
  docJSON, // vi-name-ok: existing export of goi.ts, imported not declared (rule 12 inv 3)
  docThanLoiGoi, // vi-name-ok: existing export of goi.ts, imported not declared (rule 12 inv 3)
  goiGhi, // vi-name-ok: existing export of goi.ts, imported not declared (rule 12 inv 3)
  LOI_KHONG_RO, // vi-name-ok: existing export of goi.ts, imported not declared (rule 12 inv 3)
  thamSoTheoHopDong, // vi-name-ok: existing export of goi.ts, imported not declared (rule 12 inv 3)
  thongBaoLoi, // vi-name-ok: existing export of goi.ts, imported not declared (rule 12 inv 3)
} from "./goi";
import type { KetQua } from "./goi"; // vi-name-ok: existing type of goi.ts, imported not declared (rule 12 inv 3)
import type {
  identity_citizenContactCountOut,
  identity_citizenContactCountQueryIn,
  identity_citizenContactCreatedOut,
  identity_citizenContactDeletionIn,
  identity_citizenContactDetailOut,
  identity_citizenContactIn,
  identity_citizenContactPhoneOut,
  identity_citizenContactSearchIn,
  identity_citizenContactTagsOut,
  identity_get_citizen_contact_tags,
  identity_get_citizen_contacts,
  identity_get_citizen_contacts_by_id,
  identity_get_citizen_contacts_by_id_phone,
  identity_patch_citizen_contacts_by_id,
  identity_post_citizen_contact_count_queries,
  identity_post_citizen_contact_searches,
  identity_post_citizen_contacts,
  identity_post_citizen_contacts_by_id_deletion,
  page_Result_identity_citizenContactOut,
} from "./schema.gen";

export type {
  identity_citizenContactDetailOut as CitizenContactDetail,
  identity_citizenContactIn as CitizenContactBody,
  identity_citizenContactOut as CitizenContact,
  identity_citizenContactTagOut as CitizenContactTag,
  page_Result_identity_citizenContactOut as CitizenContactPage,
} from "./schema.gen";

/** Rows per page — the screen's choice (chapter 16 §3, prototype `PAGE_SIZE`). Sent on every page read. */
export const CITIZEN_CONTACT_PAGE_SIZE = 50;

/** The server's `code` on a second live row with the same number (D5) — on create and on edit alike. */
export const PHONE_TAKEN_CODE = "phone_taken" satisfies identity_post_citizen_contacts["errorCodes"][409];

/* ---- the filter ------------------------------------------------------------------------------ */

/**
 * The filter in force on the list — chapter 16 §3. `keyword` is ALREADY NORMALISED (`normalizeCitizenKeyword`),
 * `""` meaning "no keyword". `hasOwnAppSession: true` is "Chỉ người có Zalo"; `null` is everyone.
 * `activeOnly` stays `false` on the screen: a switched-off row still belongs to the directory (§10 #8).
 */
export type CitizenContactFilter = {
  readonly keyword: string;
  readonly groupId: string;
  readonly residentialUnitId: string;
  readonly activeOnly: boolean;
  readonly hasOwnAppSession: boolean | null;
};

export const NO_CITIZEN_CONTACT_FILTER: CitizenContactFilter = {
  keyword: "",
  groupId: "",
  residentialUnitId: "",
  activeOnly: false,
  hasOwnAppSession: null,
};

/** Whether anything narrows the list — decides which route reads the page and which empty sentence is said. */
export function isCitizenContactFilterEmpty(f: CitizenContactFilter): boolean {
  return f.keyword === "" && f.groupId === "" && f.residentialUnitId === "" && !f.activeOnly && f.hasOwnAppSession === null;
}

/**
 * The keyword as the server will read it — the staff register's normaliser, because the server applies
 * THE SAME ONE (`citizenContactFilterFrom` calls `domain.ChuanHoaTuKhoaTimCanBo`): whitespace collapsed,
 * at most 200 characters, counted as the server counts them. `rong` is "no keyword", not an error.
 */
export function normalizeCitizenKeyword(typed: string): TuKhoaTim {
  return chuanHoaTuKhoaTim(typed);
}

/** The search body — every field, as the generated type requires; `limit` is the screen's page size. */
export function citizenContactSearchBody(f: CitizenContactFilter, cursor: string | null): identity_citizenContactSearchIn {
  return {
    q: f.keyword,
    group_id: f.groupId,
    residential_unit_id: f.residentialUnitId,
    active: f.activeOnly ? true : null,
    has_own_app_session: f.hasOwnAppSession,
    limit: CITIZEN_CONTACT_PAGE_SIZE,
    cursor: cursor ?? "",
  };
}

/** The count body — the search's filter without its paging, so both answers describe one set of rows. */
export function citizenContactCountBody(f: CitizenContactFilter): identity_citizenContactCountQueryIn {
  return {
    q: f.keyword,
    group_id: f.groupId,
    residential_unit_id: f.residentialUnitId,
    active: f.activeOnly ? true : null,
    has_own_app_session: f.hasOwnAppSession,
  };
}

/* ---- reads ----------------------------------------------------------------------------------- */

const LIST_PATH = "/api/v1/citizen-contacts" satisfies identity_get_citizen_contacts["duongDan"];
const SEARCH_PATH = "/api/v1/citizen-contact-searches" satisfies identity_post_citizen_contact_searches["duongDan"];
const COUNT_PATH = "/api/v1/citizen-contact-count-queries" satisfies identity_post_citizen_contact_count_queries["duongDan"];
const TAGS_PATH = "/api/v1/citizen-contact-tags" satisfies identity_get_citizen_contact_tags["duongDan"];
const DETAIL_PATH = "/api/v1/citizen-contacts/{id}" satisfies identity_get_citizen_contacts_by_id["duongDan"];
const UPDATE_PATH = "/api/v1/citizen-contacts/{id}" satisfies identity_patch_citizen_contacts_by_id["duongDan"];
const DELETION_PATH = "/api/v1/citizen-contacts/{id}/deletion" satisfies identity_post_citizen_contacts_by_id_deletion["duongDan"];
const PHONE_PATH = "/api/v1/citizen-contacts/{id}/phone" satisfies identity_get_citizen_contacts_by_id_phone["duongDan"];

/**
 * The unfiltered list's path: `limit` and, from the second page on, `cursor`. Names go through
 * `thamSoTheoHopDong<identity_get_citizen_contacts["truyVan"]>`, so a renamed parameter is a `tsc` error.
 * The first page sends no `cursor=`: an empty one is a 400 at the first opening of the screen.
 */
export function citizenContactListPath(cursor: string | null): string {
  const query = new URLSearchParams();
  const set = thamSoTheoHopDong<identity_get_citizen_contacts["truyVan"]>(query);
  set("limit", CITIZEN_CONTACT_PAGE_SIZE);
  set("cursor", cursor);
  return `${LIST_PATH}?${query.toString()}`;
}

/**
 * One page of the directory under the filter in force. EMPTY FILTER → `GET /api/v1/citizen-contacts`;
 * ANYTHING SET → `POST /api/v1/citizen-contact-searches` with the whole filter and the cursor in the
 * BODY. One place decides the branch, so no screen can put a keyword on a URL by mistake (rule 3,
 * forbidden #4). No `Idempotency-Key`: a read sent twice is two reads.
 */
export function readCitizenContactPage(
  f: CitizenContactFilter,
  cursor: string | null,
): Promise<KetQua<page_Result_identity_citizenContactOut>> {
  if (isCitizenContactFilterEmpty(f)) return docJSON<page_Result_identity_citizenContactOut>(citizenContactListPath(cursor));
  return docThanLoiGoi<page_Result_identity_citizenContactOut>(goiGhi(SEARCH_PATH, "POST", citizenContactSearchBody(f, cursor), 200));
}

/** `POST /api/v1/citizen-contact-count-queries` — how many rows the filter matches: the "Danh sách (n)" tab. */
export async function countCitizenContacts(f: CitizenContactFilter): Promise<KetQua<number>> {
  const kq = await docThanLoiGoi<identity_citizenContactCountOut>(goiGhi(COUNT_PATH, "POST", citizenContactCountBody(f), 200));
  return kq.ok ? { ok: true, duLieu: kq.duLieu.count } : kq;
}

/** `GET /api/v1/citizen-contact-tags` — every tag in use and how many people carry it. */
export function listCitizenContactTags(): Promise<KetQua<identity_citizenContactTagsOut>> {
  return docJSON<identity_citizenContactTagsOut>(TAGS_PATH);
}

/** The id percent-encoded: a ULID has nothing to escape, but a value spliced raw into a path is a value that can walk to another route. */
function withId(template: string, id: string): string {
  return template.replace("{id}", encodeURIComponent(id));
}

/** `GET /api/v1/citizen-contacts/{id}` — the edit dialog's detail (address, note, groups); the number still masked. */
export function getCitizenContact(id: string): Promise<KetQua<identity_citizenContactDetailOut>> {
  return docJSON<identity_citizenContactDetailOut>(withId(DETAIL_PATH, id));
}

/* ---- writes ---------------------------------------------------------------------------------- */

/**
 * `POST /api/v1/citizen-contacts` — add one person. 201 with the id.
 *
 * `idempotencyKey` IS A PARAMETER: minted when the dialog OPENS and kept across retries of that one
 * attempt (the route declares `idem.Required` — a retry after a lost answer replays the id, never a
 * second row with the same number). Same rule as `createResidentialUnit`.
 *
 * `withCode`: the dialog says "Số điện thoại này đã có trong danh bạ." for `phone_taken` and the
 * server's sentence for everything else — one documented branch (`KetQua.code`).
 */
export function createCitizenContact(
  body: identity_citizenContactIn,
  idempotencyKey: string,
): Promise<KetQua<identity_citizenContactCreatedOut>> {
  return docThanLoiGoi<identity_citizenContactCreatedOut>(
    goiGhi(LIST_PATH, "POST", body, 201, { "Idempotency-Key": idempotencyKey }, { withCode: true }),
  );
}

/**
 * `PATCH /api/v1/citizen-contacts/{id}` — the whole form, groups and `is_active` included. 204, nothing
 * echoed; a form identical to the row writes nothing. No key: resending one edit lands on one state.
 *
 * ⚠ `phone` MUST BE THE FULL NUMBER: the server normalises it as typed (`domain.NormalizeCitizenPhone`)
 * and a mask (`09****0001`) is refused with 400. The dialog therefore saves only after "Xem số đầy đủ".
 */
export async function updateCitizenContact(id: string, body: identity_citizenContactIn): Promise<KetQua<null>> {
  const kq = await goiGhi(withId(UPDATE_PATH, id), "PATCH", body, 204, undefined, { withCode: true });
  return kq.ok ? { ok: true, duLieu: null } : kq;
}

/**
 * `POST /api/v1/citizen-contacts/{id}/deletion` — soft delete with a MANDATORY reason (ADR 0097 D7).
 * The reason is the screen's responsibility to require; an empty one is a 400 from the server too.
 */
export async function deleteCitizenContact(id: string, reason: string): Promise<KetQua<null>> {
  const body: identity_citizenContactDeletionIn = { reason };
  const kq = await goiGhi(withId(DELETION_PATH, id), "POST", body, 204);
  return kq.ok ? { ok: true, duLieu: null } : kq;
}

/* ---- the one full number ---------------------------------------------------------------------- */

/** The reveal route's path — exported so the test can pin it without calling the network. */
export function citizenContactPhonePath(id: string): string {
  return withId(PHONE_PATH, id);
}

/**
 * The reveal's answer. `forbidden` is set on a 403 ONLY: the button then goes and the masked number
 * stays — the account lacks `citizen.update`. Every other refusal carries the server's sentence.
 */
export type CitizenPhoneResult =
  | { ok: true; duLieu: identity_citizenContactPhoneOut }
  | { ok: false; thongBao: string; forbidden?: boolean };

/**
 * `GET /api/v1/citizen-contacts/{id}/phone` — ONE person's full number (ADR 0097 D2, `citizen.update`).
 * The server writes an audit entry for EVERY call (rule 6, invariant 7), so this runs only on an explicit
 * click — never on render, never prefetched. The number lives in the calling dialog's state and nowhere
 * else: not logged, not cached (`CHUNG` is `no-store`), never put in a URL. Same shape as `revealStaffEmail`.
 */
export async function revealCitizenContactPhone(id: string): Promise<CitizenPhoneResult> {
  let res: Response;
  try {
    res = await fetch(citizenContactPhonePath(id), { ...CHUNG, method: "GET" });
  } catch {
    // No logging: the answer of this route is personal data (rule 3).
    return { ok: false, thongBao: LOI_KHONG_RO };
  }
  if (res.status !== 200) {
    const message = await thongBaoLoi(res);
    return res.status === 403 ? { ok: false, thongBao: message, forbidden: true } : { ok: false, thongBao: message };
  }
  try {
    return { ok: true, duLieu: (await res.json()) as identity_citizenContactPhoneOut };
  } catch {
    return { ok: false, thongBao: LOI_KHONG_RO };
  }
}
