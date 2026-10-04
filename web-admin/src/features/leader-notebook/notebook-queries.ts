/**
 * The three questions of the Sổ tay lãnh đạo (`docs/ui-ux/03-so-tay-lanh-dao.md`), as ADR 0071
 * defines them — ADR 0071 OVERRIDES spec 03:41-43 where the two differ. Pure: no network, no DOM.
 *
 * EVERY QUESTION GOES THROUGH THE REGISTER'S OWN BUILDERS (`lib/api/nhiem-vu.ts`): the list through
 * `duongDanSoNhiemVu`, the count through `taskCountsPath`, both over `appendTaskFilters`. The spec
 * demands that these figures match `/nhiem-vu` "tuyệt đối" (03:82); a second query builder here would
 * be the copy that drifts, and the drifted figure is the one a leader reads out.
 *
 * ALL COUNTS ARE THE SERVER'S. Counting the rows of a page in the browser is what require's prototype
 * did (300 newest rows, `kb/50-doi-chieu/2026-10-04-feat-m8-multitenant-foundation-so-tay.md` §B #5):
 * a commune past 300 tasks gets a badge that silently undercounts.
 */

import type { LocHangChoLuiHan, LocNhiemVu } from "@/lib/api/nhiem-vu";
import type { petitions_taskCountsOut } from "@/lib/api/schema.gen";

/** Rows per read of one list. The server accepts 1–100; `Xem thêm` reads the next page. */
export const PAGE_SIZE = 50;

/**
 * Column 1 — `Việc quá hạn`: EXACTLY `metric=overdue` of the Tổng quan and of `/nhiem-vu?metric=overdue`
 * (open, past `han_xu_ly`, not `tam-dung`, not finished late), whole commune, sub-tasks included.
 * The scope parameter is ABSENT, which is the server's `all`.
 */
export const OVERDUE_FILTER: LocNhiemVu = { metric: "overdue" };

/** Most late first: the oldest deadline first (`due_at` ascending) — spec 03:58. */
export const OVERDUE_LIST: LocNhiemVu = { ...OVERDUE_FILTER, sapXep: "due_at", chieu: "asc" };

/**
 * Column 2, group `Duyệt hoàn thành`: every `cho-duyet` task of the commune. `scope=all` is the
 * server's default and the register's builder sends it as an ABSENT parameter (`appendTaskFilters`),
 * so the wire carries `status=cho-duyet` alone — the same rows. Shown only to holders of
 * `task.approve` (ADR 0071): the people who can actually press "Duyệt".
 */
export const PENDING_APPROVAL_FILTER: LocNhiemVu = { trangThai: "cho-duyet" };

/**
 * Column 2, group `Duyệt lùi hạn`: requests whose task names ME as assigner (ADR 0038). `me` carries no
 * identity — the server takes the code from the session.
 */
export const MY_EXTENSION_REQUESTS: LocHangChoLuiHan = { approver: "me" };

/**
 * Column 3 — `Việc tôi đã giao`: creator OR assigner = me, every status but `hoan-thanh`. Both halves
 * are the server's (`scope=assigned-by-me`, `incomplete=true`); the session names "me".
 */
export const ASSIGNED_BY_ME_FILTER: LocNhiemVu = { phamVi: "assigned-by-me", incomplete: true };

/** Newest first — spec 03:58. Sent explicitly rather than trusting the server's default to stay put. */
export const ASSIGNED_BY_ME_LIST: LocNhiemVu = {
  ...ASSIGNED_BY_ME_FILTER,
  sapXep: "created_at",
  chieu: "desc",
};

/**
 * A `/task-counts` answer → one figure: the sum over the statuses it reports. The filter already
 * selected the rows; the sum is how many there are, whatever status each one sits in.
 */
export function sumCounts(out: petitions_taskCountsOut): number {
  return out.by_status.reduce((n, s) => n + s.count, 0);
}

/* ── Words (spec 03 §2-§3, domain review 04/10/2026) ─────────────────────────────────────────── */

export const PAGE_TITLE = "Sổ tay lãnh đạo";
export const PAGE_SUBTITLE = "Việc quá hạn, việc chờ tôi duyệt và việc tôi đã giao.";

export const OVERDUE_TITLE = "Việc quá hạn";
export const APPROVAL_TITLE = "Chờ tôi duyệt";
export const ASSIGNED_TITLE = "Việc tôi đã giao";

export const COMPLETION_GROUP_TITLE = "Duyệt hoàn thành";
export const EXTENSION_GROUP_TITLE = "Duyệt lùi hạn";

export const OVERDUE_EMPTY = "Không có việc quá hạn.";
export const APPROVAL_EMPTY = "Không có việc chờ duyệt.";
export const ASSIGNED_EMPTY = "Chưa có việc nào đang giao.";
/** One group empty while the other is not — said in the group, not as the column's empty state. */
export const COMPLETION_GROUP_EMPTY = "Không có việc chờ duyệt hoàn thành.";
export const EXTENSION_GROUP_EMPTY = "Không có đề nghị lùi hạn nào chờ bạn duyệt.";

export const LOAD_ERROR_TITLE = "Chưa tải được danh sách";
export const COUNT_ERROR_PREFIX = "Chưa đếm được:";
export const MORE_LABEL = "Xem thêm";
export const LOADING_SENTENCE = "Đang tải…";

/** Shown in place of a figure the server has not given (loading, or the count failed). */
export const NO_FIGURE = "—";

/** The gate sentence: names the right that is missing, never a bare "không có quyền". */
export const NO_ACCESS_SENTENCE =
  "Sổ tay lãnh đạo đọc từ sổ nhiệm vụ của xã. Tài khoản của bạn chưa được cấp quyền xem nhiệm vụ.";

/** A sub-task row says whose child it is (ADR 0071: sub-tasks are in the overdue column). */
export function childOfText(parentCode: string): string {
  return `việc con của ${parentCode}`;
}
