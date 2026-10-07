import type { finance_duAnRa } from "@/lib/api/schema.gen";

/**
 * The sortable headers of the project list (user decision 07/10/2026, prototype
 * `BudgetItemTable.tsx:51-87`): `KH vốn năm`, `Đã giải ngân`, `Tiến độ`. ONE direction, largest first —
 * the reader is looking for the project that spends most or lags most; pressing again returns to `code`.
 *
 * `code` IS THE SERVER'S ORDER: the list route sorts by project code (`ORDER BY da.ma`,
 * `service-finance/internal/store/du_an.go`), so returning to it is returning the rows as received.
 *
 * SORTED IN THE BROWSER, on the list already loaded: the route returns the whole year or refuses (no
 * pagination, `lib/api/du-an.ts`), so the order of every row is known here. Sorting sums nothing.
 */
export type ProjectSort = "code" | "planned_amount" | "disbursed_amount" | "disbursed_ratio";

export function sortProjects(items: readonly finance_duAnRa[], sort: ProjectSort): finance_duAnRa[] {
  if (sort === "code") return [...items];
  // `Array.prototype.sort` is stable: equal figures keep code order.
  return [...items].sort((a, b) => {
    const x = a[sort];
    const y = b[sort];
    // `null` ratio = no plan, no denominator: not a 0%, so it is not ranked among figures — last.
    if (x === null) return y === null ? 0 : 1;
    if (y === null) return -1;
    return y - x;
  });
}
