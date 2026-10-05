import { CalendarDays, RefreshCw, Users } from "lucide-react";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import { DATA_TABLE_CLASS, TableScroll } from "@/components/ui/data-table";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { Field } from "@/components/ui/field";
import { PendingSection } from "@/components/ui/pending-feature";
import { Segmented } from "@/components/ui/segmented";
import { SkeletonRows } from "@/components/ui/skeleton";
import { formatCount } from "@/features/dashboard/figures";
import { formatDateTime, PERIOD_BUTTON_LABEL, PERIOD_KINDS } from "@/features/dashboard/period";
import type { PeriodKind } from "@/features/dashboard/period";
import { DashboardBlocks } from "@/features/dashboard/view";
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
 * ORDER, spec §2: context row (period, "tính đến", picker) · the six KPI groups of `/tong-quan` ·
 * "Tình hình thực hiện theo bộ phận" · "So sánh với kỳ trước". The export row sits in the page
 * header (`header-actions.tsx`). NOT HERE (spec §1): "Cần xử lý ngay", "Tính lại ngay",
 * "Trình chiếu".
 */

export const LOADING_SENTENCE = "Đang tải…";
export const UNIT_LOAD_ERROR = "Chưa tải được bảng theo bộ phận";
export const CUSTOM_SUBMIT_LABEL = "Xem";

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
  const comparison = reportComparisonNote(windows);
  return (
    <>
      <div className="-mt-3 mb-5 flex flex-wrap items-start gap-x-4 gap-y-3 sm:pl-16">
        <div className="flex min-w-0 flex-1 basis-72 flex-col gap-1 text-[13px] text-ink-500 [&_svg]:size-3.5 [&_svg]:shrink-0">
          <p className="m-0 inline-flex items-center gap-1.5">
            <CalendarDays aria-hidden="true" focusable="false" strokeWidth={1.8} />
            {reportPeriodLabel(windows)}
          </p>
          <p className="m-0 flex items-start gap-1.5">
            <RefreshCw aria-hidden="true" focusable="false" strokeWidth={1.8} className="mt-0.5" />
            <span className="min-w-0">
              Số liệu tính đến {formatDateTime(fetchedAt)}. {comparison}
            </span>
          </p>
        </div>
        <ReportPeriodPicker windows={windows} onNamed={onNamedPeriod} onCustom={onCustomPeriod} />
      </div>

      <DashboardBlocks
        data={figures}
        visible={access.blocks}
        onReload={onReload}
        urgent={false}
      />

      {access.unitTable && (
        <div className="mt-4">
          <UnitSummarySection state={units} onReload={onReloadUnits} />
        </div>
      )}

      <div className="mt-4">
        <PendingSection info={reportPendingPart("So sánh với kỳ trước")} titleAs="h2" />
      </div>
    </>
  );
}

/**
 * Four named periods + "Tuỳ chọn". A named period applies on press, exactly as on `/tong-quan`.
 * "Tuỳ chọn" only OPENS two native date inputs; the range applies on "Xem", and only when
 * `customWindows` accepts it — a refused range shows its sentence and sends nothing.
 */
