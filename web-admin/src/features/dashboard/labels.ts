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
  {
    ten: "Đơn thư trong kỳ",
    viSao:
      "Số đơn thư công dân vào sổ trong kỳ. Hệ thống chưa có sổ đơn thư, nên chưa có gì để đếm.",
  },
  // No finance route returns the commune-wide disbursement aggregates (rate, elapsed share of the
  // budget year, delayed projects, open issues, amount disbursed); issues are not recorded at all.
  {
    ten: "Giải ngân ngân sách",
    viSao:
      "Năm chỉ số của cả xã: tỷ lệ giải ngân, thời gian đã trôi qua của năm ngân sách, số dự án " +
      "chậm, vướng mắc chưa gỡ và số tiền đã giải ngân. Hệ thống chưa tính các số tổng hợp ấy.",
  },
  {
    ten: "Kinh tế & Tài nguyên",
    viSao:
      "Số doanh nghiệp, hộ kinh doanh, cơ sở thành lập mới trong kỳ và tổng tài nguyên trên địa " +
      "bàn, đếm từ Bản đồ kinh tế số. Bản đồ đã có, nhưng phép đếm các số này cho màn Tổng quan " +
      "chưa được làm.",
  },
  {
    ten: "Xuất báo cáo PDF, XLSX, PPTX",
    viSao: "Xuất số liệu của kỳ đang chọn ra tệp PDF, Excel hoặc PowerPoint để gửi lên cấp trên.",
  },
  {
    ten: "Chế độ trình chiếu phòng họp",
    viSao:
      "Hiện màn Tổng quan toàn màn hình cho phòng họp giao ban: ẩn thanh bên và đầu trang, chữ lớn, " +
      "nền tối, chuyển khối bằng phím.",
  },
];

/**
 * The tiles an unbuilt BLOCK will have, drawn as "—" under the block's one "?" so the block keeps
 * its shape and place in the grid (prototype `DashboardWorkspace`, owner 05/10/2026). The labels are
 * the prototype's metric names (`report-display.ts` METRIC_LABELS) — the same figures each entry's
 * `viSao` above lists in words. Keyed by the entry's `ten`: build the block → delete both together.
 */
export const PENDING_BLOCK_METRICS = {
  "Giải ngân ngân sách": [
    "Tỷ lệ giải ngân",
    "Thời gian đã trôi qua",
    "Dự án chậm",
    "Vướng mắc chưa gỡ",
    "Đã giải ngân",
  ],
  "Kinh tế & Tài nguyên": ["Doanh nghiệp", "Hộ kinh doanh", "Thành lập mới", "Tổng tài nguyên"],
} as const satisfies Readonly<Record<string, readonly string[]>>;

/**
 * One entry by its `ten`. THROWS on an unknown name rather than drawing a "?" with no description:
 * the overview's tests render every placeholder, so a renamed entry turns red there, not on a screen.
 */
export function pendingPart(ten: string): PendingFeatureInfo {
  const found = PHAN_CHUA_DUNG.find((p) => p.ten === ten);
  if (found === undefined) throw new Error(`PHAN_CHUA_DUNG has no entry "${ten}"`);
  return found;
}
