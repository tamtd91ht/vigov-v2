import { ChartColumn } from "lucide-react";
import { useState } from "react";
import type { ReactNode } from "react";

import { Card } from "@/components/ui/card";
import { TableScroll } from "@/components/ui/data-table";
import { ErrorState } from "@/components/ui/error-state";
import { controlClass } from "@/components/ui/field";
import { PageHeader } from "@/components/ui/page-header";
import { SkeletonRows } from "@/components/ui/skeleton";
import type { ExportSource } from "@/features/dashboard/export-actions";
import type { ExportCommune } from "@/features/dashboard/export-model";
import { formatCount, NO_VALUE } from "@/features/dashboard/figures";
import { DEFAULT_PERIOD_KIND, formatDateTime, PERIOD_BUTTON_LABEL, PERIOD_KINDS } from "@/features/dashboard/period";
import type { PeriodKind } from "@/features/dashboard/period";
import {
  comparedFigures,
  DashboardBlocks,
  EXPORT_WAITING,
  figuresSettled,
  PeriodButtons,
} from "@/features/dashboard/view";
import type { BlocksData, Loaded } from "@/features/dashboard/view";
import { cn } from "@/lib/cn";

import { COMPARISON_TITLE, ComparisonChart } from "./comparison-chart";
import type { ReportAccess } from "./report-access";
import { buildReportExport, hasComparedBlock, REPORT_TITLE } from "./report-export";
import { ReportExportActions } from "./report-export-actions";
import {
  CUSTOM_BUTTON_LABEL,
  CUSTOM_PERIOD,
  customWindows,
  reportComparisonNote,
  reportPeriodLabel,
} from "./report-period";
import type { ReportWindows } from "./report-period";
import {
  barPercent,
  onTimeCell,
  UNIT_COLUMNS,
  UNIT_TABLE_EMPTY,
  UNIT_TABLE_NOTE,
  UNIT_TABLE_TITLE,
} from "./unit-table";
import type { UnitRow } from "./unit-table";

/**
 * The PRESENTATIONAL half of `/bao-cao` (`docs/ui-ux/13-bao-cao.md`, as amended by ADR 0053
 * 04/10/2026 and §Sửa đổi 09/10/2026 lần 2): no fetch, no clock. Everything arrives as props, so every
 * state — loading, failed, denied, empty — renders in a Node test with `renderToStaticMarkup`.
 *
 * COMPOSITION — the prototype's `ReportWorkspace` (ADR 0068 lần 5), top to bottom, `gap-5` apart:
 *   header      title · our long period line (B2/B5a, kept) · on the right the five period buttons
 *   custom box  "Từ ngày" / "Đến ngày", only while "Tuỳ chọn" is chosen — applies at once (D5)
 *   export row  "Xuất PDF" "Xuất XLSX" "Xuất PPTX", built (D3), only with `report.export`
 *   grid        the six KPI groups of `/tong-quan`, drawn in the report's tile (`layout="report"`, D4)
 *   card        "Xếp hạng bộ phận" — the prototype's table with bars (D1), our row set
 *   card        "So sánh với kỳ trước" — the hand-drawn bar chart (D2)
 * NOT HERE (spec §1): "Cần xử lý ngay", "Tính lại ngay", "Trình chiếu".
 */

export { REPORT_TITLE };
export const LOADING_SENTENCE = "Đang tải…";
export const UNIT_LOAD_ERROR = "Chưa tải được bảng theo bộ phận";

/** The prototype's hint while a date is missing (`ReportWorkspace.tsx`), verbatim. */
export const CUSTOM_PERIOD_HINT =
  "Chọn cả hai ngày để xem kỳ tuỳ chọn. Trong lúc đó vẫn hiển thị số liệu tháng này.";

/**
 * The page header. Without `subtitle` / `actions` = the gate is closed or the session still being
 * read: the title only, so the account still reads which page it is on, and no period — there is no
 * figure for it to describe. The prototype's row: `items-end gap-3`.
 */
