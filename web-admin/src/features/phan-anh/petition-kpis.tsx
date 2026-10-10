"use client";

import { CloudOff, RefreshCw, TriangleAlert } from "lucide-react";
import { useEffect, useState, type ReactNode } from "react";

import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/cn";
import { fetchCitizenReportSummary, type SummaryPeriod } from "@/lib/api/dashboard";
import type { KetQua } from "@/lib/api/goi";
import type { petitions_citizenReportSummaryOut } from "@/lib/api/schema.gen";

import {
  inProgressCaption,
  KPI_LOADING,
  KPI_NO_DEADLINE_SAMPLE,
  KPI_ON_TIME_LABEL,
  KPI_PENDING_CAPTION,
  KPI_PENDING_LABEL,
  KPI_RATING_LABEL,
  KPI_TOTAL_LABEL,
  KPI_WINDOW_DAYS,
  kpiCount,
  lowRatingCaption,
  onTimePercent,
  ratingAverage,
} from "./nhan-phieu";
import { Glyph } from "./petition-ui";

/**
 * The four KPI cards of spec §3, read from `GET /api/v1/citizen-report-summary` — the SAME route and
 * client the leadership overview reads (`lib/api/dashboard.ts`), so a figure here and the same figure
 * there cannot disagree.
 *
 * PLAIN FIGURES, NOT LINKS (customer bug sheet row 56, as the prototype `FeedbackWorkspace.tsx:94-120`):
 * the cards no longer filter the list below. The drill-down RECEIVER stays — the leadership overview
 * still links into `/phan-anh?metric=…` (`lib/drill-down.ts`), it just no longer starts here.
 *
 * NOTHING IS COUNTED HERE. Two divisions only, both the server's documented ones (sum / sample for the
 * average; on_time / on_time_sample for the rate), each `—` on an empty sample — never 0.
 *
 * Shown only with `feedback.read` AND `report.read` — the two keys the route checks. UX only: the
 * server enforces both, and the caller passes the session's keys (rule 5, forbidden #1).
 */

type Summary = KetQua<petitions_citizenReportSummaryOut>;

/** `[now − 90 days, now)`, RFC 3339 (UTC `Z`). A REPORTING window, not a deadline — rule 10 is not about it. */
export function kpiPeriod(now: Date): SummaryPeriod {
  const from = new Date(now.getTime() - KPI_WINDOW_DAYS * 24 * 60 * 60 * 1000);
  return { from: from.toISOString(), to: now.toISOString() };
}

export function PetitionKpis({
  load = fetchCitizenReportSummary,
}: {
  /** Injected only by tests; the screen always reads the contract route. */
  load?: (period: SummaryPeriod) => Promise<Summary>;
}) {
  // The window is fixed when the cards mount: a re-render must not move the period being counted.
  const [period] = useState(() => kpiPeriod(new Date()));
  const [reloads, setReloads] = useState(0);
  const [loaded, setLoaded] = useState<{ key: number; result: Summary } | null>(null);

  useEffect(() => {
    let dropped = false;
    load(period).then((result) => {
      if (!dropped) setLoaded({ key: reloads, result });
    });
    return () => {
      dropped = true;
    };
  }, [load, period, reloads]);

  const current = loaded !== null && loaded.key === reloads ? loaded.result : null;
  return <PetitionKpisView summary={current} onReload={() => setReloads((n) => n + 1)} />;
}

