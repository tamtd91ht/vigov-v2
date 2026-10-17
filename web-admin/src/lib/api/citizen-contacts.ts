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
 * ADR 0101 ADDS `email` (masked on the way out, like the number) and the commune's dynamic `attributes` to the
 * contact, and a creation-date range and attribute filters to the search — see the UNTIL REGEN block.
 * A search using those can be answered 503 when the server has no search engine configured.
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
  docThanKetQua, // vi-name-ok: existing export of goi.ts, imported not declared (rule 12 inv 3)
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
  identity_citizenContactOut,
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

export type { identity_citizenContactTagOut as CitizenContactTag } from "./schema.gen";

// ---- UNTIL REGEN (TASK E-1) ------------------------------------------------------------------------
// ADR 0101 adds to the contact a static `email` (#6) and the commune's dynamic `attributes` (#1), and to
// the search a creation-date range and per-attribute filters (#10). `schema.gen.ts` lags the backend that
// is built in parallel, so the additions are written here BY HAND, ON TOP OF the generated shapes, until
// `make kb` regenerates them — then each alias below collapses back to its `identity_*` type.
//
// The reply fields are OPTIONAL here on purpose: a server still on the old shape answers without them,
// and the screen must keep working against it. After regen they are whatever the contract says.

/** One attribute value: text / one choice -> string, several choices -> string[], number and dateTime (epoch millis) -> number. */
export type CitizenAttributeValue = string | string[] | number;
export type CitizenAttributes = Record<string, CitizenAttributeValue>;

/**
 * The create / update body. `email` is OPTIONAL in the body: on an edit it is sent only when the officer
 * typed a new one, because the detail answers it MASKED and sending the mask back would overwrite the
 * real address with asterisks. Absent = unchanged (assumption stated to the backend, TASK E-1).
 */
export type CitizenContactBody = identity_citizenContactIn & { email?: string; attributes: CitizenAttributes };
/**
 * The masked address on a reply. TASK E-1 names it `email`; this repo's other masked address is
 * `email_masked` (`danh_ba_chon_nguoi.go`), next to `phone_masked`. Both are read until regen settles the
 * name — through `contactEmailMasked`, the one reader, so the regen is a one-line change.
 */
type MaskedEmailReply = { email?: string | null; email_masked?: string | null };
/** A list row. The address arrives masked, like `phone_masked`. */
export type CitizenContact = identity_citizenContactOut & MaskedEmailReply & { attributes?: CitizenAttributes };
/** The edit dialog's detail — the address masked here too. */
export type CitizenContactDetail = identity_citizenContactDetailOut & MaskedEmailReply & { attributes?: CitizenAttributes };

/** The masked address of a row or a detail, `""` when there is none. Never a raw address: the server masks it. */
export function contactEmailMasked(r: MaskedEmailReply): string {
  return r.email_masked ?? r.email ?? "";
}
export type CitizenContactPage = Omit<page_Result_identity_citizenContactOut, "items"> & { items: CitizenContact[] };

/** One attribute filter: a set of option values (combobox / singleBox), or a range (number / dateTime) with at least one bound. */
export type CitizenAttributeFilter =
  | { readonly code: string; readonly values: readonly string[] }
  | { readonly code: string; readonly from?: number; readonly to?: number };

/** The search additions. Sent only when set, so a filter that uses none of them is today's request byte for byte. */
type AdvancedSearchFields = {
  created_from?: number;
  created_to?: number;
  filters?: CitizenAttributeFilter[];
};
// ---- end UNTIL REGEN -------------------------------------------------------------------------------

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
  /** "Ngày tạo từ" — epoch millis, inclusive; `null` = no lower bound (ADR 0101 #10). */
  readonly createdFrom: number | null;
  /** "Ngày tạo đến" — epoch millis, the LAST millisecond of the chosen day; `null` = no upper bound. */
  readonly createdTo: number | null;
  /** Per-attribute filters, each already complete (`citizen-attributes.ts` builds them). */
  readonly attributeFilters: readonly CitizenAttributeFilter[];
};

export const NO_CITIZEN_CONTACT_FILTER: CitizenContactFilter = {
  keyword: "",
  groupId: "",
  residentialUnitId: "",
  activeOnly: false,
  hasOwnAppSession: null,
  createdFrom: null,
  createdTo: null,
  attributeFilters: [],
};

/**
 * Whether the filter uses the search engine's own abilities — a creation-date range or an attribute filter.
 * Only such a search can be answered 503 "search engine not configured" (TASK E-1), and only then is that
 * 503 read as "advanced search is off" rather than as the service being down.
 */
