/**
 * Wording and DISPLAY-ONLY helpers for budget period close (`chốt kỳ ngân sách`), decided by the
 * user 30/09/2026. Pure functions: no network, no DOM, no clock.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * THE SERVER IS THE AUTHORITY ON WHAT A CLOSE LOCKS. Every guarded write checks it inside its own
 * transaction and answers 409 `budget_period_closed` with a sentence the screen shows verbatim.
 * The helpers below only decide what to DRAW — a lock badge, a disabled button, one notice — and
 * encode no more than the rule the route block states (`service-finance/internal/http/routes.go`,
 * "WHAT A CLOSE LOCKS"):
 *
 *   - a MONTH close covers entries dated in that year-month;
 *   - a YEAR close covers entries dated in that year, entries on a sheet of that year, and every
 *     sheet / line / headline / value write of that year's sheets.
 *
 * A second copy of anything beyond that would drift the day the server's rule moves, and the
 * stale copy is the one staff would read. When these helpers and the server disagree, the 409 wins.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * Only ACTIVE closes lock. A reopened close stays in the history (its row is written once, never
 * deleted) and locks nothing.
 */

import { nhanThoiDiem } from "@/features/phan-anh/nhan-phieu"; // vi-name-ok: existing shared time formatter, pinned to Asia/Ho_Chi_Minh
import type {
  finance_budgetPeriodCloseIn,
  finance_budgetPeriodCloseOut,
} from "@/lib/api/schema.gen";

import type { KetQuaDung } from "./nhan-thu-chi"; // vi-name-ok: existing result type of this feature

/** Server limit on the reopen reason (`finance_budgetPeriodReopeningIn.reason`, ≤500 characters). */
export const REOPEN_REASON_MAX = 500;

/** Length in CHARACTERS, not UTF-16 units: a composed Vietnamese letter is not counted twice. */
export function charCount(text: string): number {
  return [...text].length;
}

/** `true` for a whole-year close. `month: null` is the contract's spelling of "whole year". */
export function isYearClose(close: Pick<finance_budgetPeriodCloseOut, "month">): boolean {
  return close.month === null;
}

/** "Tháng 9/2026" or "Cả năm 2026" — the period a close names, as the list prints it. */
export function periodLabel(close: Pick<finance_budgetPeriodCloseOut, "year" | "month">): string {
  return close.month === null ? `Cả năm ${close.year}` : `Tháng ${close.month}/${close.year}`;
}

/** Same period, lower-case, for use inside a sentence. */
function periodInSentence(close: Pick<finance_budgetPeriodCloseOut, "year" | "month">): string {
  return close.month === null ? `cả năm ${close.year}` : `tháng ${close.month}/${close.year}`;
}

export function closeStatusLabel(close: Pick<finance_budgetPeriodCloseOut, "active">): string {
  return close.active ? "Đang chốt" : "Đã mở chốt";
}

/** An RFC 3339 instant in Vietnam time, or `—` when the server sent none. */
export function formatInstant(iso: string | undefined): string {
  return iso === undefined || iso === "" ? "—" : nhanThoiDiem(iso);
}

/**
 * Display order: ACTIVE closes first (they are what locks today), then the reopened history. Within
 * each group, the whole-year close before the months, months in calendar order, and the latest
 * revision of a period first.
 */
export function sortCloses(
  closes: readonly finance_budgetPeriodCloseOut[],
): finance_budgetPeriodCloseOut[] {
  const monthRank = (c: finance_budgetPeriodCloseOut) => (c.month === null ? 0 : c.month);
  return [...closes].sort(
    (a, b) =>
      Number(b.active) - Number(a.active) ||
      a.year - b.year ||
      monthRank(a) - monthRank(b) ||
      b.revision - a.revision,
  );
}

function activeYearClose(
  closes: readonly finance_budgetPeriodCloseOut[],
  year: number,
): finance_budgetPeriodCloseOut | undefined {
  return closes.find((c) => c.active && isYearClose(c) && c.year === year);
}

/**
 * Why the SHEET of `sheetYear` cannot be edited, or `null` when no active year close covers it.
 *
 * Only a YEAR close locks sheet, line, headline and value edits; a month close locks entries only.
 */
export function sheetLockReason(
  closes: readonly finance_budgetPeriodCloseOut[],
  sheetYear: number,
): string | null {
  const close = activeYearClose(closes, sheetYear);
  if (close === undefined) return null;
  return (
    `Năm ${sheetYear} đã chốt cả năm (mã ${close.code}): không sửa được bảng, khoản mục, dòng tổng ` +
    `hay số liệu của bảng này, và không thêm, không gỡ được đợt thu chi của bảng. Muốn sửa, mở ` +
    `chốt kèm lý do.`
  );
}

/**
 * Why an ENTRY dated `entryDate` on a sheet of `sheetYear` cannot be removed, or `null`.
 *
 * A date that is not `YYYY-MM-DD` answers `null` — no guess; the server still decides.
 */