export function ReportHeader({ subtitle, actions }: { subtitle?: ReactNode; actions?: ReactNode }) {
  return (
    <PageHeader
      icon={ChartColumn}
      title={REPORT_TITLE}
      subtitle={subtitle}
      actions={actions}
      className="mb-0 gap-3"
    />
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
  /**
   * The commune printed on the export files — the runtime configuration from `Host` (rule 1 inv. 10).
   * Absent = no export possible: the buttons are drawn disabled.
   */
  readonly commune?: ExportCommune;
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
  commune,
  onNamedPeriod,
  onCustomPeriod,
  onReload,
  onReloadUnits,
}: ReportViewProps) {
  // "Tuỳ chọn" works as the prototype's (D5): the box opens with BOTH INPUTS EMPTY, the month's figures
  // stay on screen until both days are set, and a valid range applies AT ONCE — no "Xem" button.
  // A native date input reports only whole dates or "", so "at once" never sends a half-typed day.
  const [customOpen, setCustomOpen] = useState(windows.kind === CUSTOM_PERIOD);
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [error, setError] = useState<string | null>(null);

  /** Back to the month while no valid range is set — the prototype's fallback, never a stale range. */
  const showMonth = () => {
    if (windows.kind !== DEFAULT_PERIOD_KIND) onNamedPeriod(DEFAULT_PERIOD_KIND);
  };

  const apply = (f: string, t: string) => {
    if (f === "" || t === "") {
      setError(null);
      showMonth();
      return;
    }
    const r = customWindows(f, t);
    if (!r.ok) {
      // Ours, not the prototype's (silent): a refused range says why and is NEVER sent.
      setError(r.message);
      showMonth();
      return;
    }
    setError(null);
    onCustomPeriod(f, t);
  };

  const choose = (v: PeriodChoice) => {
    if (v === CUSTOM_PERIOD) {
      if (customOpen) return;
      setCustomOpen(true);
      apply(from, to);
      return;
    }
    const picked = PERIOD_KINDS.find((k) => k === v);
    if (picked === undefined) return;
    setCustomOpen(false);
    onNamedPeriod(picked);
  };

  const exportSource: ExportSource | undefined =
    commune === undefined
      ? undefined
      : {
          // `/bao-cao` reads no queue (`withQueue = false`); the unit table, when shown, must have answered.
          blockedReason:
            figuresSettled(figures, access.blocks, false) && (!access.unitTable || units !== null)
              ? null
              : EXPORT_WAITING,
          build: (generatedAt) =>
            buildReportExport({ commune, windows, fetchedAt, generatedAt, figures, access, units }),
        };

  return (
    <div className="flex min-w-0 flex-col gap-5">
      <ReportHeader
        subtitle={
          // Our long line is kept (ADR 0053 B2/B5a): the period's days and what "kỳ trước" means.
          <span className="min-w-0 max-w-[46rem]">
            {reportPeriodLabel(windows)} · Số liệu tính đến {formatDateTime(fetchedAt)}.{" "}
            {reportComparisonNote(windows)}
          </span>
        }
        actions={
          <div className="flex flex-wrap items-end gap-2">
            <PeriodButtons options={PERIOD_OPTIONS} value={customOpen ? CUSTOM_PERIOD : windows.kind} onChange={choose} />
          </div>
        }
      />

      {customOpen && (
        <CustomPeriodBox
          from={from}
          to={to}
          error={error}
          onFrom={(v) => {
            setFrom(v);
            apply(v, to);
          }}
          onTo={(v) => {
            setTo(v);
            apply(from, v);
          }}
        />
      )}

      {access.exportReport && <ReportExportActions source={exportSource} />}

      <DashboardBlocks data={figures} visible={access.blocks} onReload={onReload} layout="report" />

      {access.unitTable && <UnitSummarySection state={units} onReload={onReloadUnits} />}

      {hasComparedBlock(access) && <ComparisonSection figures={figures} access={access} />}
    </div>
  );
}

const DATE_LABEL = "block text-[11.5px] leading-tight font-medium text-navy";
const DATE_INPUT = "mt-1 h-9 w-44 max-w-full text-[12.5px] md:text-[12.5px]";

/** The prototype's custom-period box: two native date inputs on a bordered strip of the page colour. */
function CustomPeriodBox({
  from,
  to,
  error,
  onFrom,
  onTo,
}: {
  from: string;
  to: string;
  error: string | null;
  onFrom: (v: string) => void;
  onTo: (v: string) => void;
}) {
  const errorId = "bao-cao-ky-tuy-chon-loi";
  const described = error !== null ? errorId : undefined;
  return (
    <div className="flex max-w-full flex-wrap items-end gap-3 rounded-[10px] border border-line bg-canvas p-3">
      <div className="min-w-0">
        <label htmlFor="bao-cao-tu-ngay" className={DATE_LABEL}>
          Từ ngày
        </label>
        <input
          id="bao-cao-tu-ngay"
          type="date"
          value={from}
          aria-invalid={error !== null}
          aria-describedby={described}
          onChange={(e) => onFrom(e.target.value)}
          className={cn(controlClass, DATE_INPUT)}
        />
      </div>
      <div className="min-w-0">
        <label htmlFor="bao-cao-den-ngay" className={DATE_LABEL}>
          Đến ngày
        </label>
        <input
          id="bao-cao-den-ngay"
          type="date"
          value={to}
          aria-invalid={error !== null}
          aria-describedby={described}
          onChange={(e) => onTo(e.target.value)}
          className={cn(controlClass, DATE_INPUT)}
        />
      </div>
      {error !== null ? (
        <p id={errorId} role="alert" className="m-0 basis-full text-[13px] text-danger-600">
          {error}
        </p>
      ) : (
        (from === "" || to === "") && <p className="m-0 pb-2 text-[12px] text-ink-muted">{CUSTOM_PERIOD_HINT}</p>
      )}
    </div>
  );
}

/** The prototype's title of the two cards under the grid: 14px bold navy, no icon. */
const SECTION_TITLE = "m-0 mb-3 text-[14px] leading-snug font-bold text-navy";

/** A cell of the unit table — the prototype's `RankingTable` (`border-b px-3 py-2`). */
const CELL = "border-b border-line px-3 py-2";

