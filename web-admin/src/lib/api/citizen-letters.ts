/**
 * The thirteen routes of the CITIZEN-LETTER register — `service-documents`, `/api/v1/citizen-letters…`
 * (ADR 0078 #2–#4; routes and their keys: `service-documents/internal/http/routes_citizen_letter.go`).
 *
 * WHAT THIS FILE NEVER SENDS, and why each absence is load-bearing:
 *   - `number`, `status`, `processing_due_at`, `resolution_due_at` on a booking. The contract declares
 *     them ONLY so the server can refuse them (400): the number is allocated under a row lock and never
 *     reissued (rule 7, invariant 3), the status is the flow's, and both deadlines are fixed by the
 *     server at the act that fixes them — the clerk's own "Hạn xử lý" goes through PATCH …/deadline
 *     AFTER booking (ADR 0079 lô 5 Q18; the server refuses it on a booking). `BookLetterInput`
 *     omits them so `tsc` is red before a form can carry one.
 *   - A sender's name or summary in a URL. The duplicate check is a POST with a BODY (rule 3,
 *     forbidden #4); the prototype's `?sender_name=` is deliberately not copied (ADR 0078, Hệ quả).
 *   - `tenant_id` or a staff code naming the caller. The commune is the Host's, the caller the session's
 *     (`goi.ts`). `scope=mine` asks the server to use the SESSION's code; nothing here names a person.
 *
 * Every reply is already MASKED by the server (phone always `09****0000`, address never returned,
 * denunciations without identity/summary on lists). This file passes the reply through untouched and
 * logs nothing — a reply body is personal data (rule 3).
 */

import { docJSON, docThanLoiGoi, goiGhi, thamSoTheoHopDong } from "./goi";
import type { KetQua } from "./goi";
import type {
  documents_bookLetterIn,
  documents_citizenLetterCountOut,
  documents_citizenLetterOut,
  documents_duplicateCheckIn,
  documents_duplicatesOut,
  documents_get_citizen_letter_counts,
  documents_get_citizen_letter_report,
  documents_get_citizen_letters,
  documents_get_citizen_letters_by_id,
  documents_get_citizen_letters_by_id_log,
  documents_letterLogEntryOut,
  documents_letterLogOut,
  documents_letterReportOut,
  documents_letterResultIn,
  documents_letterRoutingIn,
  documents_letterStatusIn,
  documents_patch_citizen_letters_by_id_sender,
  documents_post_citizen_letters,
  documents_post_citizen_letters_by_id_log_entries,
  documents_post_citizen_letters_by_id_routings,
  documents_post_citizen_letters_by_id_status,
  documents_post_citizen_letters_duplicates,
  documents_put_citizen_letters_by_id_result,
  documents_senderCorrectionIn,
  page_Result_documents_citizenLetterItemOut,
} from "./schema.gen";

/** `{id}` of a route template, encoded: the id goes into the PATH, never a body field. */
function onePath(template: string, id: string): string {
  return template.replace("{id}", encodeURIComponent(id));
}

/* ---- the register list ------------------------------------------------------------------- */

type ListQuery = documents_get_citizen_letters["truyVan"];
type CountQuery = documents_get_citizen_letter_counts["truyVan"];

/** `scope` — the server's three words (`letterFilterFromQuery`): anything else is a 400. */
export type LetterScope = "all" | "mine" | "related";

/** Filters of the register list. An absent field is NOT sent — the server then does not filter. */
export type LetterListFilter = {
  /** `received_from` / `received_to`, `YYYY-MM-DD` (the browser's date input already emits it). */
  receivedFrom?: string;
  receivedTo?: string;
  /** `holding_unit` — identity's `bo_phan.id`. */
  holdingUnit?: string;
  /** `assignee` — a staff BUSINESS code (`CB-…`). */
  assignee?: string;
  /** `status` — one of the ten C3 codes; an unknown code is refused, never ignored. */
  status?: string;
  /** `status_group` — one of ADR 0084's six display groups (the screen's status filter). */
  statusGroup?: string;
  /** `all` is the server's default and is therefore not sent. */
  scope?: LetterScope;
  limit?: number;
  /** Opaque cursor the server issued; passed back verbatim. Empty / null = first page. */
  cursor?: string | null;
};

