import { ChartColumn } from "lucide-react";
import { useState } from "react";
import type { ReactNode } from "react";

import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { DATA_TABLE_CLASS, TableScroll } from "@/components/ui/data-table";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { Field } from "@/components/ui/field";
import { PageHeader } from "@/components/ui/page-header";
import { PENDING_HOVER_TEXT, PendingMarker } from "@/components/ui/pending-feature";
import { SkeletonRows } from "@/components/ui/skeleton";
import { formatCount } from "@/features/dashboard/figures";
import { ExportPendingActions } from "@/features/dashboard/header-actions";
import { formatDateTime, PERIOD_BUTTON_LABEL, PERIOD_KINDS } from "@/features/dashboard/period";
import type { PeriodKind } from "@/features/dashboard/period";
import { DashboardBlocks, PeriodButtons } from "@/features/dashboard/view";
import type { BlocksData, Loaded } from "@/features/dashboard/view";
import { cn } from "@/lib/cn";

import { reportPendingPart } from "./labels";
import type { ReportAccess } from "./report-access";
import {
  CUSTOM_BUTTON_LABEL,
  CUSTOM_PERIOD,
  customWindows,
  reportComparisonNote,
  reportPeriodLabel,
  toDateInputValue,
} from "./report-period";
import type { ReportWindows } from "./report-period";
import {
  onTimeText,
  UNIT_COLUMNS,
  UNIT_TABLE_EMPTY,
  UNIT_TABLE_NOTE,
  UNIT_TABLE_TITLE,
} from "./unit-table";
import type { UnitRow } from "./unit-table";

/**
 * The PRESENTATIONAL half of `/bao-cao` (`docs/ui-ux/13-bao-cao.md`, as amended by ADR 0053
 * 04/10/2026): no fetch, no clock. Everything arrives as props, so every state — loading, failed,
 * denied, empty — renders in a Node test with `renderToStaticMarkup`.
 *
 * COMPOSITION — the prototype's `ReportWorkspace` (ADR 0068 lần 5), top to bottom, `gap-5` apart:
 *   header      title · "Số liệu tính đến …" line · on the right the five period buttons
 *   custom box  "Từ ngày" / "Đến ngày", only while "Tuỳ chọn" is chosen
 *   export row  "Xuất PDF" "Xuất XLSX" "Xuất PPTX" (disabled, one "?"), only with `report.export`
 *   grid        the six KPI groups of `/tong-quan`, in the prototype's report spacing
 *   card        "Tình hình thực hiện theo bộ phận" — in the prototype's "Xếp hạng bộ phận" place
 *               (ADR 0053 amendment 04/10: a table per unit, not a ranking)
 *   card        "So sánh với kỳ trước" — the prototype's chart, not built (ADR 0068 §14, "?")
 * NOT HERE (spec §1): "Cần xử lý ngay", "Tính lại ngay", "Trình chiếu".
 */

export const REPORT_TITLE = "Báo cáo điều hành";
export const LOADING_SENTENCE = "Đang tải…";
export const UNIT_LOAD_ERROR = "Chưa tải được bảng theo bộ phận";
export const CUSTOM_SUBMIT_LABEL = "Xem";

/**
 * The page header. Without `subtitle` / `actions` = the gate is closed or the session still being
 * read: the title only, so the account still reads which page it is on, and no period — there is no
 * figure for it to describe.
 */
export function ReportHeader({ subtitle, actions }: { subtitle?: ReactNode; actions?: ReactNode }) {
  return (
    <PageHeader icon={ChartColumn} title={REPORT_TITLE} subtitle={subtitle} actions={actions} className="mb-0" />
  );
}

export type ReportViewProps = {
  readonly windows: ReportWindows;
  /** The instant the windows were computed and the figures asked for. */
  readonly fetchedAt: number;
  readonly figures: BlocksData;
  readonly access: ReportAccess;
  /** `null` while loading; ignored when `access.unitTable` is false. */
  readonly units: Loaded<UnitRow[]>;
  readonly onNamedPeriod: (kind: PeriodKind) => void;
  /** Called ONLY with a range `customWindows` accepted. */
  readonly onCustomPeriod: (from: string, to: string) => void;
  /** Re-asks the period on screen. */
  readonly onReload: () => void;
  readonly onReloadUnits: () => void;
};

type PeriodChoice = PeriodKind | typeof CUSTOM_PERIOD;

const PERIOD_OPTIONS: readonly { value: PeriodChoice; label: string }[] = [
  ...PERIOD_KINDS.map((k) => ({ value: k, label: PERIOD_BUTTON_LABEL[k] })),
  { value: CUSTOM_PERIOD, label: CUSTOM_BUTTON_LABEL },
];

