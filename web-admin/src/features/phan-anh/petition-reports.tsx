"use client";

import { CloudOff, RefreshCw } from "lucide-react";
import { useEffect, useState, type ReactNode } from "react";

import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import {
  PERIOD_BUTTON_LABEL,
  PERIOD_KINDS,
  periodMetaLabel,
  periodWindows,
  toQueryPeriod,
  type PeriodKind,
} from "@/features/dashboard/period";
import { cn } from "@/lib/cn";
import type { KetQua } from "@/lib/api/goi";
import { readCitizenReportBreakdown } from "@/lib/api/phieu-phan-anh";
import type { petitions_citizenReportBreakdownOut } from "@/lib/api/schema.gen";

import {
  kpiCount,
  LINH_VUC_PHAN_ANH,
  nhanBoPhan,
  onTimePercent,
  onTimeShare,
  ratingWithSample,
  REPORT_FIELD_EMPTY,
  REPORT_FIELD_TITLE,
  REPORT_HAMLET_EMPTY,
  REPORT_HAMLET_NOTE,
  REPORT_HAMLET_TITLE,
  REPORT_LOAD_FAILED,
  REPORT_LOADING,
  REPORT_NO_HAMLET,
  REPORT_NO_UNIT,
  REPORT_ON_TIME_TITLE,
  REPORT_OVERDUE_NOW_LABEL,
  REPORT_UNCLASSIFIED,
  REPORT_UNIT_EMPTY,
  REPORT_UNIT_TITLE,
  workingHoursAverage,
} from "./nhan-phieu";
import { Glyph, toggleButtonClass, TOGGLE_TRACK } from "./petition-ui";

/**
 * The `Báo cáo` tab (prototype `FeedbackReports.tsx`; ADR 0053 §Sửa đổi 09/10/2026, C1–C5), read from
 * `GET /api/v1/citizen-report-breakdown?from=&to=`.
 *
 * THE PERIOD IS `/bao-cao`'s (C1): Tuần / Tháng / Quý / Năm in Asia/Ho_Chi_Minh, `[from, to)`, through
 * the SAME `periodWindows` + `toQueryPeriod` — so a figure here and the same figure on `/tong-quan` or
 * `/bao-cao` are one query. Never the prototype's sliding 90 days (C1 says why).
 *
 * NOTHING IS COUNTED HERE. The only arithmetic is presentation: on-time share = on_time / on_time_sample
 * (the KPI cards' `onTimePercent`), satisfaction = rating_sum / rating_sample WITH its sample (C3),
 * average handling = handling_working_seconds / handling_sample in WORKING hours (C4 — measured by
 * identity, never wall clock). Every empty sample shows `—`, never 0. "Đang trễ hạn (hiện tại)" is a
 * STOCK read now and is never turned into a rate (C2).
 *
 * WHAT THE PROTOTYPE HAS AND THE ROUTE DOES NOT: an overall satisfaction figure in the first section,
 * and per-unit totals / on time / late / rate / satisfaction. Those columns are not drawn — a column
 * computed from other rows would be a second count of a figure the server owns.
 *
 * `feedback.read` + `report.read` on the route; the caller gates the tab content on `report.read` (UX
 * only — the server checks both, rule 5).
 */

type Breakdown = KetQua<petitions_citizenReportBreakdownOut>;
type Period = { from: string; to: string };

