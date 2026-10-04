import { canSeeBlock } from "@/features/dashboard/figures";
import { blockVisibility } from "@/features/dashboard/view";
import type { BlockVisibility } from "@/features/dashboard/view";
import { coQuyen, QUYEN_XEM_NHIEM_VU, REPORT_EXPORT_PERMISSION, REPORT_READ_PERMISSION } from "@/lib/quyen";

/**
 * What `/bao-cao` draws for a session's permissions. Pure, so the DENIED branches — the ones a
 * developer holding every key never sees — are tested as functions.
 *
 *   blocks     the six KPI groups: EXACTLY `/tong-quan`'s rule (`blockVisibility`, both keys per
 *              block). Thu – Chi needs `report.read` AND `budget.read` here (ADR 0053 B6); the
 *              server route keeps `budget.read` alone, so this pair is a UI gate only.
 *   unitTable  `report.read` AND `task.read` — the nested pair `GET /api/v1/task-unit-summary`
 *              checks (B5c), although `x-vigov-permission` in openapi shows one key.
 *   exportReport  `report.read` AND `report.export` (spec 13 §9.4).
 *
 * HIDING IS CONVENIENCE, NOT PROTECTION — every route checks its keys itself (rule 5, forbidden #1).
 */
export type ReportAccess = {
  readonly blocks: BlockVisibility;
  readonly unitTable: boolean;
  readonly exportReport: boolean;
};

export function reportAccess(permissions: readonly string[]): ReportAccess {
  return {
    blocks: blockVisibility(permissions),
    unitTable: canSeeBlock(permissions, QUYEN_XEM_NHIEM_VU),
    exportReport:
      coQuyen(permissions, REPORT_READ_PERMISSION) && coQuyen(permissions, REPORT_EXPORT_PERMISSION),
  };
}