/**
 * The filter half of the query, shared by the list and its count: ONE builder, so the tab's count and
 * the page it heads cannot be asked two different questions. Names checked against the CONTRACT
 * (`thamSoTheoHopDong`) of BOTH routes: a renamed server parameter turns `tsc` red here instead of the
 * server silently answering for the whole register.
 */
function setFilters(set: (name: keyof ListQuery & keyof CountQuery, value: string | undefined) => void, filter: LetterListFilter) {
  set("received_from", filter.receivedFrom);
  set("received_to", filter.receivedTo);
  set("holding_unit", filter.holdingUnit);
  set("assignee", filter.assignee);
  set("status", filter.status);
  set("status_group", filter.statusGroup);
  set("scope", filter.scope === "all" ? undefined : filter.scope);
}

/** The list path. Split from the network call so a test can read it without a fake `fetch`. */
export function citizenLetterListPath(filter: LetterListFilter = {}): string {
  const path: documents_get_citizen_letters["duongDan"] = "/api/v1/citizen-letters";
  const query = new URLSearchParams();
  const set = thamSoTheoHopDong<ListQuery>(query);
  setFilters(set, filter);
  set("limit", filter.limit);
  set("cursor", filter.cursor);
  const text = query.toString();
  return text === "" ? path : `${path}?${text}`;
}

/** GET /api/v1/citizen-letters — one page, newest booking first (the server's order). */
export function listCitizenLetters(
  filter: LetterListFilter = {},
): Promise<KetQua<page_Result_documents_citizenLetterItemOut>> {
  return docJSON<page_Result_documents_citizenLetterItemOut>(citizenLetterListPath(filter));
}

/** The count path: the list's filters, never its paging (`limit`, `cursor` are not the count's). */
export function citizenLetterCountPath(filter: LetterListFilter = {}): string {
  const path: documents_get_citizen_letter_counts["duongDan"] = "/api/v1/citizen-letter-counts";
  const query = new URLSearchParams();
  setFilters(thamSoTheoHopDong<CountQuery>(query), filter);
  const text = query.toString();
  return text === "" ? path : `${path}?${text}`;
}

/**
 * GET /api/v1/citizen-letter-counts — how many letters the list would hold under the same filters and
 * scope (the "Đơn thư công dân (N)" tab, ADR 0084 #7). The server counts; the loaded page never is.
 */
export function countCitizenLetters(filter: LetterListFilter = {}): Promise<KetQua<documents_citizenLetterCountOut>> {
  return docJSON<documents_citizenLetterCountOut>(citizenLetterCountPath(filter));
}

/* ---- booking and the duplicate warning ---------------------------------------------------- */

/** The booking body with the four server-owned fields REMOVED (see the head of this file). */
export type BookLetterInput = Omit<
  documents_bookLetterIn,
  "number" | "status" | "processing_due_at" | "resolution_due_at"
>;

/**
 * POST /api/v1/citizen-letters — book a letter; the server issues the number. 201.
 *
 * `idempotencyKey` IS A PARAMETER, made when the form OPENS: a key made per click turns a retry after a
 * network error into a SECOND booking, and the first one may already have taken a number that is never
 * given back (rule 7, invariant 3). The route requires the header (`idem.Required`).
 */
export function bookCitizenLetter(
  input: BookLetterInput,
  idempotencyKey: string,
): Promise<KetQua<documents_citizenLetterOut>> {
  const path: documents_post_citizen_letters["duongDan"] = "/api/v1/citizen-letters";
  // Built field by field, never spread: a spread is how a stray `number` or `status` from a row just
  // read would reach the server. Optional fields that are empty are left out.
  const body: BookLetterInput = {
    received_date: input.received_date,
    letter_type: input.letter_type,
    summary: input.summary,
  };
  if (input.sender_name) body.sender_name = input.sender_name;
  if (input.sender_phone) body.sender_phone = input.sender_phone;
  if (input.sender_address) body.sender_address = input.sender_address;
  if (input.related_letter_id) body.related_letter_id = input.related_letter_id;
  if (input.holding_unit_id) body.holding_unit_id = input.holding_unit_id;
  return docThanLoiGoi<documents_citizenLetterOut>(
    goiGhi(path, "POST", body, 201, { "Idempotency-Key": idempotencyKey }),
  );
}

