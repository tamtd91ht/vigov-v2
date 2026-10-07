/**
 * Sổ theo dõi (§4.3) — the third view mode: the register as the commune office's printed book of
 * directive documents. Pure half: the column order, the per-row document split, the words.
 *
 * THE ROWS ARE THE LIST'S ROWS (`GET /api/v1/tasks`, same filters, sort and paging) READ WITH
 * `include=documents` (90d12ff), the type forced to `Theo văn bản` (spec 02 §State). The export
 * (`GET /api/v1/tasks/register-export`) writes ITS OWN columns on the server — since 07/10/2026 the
 * screen draws the prototype's thirteen and the file keeps the server's set (see `REGISTER_COLUMNS`).
 */

import type { petitions_nhiemVuRa, petitions_nhiemVuVanBanRa } from "@/lib/api/schema.gen";

import {
  MOI_NHOM_VAN_BAN,
  nhanNhomVanBan,
  type NhomVanBan, // vi-name-ok: existing type, imported not declared (rule 12 inv 3)
} from "./nhan-nhiem-vu";

/** View-mode button — verbatim §2; its `▤` glyph is a lucide icon beside the word (ADR 0068). */
export const REGISTER_VIEW_LABEL = "Sổ theo dõi";

/**
 * The Sổ theo dõi's columns IN ORDER — spec 05, THIRTEEN with the `☐` column (drawn only with
 * `task.delete`): label and the prototype's width (`TaskRegisterTable.tsx:70-103`).
 *
 * ĐỔI CHIỀU CÓ CHỦ Ý 07/10/2026 (owner, ADR 0076 lần 2 #8): eleven columns became the prototype's
 * thirteen. "Cơ quan chủ trì tham mưu" and "Chuyên viên VP tham mưu / theo dõi" are drawn again — and
 * they show the SAME data as "Đơn vị thực hiện" (unit) and its assignee line (ADR 0065 NV5: one role,
 * one field; display only, no data-model change). The EXCEL EXPORT is unchanged: its columns are the
 * server's (`task_register_export.go`), so the file and the screen no longer share one column list.
 *
 * The two document columns take their labels from the ONE source of group labels, `nhanNhomVanBan`,
 * so a header cannot disagree with the drawer's §5.4 block.
 */
export const REGISTER_COLUMNS: readonly { readonly label: string; readonly width: string }[] = [
  { label: "Mã", width: "w-16" },
  { label: "Nội dung nhiệm vụ / Trích yếu văn bản", width: "w-96" },
  { label: "Cơ quan chủ trì tham mưu", width: "w-40" },
  { label: "Chuyên viên VP tham mưu / theo dõi", width: "w-36" },
  { label: "Đơn vị thực hiện", width: "w-40" },
  { label: nhanNhomVanBan("cap-tren-giao"), width: "w-64" },
  { label: nhanNhomVanBan("chi-dao-dang-uy"), width: "w-64" },
  { label: "Hạn hoàn thành", width: "w-28" },
  { label: "Trạng thái", width: "w-28" },
  { label: "Kết quả thực hiện / Sản phẩm đầu ra", width: "w-64" },
  { label: "Lãnh đạo phê duyệt", width: "w-28 text-center" },
  { label: "Ghi chú", width: "w-56" },
];

/** One row's documents, split into §4.3's three columns. */
export type RegisterRowDocuments = {
  readonly byGroup: Readonly<Record<NhomVanBan, readonly petitions_nhiemVuVanBanRa[]>>;
  /** Documents in a group this screen does not know — not dropped silently, see `REGISTER_UNKNOWN_GROUP`. */
  readonly unknown: number;
};

/**
 * Split `documents`, or `null` when the row carries NO array — `include=documents` was asked for,
 * so an absent block is a broken promise, shown as such (`REGISTER_DOCS_MISSING`), never as three
 * empty columns that read "no documents".
 */
export function registerRowDocuments(
  task: Pick<petitions_nhiemVuRa, "documents">,
): RegisterRowDocuments | null {
  const docs = task.documents;
  if (!Array.isArray(docs)) return null;
  const byGroup = Object.fromEntries(
    MOI_NHOM_VAN_BAN.map((g) => [g, docs.filter((d) => d.group === g)]),
  ) as Record<NhomVanBan, petitions_nhiemVuVanBanRa[]>;
  const known = new Set<string>(MOI_NHOM_VAN_BAN);
  return { byGroup, unknown: docs.filter((d) => !known.has(d.group)).length };
}

export const REGISTER_DOCS_MISSING = "Máy chủ không gửi kèm văn bản của dòng này.";
export const REGISTER_UNKNOWN_GROUP =
  "Có văn bản thuộc một nhóm màn hình này chưa biết — mở nhiệm vụ để xem đủ.";
/** Accessible words of the `Lãnh đạo phê duyệt` cell's icon (spec 05: `Check` / `Minus`). */
export const APPROVAL_TICKED = "Lãnh đạo đã phê duyệt";
export const APPROVAL_UNTICKED = "Lãnh đạo chưa phê duyệt";
/** Spec 05: under the tick when the leader approved and the superior did not. */
export const SUPERIOR_NOT_YET = "Cấp trên chưa duyệt";
/* ── Xuất Excel ─────────────────────────────────────────────────────────────────────────────── */

export const REGISTER_EXPORT_BUTTON = "Xuất Excel";
export const REGISTER_EXPORT_PENDING = "Đang xuất Sổ theo dõi… Tệp tải về khi máy chủ trả lời.";
export function registerExportDoneText(fileName: string): string {
  return `Đã xuất ${fileName}.`;
}
export const REGISTER_EXPORT_REFUSED = "Chưa xuất được Sổ theo dõi:";
/**
 * Said beside the button, BEFORE the click: what the file holds, and that the export is recorded —
 * the server writes an audit entry per export (it carries staff names).
 */
export const REGISTER_EXPORT_NOTE =
  "Tệp gồm mọi nhiệm vụ khớp bộ lọc và cách sắp đang chọn — không chỉ trang đang hiện — cột đúng thứ " +
  "tự bảng này. Mỗi lần xuất được ghi vào nhật ký hệ thống.";