/**
 * "Xếp hạng bộ phận" — the prototype's `RankingTable` (D1) in its card. ONLY THE TABLE SCROLLS at 320px
 * (`TableScroll`, keyboard-reachable region), never the page. Loading / failed (server sentence
 * verbatim + "Tải lại") / empty kept from our page (ADR 0053 B1).
 */
export function UnitSummarySection({
  state,
  onReload,
}: {
  state: Loaded<UnitRow[]>;
  onReload: () => void;
}) {
  return (
    <Card as="section" aria-label={UNIT_TABLE_TITLE} className="p-4">
      <h2 className={SECTION_TITLE}>{UNIT_TABLE_TITLE}</h2>
      <UnitSummaryBody state={state} onReload={onReload} />
      <p className="m-0 mt-3 border-t border-line pt-2 text-[11px] leading-snug text-ink-muted">{UNIT_TABLE_NOTE}</p>
    </Card>
  );
}

const ON_TIME_TONE = { good: "text-leaf", warning: "text-tangerine", bad: "text-danger" } as const;

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
    return (
      <p role="status" className="m-0 py-8 text-center text-[13px] text-ink-muted">
        {UNIT_TABLE_EMPTY}
      </p>
    );
  }
  const rows = state.duLieu;
  const maxTotal = Math.max(1, ...rows.map((r) => r.total));
  const maxOverdue = Math.max(1, ...rows.map((r) => r.overdue));
  return (
    <div className="overflow-hidden rounded-[10px] border border-line">
      <TableScroll aria-label={UNIT_TABLE_TITLE}>
        <table className="w-full min-w-[520px] border-collapse text-[12.5px]">
          <thead>
            <tr className="bg-canvas text-ink-muted">
              <th scope="col" className={cn(CELL, "text-left font-semibold")}>
                {UNIT_COLUMNS.unit}
              </th>
              <th scope="col" className={cn(CELL, "text-left font-semibold")}>
                {UNIT_COLUMNS.total}
              </th>
              <th scope="col" className={cn(CELL, "text-left font-semibold whitespace-nowrap")}>
                {UNIT_COLUMNS.overdue}
              </th>
              <th scope="col" className={cn(CELL, "text-right font-semibold")}>
                {UNIT_COLUMNS.completed}
              </th>
              <th scope="col" className={cn(CELL, "text-right font-semibold")}>
                {UNIT_COLUMNS.onTime}
              </th>
            </tr>
          </thead>
          <tbody>
            {rows.map((r) => {
              const onTime = onTimeCell(r);
              return (
                <tr key={`${r.kind}|${r.key}`}>
                  <th
                    scope="row"
                    className={cn(
                      CELL,
                      "text-left",
                      r.kind === "unit" ? "font-medium text-navy" : "font-normal text-ink-muted",
                    )}
                  >
                    {r.name}
                  </th>
                  <td className={CELL}>
                    <Bar value={r.total} max={maxTotal} tone="brand" />
                  </td>
                  <td className={CELL}>
                    <Bar value={r.overdue} max={maxOverdue} tone="danger" />
                  </td>
                  <td className={cn(CELL, "text-right tabular-nums")}>{formatCount(r.completed)}</td>
                  <td
                    className={cn(
                      CELL,
                      "text-right tabular-nums",
                      onTime === null ? "font-normal text-ink-muted" : cn("font-semibold", ON_TIME_TONE[onTime.tone]),
                    )}
                  >
                    {onTime?.text ?? NO_VALUE}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </TableScroll>
    </div>
  );
}

/**
 * The prototype's bar in a cell: a 16px track up to 8rem, filled to the value against the column's
 * largest, then the number. The bar is decoration (`aria-hidden`) — the number is the figure.
 */
function Bar({ value, max, tone }: { value: number; max: number; tone: "brand" | "danger" }) {
  return (
    <div className="flex items-center gap-2" data-bar={tone}>
      <div aria-hidden="true" className="relative h-4 w-full max-w-[8rem] overflow-hidden rounded-[4px] bg-canvas">
        <div
          className={cn("absolute inset-y-0 left-0 rounded-[4px]", tone === "danger" ? "bg-danger/70" : "bg-brand/70")}
          style={{ width: `${barPercent(value, max)}%` }}
        />
      </div>
      <span
        className={cn(
          "w-8 shrink-0 text-right tabular-nums",
          tone === "danger" && value > 0 && "font-semibold text-danger",
        )}
      >
        {formatCount(value)}
      </span>
    </div>
  );
}

/**
 * "So sánh với kỳ trước" — the prototype's last card (D2). Drawn only when a compared block is visible:
 * without one there is nothing it could plot, and an empty card would read as "no previous period".
 */
function ComparisonSection({ figures, access }: { figures: BlocksData; access: ReportAccess }) {
  return (
    <Card as="section" aria-label={COMPARISON_TITLE} className="p-4">
      <h2 className={SECTION_TITLE}>{COMPARISON_TITLE}</h2>
      <ComparisonChart rows={comparedFigures(figures, access.blocks)} />
    </Card>
  );
}
