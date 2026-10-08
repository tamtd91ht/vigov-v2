"use client";

import { TriangleAlert } from "lucide-react";
import type { ReactNode } from "react";

import { ErrorState } from "@/components/ui/error-state";
import { Skeleton } from "@/components/ui/skeleton";
import type { finance_categoryProgressOut, finance_projectSummaryOut } from "@/lib/api/schema.gen";
import { cn } from "@/lib/cn";

import { CumulativeChart } from "./cumulative-chart";
import { PROGRESS_TEXT_CLASS, progressTone } from "./progress-tone";
import { nhanNgay, nhanNguongCham, nhanTien, nhanTyLeGiaiNgan, percentLabel } from "./nhan-du-an";
import { AttentionCaption } from "./pending-parts";
import { categoryRowLabel } from "./project-groups";
import { PANEL_CLASS, PANEL_TITLE_CLASS, TABLE_CLASS, TD_CLASS, TH_CLASS, TR_CLASS } from "./spec-classes";

/**
 * The year block of the Giải ngân list, spec 02 §3–§5, in the prototype's order
 * (`BudgetWorkspace.tsx:198-251`): four KPI cards, the cumulative chart, the per-category table.
 *
 * EVERY FIGURE IS `GET /api/v1/investment-project-summary`'s. Nothing here adds money up: the list
 * below may be filtered, and a total summed from a filtered list is a total that is simply too small.
 * Ratios are hundredths of a percent, never clamped; `null` (no plan) is said in words, never `0%`;
 * a negative remainder is shown negative — an over-disbursement must be visible (§13 rule 2).
 *
 * LOOK = spec 02 / 00 (ADR 0068 lần 6): KPI cards with no icon, each panel a white card with its 13px
 * title, and while loading EACH block keeps its frame (title included) with a skeleton of the spec's
 * height, so the page does not jump when the summary arrives.
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
  if (state.phase === "error") {
    return (
      <section className={cn(PANEL_CLASS, "mb-5")}>
        <ErrorState
          title="Chưa tải được số liệu tổng hợp của năm"
          message={<span role="alert">{state.message}</span>}
          onRetry={onRetry}
        />
      </section>
    );
  }
  if (state.phase === "loading") {
    // Spec 02 §3–§5: `Skeleton mb-5 h-28` for the cards, `h-64` inside the titled chart panel, `h-52`
    // inside the titled category panel (`CategoryReportPanel.tsx:49`).
    return (
      <>
        <p role="status" className="an-thi-giac">
          Đang tải số liệu tổng hợp của năm…
        </p>
        <Skeleton className="mb-5 h-28 w-full" />
        <Panel title="Luỹ kế giải ngân so với kế hoạch" titleId="tieu-de-luy-ke" className="mb-4">
          <Skeleton className="h-64 w-full" />
        </Panel>
        <Panel title="Tiến độ theo hạng mục" titleId="tieu-de-tien-do-hang-muc" className="mb-5">
          <Skeleton className="h-52 w-full" />
        </Panel>
      </>
    );
  }
  const s = state.summary;
  return (
    <>
      <KpiCards summary={s} />
      <Panel title="Luỹ kế giải ngân so với kế hoạch" titleId="tieu-de-luy-ke" className="mb-4">
        <CumulativeChart
          points={s.monthly}
          caption={`Luỹ kế giải ngân năm ${s.year} so với kế hoạch, theo tháng`}
          emptyText={`Năm ngân sách ${s.year} chưa có kế hoạch vốn hay khoản chi nào để vẽ.`}
        />
        <AfterYearNote amount={s.disbursed_after_year} year={s.year} />
      </Panel>
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

/** Spec 00 §4 "Card / Section": white card, 13px bold title. */
export function Panel({
  title,
  titleId,
  className,
  children,
}: {
  title: string;
  titleId: string;
  className?: string;
  children: ReactNode;
}) {
  return (
    <section aria-labelledby={titleId} className={cn(PANEL_CLASS, "min-w-0", className)}>
      <h2 id={titleId} className={PANEL_TITLE_CLASS}>
        {title}
      </h2>
      {children}
    </section>
  );
}

/**
 * One KPI card, spec 02 §3: NO ICON, uppercase 11px label, 18px bold value, 11px hint. The value takes
 * `accent` (a text-colour class) when given; the danger accent puts `TriangleAlert` before the hint —
 * icon + words, never the colour alone.
 */