export function hasAdvancedCitizenFilter(f: CitizenContactFilter): boolean {
  return f.createdFrom !== null || f.createdTo !== null || f.attributeFilters.length > 0;
}

/** Whether anything narrows the list — decides which route reads the page and which empty sentence is said. */
export function isCitizenContactFilterEmpty(f: CitizenContactFilter): boolean {
  return (
    f.keyword === "" &&
    f.groupId === "" &&
    f.residentialUnitId === "" &&
    !f.activeOnly &&
    f.hasOwnAppSession === null &&
    !hasAdvancedCitizenFilter(f)
  );
}

/**
 * The keyword as the server will read it — the staff register's normaliser, because the server applies
 * THE SAME ONE (`citizenContactFilterFrom` calls `domain.ChuanHoaTuKhoaTimCanBo`): whitespace collapsed,
 * at most 200 characters, counted as the server counts them. `rong` is "no keyword", not an error.
 */
export function normalizeCitizenKeyword(typed: string): TuKhoaTim {
  return chuanHoaTuKhoaTim(typed);
}

/** The ADR 0101 additions of a filter, each key present only when set. */
function advancedSearchFields(f: CitizenContactFilter): AdvancedSearchFields {
  const extra: AdvancedSearchFields = {};
  if (f.createdFrom !== null) extra.created_from = f.createdFrom;
  if (f.createdTo !== null) extra.created_to = f.createdTo;
  if (f.attributeFilters.length > 0) extra.filters = f.attributeFilters.map((a) => ({ ...a }));
  return extra;
}

/**
 * The search body — every generated field, as the generated type requires; `limit` is the screen's page
 * size; the ADR 0101 additions only when set. The keyword stays IN THE BODY (ADR 0097 D11, kept by 0101 #7).
 */
export function citizenContactSearchBody(
  f: CitizenContactFilter,
  cursor: string | null,
): identity_citizenContactSearchIn & AdvancedSearchFields {
  return {
    q: f.keyword,
    group_id: f.groupId,
    residential_unit_id: f.residentialUnitId,
    active: f.activeOnly ? true : null,
    has_own_app_session: f.hasOwnAppSession,
    limit: CITIZEN_CONTACT_PAGE_SIZE,
    cursor: cursor ?? "",
    ...advancedSearchFields(f),
  };
}

/** The count body — the search's filter without its paging, so both answers describe one set of rows. */
export function citizenContactCountBody(f: CitizenContactFilter): identity_citizenContactCountQueryIn & AdvancedSearchFields {
  return {
    q: f.keyword,
    group_id: f.groupId,
    residential_unit_id: f.residentialUnitId,
    active: f.activeOnly ? true : null,
    has_own_app_session: f.hasOwnAppSession,
    ...advancedSearchFields(f),
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
export async function readCitizenContactPage(f: CitizenContactFilter, cursor: string | null): Promise<CitizenContactPageResult> {
  if (isCitizenContactFilterEmpty(f)) return docJSON<CitizenContactPage>(citizenContactListPath(cursor));
  const advanced = hasAdvancedCitizenFilter(f);
  // 503 is accepted as an answer ONLY for an advanced search: that is the one 503 the contract gives a
  // meaning to. A keyword search answered 503 is the service (or the proxy before it) being down, and keeps
  // the ordinary refusal sentence.
  const kq = await goiGhi(SEARCH_PATH, "POST", citizenContactSearchBody(f, cursor), advanced ? [200, 503] : 200);
  if (kq.ok && kq.duLieu.status === 503) return { ok: false, thongBao: ADVANCED_SEARCH_OFF, searchUnavailable: true };
  return docThanKetQua<CitizenContactPage>(kq);
}

/**
 * Said when the server has no search engine configured and the filter needs one (TASK E-1). The list stays
 * usable: the screen offers to drop the advanced part of the filter and read again.
 */
export const ADVANCED_SEARCH_OFF = "Chưa bật tìm kiếm nâng cao trên máy chủ này";

/** A page read: the page, or a sentence — and `searchUnavailable` when that sentence is `ADVANCED_SEARCH_OFF`. */
export type CitizenContactPageResult =
  | { ok: true; duLieu: CitizenContactPage }
  | { ok: false; thongBao: string; searchUnavailable?: true };

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
export function getCitizenContact(id: string): Promise<KetQua<CitizenContactDetail>> {
  return docJSON<CitizenContactDetail>(withId(DETAIL_PATH, id));
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
  body: CitizenContactBody,
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
export async function updateCitizenContact(id: string, body: CitizenContactBody): Promise<KetQua<null>> {
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