export function ReportPeriodPicker({
  windows,
  onNamed,
  onCustom,
}: {
  windows: ReportWindows;
  onNamed: (kind: PeriodKind) => void;
  onCustom: (from: string, to: string) => void;
}) {
  const [customOpen, setCustomOpen] = useState(windows.kind === CUSTOM_PERIOD);
  const [from, setFrom] = useState(() => toDateInputValue(windows.current.start));
  const [to, setTo] = useState(() => toDateInputValue(windows.current.end - 1));
  const [error, setError] = useState<string | null>(null);
  const errorId = "bao-cao-ky-tuy-chon-loi";

  return (
    <div className="flex min-w-0 flex-col items-start gap-2">
      <Segmented
        mode="buttons"
        legend="Kỳ báo cáo"
        name="report-period"
        value={customOpen ? CUSTOM_PERIOD : windows.kind}
        options={[
          ...PERIOD_KINDS.map((k) => ({ value: k, label: PERIOD_BUTTON_LABEL[k] })),
          { value: CUSTOM_PERIOD, label: CUSTOM_BUTTON_LABEL },
        ]}
        onChange={(v) => {
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
          setError(null);
          onNamed(picked);
        }}
      />
      {customOpen && (
        <form
          className="flex max-w-full flex-wrap items-end gap-3"
          noValidate
          onSubmit={(e) => {
            e.preventDefault();
            const r = customWindows(from, to);
            if (!r.ok) {
              setError(r.message);
              return;
            }
            setError(null);
            onCustom(from, to);
          }}
        >
          <Field label="Từ ngày" htmlFor="bao-cao-tu-ngay" grow="auto" className="min-w-0">
            <input
              id="bao-cao-tu-ngay"
              type="date"
              value={from}
              aria-invalid={error !== null}
              aria-describedby={error !== null ? errorId : undefined}
              onChange={(e) => setFrom(e.target.value)}
            />
          </Field>
          <Field label="Đến ngày" htmlFor="bao-cao-den-ngay" grow="auto" className="min-w-0">
            <input
              id="bao-cao-den-ngay"
              type="date"
              value={to}
              aria-invalid={error !== null}
              aria-describedby={error !== null ? errorId : undefined}
              onChange={(e) => setTo(e.target.value)}
            />
          </Field>
          <Button type="submit" variant="primary" size="md">
            {CUSTOM_SUBMIT_LABEL}
          </Button>
          {error !== null && (
            <p id={errorId} role="alert" className="m-0 basis-full text-[13px] text-danger-600">
              {error}
            </p>
          )}
        </form>
      )}
    </div>
  );
}

const TH = "px-3 py-2.5 text-left text-xs font-semibold whitespace-nowrap text-ink-700";
const TD = "px-3 py-2 text-[13px] text-ink-900";
const NUM = "text-right tabular-nums whitespace-nowrap";

/**
 * "Tình hình thực hiện theo bộ phận" — ONLY THE TABLE SCROLLS at 320px (`TableScroll`), never the
 * page. Loading / failed (server sentence verbatim + "Tải lại") / empty, like the Sổ tay columns.
 */
export function UnitSummarySection({
  state,
  onReload,
}: {
  state: Loaded<UnitRow[]>;
  onReload: () => void;
}) {
  return (
    <section aria-label={UNIT_TABLE_TITLE} className="flex min-w-0 flex-col gap-2">
      <h2 className="m-0 flex items-center gap-2 text-base font-semibold text-ink-900">
        <Users aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-[18px] text-brand-600" />
        {UNIT_TABLE_TITLE}
      </h2>
      <UnitSummaryBody state={state} onReload={onReload} />
      <p className="m-0 text-xs leading-relaxed text-ink-500">{UNIT_TABLE_NOTE}</p>
    </section>
  );
}

function UnitSummaryBody({ state, onReload }: { state: Loaded<UnitRow[]>; onReload: () => void }) {
  if (state === null) {
    return (
      <div aria-busy="true" className="rounded-xl border border-line bg-surface p-4">
        <p className="an-thi-giac" role="status">
          {LOADING_SENTENCE}
        </p>
        <SkeletonRows rows={4} columns={4} />
      </div>
    );
  }
  if (!state.ok) {
    return (
      <div className="rounded-xl border border-line bg-surface">
        <ErrorState role="alert" title={UNIT_LOAD_ERROR} message={state.thongBao} onRetry={onReload} className="py-8" />
      </div>
    );
  }
  if (state.duLieu.length === 0) {
    return (
      <div className="rounded-xl border border-line bg-surface">
        <EmptyState tone="neutral" title={UNIT_TABLE_EMPTY} role="status" className="py-8" />
      </div>
    );
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
