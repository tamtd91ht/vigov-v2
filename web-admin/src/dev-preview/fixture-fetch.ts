import type { identity_phienHienTaiRa } from "@/lib/api/schema.gen";

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
import { devPreviewEnabled, isDevPreviewPath } from "./preview-gate";

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
  if (p === "/api/v1/staff-directory") return json(PREVIEW_STAFF);
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
