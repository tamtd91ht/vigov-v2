/**
 * The six figure routes of the leadership overview (`/tong-quan`, `docs/ui-ux/01-tong-quan-dieu-hanh.md`).
 *
 * THE WEB COMPOSES, NOTHING IS SNAPSHOTTED. Each figure is read live from the service that owns
 * the register it counts — `petitions` for tasks and citizen reports, `documents` for the incoming
 * register. There is no `snapshot_tong_quan` and no "Tính lại ngay": spec §6-§7 proposed both, and
 * the user decided against them (figures live, the page states the instant they were read).
 *
 * TYPES COME FROM `schema.gen.ts`, never re-typed here (rule 2 invariant 7, agent rule 6).
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * WHAT THE CONTRACT DOES NOT SAY, MEASURED FROM THE HANDLERS:
 *
 *  1. `from`/`to` are generated OPTIONAL on the three summaries, and all three handlers answer
 *     **400** without them (`service-petitions/internal/http/summary.go` `parsePeriod`,
 *     `service-documents/internal/http/incoming_dashboard.go`). `SummaryPeriod` below therefore
 *     makes both required: a call without a period does not compile.
 *  2. Every route is guarded by TWO keys at runtime — `report.read` AND the module's read key
 *     (`routes.go` of each service) — while `x-vigov-permission` in openapi shows one. The screen
 *     gates each block on both; the server stays the authority.
 *  3. The two petitions queues and the document queue answer **503** when identity's working
 *     calendar is unreachable. That reaches the screen as an explicit error line, never as an empty
 *     list — an empty panel at the one moment the server cannot judge "critical" reads as "nothing
 *     urgent".
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * NO `tenant_id` ANYWHERE: the commune comes from `Host` at the edge (rule 1, forbidden #2).
 */

import { docJSON, thamSoTheoHopDong } from "./goi";
import type { KetQua } from "./goi";
import type {
  documents_get_incoming_document_overdue_queue,
  documents_get_incoming_document_summary,
  documents_incomingSummaryOut,
  documents_overdueQueueOut,
  petitions_citizenReportSummaryOut,
  petitions_get_citizen_report_summary,
  petitions_get_overdue_citizen_reports,
  petitions_get_overdue_tasks,
  petitions_get_task_summary,
  petitions_overdueQueueOut,
  petitions_taskSummaryOut,
} from "./schema.gen";

/** A half-open period `[from, to)`, both RFC 3339 WITH an offset. Both required — point 1 above. */
export type SummaryPeriod = {
  readonly from: string;
  readonly to: string;
};

/**
 * Rows asked of each queue. The servers default to and cap at 10 (spec §5 "tối đa 10 mục"); it is
 * sent explicitly because the document queue declares `limit` REQUIRED in the contract.
 */
export const QUEUE_LIMIT = 10;

/** GET /api/v1/task-summary path. Split from the network call so it is testable without `fetch`. */
export function taskSummaryPath(period: SummaryPeriod): string {
  const path: petitions_get_task_summary["duongDan"] = "/api/v1/task-summary";
  const query = new URLSearchParams();
  const set = thamSoTheoHopDong<petitions_get_task_summary["truyVan"]>(query);
  set("from", period.from);
  set("to", period.to);
  return `${path}?${query.toString()}`;
}

export function fetchTaskSummary(
  period: SummaryPeriod,
): Promise<KetQua<petitions_taskSummaryOut>> {
  return docJSON<petitions_taskSummaryOut>(taskSummaryPath(period));
}

/**
 * GET /api/v1/citizen-report-summary path.
 *
 * Without `feedback.restricted` the server silently leaves the `can-bo` field out of every count —
 * the same rows the drill-down list can reach. The screen never says so: saying it would tell a
 * colleague that reports about staff exist.
 */
export function citizenReportSummaryPath(period: SummaryPeriod): string {
  const path: petitions_get_citizen_report_summary["duongDan"] = "/api/v1/citizen-report-summary";
  const query = new URLSearchParams();
  const set = thamSoTheoHopDong<petitions_get_citizen_report_summary["truyVan"]>(query);
  set("from", period.from);
  set("to", period.to);
  return `${path}?${query.toString()}`;
}

export function fetchCitizenReportSummary(
  period: SummaryPeriod,
): Promise<KetQua<petitions_citizenReportSummaryOut>> {
  return docJSON<petitions_citizenReportSummaryOut>(citizenReportSummaryPath(period));
}

/** GET /api/v1/incoming-document-summary path. */
export function incomingDocumentSummaryPath(period: SummaryPeriod): string {
  const path: documents_get_incoming_document_summary["duongDan"] =
    "/api/v1/incoming-document-summary";
  const query = new URLSearchParams();
  const set = thamSoTheoHopDong<documents_get_incoming_document_summary["truyVan"]>(query);
  set("from", period.from);
  set("to", period.to);
  return `${path}?${query.toString()}`;
}

export function fetchIncomingDocumentSummary(
  period: SummaryPeriod,
): Promise<KetQua<documents_incomingSummaryOut>> {
  return docJSON<documents_incomingSummaryOut>(incomingDocumentSummaryPath(period));
}

export function overdueTasksPath(): string {
  const path: petitions_get_overdue_tasks["duongDan"] = "/api/v1/overdue-tasks";
  const query = new URLSearchParams();
  thamSoTheoHopDong<petitions_get_overdue_tasks["truyVan"]>(query)("limit", String(QUEUE_LIMIT));
  return `${path}?${query.toString()}`;
}

/** GET /api/v1/overdue-tasks — code, task-type code, missed deadline, critical. No free text. */
export function fetchOverdueTasks(): Promise<KetQua<petitions_overdueQueueOut>> {
  return docJSON<petitions_overdueQueueOut>(overdueTasksPath());
}

export function overdueCitizenReportsPath(): string {
  const path: petitions_get_overdue_citizen_reports["duongDan"] = "/api/v1/overdue-citizen-reports";
  const query = new URLSearchParams();
  thamSoTheoHopDong<petitions_get_overdue_citizen_reports["truyVan"]>(query)(
    "limit",
    String(QUEUE_LIMIT),
  );
  return `${path}?${query.toString()}`;
}

/** GET /api/v1/overdue-citizen-reports — lookup code, field code ("" = unclassified), deadline. */
export function fetchOverdueCitizenReports(): Promise<KetQua<petitions_overdueQueueOut>> {
  return docJSON<petitions_overdueQueueOut>(overdueCitizenReportsPath());
}

export function incomingDocumentOverdueQueuePath(): string {
  const path: documents_get_incoming_document_overdue_queue["duongDan"] =
    "/api/v1/incoming-document-overdue-queue";
  const query = new URLSearchParams();
  thamSoTheoHopDong<documents_get_incoming_document_overdue_queue["truyVan"]>(query)(
    "limit",
    String(QUEUE_LIMIT),
  );
  return `${path}?${query.toString()}`;
}

/**
 * GET /api/v1/incoming-document-overdue-queue — a DIFFERENT row shape from the petitions queues
 * (`due_at`, `id`, optional `holding_unit`). The screen merges both shapes in
 * `features/dashboard/figures.ts`, and shows neither `id` nor `holding_unit`.
 */
export function fetchIncomingDocumentOverdueQueue(): Promise<KetQua<documents_overdueQueueOut>> {
  return docJSON<documents_overdueQueueOut>(incomingDocumentOverdueQueuePath());
}