export function entryLockReason(
  closes: readonly finance_budgetPeriodCloseOut[],
  entryDate: string,
  sheetYear: number,
): string | null {
  const match = /^(\d{4})-(\d{2})-\d{2}$/.exec(entryDate);
  if (match === null) return null;
  const year = Number(match[1]);
  const month = Number(match[2]);

  const covering =
    activeYearClose(closes, year) ??
    activeYearClose(closes, sheetYear) ??
    closes.find((c) => c.active && c.year === year && c.month === month);
  if (covering === undefined) return null;
  return (
    `Đợt thuộc kỳ đã chốt: ${periodInSentence(covering)} (mã ${covering.code}). Không gỡ được; ` +
    `sửa sai bằng một đợt điều chỉnh ghi ở kỳ còn mở.`
  );
}

/**
 * One sentence above the entry form naming the closed months of `year`, or `null` when none.
 * A year close is not listed here: it disables the whole form (`sheetLockReason`).
 */
export function closedMonthsHint(
  closes: readonly finance_budgetPeriodCloseOut[],
  year: number,
): string | null {
  const months = closes
    .filter((c) => c.active && c.year === year && c.month !== null)
    .map((c) => c.month as number)
    .sort((a, b) => a - b);
  if (months.length === 0) return null;
  const list = months.map((m) => `${m}/${year}`).join(", ");
  return (
    `Các tháng đã chốt: ${list}. Đợt có ngày trong các tháng này không ghi được; sai sót của kỳ ` +
    `đã chốt được sửa bằng một đợt điều chỉnh ghi ở kỳ còn mở, kèm lý do điều chỉnh.`
  );
}

/**
 * Body of `POST /budget-period-closes` from the close form's choice: `"year"` for the whole year,
 * `"1"`…`"12"` for a month. Anything else is refused here rather than sent as a 400.
 */
export function buildCloseBody(
  year: number,
  choice: string,
): KetQuaDung<finance_budgetPeriodCloseIn> {
  if (choice === "year") return { ok: true, than: { year } };
  if (/^(?:[1-9]|1[0-2])$/.test(choice)) return { ok: true, than: { year, month: Number(choice) } };
  return { ok: false, thongBao: "Chọn tháng cần chốt, hoặc chọn chốt cả năm." };
}

/**
 * What the close will lock, in one sentence, shown on the confirm step BEFORE anything is sent.
 * Month and year lock different things, and saying the wrong one is saying something false.
 */
export function closeConsequence(body: finance_budgetPeriodCloseIn): string {
  const { year, month } = body;
  if (month === undefined || month === null) {
    return (
      `Chốt cả năm ${year}: không thêm, không gỡ được đợt thu chi nào của năm ${year}, và không ` +
      `sửa được bảng, khoản mục, dòng tổng hay số liệu của các bảng thu, chi năm ${year}.`
    );
  }
  return (
    `Chốt tháng ${month}/${year}: không thêm, không gỡ được đợt thu chi có ngày trong tháng ` +
    `${month}/${year}; bảng, khoản mục và số liệu nhập trực tiếp vẫn sửa được.`
  );
}

/** Title of the confirm step. */
export function closeConfirmTitle(body: finance_budgetPeriodCloseIn): string {
  return body.month === undefined || body.month === null
    ? `Xác nhận chốt cả năm ${body.year}`
    : `Xác nhận chốt tháng ${body.month}/${body.year}`;
}

/** What happens after a close — the two ways out, both leaving a trail. */
export const AFTER_CLOSE_NOTE =
  "Sai sót phát hiện sau khi chốt được sửa bằng một đợt điều chỉnh ghi ở kỳ còn mở, kèm lý do. " +
  "Mở chốt cần quyền xác nhận ngân sách và lý do; lần chốt và lần mở chốt đều được lưu lại.";

/** Reopen reason: required, trimmed, at most `REOPEN_REASON_MAX` characters. */
export function validateReopenReason(raw: string): KetQuaDung<string> {
  const reason = raw.trim();
  if (reason === "") return { ok: false, thongBao: "Nhập lý do mở chốt." };
  const count = charCount(reason);
  if (count > REOPEN_REASON_MAX) {
    return {
      ok: false,
      thongBao: `Lý do mở chốt dài quá ${REOPEN_REASON_MAX} ký tự (hiện có ${count} ký tự).`,
    };
  }
  return { ok: true, than: reason };
}

/** Sentence of the reopen dialog: what reopening gives back, and that it is recorded. */
export function reopenConsequence(close: finance_budgetPeriodCloseOut): string {
  return isYearClose(close)
    ? `Mở chốt cả năm ${close.year}: đợt thu chi, bảng, khoản mục và số liệu của năm ${close.year} ` +
        `sửa được trở lại. Lần chốt ${close.code} vẫn được giữ kèm người mở chốt và lý do.`
    : `Mở chốt tháng ${close.month}/${close.year}: đợt thu chi có ngày trong tháng này ghi, gỡ được ` +
        `trở lại. Lần chốt ${close.code} vẫn được giữ kèm người mở chốt và lý do.`;
}
