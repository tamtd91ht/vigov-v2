/**
 * Spec §5 row names and §7.2 "Gộp theo hạng mục" groups. Pure: no network, no DOM, no clock.
 *
 * GROUP TOTALS ARE THE SERVER'S `by_category` FIGURES, NEVER A BROWSER SUM (rule of `bang-du-an.tsx`):
 * two places adding the same money up are two places that can disagree, and the header is read as
 * the category's total. A server total describes the WHOLE category of the year, so it is printed
 * only while the rows under it are that whole category:
 *
 *   - no search keyword and no `Chỉ dự án chậm` (both leave a subset of the category on screen);
 *   - the summary is of the list's own year;
 *   - the summary's `project_count` equals the number of rows in the group — the two reads are
 *     separate requests, and a project added between them would put a total over rows it does not
 *     describe.
 *
 * Otherwise the header carries the COUNT OF ROWS SHOWN only, worded so it is not read as the
 * category's size when a filter is on ("N dự án khớp bộ lọc"). Counting visible rows is not summing
 * money; there is no figure on it that could be reported upward.
 */

import type { finance_categoryProgressOut, finance_duAnRa, finance_hangMucRa } from "@/lib/api/schema.gen";

import { hangMucDuAn, nhanHangMuc, nhanTien } from "./nhan-du-an";

/**
 * Name of a §5 row. The server leaves `label` empty on a row whose projects point at a category no
 * longer in the catalogue (`in_catalogue: false`) and on the row of projects with no category; it says
 * the screen names those rows itself — they are shown so the rows add up to `Tổng cộng`.
 */
export function categoryRowLabel(row: finance_categoryProgressOut): string {
  if (row.label !== undefined && row.label !== "") return row.label;
  if (row.category_id === undefined || row.category_id === "") return "Chưa gắn hạng mục";
  return "Hạng mục không còn trong danh mục";
}

export type ProjectGroup = {
  /** `category_id` of the rows ("" = no category). */
  readonly key: string;
  readonly title: string;
  /** "19 dự án · kế hoạch … · đã giải ngân …", or the row count alone — see the file comment. */
  readonly detail: string;
  readonly items: readonly finance_duAnRa[];
};

export function groupProjects(
  items: readonly finance_duAnRa[],
  {
    byCategory,
    danhMuc,
    totalsApply,
    filtered,
  }: {
    /** The summary's `by_category`; `null` while it is loading or failed — counts only then. */
    byCategory: readonly finance_categoryProgressOut[] | null;
    danhMuc: readonly finance_hangMucRa[];
    /** No keyword, no `delayed_only`, summary of the same year. */
    totalsApply: boolean;
    /** A keyword or `delayed_only` is on: the count is of matching rows, not of the category. */
    filtered: boolean;
  },
): ProjectGroup[] {
  const buckets = new Map<string, finance_duAnRa[]>();
  for (const d of items) {
    const bucket = buckets.get(d.category_id);
    if (bucket === undefined) buckets.set(d.category_id, [d]);
    else bucket.push(d);
  }

  const rowOf = (id: string) => byCategory?.find((r) => (r.category_id ?? "") === id);

  // The summary's order (the catalogue's own), then any category it does not list, as first met.
  const order: string[] = [];
  for (const r of byCategory ?? []) {
    const id = r.category_id ?? "";
    if (buckets.has(id) && !order.includes(id)) order.push(id);
  }
  for (const id of buckets.keys()) if (!order.includes(id)) order.push(id);

  return order.map((key) => {
    const rows = buckets.get(key)!;
    const row = rowOf(key);
    const title = row !== undefined ? categoryRowLabel(row) : nhanHangMuc(hangMucDuAn(key, danhMuc));
    const detail =
      totalsApply && row !== undefined && row.project_count === rows.length
        ? `${row.project_count} dự án · kế hoạch ${nhanTien(row.planned)} · đã giải ngân ${nhanTien(row.disbursed)}`
        : filtered
          ? `${rows.length} dự án khớp bộ lọc`
          : `${rows.length} dự án`;
    return { key, title, detail, items: rows };
  });
}