function Kpi({ label, value, hint, accent }: { label: string; value: string; hint: ReactNode; accent?: string }) {
  return (
    <div className="border-line shadow-card rounded-card min-w-0 border border-solid bg-white p-4" data-kpi={label}>
      <p className="text-ink-muted m-0 text-[11px] font-semibold tracking-wide uppercase">{label}</p>
      <p className={cn("m-0 mt-1.5 text-[18px] font-bold", accent ?? "text-navy")} data-kpi-value="">
        {value}
      </p>
      <div className="text-ink-muted mt-1 flex items-start gap-1 text-[11px]">
        {accent === DANGER_ACCENT && (
          <TriangleAlert aria-hidden="true" focusable="false" className="mt-0.5 size-3 shrink-0" />
        )}
        <span className="min-w-0">{hint}</span>
      </div>
    </div>
  );
}

const DANGER_ACCENT = "text-danger";

/**
 * §3 — four cards, prototype `BudgetWorkspace.tsx:201-227`.
 *
 * `Đã giải ngân` takes the 80/50/30 colour of its ratio (spec `disbursementColor`) — a presentation
 * tier, not the late rule (`progress-tone.ts`). `Cần chú ý` is danger when the SERVER counts a late
 * project, leaf otherwise. A year with no plan (`null` ratio) has no tier and stays navy.
 */
export function KpiCards({ summary: s }: { summary: finance_projectSummaryOut }) {
  const ratio = s.disbursed_ratio;
  const known = ratio !== null && Number.isFinite(ratio);
  const ratioWords = known ? `${nhanTyLeGiaiNgan(ratio)} kế hoạch` : nhanTyLeGiaiNgan(null);
  return (
    <div className="mb-5 grid min-w-0 gap-3 sm:grid-cols-2 xl:grid-cols-4" data-kpi-cards="">
      <Kpi label="Kế hoạch vốn năm" value={nhanTien(s.planned_total)} hint={`${s.project_count} dự án`} />
      <Kpi
        label="Đã giải ngân"
        value={nhanTien(s.disbursed_total)}
        hint={`${ratioWords} · thời gian đã qua ${percentLabel(s.time_elapsed_ratio)}`}
        accent={known ? PROGRESS_TEXT_CLASS[progressTone(ratio)] : undefined}
      />
      <Kpi label="Còn phải giải ngân" value={nhanTien(s.remaining_total)} hint={nhanNguongCham(s.delay_threshold)} />
      <Kpi
        label="Cần chú ý"
        value={`${s.delayed_project_count} dự án chậm`}
        hint={<AttentionCaption openIssueCount={s.open_issue_count} atRiskCount={s.at_risk_count} />}
        accent={s.delayed_project_count > 0 ? DANGER_ACCENT : "text-leaf"}
      />
    </div>
  );
}

/**
 * Paid after 31/12 for this year's projects: in `Đã giải ngân` but on no month of the curve, so
 * December can sit below the card. Said, so the gap is not read as an error. Usually 0 → nothing.
 * Not in the prototype (it has no such figure); kept because without it a reported total looks wrong.
 */
export function AfterYearNote({ amount, year }: { amount: number; year: number }) {
  if (!(amount > 0)) return null;
  return (
    <p className="text-ink-muted m-0 mt-2 text-[11.5px]" data-after-year="">
      Có thêm {nhanTien(amount)} chi sau ngày 31/12/{year}: đã tính vào số đã giải ngân nhưng không nằm
      trên tháng nào của biểu đồ.
    </p>
  );
}

