/**
 * The decision half of "Xoá bộ phận" (`docs/ui-ux/14-cau-hinh.md §1, §12.4`; ADR 0056). Pure, so the
 * refusal a working screen rarely shows — "this unit still holds 3 staff and 2 open tasks" — has a
 * test that renders it.
 *
 * THE BLOCK IS THE SERVER'S. It counts staff and sub-units itself and asks petitions and documents
 * for open records; the screen never pre-checks `staff_count` to hide the button, because that
 * number is one of five and a unit with 0 staff may still hold open records elsewhere.
 */

import type { identity_orgUnitHoldingsOut } from "@/lib/api/schema.gen";

/**
 * Each non-zero count as a line, in the order a person must work through them — people and
 * sub-units first, then records. The labels are the server's own words (`OrgUnitHoldings.Sentence`,
 * `service-identity/internal/domain/org_unit_delete.go`), so the list and the sentence above it
 * never name one thing two ways.
 */
export function holdingLines(h: identity_orgUnitHoldingsOut): readonly string[] {
  const lines: string[] = [];
  const add = (n: number, label: string) => {
    if (n > 0) lines.push(`${n} ${label}`);
  };
  add(h.staff, "cán bộ");
  add(h.child_units, "bộ phận con");
  add(h.open_petitions, "phản ánh chưa xử lý xong");
  add(h.open_tasks, "nhiệm vụ chưa hoàn thành");
  add(h.open_incoming_documents, "văn bản đến chưa xử lý xong");
  return lines;
}

// No emoji: the screen draws a lucide `Trash2` beside the word (ADR 0068 §2).
export const DELETE_BUTTON = "Xoá";
export const DELETE_CONFIRM_BUTTON = "Xác nhận xoá";
export const DELETE_CANCEL_BUTTON = "Huỷ";
export const DELETE_REASON_LABEL = "Lý do xoá";
export const HOLDINGS_HEADING = "Bộ phận này còn đang giữ:";

export function deleteButtonLabel(unitName: string): string {
  return `Xoá bộ phận ${unitName}`;
}

export function deleteFormTitle(unitName: string): string {
  return `Xoá bộ phận ${unitName}`;
}

/**
 * Said before the click. What happens (soft delete, code never reused), and what blocks it — so a
 * refusal does not read as a fault.
 */
export const DELETE_EXPLANATION =
  "Bộ phận bị xoá sẽ không còn trên sơ đồ và ô chọn bộ phận, nhưng hồ sơ cũ vẫn giữ tên bộ phận này; " +
  "mã của bộ phận không được cấp lại. Hệ thống chỉ cho xoá khi bộ phận không còn cán bộ, bộ phận con " +
  "và hồ sơ đang xử lý — nếu còn, hãy chuyển sang bộ phận khác trước.";

export const DELETE_REASON_HINT =
  "Lý do được lưu lại cùng bộ phận đã xoá và là câu trả lời khi có người hỏi vì sao sơ đồ tổ chức thay đổi.";

export function deletedSentence(unitName: string): string {
  return `Đã xoá bộ phận ${unitName}.`;
}
