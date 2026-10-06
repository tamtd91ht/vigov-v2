"use client";

import { Banknote, Hourglass, TriangleAlert, Wallet } from "lucide-react";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { DATA_TABLE_CLASS, TableScroll } from "@/components/ui/data-table";
import { ErrorState } from "@/components/ui/error-state";
import { Skeleton } from "@/components/ui/skeleton";
import { StatCard } from "@/components/ui/stat-card";
import type { finance_categoryProgressOut, finance_projectSummaryOut } from "@/lib/api/schema.gen";
import { cn } from "@/lib/cn";

import { CumulativeChart } from "./cumulative-chart";
import { nhanNgay, nhanNguongCham, nhanTien, nhanTyLeGiaiNgan, percentLabel } from "./nhan-du-an";
import { AttentionIssuesPending } from "./pending-parts";
import { categoryRowLabel } from "./project-groups";

/**
 * The year block of the Giải ngân list, spec §3–§5, in the prototype's order
 * (`BudgetWorkspace.tsx:198-251`): four KPI cards, the cumulative chart, the per-category table.
 *
 * EVERY FIGURE IS `GET /api/v1/investment-project-summary`'s. Nothing here adds money up: the list
 * below may be filtered, and a total summed from a filtered list is a total that is simply too small.
 * Ratios are hundredths of a percent, never clamped; `null` (no plan) is said in words, never `0%`;
 * a negative remainder is shown negative — an over-disbursement must be visible (§13 rule 2).
 */

export type SummaryState =
  | { phase: "loading" }
  | { phase: "error"; message: string }
  | { phase: "ready"; summary: finance_projectSummaryOut };

export function DisbursementOverview({
  state,
  selectedCategoryId,
  onSelectCategory,
  onRetry,
}: {
  state: SummaryState;
  /** The list's category filter; its row is drawn selected. "" = no filter. */
  selectedCategoryId: string;
  /** Called with the row's `category_id` — or "" when the selected row is pressed again. */
  onSelectCategory: (categoryId: string) => void;
  onRetry: () => void;
}) {
  if (state.phase === "loading") {
    return (
      <div className="mb-5">
        <p role="status" className="an-thi-giac">
          Đang tải số liệu tổng hợp của năm…
        </p>
        <div aria-hidden="true" className="flex flex-col gap-3">
          <Skeleton className="h-28 w-full" />
          <Skeleton className="h-64 w-full" />
        </div>
      </div>
    );
  }
  if (state.phase === "error") {
    return (
      <Card className="mb-5">
        <ErrorState
          title="Chưa tải được số liệu tổng hợp của năm"
          message={<span role="alert">{state.message}</span>}
          onRetry={onRetry}
        />
      </Card>
    );
  }
  const s = state.summary;
  return (
    <>
      <KpiCards summary={s} />
      <Card as="section" aria-labelledby="tieu-de-luy-ke" className="mb-4">
        <CardHeader>
          <CardTitle id="tieu-de-luy-ke">Luỹ kế giải ngân so với kế hoạch</CardTitle>
        </CardHeader>
        <CardContent>
          <CumulativeChart
            points={s.monthly}
            caption={`Luỹ kế giải ngân năm ${s.year} so với kế hoạch, theo tháng`}
            emptyText={`Năm ngân sách ${s.year} chưa có kế hoạch vốn hay khoản chi nào để vẽ.`}
          />
          <AfterYearNote amount={s.disbursed_after_year} year={s.year} />
        </CardContent>
      </Card>
      <CategoryProgressTable
        year={s.year}
        rows={s.by_category}
        total={s.total}
        selectedCategoryId={selectedCategoryId}
        onSelectCategory={onSelectCategory}
      />
    </>
  );
}