/** Presentational half — rendered to a string in tests. `summary === null` = loading. */
export function PetitionKpisView({
  summary,
  onReload,
}: {
  summary: Summary | null;
  onReload?: () => void;
}) {
  if (summary !== null && !summary.ok) {
    return (
      <div className="flex flex-wrap items-center gap-3 rounded-xl border border-danger-200 bg-danger-50 px-3.5 py-3">
        <Glyph icon={CloudOff} className="size-[18px] shrink-0 text-danger-600" />
        <p className="m-0 min-w-0 flex-1 text-sm text-ink-900" role="alert">
          Chưa tải được số liệu phản ánh: {summary.thongBao}
        </p>
        {onReload !== undefined && (
          <Button type="button" variant="secondary" size="sm" icon={<Glyph icon={RefreshCw} />} onClick={onReload}>
            Tải lại
          </Button>
        )}
      </div>
    );
  }

  const s = summary?.ok === true ? summary.duLieu : null;

  if (s === null) {
    // The prototype's loading state (`FeedbackWorkspace.tsx:91-92`): ONE block the height of the row.
    return (
      <section aria-label="Số liệu phản ánh" aria-busy="true" className="m-0">
        <p className="an-thi-giac" role="status">
          {KPI_LOADING}
        </p>
        <Skeleton className="h-24 w-full rounded-card" />
      </section>
    );
  }

  const pct = onTimePercent(s.on_time, s.on_time_sample);
  const average = ratingAverage(s.rating_sum, s.rating_sample);
  const lowRating = s.low_rating;
  const pending = s.publication_pending;
  const ratingKnown = typeof s.rating_sample === "number" && typeof s.rating_sum === "number";
  const late = s.late > 0;

  return (
    <section aria-label="Số liệu phản ánh" aria-busy={false} className="m-0">
      {/* The prototype's grid (`FeedbackWorkspace.tsx:94`): 1 / 2 / 4 columns, 12px apart. */}
      <ul className="m-0 grid list-none grid-cols-1 gap-3 p-0 sm:grid-cols-2 xl:grid-cols-4">
        <KpiCard label={KPI_TOTAL_LABEL} value={kpiCount(s.received)} hint={inProgressCaption(s.in_progress)} />
        {/* `FeedbackWorkspace.tsx:100-107`: the WHOLE value is danger when anything is late, leaf
            otherwise; danger also puts the warning icon before the hint — never colour alone. The two
            figures keep a hidden word each: "1 / 2" alone does not say which is which. */}
        <KpiCard
          label={KPI_ON_TIME_LABEL}
          accent={late ? "danger" : "leaf"}
          value={
            <>
              <span className="an-thi-giac">Đúng hạn: </span>
              {kpiCount(s.on_time)} / <span className="an-thi-giac">Trễ hạn trong kỳ: </span>
              {kpiCount(s.late)}
            </>
          }
          hint={pct === null ? KPI_NO_DEADLINE_SAMPLE : `${pct} đúng hạn`}
        />
        <KpiCard
          label={KPI_RATING_LABEL}
          // `—` when nobody has rated (sample 0) or the server predates the fields — NEVER 0,0/5.
          value={average === null || !ratingKnown ? "—" : average}
          hint={ratingKnown && typeof lowRating === "number" ? lowRatingCaption(lowRating) : undefined}
        />
        <KpiCard
          label={KPI_PENDING_LABEL}
          value={typeof pending === "number" ? kpiCount(pending) : "—"}
          hint={KPI_PENDING_CAPTION}
        />
      </ul>
    </section>
  );
}

/**
 * One KPI card — the prototype's `SummaryCard` (`FeedbackWorkspace.tsx:290-320`), local like
 * `features/giai-ngan/disbursement-overview.tsx` `Kpi`: NO icon tile, uppercase 11px label, 18px bold
 * value, 11px hint. `danger` puts `TriangleAlert` before the hint.
 */
function KpiCard({
  label,
  value,
  hint,
  accent,
}: {
  label: string;
  value: ReactNode;
  hint?: ReactNode;
  accent?: "danger" | "leaf";
}) {
  return (
    <li className="border-line shadow-card rounded-card min-w-0 border border-solid bg-white p-4" data-kpi={label}>
      <p className="text-ink-muted m-0 text-[11px] font-semibold tracking-wide uppercase">{label}</p>
      <p
        data-kpi-value=""
        className={cn(
          "m-0 mt-1.5 text-[18px] font-bold tabular-nums",
          accent === "danger" ? "text-danger" : accent === "leaf" ? "text-leaf" : "text-navy",
        )}
      >
        {value}
      </p>
      {hint !== undefined && (
        <div className="text-ink-muted mt-1 flex items-start gap-1 text-[11px]">
          {accent === "danger" && (
            <TriangleAlert aria-hidden="true" focusable="false" className="mt-0.5 size-3 shrink-0" />
          )}
          <span className="min-w-0">{hint}</span>
        </div>
      )}
    </li>
  );
}

