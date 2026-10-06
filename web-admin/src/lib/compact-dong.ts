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
