import type { PendingFeatureInfo } from "@/components/ui/pending-feature";

/**
 * The parts of `docs/ui-ux/01-tong-quan-dieu-hanh.md` the overview page does NOT build yet, each with
 * the reason that is true today. They are drawn at their spec position as disabled controls carrying
 * the "?" of ADR 0068 §14 (`view.tsx`, `header-actions.tsx`); pressing "?" reads the entry here.
 *
 * WHY THE CONSTANT KEEPS ITS VIETNAMESE NAME: `tools/tien_do_san_pham.py` finds a screen's unbuilt
 * parts by the `PHAN_CHUA_DUNG` declaration in any `features/<dir>/*.ts` and counts the `ten: "`
 * lines INSIDE that block. Another name and the product progress report prints "không khai" for the
 * overview while the page still shows every "?".
 *
 * DELIBERATELY ABSENT — the owner decided not to build them, so they get no placeholder (ADR 0068
 * §14, ADR 0053): `⟳ Tính lại ngay` and the "số liệu cũ hơn 10 phút" badge. The page counts live at
 * the owning service; there is no snapshot to recompute or to grow stale.
 *
 * EVERY ENTRY MUST BE TRUE ON THE DAY IT IS READ. Build a part → delete its entry in the same change.
 */
// vi-name-ok: the progress tool (`tools/tien_do_san_pham.py`) matches this exact constant name
export const PHAN_CHUA_DUNG: readonly PendingFeatureInfo[] = [
  // "Đơn thư trong kỳ" was built on 15/10/2026 from `citizen-letter-counts` (`view.tsx` `letterFigure`)
  // and left this list; so did the block "Kinh tế & Tài nguyên" (`EconomyBlock`, `map-asset-summary`),
  // except the one figure below.
  // "Giải ngân ngân sách" was built on 10/10/2026 from `investment-project-summary` (`view.tsx`
  // `BudgetBlock`, customer sheet row 6) and left this list.
  {
    // The map summary counts the register AS IT STANDS — per group, total, verified — and nothing per
    // creation date; a count of "new in the period" would need the server to count by `created_at`.
    ten: "Thành lập mới",
    viSao:
      "Số cơ sở được đưa lên Bản đồ kinh tế số trong kỳ. Số liệu bản đồ hiện chỉ đếm tổng số đang có " +
      "theo từng nhóm, chưa đếm theo ngày thêm vào, nên chưa có số của kỳ.",
  },
  // "Xuất báo cáo PDF, XLSX, PPTX" was built on 06/10/2026 (`export-actions.tsx`) and left this list;
  // `/bao-cao`'s followed on 09/10/2026 (`features/report/report-export-actions.tsx`).
  // "Chế độ trình chiếu phòng họp" was built on 07/10/2026 (`presentation.tsx`) and left it too.
];

/**
 * One entry by its `ten`. THROWS on an unknown name rather than drawing a "?" with no description:
 * the overview's tests render every placeholder, so a renamed entry turns red there, not on a screen.
 */
export function pendingPart(ten: string): PendingFeatureInfo {
  const found = PHAN_CHUA_DUNG.find((p) => p.ten === ten);
  if (found === undefined) throw new Error(`PHAN_CHUA_DUNG has no entry "${ten}"`);
  return found;
}
