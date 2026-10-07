import type { identity_danhBaChonNguoiRa, identity_phienHienTaiRa } from "@/lib/api/schema.gen";
import { QUYEN_DUYET_GIA_HAN } from "@/lib/quyen";

import {
  PREVIEW_CATEGORIES,
  PREVIEW_STAFF,
  PREVIEW_UNITS,
  previewComments,
  previewCurve,
  previewFundingSourceProjects,
  previewFundingSources,
  previewIssues,
  previewNotifications,
  previewProject,
  previewProjects,
  previewSummary,
  previewVouchers,
} from "./disbursement.fixture";
import {
  PREVIEW_DOCUMENT_TYPES,
  PREVIEW_REGISTER_ERROR,
  documentPreviewStateNow,
  previewIncomingDocument,
  previewIncomingDocuments,
  previewOutgoingDocuments,
  previewRoutings,
} from "./documents.fixture";
import { devPreviewEnabled, isDevPreviewPath } from "./preview-gate";
import {
  PREVIEW_EXTENSION_APPROVERS,
  PREVIEW_TASK_BLOCS,
  PREVIEW_TASK_PRIORITIES,
  PREVIEW_TASK_STATUSES,
  PREVIEW_TASK_TYPES,
  PREVIEW_WORKING_HOURS,
  previewTask,
  previewTaskCounts,
  previewTaskExtensions,
  previewTaskLog,
  previewTasks,
  type PreviewTaskQuery,
} from "./tasks.fixture";

/**
 * The preview's ANSWERING MACHINE: while the browser is on a `/xem-thu/**` page, every same-origin
 * `/api/` request is answered here from the fixtures, and nothing leaves the browser. Every other request
 * — and every request on any other page — goes to the real `fetch`, untouched.
 *
 * WHY SWAP `fetch` RATHER THAN THE API CLIENT: the preview must render the REAL components, and they call
 * the real client (`lib/api/*`), which calls `fetch` with a relative path (`goi.ts`). Swapping the layer
 * underneath leaves every component and every client function exactly as production runs them, with
 * no mock mode, flag or branch added to either (task card T3).
 *
 * WHY KEYED ON THE CURRENT PATH, NOT INSTALLED AND REMOVED WITH THE PAGE: React runs a child's effects
 * BEFORE its parent's, and in dev Strict Mode it unmounts and re-runs them — a parent that removed the
 * swap on unmount would let a child's re-run reach the real network. Installed once, and asking "am I on
 * the preview?" on every call, the swap is right on every run and inert everywhere else.
 *
 * DEV ONLY, TWICE OVER: the pages that call `installFixtureFetch` 404 in production, and the function
 * itself refuses when `devPreviewEnabled()` is false.
 *
 * WRITES ARE REFUSED, not faked: a preview that pretended to save would show a result no server gave.
 * The refusal travels as the server's own error shape, so the screen shows it where it shows any
 * server sentence.
 */

let installed = false;
let session: identity_phienHienTaiRa | null = null;

export function installFixtureFetch(previewSession: identity_phienHienTaiRa): void {
  session = previewSession;
  if (installed || typeof window === "undefined" || !devPreviewEnabled()) return;
  installed = true;
  const realFetch = window.fetch.bind(window);
  window.fetch = (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
    if (!isDevPreviewPath(window.location.pathname)) return realFetch(input, init);
    const raw = typeof input === "string" ? input : input instanceof URL ? input.href : input.url;
    const url = new URL(raw, window.location.origin);
    if (url.origin !== window.location.origin || !url.pathname.startsWith("/api/")) return realFetch(input, init);
    const method = (init?.method ?? (input instanceof Request ? input.method : "GET")).toUpperCase();
    // `?state=loading` of the Văn bản preview: the register read never settles (no stream involved —
    // headless Chrome's virtual clock never runs out while a body stream is pending).
    if (method === "GET" && isDocumentRegisterPath(url.pathname) && documentPreviewStateNow() === "loading") {
      return new Promise<Response>(() => {});
    }
    return Promise.resolve(answer(method, url));
  };
}

