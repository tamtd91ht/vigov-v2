/**
 * Sổ theo dõi (§4.3) — the third view mode: the register as the commune office's printed book of
 * directive documents. Pure half: the column order, the per-row document split, the words.
 *
 * THE ROWS ARE THE LIST'S ROWS (`GET /api/v1/tasks`, same filters, sort and paging) READ WITH
 * `include=documents` (90d12ff). The export (`GET /api/v1/tasks/register-export`) writes the same
 * columns in the same order on the server — "Xuất Excel của bảng này phải giữ đúng thứ tự cột" (§4.3,
 * `docs/ui-ux/02-nhiem-vu.md:128`) — so this file's order and the file's order are one spec, pinned in
 * a test against the spec lines, not copied from the server.
 */

import type { petitions_nhiemVuRa, petitions_nhiemVuVanBanRa } from "@/lib/api/schema.gen";

import {
  MOI_NHOM_VAN_BAN,
  nhanNhomVanBan,
  type NhomVanBan, // vi-name-ok: existing type, imported not declared (rule 12 inv 3)
} from "./nhan-nhiem-vu";

/** View-mode button — verbatim §2. */
export const REGISTER_VIEW_LABEL = "▤ Sổ theo dõi";

/**
 * §4.3's columns IN ORDER (`02-nhiem-vu.md:117-126`), after the `☐` column (drawn only with
 * `task.delete`, like the list). The three document columns take their labels from the ONE source of
 * group labels, `nhanNhomVanBan`, so a header cannot disagree with the drawer's §5.4 block.
 *
 * ELEVEN COLUMNS, not thirteen (ADR 0065 NV5, user decision 30/09/2026): "Cơ quan chủ trì tham mưu"
 * and "Chuyên viên VP tham mưu / theo dõi" ARE the unit and the assignee, both printed in "Đơn vị
 * thực hiện" — the same order the server's export writes (`task_register_export.go`).
 */
export const REGISTER_COLUMNS: readonly string[] = [
  "Mã",
  "Nội dung nhiệm vụ / Trích yếu văn bản",
  "Đơn vị thực hiện",
  ...MOI_NHOM_VAN_BAN.map(nhanNhomVanBan),
  "Hạn xử lý",
  "Tóm tắt kết quả",
  "Ghi chú",
  "Lãnh đạo xã đã phê duyệt hoàn thành",
  "Cấp trên đã công nhận hoàn thành",
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
export const APPROVAL_TICKED = "Đã đánh dấu";
export const APPROVAL_UNTICKED = "Chưa đánh dấu";

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
