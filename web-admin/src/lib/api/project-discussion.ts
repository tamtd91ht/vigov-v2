/**
 * §8.1 "Vướng mắc" and §8.4 "Trao đổi" of the project page (`docs/ui-ux/06-giai-ngan.md`), the five
 * routes `service-finance` declares for them (889d4598):
 *
 *   GET  /api/v1/investment-projects/{id}/issues      budget.read
 *   POST /api/v1/investment-projects/{id}/issues      budget.update  + Idempotency-Key REQUIRED
 *   POST /api/v1/project-issues/{id}/resolution       budget.update  (no body; one-way, 409 when repeated)
 *   GET  /api/v1/investment-projects/{id}/comments    budget.read
 *   POST /api/v1/investment-projects/{id}/comments    budget.read    + Idempotency-Key REQUIRED
 *
 * TYPES COME FROM THE CONTRACT (`schema.gen.ts`), never retyped here (rule 9, forbidden #2).
 *
 * THE SERVER SPLITS THE ISSUE TEXT (first line → title, the rest → description, `SplitIssueText`) and
 * REFUSES an over-long first line rather than cutting it. So the text goes up exactly as typed: a client
 * split would be a second copy of that rule, and a client cut would lose the end of the sentence.
 *
 * NO `tenant_id` ANYWHERE — not body, not query, not header. The commune comes from `Host` (rule 1,
 * forbidden #2). No author field either: the author is the session's principal (rule 6, invariant 8).
 *
 * NOTHING IS LOGGED: these bodies are free text typed by staff, and nothing stops a clerk typing a
 * household's name into them (rule 3, forbidden #1).
 */

import { docJSON, docThanLoiGoi, goiGhi, type KetQua } from "./goi"; // vi-name-ok: existing shared call helpers of goi.ts, not renamed (rule 12 invariant 3)
import type {
  finance_get_investment_projects_by_id_comments,
  finance_get_investment_projects_by_id_issues,
  finance_post_investment_projects_by_id_comments,
  finance_post_investment_projects_by_id_issues,
  finance_post_project_issues_by_id_resolution,
  finance_projectCommentIn,
  finance_projectCommentOut,
  finance_projectCommentsOut,
  finance_projectIssueIn,
  finance_projectIssueOut,
  finance_projectIssuesOut,
} from "./schema.gen";

const ISSUES_READ: finance_get_investment_projects_by_id_issues["duongDan"] = "/api/v1/investment-projects/{id}/issues";
const ISSUES_WRITE: finance_post_investment_projects_by_id_issues["duongDan"] = "/api/v1/investment-projects/{id}/issues";
const RESOLUTION: finance_post_project_issues_by_id_resolution["duongDan"] = "/api/v1/project-issues/{id}/resolution";
const COMMENTS_READ: finance_get_investment_projects_by_id_comments["duongDan"] =
  "/api/v1/investment-projects/{id}/comments";
const COMMENTS_WRITE: finance_post_investment_projects_by_id_comments["duongDan"] =
  "/api/v1/investment-projects/{id}/comments";

/** The id goes into the PATH, so it is encoded — never concatenated raw. */
function withId(template: string, id: string): string {
  return template.replace("{id}", encodeURIComponent(id));
}

/**
 * GET — the project's issue timeline, NEWEST FIRST, resolved ones included, with the server's `count`
 * and `open_count`. Same 404 for "no such project" and "another commune's project" as the detail route.
 */
export function getProjectIssues(projectId: string): Promise<KetQua<finance_projectIssuesOut>> {
  return docJSON<finance_projectIssuesOut>(withId(ISSUES_READ, projectId));
}

/**
 * POST — record one issue. 201 with the stored issue.
 *
 * `idempotencyKey` IS A PARAMETER, MINTED WHEN THE FORM OPENS: minted here, every retry after a network
 * failure would be a new key — protecting nothing, while the first send may well have arrived.
 */
export function recordProjectIssue(
  projectId: string,
  text: string,
  idempotencyKey: string,
): Promise<KetQua<finance_projectIssueOut>> {
  const body: finance_projectIssueIn = { text };
  return docThanLoiGoi<finance_projectIssueOut>(
    goiGhi(withId(ISSUES_WRITE, projectId), "POST", body, 201, { "Idempotency-Key": idempotencyKey }),
  );
}

/**
 * POST, NO BODY — mark one issue "Đã gỡ". ONE-WAY: a second call answers 409 with the server's own
 * sentence ("…hãy ghi nhận một vướng mắc mới"), shown verbatim — never rewritten here.
 */
export function resolveProjectIssue(issueId: string): Promise<KetQua<finance_projectIssueOut>> {
  return docThanLoiGoi<finance_projectIssueOut>(goiGhi(withId(RESOLUTION, issueId), "POST", undefined, 200));
}

/** GET — the project's discussion, OLDEST FIRST, with the server's `count`. */
export function getProjectComments(projectId: string): Promise<KetQua<finance_projectCommentsOut>> {
  return docJSON<finance_projectCommentsOut>(withId(COMMENTS_READ, projectId));
}

/**
 * POST — one message, with the STAFF BUSINESS CODES (`CB-…`) it mentions. 201 with the stored message.
 *
 * Built field by field, never `...input`: a spread is how an extra field rides up to the server the day
 * somebody passes a different object. The server stores the codes; it sends nobody anything yet.
 */
export function postProjectComment(
  projectId: string,
  input: finance_projectCommentIn,
  idempotencyKey: string,
): Promise<KetQua<finance_projectCommentOut>> {
  const body: finance_projectCommentIn = {
    body: input.body,
    mentioned_staff_codes: [...(input.mentioned_staff_codes ?? [])],
  };
  return docThanLoiGoi<finance_projectCommentOut>(
    goiGhi(withId(COMMENTS_WRITE, projectId), "POST", body, 201, { "Idempotency-Key": idempotencyKey }),
  );
}