export function ReportView({
  windows,
  fetchedAt,
  figures,
  access,
  units,
  onNamedPeriod,
  onCustomPeriod,
  onReload,
  onReloadUnits,
}: ReportViewProps) {
  // "Tuỳ chọn" only OPENS the custom box; the figures on screen stay those of the period already
  // asked for until "Xem" sends a range `customWindows` accepted.
  const [customOpen, setCustomOpen] = useState(windows.kind === CUSTOM_PERIOD);
  const [from, setFrom] = useState(() => toDateInputValue(windows.current.start));
  const [to, setTo] = useState(() => toDateInputValue(windows.current.end - 1));

  const choose = (v: PeriodChoice) => {
    if (v === CUSTOM_PERIOD) {
      // Opening from a named period starts the inputs on the window on screen, so "Xem" right
      // away asks for the days the reader is already looking at.
      if (!customOpen && windows.kind !== CUSTOM_PERIOD) {
        setFrom(toDateInputValue(windows.current.start));
        setTo(toDateInputValue(windows.current.end - 1));
      }
      setCustomOpen(true);
      return;
    }
    const picked = PERIOD_KINDS.find((k) => k === v);
    if (picked === undefined) return;
    setCustomOpen(false);
    onNamedPeriod(picked);
  };

  return (
    <div className="flex min-w-0 flex-col gap-5">
      <ReportHeader
        subtitle={
          <span className="min-w-0">
            {reportPeriodLabel(windows)} · Số liệu tính đến {formatDateTime(fetchedAt)}.{" "}
            {reportComparisonNote(windows)}
          </span>
        }
        actions={
          <PeriodButtons options={PERIOD_OPTIONS} value={customOpen ? CUSTOM_PERIOD : windows.kind} onChange={choose} />
        }
      />

      {customOpen && (
        <CustomPeriodBox
          from={from}
          to={to}
          applied={windows.kind === CUSTOM_PERIOD}
          onFrom={setFrom}
          onTo={setTo}
          onApply={() => onCustomPeriod(from, to)}
        />
      )}

      {access.exportReport && (
        <div className="flex flex-wrap gap-2">
          <ExportPendingActions size="md" labelPrefix="Xuất " />
        </div>
      )}

      <DashboardBlocks data={figures} visible={access.blocks} onReload={onReload} layout="report" />

      {access.unitTable && <UnitSummarySection state={units} onReload={onReloadUnits} />}

      <ComparisonPendingSection />
    </div>
  );
}

/**
 * The prototype's custom-period box: two native date inputs on a bordered strip. Unlike the prototype
 * the range applies on "Xem", and only when `customWindows` accepts it — a refused range shows its
 * sentence and sends nothing; applying on every keystroke would ask the servers for half-typed dates.
 */
function CustomPeriodBox({
  from,
  to,
  applied,
  onFrom,
  onTo,
  onApply,
}: {
  from: string;
  to: string;
  /** a custom range is already on screen */
  applied: boolean;
  onFrom: (v: string) => void;
  onTo: (v: string) => void;
  onApply: () => void;
}) {
  const [error, setError] = useState<string | null>(null);
  const errorId = "bao-cao-ky-tuy-chon-loi";
  return (
    <form
      className="flex max-w-full flex-wrap items-end gap-3 rounded-[10px] border border-line bg-surface p-3"
      noValidate
      onSubmit={(e) => {
        e.preventDefault();
        const r = customWindows(from, to);
        if (!r.ok) {
          setError(r.message);
          return;
        }
        setError(null);
        onApply();
      }}
    >
      <Field label="Từ ngày" htmlFor="bao-cao-tu-ngay" grow="auto" className="min-w-0">
        <input
          id="bao-cao-tu-ngay"
          type="date"
          value={from}
          aria-invalid={error !== null}
          aria-describedby={error !== null ? errorId : undefined}
          onChange={(e) => onFrom(e.target.value)}
        />
      </Field>
      <Field label="Đến ngày" htmlFor="bao-cao-den-ngay" grow="auto" className="min-w-0">
        <input
          id="bao-cao-den-ngay"
          type="date"
          value={to}
          aria-invalid={error !== null}
          aria-describedby={error !== null ? errorId : undefined}
          onChange={(e) => onTo(e.target.value)}
        />
      </Field>
      <Button type="submit" variant="primary" size="md">
        {CUSTOM_SUBMIT_LABEL}
      </Button>
      {error !== null ? (
        <p id={errorId} role="alert" className="m-0 basis-full text-[13px] text-danger-600">
          {error}
        </p>
      ) : (
        !applied && (
          <p className="m-0 pb-2 text-[12px] text-ink-500">
            Chọn cả hai ngày rồi bấm Xem. Trong lúc đó vẫn hiển thị số liệu của kỳ đang xem.
          </p>
        )
      )}
    </form>
  );
}

