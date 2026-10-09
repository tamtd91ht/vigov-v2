import type { PendingFeatureInfo } from "@/components/ui/pending-feature";

/**
 * The parts of `docs/ui-ux/13-bao-cao.md` that `/bao-cao` does NOT build this round and that are not
 * already declared by `features/dashboard/labels.ts` (the reused KPI blocks' placeholders live there,
 * once). Drawn at their spec position as disabled controls with the "?" of ADR 0068 §14; pressing "?"
 * reads the entry here. `features/dashboard/view.tsx` reads "Tổng chi" from here: the tile is drawn
 * only under `layout="report"`, so it belongs to this page's list, not Tổng quan's.
 *
 * BUILT ON 09/10/2026 and therefore gone from this list (owner, ADR 0053 §Sửa đổi 09/10/2026 lần 2):
 * "So sánh với kỳ trước" (the hand-drawn chart, `comparison-chart.tsx`) and "Xuất báo cáo PDF, XLSX,
 * PPTX" (Tổng quan's browser-side builders reused, `report-export-actions.tsx`). The scheduled report
 * job and the "Thành lập mới" indicator have no position on this page, so they get no placeholder here.
 *
 * WHY THE CONSTANT KEEPS ITS VIETNAMESE NAME: `tools/tien_do_san_pham.py` finds a screen's unbuilt
 * parts by the `PHAN_CHUA_DUNG` declaration in any `features/<dir>/*.ts`.
 *
 * EVERY ENTRY MUST BE TRUE ON THE DAY IT IS READ. Build a part → delete its entry in the same change.
 */
// vi-name-ok: the progress tool (`tools/tien_do_san_pham.py`) matches this exact constant name
export const PHAN_CHUA_DUNG: readonly PendingFeatureInfo[] = [
  // The prototype's fiscal block has "Tổng chi" between "Chi đạt dự toán" and the balance (D6). The
  // fiscal summary route (`finance_chiSoNamRa`) returns revenue totals and the balance but NO
  // expenditure total, so the tile stands at its place, "—", with this "?" — never a number derived
  // in the browser from the balance (a second computation of a finance figure).
  {
    ten: "Tổng chi",
    viSao:
      "Tổng số đã chi luỹ kế năm của xã. Chỉ số thu – chi năm hiện chưa trả về con số này (mới có tỷ " +
      "lệ chi đạt dự toán và chênh lệch thu – chi), nên ô để trống.",
  },
];

/** One entry by its `ten`. THROWS on an unknown name — a "?" with no description is never drawn. */
export function reportPendingPart(ten: string): PendingFeatureInfo {
  const found = PHAN_CHUA_DUNG.find((p) => p.ten === ten);
  if (found === undefined) throw new Error(`PHAN_CHUA_DUNG has no entry "${ten}"`);
  return found;
}