export const PREVIEW_WRITE_REFUSAL = "Trang xem thử chỉ có dữ liệu mẫu — không ghi được gì.";
const NO_FIXTURE = "Trang xem thử chưa có dữ liệu mẫu cho tuyến này.";

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

function refuse(status: number, message: string): Response {
  return json({ code: "xem-thu", message, trace_id: "" }, status);
}

/** Exported for the test: which answer a (method, URL) gets. */
export function answer(method: string, url: URL): Response {
  if (method !== "GET") return refuse(409, PREVIEW_WRITE_REFUSAL);
  const year = Number(url.searchParams.get("year")) || new Date().getFullYear();
  const p = url.pathname;
  let m: RegExpExecArray | null;

  if (p === "/api/v1/sessions/current") return session === null ? refuse(401, NO_FIXTURE) : json(session);
  if (p === "/api/v1/notifications/unread-count") return json({ unread: 1 });
  if (p === "/api/v1/notifications") return json(previewNotifications(year));
  if (p === "/api/v1/capital-plan-categories") return json(PREVIEW_CATEGORIES);
  if (p === "/api/v1/org-units") return json(PREVIEW_UNITS);
  if (p === "/api/v1/staff-directory") return json(staffDirectory(url.searchParams));
  const tasks = answerTasks(p, url.searchParams);
  if (tasks !== null) return tasks;
  const documents = answerDocuments(p, url.searchParams);
  if (documents !== null) return documents;
  if (p === "/api/v1/investment-projects") return json(previewProjects(year));
  if (p === "/api/v1/investment-project-summary") return json(previewSummary(year));
  if (p === "/api/v1/funding-sources") return json(previewFundingSources(year));
  if ((m = /^\/api\/v1\/funding-sources\/([^/]+)\/projects$/.exec(p))) {
    const out = previewFundingSourceProjects(decodeURIComponent(m[1]!), year);
    return out === null ? refuse(404, NO_FIXTURE) : json(out);
  }
  if ((m = /^\/api\/v1\/investment-projects\/([^/]+)(?:\/([a-z-]+))?$/.exec(p))) {
    const id = decodeURIComponent(m[1]!);
    switch (m[2]) {
      case undefined: {
        const one = previewProject(id, year);
        return one === null ? refuse(404, "Không tìm thấy dự án.") : json(one);
      }
      case "disbursements":
        return json(previewVouchers(id, year));
      case "disbursement-curve":
        return json(previewCurve(id, year));
      case "issues":
        return json(previewIssues(id, year));
      case "comments":
        return json(previewComments(id, year));
    }
  }
  return refuse(404, NO_FIXTURE);
}

/** `?unit=` narrows to one department; `?permission=task.extend` to who holds it (the leader picker). */
function staffDirectory(q: URLSearchParams): identity_danhBaChonNguoiRa {
  const unit = q.get("unit");
  const permission = q.get("permission");
  return {
    items: PREVIEW_STAFF.items.filter(
      (s) =>
        (unit === null || s.department_id === unit) &&
        (permission === null || (permission === QUYEN_DUYET_GIA_HAN && PREVIEW_EXTENSION_APPROVERS.includes(s.code))),
    ),
  };
}

export const PREVIEW_NO_FILE = "Trang xem thử không có tệp thật để tải về.";

