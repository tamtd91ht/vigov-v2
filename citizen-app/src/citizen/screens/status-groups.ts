/**
 * THE FOUR STATUS GROUPS A CITIZEN SEES — shared by BOTH citizen screens: the shared ViHAT app
 * (`frame.tsx`, `SubmitReportScreen.tsx`, `MyReportsScreen.tsx`) and the commune's own app
 * (`CommuneAppReports.tsx`, through the older names kept in `commune-app-model.ts`). ADR 0050 #5; scope widened to
 * the shared app by the owner on 28/09/2026 ("3 điểm còn lại cũng theo require nhé").
 *
 * Source: the requirements repo's prototype, `apps/miniapp/src/components/ui/StatusChip.tsx` (four chip
 * labels) and `services/feedback-adapter.ts:79-101` (nine-to-four mapping + per-step labels). The merge
 * happens HERE, never on the server: staff still see all nine (ADR 0027), and the nine codes and the wire
 * contract are unchanged.
 *
 * Why "rejected" and "moved up a level" become "Đã đóng": for the sender the ticket is closed, and the
 * REASON is what they need to read — each screen shows it next to the chip (`statusExplanation`, the
 * reason / receiving-body rows), not in the chip.
 */

/** The filter options of the commune app's list: the four groups plus "all". */
export type StatusFilter = "tat-ca" | StatusGroup;

export type StatusGroup = "da-tiep-nhan" | "dang-xu-ly" | "da-xu-ly-xong" | "da-dong";

const GROUP_OF_CODE: Readonly<Record<string, StatusGroup>> = {
  "da-tiep-nhan": "da-tiep-nhan",
  "dang-phan-loai": "da-tiep-nhan",
  "da-chuyen-xu-ly": "dang-xu-ly",
  "dang-xu-ly": "dang-xu-ly",
  "da-xu-ly": "da-xu-ly-xong",
  "cho-dan-xac-nhan": "da-xu-ly-xong",
  "da-dong": "da-dong",
  "khong-tiep-nhan": "da-dong",
  "chuyen-cap-tren": "da-dong",
};

/**
 * The group of one of the nine codes, or `null` for a code this app does not know.
 *
 * `null`, NOT a group: the contract types `status` as a bare string, so a tenth status can arrive before
 * this app is updated. Guessing a group for it tells the citizen something false — "Đã đóng" on a ticket
 * that is still open, or "Đã tiếp nhận" (the prototype's fallback) on one already closed. The shared app
 * shows a neutral sentence instead (`statusLabel`, `STATUS_UNLABELLED`).
 */
export function groupOf(code: string): StatusGroup | null {
  return Object.prototype.hasOwnProperty.call(GROUP_OF_CODE, code) ? GROUP_OF_CODE[code]! : null;
}

export const STATUS_GROUP_LABEL: Readonly<Record<StatusGroup, string>> = {
  "da-tiep-nhan": "Đã tiếp nhận",
  "dang-xu-ly": "Đang xử lý",
  "da-xu-ly-xong": "Đã xử lý xong",
  "da-dong": "Đã đóng",
};

/** Per-step labels on a timeline — the prototype's wording (`feedback-adapter.ts:91-101`). */
export const STEP_LABEL: Readonly<Record<string, string>> = {
  "da-tiep-nhan": "Đã tiếp nhận",
  "dang-phan-loai": "Đang phân loại",
  "da-chuyen-xu-ly": "Đã chuyển bộ phận xử lý",
  "dang-xu-ly": "Đang xử lý",
  "da-xu-ly": "Đã xử lý xong",
  "cho-dan-xac-nhan": "Chờ bà con xác nhận",
  "da-dong": "Đã đóng",
  "khong-tiep-nhan": "Không tiếp nhận",
  "chuyen-cap-tren": "Chuyển cấp trên",
};