/**
 * POST /api/v1/citizen-letters/duplicates — the C11 warning. A POST BECAUSE THE NAME TRAVELS IN THE
 * BODY: a name in a query string lands in every access log and browser history (rule 3, forbidden #4).
 * It writes nothing; the server only ranks candidates — linking one is the clerk's confirmed choice.
 */
export function checkLetterDuplicates(
  input: documents_duplicateCheckIn,
): Promise<KetQua<documents_duplicatesOut>> {
  const path: documents_post_citizen_letters_duplicates["duongDan"] = "/api/v1/citizen-letters/duplicates";
  const body: documents_duplicateCheckIn = { summary: input.summary };
  if (input.sender_name) body.sender_name = input.sender_name;
  if (input.letter_type) body.letter_type = input.letter_type;
  return docThanLoiGoi<documents_duplicatesOut>(goiGhi(path, "POST", body, 200));
}

/* ---- one letter ------------------------------------------------------------------------- */

/**
 * GET /api/v1/citizen-letters/{id} — the drawer. 404 is ONE sentence for never-existed, removed and
 * another commune's; the screen shows it as is. Reading a denunciation's identity writes an audit
 * entry on the server — a second read is a second recorded read.
 */
export function getCitizenLetter(id: string): Promise<KetQua<documents_citizenLetterOut>> {
  const template: documents_get_citizen_letters_by_id["duongDan"] = "/api/v1/citizen-letters/{id}";
  return docJSON<documents_citizenLetterOut>(onePath(template, id));
}

/** GET /api/v1/citizen-letters/{id}/log — the processing log, NEWEST FIRST as sent. Staff-internal. */
export function getCitizenLetterLog(id: string): Promise<KetQua<documents_letterLogOut>> {
  const template: documents_get_citizen_letters_by_id_log["duongDan"] = "/api/v1/citizen-letters/{id}/log";
  return docJSON<documents_letterLogOut>(onePath(template, id));
}

/**
 * POST /api/v1/citizen-letters/{id}/routings — assign to a unit (and an officer). Assignment is an
 * ATTRIBUTE (C3): no status moves. No Idempotency-Key, on purpose: each routing is a real act with its
 * own log line, and a retry IS a second routing (the server says so on the route).
 */
export function routeCitizenLetter(
  id: string,
  input: documents_letterRoutingIn,
): Promise<KetQua<documents_citizenLetterOut>> {
  const template: documents_post_citizen_letters_by_id_routings["duongDan"] =
    "/api/v1/citizen-letters/{id}/routings";
  const body: documents_letterRoutingIn = { to_unit: input.to_unit, reason: input.reason };
  if (input.assignee) body.assignee = input.assignee;
  return docThanLoiGoi<documents_citizenLetterOut>(goiGhi(onePath(template, id), "POST", body, 200));
}

/**
 * POST /api/v1/citizen-letters/{id}/status — one C3 arrow. A refused arrow and "Đã giải quyết without a
 * result" come back as 409 with the server's own sentence, shown verbatim.
 */
export function moveCitizenLetter(
  id: string,
  input: documents_letterStatusIn,
): Promise<KetQua<documents_citizenLetterOut>> {
  const template: documents_post_citizen_letters_by_id_status["duongDan"] = "/api/v1/citizen-letters/{id}/status";
  const body: documents_letterStatusIn = { status: input.status };
  if (input.note) body.note = input.note;
  return docThanLoiGoi<documents_citizenLetterOut>(goiGhi(onePath(template, id), "POST", body, 200));
}

/**
 * PUT /api/v1/citizen-letters/{id}/result — the reply, and for a complaint or a denunciation the issued
 * document (ADR 0084 #2). A document key the caller did not set is NOT sent: a summary-only reply is
 * exactly `{ result_summary }`. Which keys are required is the server's rule, by the letter's type.
 */
export function recordCitizenLetterResult(
  id: string,
  input: documents_letterResultIn,
): Promise<KetQua<documents_citizenLetterOut>> {
  const template: documents_put_citizen_letters_by_id_result["duongDan"] = "/api/v1/citizen-letters/{id}/result";
  const body: documents_letterResultIn = { result_summary: input.result_summary };
  if (input.result_document_no !== undefined) body.result_document_no = input.result_document_no;
  if (input.result_document_date !== undefined) body.result_document_date = input.result_document_date;
  if (input.result_signer !== undefined) body.result_signer = input.result_signer;
  if (input.result_issuer !== undefined) body.result_issuer = input.result_issuer;
  return docThanLoiGoi<documents_citizenLetterOut>(goiGhi(onePath(template, id), "PUT", body, 200));
}

