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
 * One sentence per kind the unit still holds, in the order a person must work through them — people
 * and sub-units first, then records. The first two are the prototype's own refusals
 * (`OrgChart.tsx:142-147`, "Bộ phận còn cán bộ. Hãy chuyển cán bộ sang bộ phận khác trước."), with the
 * server's count added; the nouns are the server's (`OrgUnitHoldings.Sentence`,
 * `service-identity/internal/domain/org_unit_delete.go`), so the dialog never names one thing two ways.
 *
 * Built from the 409's `holdings` (the STATUS and the SHAPE of the body), never from its `code`.
 */
export function holdingSentences(h: identity_orgUnitHoldingsOut): readonly string[] {
  const out: string[] = [];
  if (h.staff > 0) {
    out.push(`Bộ phận còn ${h.staff} cán bộ trực thuộc. Hãy chuyển cán bộ sang bộ phận khác trước.`);
  }
  if (h.child_units > 0) {
    out.push(`Bộ phận còn ${h.child_units} bộ phận con. Hãy xoá hoặc chuyển bộ phận con trước.`);
  }
  if (h.open_petitions > 0) {
    out.push(`Bộ phận còn ${h.open_petitions} phản ánh chưa xử lý xong. Hãy chuyển sang bộ phận khác trước.`);
  }
  if (h.open_tasks > 0) {
    out.push(`Bộ phận còn ${h.open_tasks} nhiệm vụ chưa hoàn thành. Hãy chuyển sang bộ phận khác trước.`);
  }
  if (h.open_incoming_documents > 0) {
    out.push(
      `Bộ phận còn ${h.open_incoming_documents} văn bản đến chưa xử lý xong. Hãy chuyển sang bộ phận khác trước.`,
    );
  }
  return out;
}

// No emoji: the screen draws a lucide `Trash2` beside the word (ADR 0068 §2).
export const DELETE_BUTTON = "Xoá";
export const DELETE_CONFIRM_BUTTON = "Xoá";
export const DELETE_CANCEL_BUTTON = "Huỷ";

export function deleteButtonLabel(unitName: string): string {
  return `Xoá bộ phận ${unitName}`;
}

/** The dialog's question (spec v2 §7: a confirm's title is the concrete question). */
export function deleteFormTitle(unitName: string): string {
  return `Xoá bộ phận ${unitName}?`;
}

/**
 * Said before the click: what happens and what blocks it — the prototype's rule (`OrgChart.tsx:46`),
 * so a refusal does not read as a fault. NO REASON BOX (bug sheet row 30, owner decision 10/10/2026):
 * the server records a fixed sentence in the trail.
 */
export const DELETE_EXPLANATION =
  "Bộ phận bị xoá sẽ không còn trên sơ đồ, nhưng hồ sơ cũ vẫn giữ tên bộ phận này. Không xoá được " +
  "khi bộ phận còn cán bộ, bộ phận con hoặc hồ sơ đang xử lý.";

export function deletedSentence(unitName: string): string {
  return `Đã xoá bộ phận ${unitName}.`;
}
