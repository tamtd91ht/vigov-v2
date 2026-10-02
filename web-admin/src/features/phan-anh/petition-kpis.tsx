"use client";

import { AlarmClock, CloudOff, Eye, Inbox, RefreshCw, Star, Target } from "lucide-react";
import Link from "next/link";
import { useEffect, useState, type ReactNode } from "react";

import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { StatCard } from "@/components/ui/stat-card";
import { cn } from "@/lib/cn";
import { fetchCitizenReportSummary, type SummaryPeriod } from "@/lib/api/dashboard";
import type { KetQua } from "@/lib/api/goi";
import type { petitions_citizenReportSummaryOut } from "@/lib/api/schema.gen";
import { isPeriodMetric, type CitizenReportMetric } from "@/lib/drill-down";

import {
  inProgressCaption,
  KPI_LOADING,
  KPI_NO_DEADLINE_SAMPLE,
  KPI_NO_RATING,
  KPI_ON_TIME_LABEL,
  KPI_PENDING_CAPTION,
  KPI_PENDING_LABEL,
  KPI_RATING_LABEL,
  KPI_TOTAL_LABEL,
  KPI_WINDOW_DAYS,
  kpiCount,
  kpiLinkLabel,
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
 * EVERY FIGURE IS A LINK TO THE LIST BEHIND IT (`/phan-anh?metric=…`, received by `lib/drill-down.ts`
 * on this very page): the number and the rows it opens are counted by one predicate on the server. A
 * period figure carries the card's own [from, to); a stock figure carries none.
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

/**
 * The list behind one figure. `URLSearchParams` encodes the RFC 3339 instants (a raw `+` would arrive
 * as a space and the receiver would call the link invalid). The period goes ONLY with a period metric:
 * the receiver refuses a period on a stock metric.
 */
export function kpiHref(metric: CitizenReportMetric, period: SummaryPeriod): string {
  const q = new URLSearchParams();
  q.set("metric", metric);
  if (isPeriodMetric("citizen-reports", metric)) {
    q.set("from", period.from);
    q.set("to", period.to);
  }
  return `/phan-anh?${q.toString()}`;
}

export function PetitionKpis({
  load = fetchCitizenReportSummary,
}: {
  /** Injected only by tests; the screen always reads the contract route. */
  load?: (period: SummaryPeriod) => Promise<Summary>;
}) {
  // The window is fixed when the cards mount: a re-render must not move the period the links carry.
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
  return <PetitionKpisView summary={current} period={period} onReload={() => setReloads((n) => n + 1)} />;
}

/** Presentational half — rendered to a string in tests. `summary === null` = loading. */
export function PetitionKpisView({
  summary,
  period,
  onReload,
}: {
  summary: Summary | null;
  period: SummaryPeriod;
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
  const busy = <Skeleton className="h-[26px] w-16" />;

  const pct = s === null ? null : onTimePercent(s.on_time, s.on_time_sample);
  const average = s === null ? null : ratingAverage(s.rating_sum, s.rating_sample);
  const lowRating = s?.low_rating;
  const pending = s?.publication_pending;
  const ratingKnown = typeof s?.rating_sample === "number" && typeof s?.rating_sum === "number";

  return (
    <section aria-label="Số liệu phản ánh" aria-busy={s === null} className="m-0">
      {s === null && (
        <p className="an-thi-giac" role="status">
          {KPI_LOADING}
        </p>
      )}
      <ul className="m-0 grid list-none grid-cols-1 gap-4 p-0 sm:grid-cols-2 xl:grid-cols-4">
        <li className="min-w-0">
          <StatCard
            icon={Inbox}
            label={KPI_TOTAL_LABEL}
            value={s === null ? busy : <FigureLink href={kpiHref("received", period)} label={KPI_TOTAL_LABEL}>{kpiCount(s.received)}</FigureLink>}
            caption={
              s === null ? undefined : (
                <FigureLink href={kpiHref("in_progress", period)} label={inProgressCaption(s.in_progress)} small>
                  {inProgressCaption(s.in_progress)}
                </FigureLink>
              )
            }
          />
        </li>
        <li className="min-w-0">
          <StatCard
            icon={Target}
            label={KPI_ON_TIME_LABEL}
            value={
              s === null ? (
                busy
              ) : (
                <span className="inline-flex items-baseline gap-1.5">
                  <FigureLink href={kpiHref("on_time", period)} label="Đúng hạn">
                    {kpiCount(s.on_time)}
                  </FigureLink>
                  <span aria-hidden="true" className="text-ink-400">
                    /
                  </span>
                  {/* Red ONLY with the icon beside it — never colour alone (spec §7). */}
                  <FigureLink
                    href={kpiHref("late", period)}
                    label="Trễ hạn trong kỳ"
                    className={s.late > 0 ? "text-danger-600" : undefined}
                  >
                    {s.late > 0 && <Glyph icon={AlarmClock} className="mr-1 inline size-5 align-[-2px]" />}
                    {kpiCount(s.late)}
                  </FigureLink>
                </span>
              )
            }
            caption={s === null ? undefined : pct === null ? KPI_NO_DEADLINE_SAMPLE : `${pct} đúng hạn`}
          />
        </li>
        <li className="min-w-0">
          <StatCard
            icon={Star}
            tone="warning"
            label={KPI_RATING_LABEL}
            value={
              s === null
                ? busy
                : !ratingKnown
                  ? // The server predates the fields: "Chưa có dữ liệu", never 0.
                    null
                  : average === null
                    ? // Nobody has rated yet (sample 0): no value, NEVER 0,0/5.
                      "—"
                    : (
                        <FigureLink href={kpiHref("rating_sample", period)} label={KPI_RATING_LABEL}>
                          {average}
                        </FigureLink>
                      )
            }
            caption={
              s === null || !ratingKnown ? undefined : (
                <span className="flex min-w-0 flex-col gap-0.5">
                  {average === null && <span>{KPI_NO_RATING}</span>}
                  {typeof lowRating === "number" && (
                    <FigureLink href={kpiHref("low_rating", period)} label={lowRatingCaption(lowRating)} small>
                      {lowRatingCaption(lowRating)}
                    </FigureLink>
                  )}
                </span>
              )
            }
          />
        </li>
        <li className="min-w-0">
          <StatCard
            icon={Eye}
            tone="neutral"
            label={KPI_PENDING_LABEL}
            value={
              s === null ? (
                busy
              ) : typeof pending === "number" ? (
                <FigureLink href={kpiHref("publication_pending", period)} label={KPI_PENDING_LABEL}>
                  {kpiCount(pending)}
                </FigureLink>
              ) : (
                null
              )
            }
            caption={s !== null && typeof pending !== "number" ? undefined : KPI_PENDING_CAPTION}
          />
        </li>
      </ul>
    </section>
  );
}

/**
 * A figure as a link. The accessible name says where it goes AND keeps the figure (`aria-describedby`
 * would need an id per figure; the visible text is short enough to stay part of the name).
 */
function FigureLink({
  href,
  label,
  small = false,
  className,
  children,
}: {
  href: string;
  label: string;
  small?: boolean;
  className?: string;
  children: ReactNode;
}) {
  return (
    <Link
      href={href}
      title={kpiLinkLabel(label)}
      className={cn(
        "rounded text-inherit underline-offset-4 hover:underline",
        "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-500",
        small && "text-xs",
        className,
      )}
    >
      <span className="an-thi-giac">{kpiLinkLabel(label)}: </span>
      {children}
    </Link>
  );
}
