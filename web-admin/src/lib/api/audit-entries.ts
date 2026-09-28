/**
 * "Nhật ký hệ thống" — the five per-service reads of ADR 0054 §3:
 * `GET /api/v1/{identity,documents,finance,comms,petitions}-audit-entries`, all `admin.audit`.
 *
 * FIVE ROUTES, NOT ONE, BY DECISION: each service reads its own `audit_log` (the trail is written in
 * the same transaction as the change, so it cannot live anywhere else — ADR 0054 §1). This file only
 * knows how to ask ONE service for ONE page; ordering the five together is `features/cau-hinh/
 * audit-log-merge.ts`, and it is the screen's job, not the database's (§6).
 *
 * Every response is `page.Result` — `items · next_cursor · has_more`, no `total`. One cursor per
 * service; there is no merged cursor, because none of the services could read it.
 *
 * Nothing here logs the body: `delta` carries masked before/after values and the IP of staff (rule 3).
 */

import { docJSON, thamSoTheoHopDong } from "./goi";
import type { KetQua } from "./goi";
import type {
  comms_get_comms_audit_entries,
  documents_get_documents_audit_entries,
  finance_get_finance_audit_entries,
  identity_get_identity_audit_entries,
  page_Result_audit_EntryView,
  petitions_get_petitions_audit_entries,
} from "./schema.gen";

/**
 * The five sources, in the order ADR 0054 §2 names the services the web proxy routes. Each path is
 * checked against its own operation in the contract: a renamed route turns `tsc` red on its line.
 */
export const AUDIT_SOURCES = [
  {
    key: "identity",
    path: "/api/v1/identity-audit-entries" satisfies identity_get_identity_audit_entries["duongDan"],
  },
  {
    key: "documents",
    path: "/api/v1/documents-audit-entries" satisfies documents_get_documents_audit_entries["duongDan"],
  },
  {
    key: "finance",
    path: "/api/v1/finance-audit-entries" satisfies finance_get_finance_audit_entries["duongDan"],
  },
  {
    key: "comms",
    path: "/api/v1/comms-audit-entries" satisfies comms_get_comms_audit_entries["duongDan"],
  },
  {
    key: "petitions",
    path: "/api/v1/petitions-audit-entries" satisfies petitions_get_petitions_audit_entries["duongDan"],
  },
] as const;

export type AuditSourceKey = (typeof AUDIT_SOURCES)[number]["key"];

/**
 * The five routes share one query shape; typed from ONE of them and checked against the other four
 * below, so a parameter one service drops turns `tsc` red here instead of being silently ignored by
 * that service — which would return its WHOLE trail while staff believe they are reading a slice.
 */
type AuditQuery = identity_get_identity_audit_entries["truyVan"];
type SameQuery<T> = [T] extends [AuditQuery] ? ([AuditQuery] extends [T] ? true : never) : never;
const QUERY_SHAPES_AGREE: [
  SameQuery<documents_get_documents_audit_entries["truyVan"]>,
  SameQuery<finance_get_finance_audit_entries["truyVan"]>,
  SameQuery<comms_get_comms_audit_entries["truyVan"]>,
  SameQuery<petitions_get_petitions_audit_entries["truyVan"]>,
] = [true, true, true, true];
void QUERY_SHAPES_AGREE;

/**
 * The filters, as the server reads them (ADR 0054 §4): exact match, all optional. `from`/`to` are
 * RFC 3339 instants of a half-open `[from, to)` the CLIENT computed (ADR 0053 §3).
 */
export type AuditFilter = {
  readonly from?: string;
  readonly to?: string;
  readonly actor?: string;
  readonly action?: string;
  readonly subject?: string;
};

/** The query string for one page of one source. Empty values are not sent (`thamSoTheoHopDong`). */
export function auditQuery(filter: AuditFilter, cursor: string): string {
  const q = new URLSearchParams();
  const set = thamSoTheoHopDong<AuditQuery>(q);
  set("from", filter.from);
  set("to", filter.to);
  set("actor", filter.actor);
  set("action", filter.action);
  set("subject", filter.subject);
  set("cursor", cursor);
  const s = q.toString();
  return s === "" ? "" : `?${s}`;
}

/**
 * One page of one service's trail. `cursor` is that service's own `next_cursor` ("" = first page).
 * No `limit`: the server's default (20, `core/page`) is what every service applies alike.
 */
export function getAuditEntries(
  source: AuditSourceKey,
  filter: AuditFilter,
  cursor: string,
): Promise<KetQua<page_Result_audit_EntryView>> {
  const s = AUDIT_SOURCES.find((x) => x.key === source);
  // Unreachable through the type; refuse rather than call some other path.
  if (s === undefined) return Promise.resolve({ ok: false, thongBao: "Phân hệ không xác định." });
  return docJSON<page_Result_audit_EntryView>(`${s.path}${auditQuery(filter, cursor)}`);
}
