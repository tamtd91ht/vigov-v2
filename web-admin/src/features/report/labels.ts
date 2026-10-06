import type { PendingFeatureInfo } from "@/components/ui/pending-feature";

/**
 * The parts of `docs/ui-ux/13-bao-cao.md` that `/bao-cao` does NOT build this round and that are not
 * already declared by `features/dashboard/labels.ts` (the reused KPI blocks' placeholders live there,
 * once). Drawn at their spec position as disabled controls with the "?" of ADR 0068 §14; pressing "?"
 * reads the entry here.
 *
 * THE EXPORT ENTRY MOVED HERE on 06/10/2026: Tổng quan's export was built (user decision, browser-side
 * files from the figures on screen), Báo cáo's was not — so the "?" and its reason now belong to this
 * page alone, and Tổng quan's registry no longer lists it.
 *
 * Out of this round by the owner's decision (ADR 0053, amendment 04/10/2026, B4): the comparison
 * chart, the export, the scheduled report job and the "Thành lập mới" indicator. The job and the
 * indicator have no position on this page, so they get no placeholder here.
 *
 * WHY THE CONSTANT KEEPS ITS VIETNAMESE NAME: `tools/tien_do_san_pham.py` finds a screen's unbuilt
 * parts by the `PHAN_CHUA_DUNG` declaration in any `features/<dir>/*.ts`.
 *
 * EVERY ENTRY MUST BE TRUE ON THE DAY IT IS READ. Build a part → delete its entry in the same change.
 */
// vi-name-ok: the progress tool (`tools/tien_do_san_pham.py`) matches this exact constant name
export const PHAN_CHUA_DUNG: readonly PendingFeatureInfo[] = [
  {
    ten: "So sánh với kỳ trước",
    viSao:
      "Biểu đồ thanh ngang so từng chỉ tiêu của kỳ đang chọn với kỳ so sánh, theo phần trăm tăng " +
      "hoặc giảm. Chưa dựng trong đợt này; từng ô số liệu phía trên đã ghi số của kỳ trước.",
  },
  {
    ten: "Xuất báo cáo PDF, XLSX, PPTX",
    viSao: "Xuất số liệu của kỳ đang chọn ra tệp PDF, Excel hoặc PowerPoint để gửi lên cấp trên.",
  },
];

/** One entry by its `ten`. THROWS on an unknown name — a "?" with no description is never drawn. */
export function reportPendingPart(ten: string): PendingFeatureInfo {
  const found = PHAN_CHUA_DUNG.find((p) => p.ten === ten);
  if (found === undefined) throw new Error(`PHAN_CHUA_DUNG has no entry "${ten}"`);
  return found;
}