/**
 * §5 table (spec 02 §5, `CategoryReportPanel.tsx`), `Tổng cộng` bold on the grey row. Pressing a row
 * sets the project list's category filter; pressing the selected row again clears it.
 *
 * A row with no `category_id` (projects with no category) cannot filter — the list route reads an
 * empty `category` as no filter at all — so it is not a button. The name of a filtering row IS a
 * button, drawn as plain text: the row has no tab stop of its own, and the keyboard needs one.
 *
 * THE DEADLINE IS NEVER RED, even when passed with money left (spec 02 §5 says red): ADR 0077 #5.
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
    <Panel title="Tiến độ theo hạng mục" titleId="tieu-de-tien-do-hang-muc" className="mb-5">
      {rows.length === 0 ? (
        <p className="text-ink-muted m-0 py-8 text-center text-[12.5px]">
          Xã chưa khai báo hạng mục nào. Thêm ở Cấu hình → Danh mục → Hạng mục kế hoạch vốn.
        </p>
      ) : (
        <>
          <div
            role="region"
            tabIndex={0}
            aria-label={`Tiến độ giải ngân theo hạng mục năm ${year}`}
            className="relative w-full overflow-x-auto"
          >
            <table className={TABLE_CLASS} data-category-table="">
              <caption className="an-thi-giac">Tiến độ giải ngân theo hạng mục, năm ngân sách {year}</caption>
              <thead>
                <tr className={TR_CLASS}>
                  <th scope="col" className={cn(TH_CLASS, "min-w-64")}>
                    Hạng mục
                  </th>
                  <th scope="col" className={cn(TH_CLASS, "w-16 text-right")}>
                    Số DA
                  </th>
                  <th scope="col" className={cn(TH_CLASS, "w-36 text-right")}>
                    KH vốn năm
                  </th>
                  <th scope="col" className={cn(TH_CLASS, "w-36 text-right")}>
                    Đã giải ngân
                  </th>
                  <th scope="col" className={cn(TH_CLASS, "w-20 text-right")}>
                    Tỷ lệ
                  </th>
                  <th scope="col" className={cn(TH_CLASS, "w-36 text-right")}>
                    Chưa giải ngân
                  </th>
                  <th scope="col" className={cn(TH_CLASS, "w-20 text-right")}>
                    Tỷ lệ
                  </th>
                  <th scope="col" className={cn(TH_CLASS, "w-28")}>
                    Thời hạn giải ngân
                  </th>
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
                      className={cn(TR_CLASS, selectable && "cursor-pointer", selected && "bg-brand/6")}
                      onClick={selectable ? () => onSelectCategory(selected ? "" : id) : undefined}
                    >
                      <td className={cn(TD_CLASS, "whitespace-normal")}>
                        {selectable ? (
                          <button
                            type="button"
                            aria-pressed={selected}
                            className="text-navy cursor-pointer border-0 bg-transparent p-0 text-left [font-family:inherit] [font-size:inherit] font-semibold focus-visible:outline-2 focus-visible:outline-brand"
                            // The row's own click does the work; stopping here keeps it from running twice.
                            onClick={(e) => {
                              e.stopPropagation();
                              onSelectCategory(selected ? "" : id);
                            }}
                          >
                            {label}
                          </button>
                        ) : (
                          <span className="text-navy font-semibold">{label}</span>
                        )}
                      </td>
                      <CategoryFigures row={r} />
                    </tr>
                  );
                })}
                <tr className={cn(TR_CLASS, "bg-canvas hover:bg-canvas font-bold")} data-category-total="">
                  <th scope="row" className={cn(TD_CLASS, "text-navy text-left font-bold")}>
                    Tổng cộng
                  </th>
                  <CategoryFigures row={total} />
                </tr>
              </tbody>
            </table>
          </div>
          <p className="text-ink-muted m-0 mt-2 text-[11.5px]">Chạm một hạng mục để lọc danh sách dự án bên dưới.</p>
        </>
      )}
    </Panel>
  );
}

function CategoryFigures({ row }: { row: finance_categoryProgressOut }) {
  return (
    <>
      <td className={cn(TD_CLASS, "text-ink-muted text-right tabular-nums")}>{row.project_count}</td>
      <td className={cn(TD_CLASS, "text-right tabular-nums")}>{nhanTien(row.planned)}</td>
      <td className={cn(TD_CLASS, "text-right tabular-nums")}>{nhanTien(row.disbursed)}</td>
      <DisbursedRatioCell hundredths={row.disbursed_ratio} />
      <td className={cn(TD_CLASS, "text-right tabular-nums")}>{nhanTien(row.undisbursed)}</td>
      <td className={cn(TD_CLASS, "text-ink-muted text-right tabular-nums")}>{ratioCell(row.undisbursed_ratio)}</td>
      <td className={cn(TD_CLASS, "text-[12px] tabular-nums")}>
        {row.disbursement_deadline === undefined || row.disbursement_deadline === ""
          ? "—"
          : nhanNgay(row.disbursement_deadline)}
      </td>
    </>
  );
}

/** The disbursed ratio in its tier colour; `null` ("—") is not a figure and is not coloured. */
function DisbursedRatioCell({ hundredths }: { hundredths: number | null }) {
  if (hundredths === null || !Number.isFinite(hundredths)) {
    return <td className={cn(TD_CLASS, "text-right tabular-nums")}>{ratioCell(hundredths)}</td>;
  }
  const tone = progressTone(hundredths);
  return (
    <td
      className={cn(TD_CLASS, "text-right font-semibold tabular-nums", PROGRESS_TEXT_CLASS[tone])}
      data-ratio-tone={tone}
    >
      {ratioCell(hundredths)}
    </td>
  );
}

/** `null` = no plan in the row: "—", never `0%` (and not the long "Chưa bố trí vốn" in a narrow cell). */
function ratioCell(hundredths: number | null): string {
  return hundredths === null ? "—" : nhanTyLeGiaiNgan(hundredths);
}