/** The Nhiệm vụ reads (`lib/api/nhiem-vu.ts`, the catalogues the screen loads); `null` = not a task route. */
function answerTasks(p: string, q: URLSearchParams): Response | null {
  if (p === "/api/v1/task-types") return json(PREVIEW_TASK_TYPES);
  if (p === "/api/v1/task-priorities") return json(PREVIEW_TASK_PRIORITIES);
  if (p === "/api/v1/task-blocs") return json(PREVIEW_TASK_BLOCS);
  if (p === "/api/v1/task-statuses") return json(PREVIEW_TASK_STATUSES);
  if (p === "/api/v1/working-hours") return json(PREVIEW_WORKING_HOURS);
  if (p === "/api/v1/tasks") return json(previewTasks(taskQuery(q)));
  if (p === "/api/v1/task-counts") return json(previewTaskCounts(taskQuery(q)));
  if (p === "/api/v1/task-extensions") return json(previewTaskExtensions(q.get("task")));
  const m = /^\/api\/v1\/tasks\/([^/]+)(\/.*)?$/.exec(p);
  if (m === null) return null;
  const code = decodeURIComponent(m[1]!);
  if (m[2] === undefined) {
    const task = previewTask(code);
    return task === null ? refuse(404, "Không tìm thấy nhiệm vụ.") : json(task);
  }
  if (m[2] === "/log-entries") {
    const log = previewTaskLog(code);
    return log === null ? refuse(404, "Không tìm thấy nhiệm vụ.") : json(log);
  }
  // `…/attachments/{id}/download`: there is no stored object behind the fixture — said, not faked.
  if (/^\/attachments\/[^/]+\/download$/.test(m[2])) return refuse(409, PREVIEW_NO_FILE);
  return refuse(404, NO_FIXTURE);
}

/**
 * The Văn bản reads (`lib/api/van-ban.ts`, `layLoaiVanBan`); `null` = not a document route. The two
 * REGISTER lists follow `?state=` of the preview (`documentPreviewStateNow`): `loading` is held unsettled
 * by the fetch wrapper above — the screen's real first-load state, kept for the screenshot — `empty` with
 * no row, `error` with a refusal in the server's error shape. Single reads and the timeline stay as they
 * are, so the drawer still opens in every state.
 */
function isDocumentRegisterPath(p: string): boolean {
  return p === "/api/v1/incoming-documents" || p === "/api/v1/outgoing-documents";
}

function answerDocuments(p: string, q: URLSearchParams): Response | null {
  if (p === "/api/v1/document-types") return json(PREVIEW_DOCUMENT_TYPES);
  const year = Number(q.get("year")) || new Date().getFullYear();
  const one = (name: string) => q.get(name) ?? undefined;
  const ascending = q.get("order") === "asc";
  if (p === "/api/v1/incoming-documents" || p === "/api/v1/outgoing-documents") {
    const state = documentPreviewStateNow();
    // `loading` never reaches here in the browser: the fetch wrapper holds those reads unsettled.
    if (state === "error") return refuse(503, PREVIEW_REGISTER_ERROR);
    if (state === "empty") return json({ items: [], next_cursor: "", has_more: false });
    return p === "/api/v1/incoming-documents"
      ? json(
          previewIncomingDocuments({
            year,
            status: one("status"),
            unit: one("holding_unit"),
            type: one("document_type"),
            q: one("q"),
            metric: one("metric"),
            ascending,
          }),
        )
      : json(previewOutgoingDocuments({ year, type: one("document_type"), q: one("q"), ascending }));
  }
  const m = /^\/api\/v1\/incoming-documents\/([^/]+)(\/routings)?$/.exec(p);
  if (m === null) return null;
  const id = decodeURIComponent(m[1]!);
  const found = m[2] === undefined ? previewIncomingDocument(id) : previewRoutings(id);
  return found === null ? refuse(404, "Không tìm thấy văn bản đến này.") : json(found);
}

function taskQuery(q: URLSearchParams): PreviewTaskQuery {
  const one = (name: string) => q.get(name) ?? undefined;
  return {
    status: one("status"),
    parent: one("parent"),
    type: one("type"),
    priority: one("priority"),
    bloc: one("bloc"),
    unit: one("unit"),
    assignee: one("assignee"),
    source: one("source"),
    late: q.get("late") === "true",
    documents: q.get("include") === "documents",
  };
}