export function PetitionReports({
  unitNames,
  load = readCitizenReportBreakdown,
  now = () => new Date(),
}: {
  /** Org-unit id → name, the register's own lookup (`layDanhMucBoPhan`). */
  unitNames: ReadonlyMap<string, string>;
  /** Injected only by tests. */
  load?: (period: Period) => Promise<Breakdown>;
  now?: () => Date;
}) {
  const [kind, setKind] = useState<PeriodKind>("month");
  const [reloads, setReloads] = useState(0);
  // The windows are fixed when the kind is picked: a re-render must not move the period being shown.
  const [windows, setWindows] = useState(() => periodWindows("month", now()));
  const [loaded, setLoaded] = useState<{ key: string; result: Breakdown } | null>(null);
  const period = toQueryPeriod(windows.current);
  const key = `${period.from}|${period.to}|${reloads}`;

  useEffect(() => {
    let dropped = false;
    load(period).then((result) => {
      if (!dropped) setLoaded({ key, result });
    });
    return () => {
      dropped = true;
    };
    // `key` carries the period (by value) and the reload counter.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key, load]);

  const current = loaded !== null && loaded.key === key ? loaded.result : null;

  return (
    <div className="flex min-w-0 flex-col gap-4 [&>*]:my-0">
      <div className="flex flex-wrap items-center gap-3">
        <div className={TOGGLE_TRACK} role="group" aria-label="Kỳ báo cáo">
          {PERIOD_KINDS.map((k) => (
            <button
              key={k}
              type="button"
              className={toggleButtonClass(k === kind)}
              aria-pressed={k === kind}
              onClick={() => {
                setKind(k);
                setWindows(periodWindows(k, now()));
              }}
            >
              {PERIOD_BUTTON_LABEL[k]}
            </button>
          ))}
        </div>
        <p className="m-0 text-[12px] text-ink-muted tabular-nums">{periodMetaLabel(windows)}</p>
      </div>
      <PetitionReportsView breakdown={current} unitNames={unitNames} onReload={() => setReloads((n) => n + 1)} />
    </div>
  );
}

/** Presentational half — rendered to a string in tests. `breakdown === null` = loading. */
export function PetitionReportsView({
  breakdown,
  unitNames,
  onReload,
}: {
  breakdown: Breakdown | null;
  unitNames: ReadonlyMap<string, string>;
  onReload?: () => void;
}) {
  if (breakdown === null) {
    return (
      <section aria-busy="true" className="m-0">
        <p className="an-thi-giac" role="status">
          {REPORT_LOADING}
        </p>
        <Skeleton className="h-40 w-full rounded-card" />
      </section>
    );
  }
  if (!breakdown.ok) {
    // 503 `working_hours_unavailable`, 409 `working_calendar_not_configured`, 403… — the server's
    // sentence verbatim: each says what is missing and where it is set.
    return (
      <div className="flex flex-wrap items-center gap-3 rounded-xl border border-danger-200 bg-danger-50 px-3.5 py-3">
        <Glyph icon={CloudOff} className="size-[18px] shrink-0 text-danger-600" />
        <p className="m-0 min-w-0 flex-1 text-sm text-ink-900" role="alert">
          {REPORT_LOAD_FAILED}: {breakdown.thongBao}
        </p>
        {onReload !== undefined && (
          <Button type="button" variant="secondary" size="sm" icon={<Glyph icon={RefreshCw} />} onClick={onReload}>
            Tải lại
          </Button>
        )}
      </div>
    );
  }

  const b = breakdown.duLieu;
  const t = b.totals;
  const pct = onTimePercent(t.on_time, t.on_time_sample);
  const share = onTimeShare(t.on_time, t.on_time_sample);

  return (
    <div className="flex min-w-0 flex-col gap-4 [&>*]:my-0">
      <ReportSection title={REPORT_ON_TIME_TITLE}>
        <div className="flex flex-wrap items-end gap-6">
          <Figure value={kpiCount(t.on_time)} label="đúng hạn" tone="text-leaf" />
          <Figure value={kpiCount(t.late)} label="trễ hạn" tone="text-danger" />
          <Figure
            value={pct ?? "—"}
            label="tỷ lệ đúng hạn"
            tone={share === null ? "text-navy" : share >= 80 ? "text-leaf" : share >= 50 ? "text-tangerine" : "text-danger"}
          />
          <Figure value={kpiCount(t.overdue)} label={REPORT_OVERDUE_NOW_LABEL} tone={t.overdue > 0 ? "text-danger" : "text-navy"} />
        </div>
        {/* The bar only on a non-empty sample; its words are the figures above (never colour alone). */}
        {share !== null && (
          <div className="mt-3 flex h-2.5 overflow-hidden rounded-full bg-[#EDF0F3]" aria-hidden="true" data-on-time-bar="">
            <div className="h-full bg-leaf" style={{ width: `${share}%` }} />
            <div className="h-full flex-1 bg-danger" />
          </div>
        )}
      </ReportSection>

      <ReportSection title={REPORT_FIELD_TITLE}>
        <ReportTable
          headers={["Lĩnh vực", "Tổng", "Đã xử lý", "Trễ hạn", "Đang trễ hạn", "Hài lòng"]}
          rows={b.fields.map((r) => [
            r.field_code === "" ? REPORT_UNCLASSIFIED : fieldLabel(r.field_code),
            kpiCount(r.received),
            kpiCount(r.finished),
            <Late key="late" n={r.late} />,
            <Late key="overdue" n={r.overdue} />,
            ratingWithSample(r.rating_sum, r.rating_sample),
          ])}
          empty={REPORT_FIELD_EMPTY}
        />
      </ReportSection>

      <ReportSection title={REPORT_UNIT_TITLE}>
        <ReportTable
          headers={["Bộ phận", "Đã xử lý xong", "TB giờ xử lý"]}
          rows={b.units.map((r) => [
            r.org_unit_id === "" ? REPORT_NO_UNIT : nhanBoPhan(r.org_unit_id, unitNames),
            kpiCount(r.finished),
            workingHoursAverage(r.handling_working_seconds, r.handling_sample) ?? "—",
          ])}
          empty={REPORT_UNIT_EMPTY}
        />
      </ReportSection>

      <ReportSection title={REPORT_HAMLET_TITLE} note={REPORT_HAMLET_NOTE}>
        <ReportTable
          headers={["Thôn, tổ dân phố", "Số phản ánh", "Đang trễ hạn"]}
          rows={b.residential_units.map((r) => [
            r.residential_unit_id === "" ? REPORT_NO_HAMLET : (r.residential_unit_name ?? "") || r.residential_unit_id,
            kpiCount(r.received),
            <Late key="overdue" n={r.overdue} />,
          ])}
          empty={REPORT_HAMLET_EMPTY}
        />
      </ReportSection>
    </div>
  );
}

/** The commune-facing label of a field code — the screen's catalogue; an unknown code shows itself. */
function fieldLabel(code: string): string {
  return LINH_VUC_PHAN_ANH.find((l) => l.ma === code)?.nhan ?? code;
}

function Late({ n }: { n: number }) {
  return n > 0 ? <span className="font-semibold text-danger">{kpiCount(n)}</span> : <>{kpiCount(n)}</>;
}

function ReportSection({ title, note, children }: { title: string; note?: string; children: ReactNode }) {
  return (
    <section className="rounded-card border border-solid border-line bg-white p-4 shadow-card">
      <h3 className={cn("m-0 text-[13px] font-bold text-navy", note === undefined ? "mb-3" : "mb-1")}>{title}</h3>
      {note !== undefined && <p className="m-0 mb-3 text-[11.5px] text-ink-muted">{note}</p>}
      {children}
    </section>
  );
}

function Figure({ value, label, tone }: { value: string; label: string; tone: string }) {
  return (
    <div className="[&>p]:m-0">
      <p className={cn("text-[24px] leading-none font-bold tabular-nums", tone)}>{value}</p>
      <p className="mt-1 text-[11.5px] text-ink-muted">{label}</p>
    </div>
  );
}

function ReportTable({
  headers,
  rows,
  empty,
}: {
  headers: readonly string[];
  rows: readonly (readonly ReactNode[])[];
  empty: string;
}) {
  if (rows.length === 0) return <p className="m-0 text-[12.5px] text-ink-muted">{empty}</p>;
  return (
    <div className="overflow-x-auto">
      <table className="w-full border-collapse text-[12.5px]">
        <thead>
          <tr className="border-b border-solid border-line text-left text-[11px] text-ink-muted uppercase">
            {headers.map((h) => (
              <th key={h} scope="col" className="py-2 pr-4 font-semibold">
                {h}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row, i) => (
            <tr key={i} className="border-b border-solid border-line last:border-b-0">
              {row.map((cell, j) => (
                <td key={j} className={cn("py-2 pr-4 tabular-nums", j === 0 ? "font-semibold text-navy" : "text-ink")}>
                  {cell}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
