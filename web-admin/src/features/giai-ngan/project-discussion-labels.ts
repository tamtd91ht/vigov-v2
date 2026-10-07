import type {
  finance_latestIssueOut,
  finance_projectIssueOut,
  identity_canBoChonNguoiRa,
} from "@/lib/api/schema.gen";

import { nhanNgay } from "./nhan-du-an"; // vi-name-ok: existing date formatter (rule 12, invariant 3)
import { shortDayLabel } from "./nhan-ghi-giai-ngan";
import type { PeopleCatalogue } from "./project-people";

/**
 * Words and pure rules of §8.1 "Vướng mắc", §8.4 "Trao đổi", §7.2's "Vướng mắc mới nhất" and §3's
 * fourth-card line. No hooks, no network: each rule is testable on its own.
 *
 * THE STAFF CODE IS WHAT THE SERVER STORES for an author, a resolver and a mention (`CB-00123`, rule 6
 * invariant 8); names come from `GET /api/v1/staff-directory` (`project-people.ts`), read once per page.
 */

export const ISSUE_PLACEHOLDER = "Vướng mắc đang gặp ở dự án này…";
export const COMMENT_PLACEHOLDER = "Nhập ý kiến trao đổi về dự án này…";

/** The prototype's chip row shows eight people (`BudgetItemDetail.tsx:746`). */
export const MENTION_PICKER_LIMIT = 8;

/**
 * Who wrote / resolved / was mentioned: the directory's name, else THE CODE ITSELF. Unlike a project's
 * assignee (`project-people.ts`, where an unknown reference reads "not assigned"), an author always
 * exists — the code is the accountable "who" of the record, and hiding it would leave a timeline entry
 * by nobody.
 */
export function staffLabel(code: string, staff: PeopleCatalogue<identity_canBoChonNguoiRa>): string {
  if (staff.phase !== "ready") return code;
  return staff.names.get(code) ?? code;
}

/**
 * The issue's follow-up line (prototype `BudgetItemDetail.tsx:583-589`): "Người theo dõi: {name} · hạn
 * {dd/mm/yyyy}", either half alone, `null` when neither (65afbdcd: `owner_code` defaults to the
 * project's officer at the time of recording; `due_on` is optional).
 *
 * THE NAME ONLY, NEVER THE CODE: an owner the directory cannot name (not loaded, or no longer listed)
 * drops the owner half rather than printing `CB-…` where the prototype prints a person.
 */
export function issueFollowUpLine(
  issue: Pick<finance_projectIssueOut, "owner_code" | "due_on">,
  staff: PeopleCatalogue<identity_canBoChonNguoiRa>,
): string | null {
  const code = (issue.owner_code ?? "").trim();
  const name = code !== "" && staff.phase === "ready" ? staff.names.get(code) : undefined;
  const due = (issue.due_on ?? "").trim();
  const parts = [
    name !== undefined ? `Người theo dõi: ${name}` : null,
    due !== "" ? `hạn ${nhanNgay(due)}` : null,
  ].filter((p): p is string => p !== null);
  return parts.length === 0 ? null : parts.join(" · ");
}

/** §7.2 second line: `27/8/2026 · đã gỡ`, or the day alone for an open issue. */
export function latestIssueDateLine(issue: finance_latestIssueOut): string {
  const day = shortDayLabel(issue.recorded_at);
  return issue.resolved ? `${day} · đã gỡ` : day;
}

/**
 * §3 fourth card: `N vướng mắc đang theo dõi`. ABSENT IS NOT ZERO: the field is optional in the contract
 * (added to a published reply), and "0 vướng mắc" would tell leadership "none" — so an absent count
 * says it was not read.
 */
export function openIssuesLabel(count: number | null | undefined): string {
  if (count === null || count === undefined || !Number.isFinite(count)) return "Chưa đọc được số vướng mắc";
  return `${count} vướng mắc đang theo dõi`;
}

/**
 * Spec 07 line under a message: "Nhắc: @A, @B" — names from the directory, the code itself when the
 * directory does not resolve it (the record still says who was meant). `null` = nobody mentioned.
 */
export function mentionLine(
  codes: readonly string[],
  staff: PeopleCatalogue<identity_canBoChonNguoiRa>,
): string | null {
  if (codes.length === 0) return null;
  return `Nhắc: ${codes.map((c) => `@${staffLabel(c, staff)}`).join(", ")}`;
}