/**
 * Whether the year as a whole is behind: §3's own formula applied to the year totals — time elapsed
 * minus disbursed ratio, STRICTLY above the commune's threshold, as `domain.LaCham` does per project.
 * All three operands come from the SAME response, so no browser clock is read; a year with no plan
 * (`null` ratio) is not behind, as a project with no plan is not.
 */
export function yearIsBehind(s: finance_projectSummaryOut): boolean {
  if (s.disbursed_ratio === null || !Number.isFinite(s.disbursed_ratio)) return false;
  return s.time_elapsed_ratio - s.disbursed_ratio > s.delay_threshold;
}

/** §3 — four cards, prototype `BudgetWorkspace.tsx:201-227`. */
export function KpiCards({ summary: s }: { summary: finance_projectSummaryOut }) {
  const behind = yearIsBehind(s);
  const ratio = s.disbursed_ratio === null ? nhanTyLeGiaiNgan(null) : `${nhanTyLeGiaiNgan(s.disbursed_ratio)} kế hoạch`;
  return (
    <div className="mb-5 grid min-w-0 gap-3 sm:grid-cols-2 xl:grid-cols-4" data-kpi-cards="">
      <StatCard icon={Wallet} label="Kế hoạch vốn năm" value={nhanTien(s.planned_total)} caption={`${s.project_count} dự án`} />
      <StatCard
        icon={Banknote}
        tone={behind ? "danger" : "success"}
        label="Đã giải ngân"
        value={nhanTien(s.disbursed_total)}
        alert={behind}
        caption={`${ratio} · thời gian đã qua ${percentLabel(s.time_elapsed_ratio)}`}
      />
      <StatCard
        icon={Hourglass}
        tone="warning"
        label="Còn phải giải ngân"
        value={nhanTien(s.remaining_total)}
        caption={nhanNguongCham(s.delay_threshold)}
      />
      <StatCard
        icon={TriangleAlert}
        tone={s.delayed_project_count > 0 ? "danger" : "neutral"}
        label="Cần chú ý"
        value={`${s.delayed_project_count} dự án chậm`}
        alert={s.delayed_project_count > 0}
        caption={<AttentionIssuesPending />}
      />
    </div>
  );
}

/**
 * Paid after 31/12 for this year's projects: in `Đã giải ngân` but on no month of the curve, so
 * December can sit below the card. Said, so the gap is not read as an error. Usually 0 → nothing.
 */
export function AfterYearNote({ amount, year }: { amount: number; year: number }) {
  if (!(amount > 0)) return null;
  return (
    <p className="m-0 mt-2 text-xs text-ink-500" data-after-year="">
      Có thêm {nhanTien(amount)} chi sau ngày 31/12/{year}: đã tính vào số đã giải ngân nhưng không nằm
      trên tháng nào của biểu đồ.
    </p>
  );
}

/**
 * §5 table, `Tổng cộng` bold. Pressing a row sets the project list's category filter (prototype
 * `CategoryReportPanel`, `onSelect`); pressing the selected row again clears it.
 *
 * A row with no `category_id` (projects with no category) cannot filter — the list route reads an
 * empty `category` as no filter at all — so it is not a button. The `Tỷ lệ` columns are not
 * coloured: §5 says "màu theo ngưỡng" but no threshold for a category exists, and inventing one here
 * would colour a public report by a rule nobody set.
 */