/**
 * PATCH /api/v1/citizen-letters/{id}/sender — correction or Decree 13 anonymisation. KEY PRESENCE IS
 * THE MEANING: an absent key is left alone, `null` clears. The caller builds exactly the keys it means.
 */
export function correctCitizenLetterSender(
  id: string,
  input: documents_senderCorrectionIn,
): Promise<KetQua<documents_citizenLetterOut>> {
  const template: documents_patch_citizen_letters_by_id_sender["duongDan"] = "/api/v1/citizen-letters/{id}/sender";
  const body: documents_senderCorrectionIn = {};
  if ("sender_name" in input) body.sender_name = input.sender_name;
  if ("sender_phone" in input) body.sender_phone = input.sender_phone;
  if ("sender_address" in input) body.sender_address = input.sender_address;
  return docThanLoiGoi<documents_citizenLetterOut>(goiGhi(onePath(template, id), "PATCH", body, 200));
}

/**
 * Body of PATCH …/deadline: EXACTLY one key. An RFC 3339 instant sets the clerk's "Hạn xử lý"; `null`
 * clears it ("Không đặt"). The server refuses any other key, and any body without `due_at` (400).
 *
 * HAND-TYPED until contract regen: the committed `schema.gen.ts` predates this route
 * (`service-documents/internal/http/citizen_letter.go` `readLetterDeadline`). Replace with the
 * generated `documents_patch_citizen_letters_by_id_deadline` once the contract is regenerated.
 */
export type LetterDeadlineInput = { due_at: string | null };

/**
 * PATCH /api/v1/citizen-letters/{id}/deadline — the clerk's "Hạn xử lý" (ADR 0079 lô 5 Q18). The SERVER
 * decides which stored deadline it is (processing before Thụ lý, resolution from it) and stores the
 * instant as sent — nothing here computes a deadline (rule 10). 409 = the letter is finished; 400 = a
 * malformed instant or a year outside 2000–2200; both carry the server's sentence, shown verbatim.
 */
export function setCitizenLetterDeadline(
  id: string,
  input: LetterDeadlineInput,
): Promise<KetQua<documents_citizenLetterOut>> {
  // Literal path, until contract regen (see `LetterDeadlineInput`).
  const template = "/api/v1/citizen-letters/{id}/deadline";
  const body: LetterDeadlineInput = { due_at: input.due_at };
  return docThanLoiGoi<documents_citizenLetterOut>(goiGhi(onePath(template, id), "PATCH", body, 200));
}

/**
 * POST /api/v1/citizen-letters/{id}/log-entries — one note. The route requires an Idempotency-Key: a
 * double-submitted note is a duplicate line in an append-only log nobody can remove. The key belongs to
 * the DRAFT (made once per note), so a retry after a network error replays instead of adding a line.
 */
export function addCitizenLetterNote(
  id: string,
  content: string,
  idempotencyKey: string,
): Promise<KetQua<documents_letterLogEntryOut>> {
  const template: documents_post_citizen_letters_by_id_log_entries["duongDan"] =
    "/api/v1/citizen-letters/{id}/log-entries";
  return docThanLoiGoi<documents_letterLogEntryOut>(
    goiGhi(onePath(template, id), "POST", { content }, 201, { "Idempotency-Key": idempotencyKey }),
  );
}

/* ---- the report ------------------------------------------------------------------------- */

/**
 * GET /api/v1/citizen-letter-report?year= — `year` is REQUIRED by the server: a report silently
 * defaulting to "this year" is a figure filed under the wrong year without anybody noticing.
 */
export function citizenLetterReportPath(year: number): string {
  const path: documents_get_citizen_letter_report["duongDan"] = "/api/v1/citizen-letter-report";
  const query = new URLSearchParams();
  thamSoTheoHopDong<documents_get_citizen_letter_report["truyVan"]>(query)("year", String(year));
  return `${path}?${query.toString()}`;
}

export function getCitizenLetterReport(year: number): Promise<KetQua<documents_letterReportOut>> {
  return docJSON<documents_letterReportOut>(citizenLetterReportPath(year));
}
