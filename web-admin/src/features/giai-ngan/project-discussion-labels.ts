import type { finance_latestIssueOut, identity_canBoChonNguoiRa } from "@/lib/api/schema.gen";

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
export const COMMENT_PLACEHOLDER = "Nhập ý kiến trao đổi về dự án này… Gõ @ để nhắc tên cán bộ";

export const ISSUES_DENIED =
  "Tài khoản của bạn chưa được cấp quyền “Cập nhật giải ngân”, nên phần ghi nhận vướng mắc và đánh " +
  "dấu đã gỡ không hiển thị. Liên hệ quản trị viên của đơn vị nếu bạn cần quyền này.";
export const COMMENTS_DENIED =
  "Tài khoản của bạn chưa được cấp quyền “Xem giải ngân”, nên không gửi được ý kiến trao đổi.";

/** The prototype's picker shows eight people (`BudgetItemDetail.tsx:746`). */
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

/* ── Mentions ─────────────────────────────────────────────────────────────────────────────────── */

/** The `@…` being typed at the caret: where it starts (the `@`), where it ends (the caret), what follows `@`. */
export type MentionQuery = { readonly start: number; readonly end: number; readonly query: string };

/** Longest query still looked up: past this the clerk is writing a sentence, not a name. */
const MENTION_QUERY_MAX = 40;

/**
 * The mention being typed, or `null`. An `@` counts only at the start or after whitespace — `a@b` is an
 * address, not a mention — and the query may hold spaces (names do) but never a line break or another `@`.
 */
export function mentionQueryAt(text: string, caret: number): MentionQuery | null {
  const before = text.slice(0, caret);
  const at = before.lastIndexOf("@");
  if (at < 0) return null;
  if (at > 0 && !/\s/.test(before.charAt(at - 1))) return null;
  const query = before.slice(at + 1);
  if (query.includes("\n") || query.length > MENTION_QUERY_MAX) return null;
  return { start: at, end: caret, query };
}

/** Lower case, no diacritics, `đ` → `d`: "nguyen" finds "Nguyễn" — what a clerk types on any keyboard. */
function fold(s: string): string {
  return s.normalize("NFD").replace(/\p{M}/gu, "").replace(/đ/g, "d").replace(/Đ/g, "D").toLocaleLowerCase("vi");
}

/** Up to `MENTION_PICKER_LIMIT` directory entries whose name or code contains the query, directory order. */
export function matchingStaff(
  items: readonly identity_canBoChonNguoiRa[],
  query: string,
): readonly identity_canBoChonNguoiRa[] {
  const needle = fold(query);
  return items
    .filter((s) => needle === "" || fold(s.full_name).includes(needle) || fold(s.code).includes(needle))
    .slice(0, MENTION_PICKER_LIMIT);
}

/** Replace the `@query` with `@Full Name ` and say where the caret goes. */
export function insertMention(
  text: string,
  q: MentionQuery,
  name: string,
): { readonly text: string; readonly caret: number } {
  const inserted = `@${name} `;
  return { text: text.slice(0, q.start) + inserted + text.slice(q.end), caret: q.start + inserted.length };
}

export type PickedMention = { readonly code: string; readonly name: string };

/**
 * The codes to send: those picked whose `@Name` is STILL in the body — deleting the words un-mentions
 * the person, as the clerk would expect. Picked order, no duplicates.
 */
export function mentionedCodes(body: string, picked: readonly PickedMention[]): string[] {
  const out: string[] = [];
  for (const p of picked) {
    if (!out.includes(p.code) && body.includes(`@${p.name}`)) out.push(p.code);
  }
  return out;
}

export type BodySegment = { readonly text: string; readonly mention: boolean };

/**
 * The body cut into plain text and `@Name` runs for the names given (the message's mentioned staff,
 * resolved). PLAIN STRINGS, rendered as React text — never HTML (rule 13). Longest name first, so
 * "@Nguyễn Văn An" is not cut as "@Nguyễn Văn A" + "n".
 */
export function mentionSegments(body: string, names: readonly string[]): BodySegment[] {
  const sorted = [...new Set(names.filter((n) => n !== ""))].sort((a, b) => b.length - a.length);
  const out: BodySegment[] = [];
  let plain = "";
  let i = 0;
  while (i < body.length) {
    const hit = body.charAt(i) === "@" ? sorted.find((n) => body.startsWith(n, i + 1)) : undefined;
    if (hit === undefined) {
      plain += body.charAt(i);
      i += 1;
      continue;
    }
    if (plain !== "") out.push({ text: plain, mention: false });
    plain = "";
    out.push({ text: `@${hit}`, mention: true });
    i += hit.length + 1;
  }
  if (plain !== "") out.push({ text: plain, mention: false });
  return out;
}