export function CategoryProgressTable({
  year,
  rows,
  total,
  selectedCategoryId,
  onSelectCategory,
}: {
  year: number;
  rows: readonly finance_categoryProgressOut[];
  total: finance_categoryProgressOut;
  selectedCategoryId: string;
  onSelectCategory: (categoryId: string) => void;
}) {
  return (
    <Card as="section" aria-labelledby="tieu-de-tien-do-hang-muc" className="mb-5">
      <CardHeader>
        <CardTitle id="tieu-de-tien-do-hang-muc">Tiến độ theo hạng mục</CardTitle>
      </CardHeader>
      {rows.length === 0 ? (
        <CardContent className="text-sm text-ink-500">
          Năm ngân sách {year} chưa có hạng mục nào đang dùng hay dự án nào.
        </CardContent>
      ) : (
        <>
          <TableScroll aria-label={`Tiến độ giải ngân theo hạng mục năm ${year}`}>
            <table className={DATA_TABLE_CLASS} data-category-table="">
              <caption className="an-thi-giac">Tiến độ giải ngân theo hạng mục, năm ngân sách {year}</caption>
              <thead>
                <tr>
                  <th scope="col" className="min-w-56">
                    Hạng mục
                  </th>
                  <th scope="col" className="text-right">
                    Số DA
                  </th>
                  <th scope="col" className="text-right">
                    KH vốn năm
                  </th>
                  <th scope="col" className="text-right">
                    Đã giải ngân
                  </th>
                  <th scope="col" className="text-right">
                    Tỷ lệ
                  </th>
                  <th scope="col" className="text-right">
                    Chưa giải ngân
                  </th>
                  <th scope="col" className="text-right">
                    Tỷ lệ
                  </th>
                  <th scope="col">Thời hạn giải ngân</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((r, i) => {
                  const id = r.category_id ?? "";
                  const selectable = id !== "";
                  const selected = selectable && id === selectedCategoryId;
                  const label = categoryRowLabel(r);
                  return (
                    <tr
                      key={id === "" ? `none-${i}` : id}
                      data-category-row={id}
                      className={cn(selectable && "cursor-pointer", selected && "bg-brand-50")}
                      onClick={selectable ? () => onSelectCategory(selected ? "" : id) : undefined}
                    >
                      <td className="whitespace-normal">
                        {selectable ? (
                          <button
                            type="button"
                            aria-pressed={selected}
                            className="cursor-pointer border-0 bg-transparent p-0 text-left [font-family:inherit] text-sm font-semibold text-ink-900 hover:underline focus-visible:outline-2 focus-visible:outline-brand-500"
                            // The row's own click does the work; stopping here keeps it from running twice.
                            onClick={(e) => {
                              e.stopPropagation();
                              onSelectCategory(selected ? "" : id);
                            }}
                          >
                            {label}
                          </button>
                        ) : (
                          <span className="font-semibold text-ink-700">{label}</span>
                        )}
                      </td>
                      <CategoryFigures row={r} />
                    </tr>
                  );
                })}
                <tr className="font-bold" data-category-total="">
                  <th scope="row" className="text-left font-bold">
                    Tổng cộng
                  </th>
                  <CategoryFigures row={total} />
                </tr>
              </tbody>
            </table>
          </TableScroll>
          <p className="m-0 px-4 py-3 text-xs text-ink-500">Chạm một hạng mục để lọc danh sách dự án bên dưới.</p>
        </>
      )}
    </Card>
  );
}

function CategoryFigures({ row }: { row: finance_categoryProgressOut }) {
  return (
    <>
      <td className="text-right tabular-nums">{row.project_count}</td>
      <td className="text-right tabular-nums">{nhanTien(row.planned)}</td>
      <td className="text-right tabular-nums">{nhanTien(row.disbursed)}</td>
      <td className="text-right tabular-nums">{ratioCell(row.disbursed_ratio)}</td>
      <td className="text-right tabular-nums">{nhanTien(row.undisbursed)}</td>
      <td className="text-right tabular-nums">{ratioCell(row.undisbursed_ratio)}</td>
      <td className="tabular-nums">
        {row.disbursement_deadline === undefined || row.disbursement_deadline === ""
          ? "—"
          : nhanNgay(row.disbursement_deadline)}
      </td>
    </>
  );
}

/** `null` = no plan in the row: "—", never `0%` (and not the long "Chưa bố trí vốn" in a narrow cell). */
function ratioCell(hundredths: number | null): string {
  return hundredths === null ? "—" : nhanTyLeGiaiNgan(hundredths);
}
