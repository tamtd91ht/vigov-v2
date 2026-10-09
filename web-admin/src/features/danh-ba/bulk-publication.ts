/**
 * Publishing MANY people to the Zalo Mini App directory at once — wording and decisions
 * (`POST /api/v1/staff/publications`, user decision 30/09/2026; the prototype's selection bar, owner
 * decision 09/10/2026).
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * WHAT THE USER DECIDED, AND WHAT IT KEEPS OF #12: the administrator ticks people in the table, then
 * "Thêm vào danh bạ Mini App" opens a confirmation where EACH selected person carries their own
 * "đã hỏi ý người này" tick. Only ticked rows are published. So consent is still asked of EACH
 * person — the selection saves clicks, not the question. There is no "tick all consents" control on
 * purpose: one click confirming twenty conversations is a confirmation nobody had.
 *
 * Taking people OFF has no bulk route: "Rút khỏi danh bạ" sends the single-person
 * `PUT …/publication` once per selected person (`danh-ba-lien-he.tsx`).
 *
 * PURE MODULE, NO JSX: every decision here is checked by comparing values.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */

import type { BulkPublicationRow } from "@/lib/api/can-bo";
import type { identity_bulkPublicationItemOut, identity_canBoTomTat } from "@/lib/api/schema.gen";

/* ---- the selection bar (prototype `StaffDirectoryWorkspace.tsx:224-255`, verbatim) ----------- */

export function selectedCountText(n: number): string {
  return `Đã chọn ${n} người`;
}
export const BULK_ADD_BUTTON = "Thêm vào danh bạ Mini App";
export const BULK_WITHDRAW_BUTTON = "Rút khỏi danh bạ";
export const BULK_DELETE_BUTTON = "Xoá đã chọn";

/** The prototype's two toasts, verbatim. `n` is what the SERVER changed, never the selection size. */
export function bulkAddedToast(n: number): string {
  return `Đã thêm ${n} người vào danh bạ trên Mini App.`;
}
export function bulkWithdrawnToast(n: number): string {
  return `Đã rút ${n} người khỏi danh bạ trên Mini App.`;
}

/** Withdrawing people none of whom is on the Mini App — said, not sent. */
export const BULK_NOTHING_TO_WITHDRAW = "Trong những người đã chọn không có ai đang hiện trên Mini App.";

/** Some single withdrawals refused: how many, then the first server sentence verbatim. */
export function bulkWithdrawFailedText(failed: number, firstMessage: string): string {
  return `Chưa rút được ${failed} người. ${firstMessage}`;
}

/* ---- the consent dialog --------------------------------------------------------------------- */

export const BULK_TITLE = "Thêm những người đã chọn vào danh bạ Mini App";

/** Said before the privacy note (`CANH_BAO_CONG_KHAI`, reused verbatim — one wording, one owner). */
export const BULK_INTRO =
  "Đánh dấu “Đã hỏi ý người này” cho TỪNG người đã được hỏi ý và đồng ý. Người chưa được đánh " +
  "dấu sẽ được bỏ qua. Với mỗi người được công khai:";

/** Everyone selected is already on the Mini App, or locked: nobody to ask. */
export const BULK_NO_CANDIDATES =
  "Những người đã chọn đều đang hiện trên Mini App hoặc có tài khoản đang bị khoá, nên không có ai " +
  "để thêm.";

export const BULK_CONSENT_LABEL = "Đã hỏi ý người này";

export function bulkConsentAria(fullName: string): string {
  return `${BULK_CONSENT_LABEL} và người này đồng ý công khai số di động: ${fullName}`;
}

export const BULK_SUBMIT = "Công khai những người đã đánh dấu";
export const BULK_CLOSE = "Đóng";

export function bulkCountText(selected: number, consented: number): string {
  return `Đã chọn ${selected} người, trong đó ${consented} người đã được đánh dấu đã hỏi ý.`;
}

export const BULK_NONE_SELECTED = "Hãy chọn ít nhất một người.";
export const BULK_NONE_CONSENTED =
  "Chưa có người nào được đánh dấu đã hỏi ý. Chỉ người đã được hỏi ý và đồng ý mới được công khai.";

/** The server's cap (`domain.MaxBulkPublication`). Checked here only to refuse before sending. */
export const BULK_MAX = 200;
export const BULK_TOO_MANY = `Mỗi lần công khai tối đa ${BULK_MAX} người. Hãy bỏ bớt người đã chọn.`;

export const BULK_RESULT_TITLE = "Kết quả công khai";

export const BULK_REPLAYED =
  "Yêu cầu này đã được máy chủ nhận từ lần gửi trước, nên không có danh sách kết quả từng người. " +
  "Danh bạ đã được tải lại: xem cột “Trên Mini App” để biết ai đang hiện.";

/* ---- decisions ------------------------------------------------------------------------------ */

/**
 * Offered in the bulk list: not yet on the Mini App, and not locked. Locked people are HIDDEN
 * rather than shown disabled — they cannot be published — but this is only convenience: the server
 * refuses a locked row itself (`staff_locked`) whatever the screen sends.
 */
