/**
 * A sum of đồng for a TILE, short like the prototype's "4.317 tỷ": "9,64 tỷ đồng", "690 triệu đồng".
 *
 * Only tiles are shortened — the exact amount stays on hover (the tile's `title`) and in the Thu – Chi
 * register, which is where a figure is checked. Under a million it is written in full.
 *
 * SHARED BY THE DASHBOARD AND THE THU – CHI CARDS (06/10/2026): a tester screenshot showed
 * "690.000.000 đồn" clipped and "9.640.000.000 / đồng" broken over two lines in a tile. Two copies of
 * the rounding rule would let the same sum read differently on two screens.
 */
export function compactDong(amount: number): string {
  const sign = amount < 0 ? "-" : "";
  const abs = Math.abs(amount);
  const fmt = (n: number, digits: number) =>
    new Intl.NumberFormat("vi-VN", { maximumFractionDigits: digits }).format(n);
  if (abs >= 1e9) return `${sign}${fmt(abs / 1e9, 2)} tỷ đồng`;
  if (abs >= 1e6) return `${sign}${fmt(abs / 1e6, 1)} triệu đồng`;
  return `${sign}${fmt(abs, 0)} đồng`;
}

const VI_GROUPED = new Intl.NumberFormat("vi-VN", { maximumFractionDigits: 0 });

/**
 * `/bao-cao`'s sum of đồng — the prototype's `formatDongShort` (`report-display.ts`), verbatim in
 * effect: "4.317 tỷ" from a thousand tỷ up, "3,4 tỷ", "690 triệu", and no "đồng". Owner decision
 * 09/10/2026 (ADR 0053 §Sửa đổi 09/10/2026 lần 2, D6) for the REPORT ONLY.
 *
 * A SEPARATE FUNCTION, NOT A CHANGE TO `compactDong`: Tổng quan and the Thu – Chi cards keep their
 * two-decimal "9,64 tỷ đồng". Changing the shared one would move figures on two screens that were not
 * asked to change. The exact amount stays on hover (the tile's `title`) on both.
 */
export function compactDongReport(amount: number): string {
  const abs = Math.abs(amount);
  // From a thousand tỷ no decimal: a tenth of a tỷ decides nothing at that size, and "4316,8 tỷ" is
  // wider than its tile.
  if (abs >= 1e12) return `${VI_GROUPED.format(Math.round(amount / 1e9))} tỷ`;
  if (abs >= 1e9) return `${(amount / 1e9).toFixed(1).replace(".", ",")} tỷ`;
  if (abs >= 1e6) return `${(amount / 1e6).toFixed(0)} triệu`;
  return VI_GROUPED.format(Math.round(amount));
}