/** The prototype's section title under the grid: 14px bold navy, no icon. */
const SECTION_TITLE = "m-0 text-[14px] leading-snug font-bold text-ink-900";

const TH = "px-3 py-2.5 text-left text-xs font-semibold whitespace-nowrap text-ink-700";
const TD = "px-3 py-2 text-[13px] text-ink-900";
const NUM = "text-right tabular-nums whitespace-nowrap";

/**
 * "Tình hình thực hiện theo bộ phận" — a white card in the prototype's "Xếp hạng bộ phận" place.
 * ONLY THE TABLE SCROLLS at 320px (`TableScroll`), never the page. Loading / failed (server sentence
 * verbatim + "Tải lại") / empty, like the Sổ tay columns.
 */
export function UnitSummarySection({
  state,
  onReload,
}: {
  state: Loaded<UnitRow[]>;
  onReload: () => void;
}) {
  return (
    <Card as="section" aria-label={UNIT_TABLE_TITLE} className="flex flex-col gap-3 p-4">
      <h2 className={SECTION_TITLE}>{UNIT_TABLE_TITLE}</h2>
      <UnitSummaryBody state={state} onReload={onReload} />
      <p className="m-0 text-xs leading-relaxed text-ink-500">{UNIT_TABLE_NOTE}</p>
    </Card>
  );
}

function UnitSummaryBody({ state, onReload }: { state: Loaded<UnitRow[]>; onReload: () => void }) {
  if (state === null) {
    return (
      <div aria-busy="true">
        <p className="an-thi-giac" role="status">
          {LOADING_SENTENCE}
        </p>
        <SkeletonRows rows={4} columns={4} />
      </div>
    );
  }
  if (!state.ok) {
    return <ErrorState role="alert" title={UNIT_LOAD_ERROR} message={state.thongBao} onRetry={onReload} className="py-8" />;
  }
  if (state.duLieu.length === 0) {
    return <EmptyState tone="neutral" title={UNIT_TABLE_EMPTY} role="status" className="py-8" />;
  }
  return (
    <TableScroll aria-label={UNIT_TABLE_TITLE}>
      <table className={cn(DATA_TABLE_CLASS, "w-full min-w-[560px] border-collapse")}>
        <caption className="an-thi-giac">{UNIT_TABLE_TITLE}</caption>
        <thead className="bg-surface-muted">
          <tr>
            <th scope="col" className={TH}>
              {UNIT_COLUMNS.unit}
            </th>
            <th scope="col" className={cn(TH, "text-right")}>
              {UNIT_COLUMNS.total}
            </th>
            <th scope="col" className={cn(TH, "text-right")}>
              {UNIT_COLUMNS.completed}
            </th>
            <th scope="col" className={cn(TH, "text-right")}>
              {UNIT_COLUMNS.onTime}
            </th>
            <th scope="col" className={cn(TH, "text-right")}>
              {UNIT_COLUMNS.overdue}
            </th>
          </tr>
        </thead>
        <tbody>
          {state.duLieu.map((r) => (
            <tr key={`${r.kind}|${r.key}`} className="border-t border-line">
              <th
                scope="row"
                className={cn(TD, "text-left font-medium", r.kind !== "unit" && "font-normal text-ink-500")}
              >
                {r.name}
              </th>
              <td className={cn(TD, NUM)}>{formatCount(r.total)}</td>
              <td className={cn(TD, NUM)}>{formatCount(r.completed)}</td>
              <td className={cn(TD, NUM)}>{onTimeText(r)}</td>
              <td className={cn(TD, NUM, r.overdue > 0 && "font-semibold text-danger-600")}>
                {formatCount(r.overdue)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </TableScroll>
  );
}

/**
 * "So sánh với kỳ trước" — the prototype's last card (a horizontal bar chart of each figure's change),
 * not built this round (ADR 0053 amendment 04/10, B4). The card stands at the chart's place with its
 * title and the "?" (ADR 0068 §14); the body says it is not built, never draws an empty chart.
 */
function ComparisonPendingSection() {
  const info = reportPendingPart("So sánh với kỳ trước");
  return (
    <Card as="section" aria-label={info.ten} data-pending="" className="flex flex-col gap-3 p-4">
      <div className="flex items-center gap-2">
        <h2 className={cn(SECTION_TITLE, "text-ink-500")}>{info.ten}</h2>
        <PendingMarker info={info} />
      </div>
      <p className="m-0 py-8 text-center text-[13px] text-ink-500">{PENDING_HOVER_TEXT}</p>
    </Card>
  );
}