export function eligibleForBulk(cb: identity_canBoTomTat): boolean {
  return !cb.published && cb.active;
}

/** One selected person. The name is kept so the result list can say WHO, across pages. */
export type BulkSelected = {
  readonly id: string;
  readonly code: string;
  readonly fullName: string;
  readonly consentAsked: boolean;
};

/** Selection in the order people were picked — that order is the request order. */
export type BulkSelection = readonly BulkSelected[];

/** Select one person. A newly selected row starts UNTICKED: consent is asserted, never assumed. */
export function addSelected(sel: BulkSelection, cb: identity_canBoTomTat): BulkSelection {
  if (sel.some((s) => s.id === cb.id)) return sel;
  return [...sel, { id: cb.id, code: cb.code, fullName: cb.full_name, consentAsked: false }];
}

export function removeSelected(sel: BulkSelection, id: string): BulkSelection {
  return sel.filter((s) => s.id !== id);
}

/**
 * What the consent dialog asks about, from the people ticked in the table: the eligible ones, in
 * the order they were ticked, EVERY consent box unticked — consent is asserted per person, never
 * carried over from an earlier opening.
 */
export function consentSelection(selected: readonly identity_canBoTomTat[]): BulkSelection {
  return selected.filter(eligibleForBulk).reduce<BulkSelection>((sel, cb) => addSelected(sel, cb), []);
}

/** The table selection after ticking or unticking one row. Order of ticking is kept. */
export function toggleRow(
  selected: readonly identity_canBoTomTat[],
  cb: identity_canBoTomTat,
  on: boolean,
): readonly identity_canBoTomTat[] {
  const rest = selected.filter((s) => s.id !== cb.id);
  return on ? [...rest, cb] : rest;
}

/** The header box: tick every row of the page (keeping people ticked on other pages), or untick them. */
export function togglePage(
  selected: readonly identity_canBoTomTat[],
  page: readonly identity_canBoTomTat[],
  on: boolean,
): readonly identity_canBoTomTat[] {
  const onPage = new Set(page.map((cb) => cb.id));
  const rest = selected.filter((s) => !onPage.has(s.id));
  return on ? [...rest, ...page] : rest;
}

/** Results of a bulk publish: how many the server published. */
export function publishedCount(items: readonly identity_bulkPublicationItemOut[]): number {
  return items.filter((i) => i.result === "published").length;
}

export function setConsent(sel: BulkSelection, id: string, consentAsked: boolean): BulkSelection {
  return sel.map((s) => (s.id === id ? { ...s, consentAsked } : s));
}

/**
 * Pressing submit: either a refusal sentence (no network call) or the rows to send.
 *
 * NOTHING TICKED → NO REQUEST: every row would come back skipped, and a request that can publish
 * nobody is only an audit-free round trip that looks like it did something.
 */
export function bulkRequest(
  sel: BulkSelection,
): { readonly error: string } | { readonly rows: readonly BulkPublicationRow[] } {
  if (sel.length === 0) return { error: BULK_NONE_SELECTED };
  if (sel.length > BULK_MAX) return { error: BULK_TOO_MANY };
  if (!sel.some((s) => s.consentAsked)) return { error: BULK_NONE_CONSENTED };
  return { rows: sel.map((s) => ({ id: s.id, consentAsked: s.consentAsked })) };
}

/** The Vietnamese sentence for one row of the answer. */
export function bulkResultText(item: identity_bulkPublicationItemOut): string {
  if (item.result === "published") return "Đã công khai";
  switch (item.reason_code) {
    case "consent_required":
      return "Bỏ qua — chưa hỏi ý";
    case "staff_locked":
      return "Bỏ qua — tài khoản đang khoá";
    case "staff_not_found":
      return "Không tìm thấy";
    default:
      // A code this screen does not know: say "skipped" without inventing a reason.
      return "Bỏ qua";
  }
}

/** One line of the result list: who (name + staff code, never a phone number) and what happened. */
export type BulkResultLine = {
  readonly id: string;
  readonly who: string;
  readonly text: string;
  readonly published: boolean;
};

/**
 * Join the answer to the selection on `id` — the server returns ids only (no name, no telephone,
 * rule 3). An id the selection does not hold (should not happen) is shown as "Không rõ người".
 */
export function bulkResultLines(
  items: readonly identity_bulkPublicationItemOut[],
  sel: BulkSelection,
): readonly BulkResultLine[] {
  const byId = new Map(sel.map((s) => [s.id, s]));
  return items.map((item) => {
    const s = byId.get(item.id);
    return {
      id: item.id,
      who: s === undefined ? "Không rõ người" : `${s.fullName} (${s.code})`,
      text: bulkResultText(item),
      published: item.result === "published",
    };
  });
}

export function bulkSummaryText(lines: readonly BulkResultLine[]): string {
  const done = lines.filter((l) => l.published).length;
  return `Đã công khai ${done}/${lines.length} người. Danh bạ đã được tải lại.`;
}
