import {
  AlarmClock,
  BellRing,
  CalendarDays,
  CircleCheck,
  CirclePause,
  CloudOff,
  Database,
  History,
  Hourglass,
  Inbox,
  Loader,
  Mail,
  MessageSquareWarning,
  Minus,
  RefreshCw,
  RotateCw,
  Siren,
  Star,
  Target,
  TrendingDown,
  TrendingUp,
  Wallet,
  Activity,
  type LucideIcon,
} from "lucide-react";
import Link from "next/link";
import type { ReactNode } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { PendingSection, PendingStatCard } from "@/components/ui/pending-feature";
import { Segmented } from "@/components/ui/segmented";
import { NO_DATA_CAPTION, StatCard, type StatTone } from "@/components/ui/stat-card";
import { TodoList, type TodoItem } from "@/components/ui/todo-list";
import { KPI_NO_RATING, ratingAverage } from "@/features/phan-anh/nhan-phieu";
import { OTien } from "@/features/thu-chi/o-tien";
import {
  GHI_CHU_CHENH_LECH,
  lyDoKhongTinh,
  NHAN_CHENH_LECH,
  nhanChiSo,
  nhanSoTien,
  nhanSoTienChiSo,
} from "@/features/thu-chi/nhan-thu-chi";
import type { KetQua } from "@/lib/api/goi";
import type {
  documents_incomingSummaryOut,
  finance_chiSoNamRa,
  finance_chiSoRa,
  petitions_citizenReportSummaryOut,
  petitions_taskSummaryOut,
} from "@/lib/api/schema.gen";
import { cn } from "@/lib/cn";
import {
  QUYEN_XEM_GIAI_NGAN,
  QUYEN_XEM_NHIEM_VU,
  QUYEN_XEM_PHAN_ANH,
  QUYEN_XEM_VAN_BAN,
} from "@/lib/quyen";

import {
  canSeeBlock,
  categoryLabel,
  comparisonLine,
  drillHref,
  drillLabel,
  drillTargetLabel,
  FISCAL_SCREEN_PATH,
  formatCount,
  formatPercent,
  kindLabel,
  missedSinceText,
  MODULE_LABEL,
  NO_SOURCE_DATA,
  NO_VALUE,
  NOTHING_URGENT,
  ratioPercent,
} from "./figures";
import type { ComparisonLine, DrillTarget, FigureKind, MergedQueue, Trend } from "./figures";
import { pendingPart } from "./labels";
import {
  comparisonNote,
  formatDateTime,
  PERIOD_BUTTON_LABEL,
  PERIOD_KINDS,
  periodMetaLabel,
  toQueryPeriod,
} from "./period";
import type { PeriodKind, PeriodWindows } from "./period";

/**
 * The PRESENTATIONAL half of the overview: no hook, no fetch, no clock. Everything arrives as props,
 * so every state — loading, failed, denied, empty sample, 503 queue — renders with
 * `renderToStaticMarkup` in a test (pattern `features/quyen/cong-quyen.tsx` `KhungQuyen`). The
 * branches that matter most are the ones a developer with every key never sees.
 *
 * LAYOUT (owner's spec §8.2, revised 02/10/2026): the page answers "what do I handle next" first.
 *   row 1   "Cần xử lý" (pending work, from figures already read) · "Tình hình trong kỳ" (period)
 *   row 2   "Cần xử lý ngay" — the merged overdue queue, unchanged in content
 *   row 3   compact module cards for the figures rows 1–2 do not show
 * EVERY FIGURE THE PAGE SHOWED BEFORE IS STILL SHOWN EXACTLY ONCE, each still a link to its list,
 * under the same read keys. Only where it sits and how it looks changed (ADR 0068 §1).
 *
 * STYLED WITH UTILITIES ONLY. The old CSS module sat outside every cascade layer and beat every
 * utility; nothing here may reintroduce an unlayered rule.
 */

/**
 * What the blocks need of a period: the window the figures count and the one they compare with.
 * Narrower than `PeriodWindows` so `/bao-cao`'s custom period (no named kind) feeds the same blocks.
 */
export type ComparedWindows = Pick<PeriodWindows, "current" | "previous">;

/** `null` = still loading — distinct from a failed call, which is `{ ok: false }`. */
export type Loaded<T> = KetQua<T> | null;

/** One summary, read twice: the current window and the same elapsed portion of the previous one. */
export type SummaryPair<T> = {
  readonly current: Loaded<T>;
  readonly previous: Loaded<T>;
};

/** A figure read from a load: `undefined` = loading · `null` = no value (failed, empty sample). */
type Read = number | null | undefined;

function read<T>(l: Loaded<T>, pick: (t: T) => number | null): Read {
  if (l === null) return undefined;
  return l.ok ? pick(l.duLieu) : null;
}

const LOADING = "…";

function display(v: Read, format: (n: number) => string): string {
  if (v === undefined) return LOADING;
  return v === null ? NO_VALUE : format(v);
}

/** Which way the figure moved — picks the caption icon only; the words are `comparisonLine`'s. */
type Movement = "up" | "down" | "same" | "none";

/** Mirrors `comparisonLine`'s branches: a movement exists only against a non-zero previous value. */
function movement(current: Read, previous: Read): Movement {
  if (typeof current !== "number" || typeof previous !== "number" || previous === 0) return "none";
  if (current === previous) return "same";
  return current > previous ? "up" : "down";
}

/** One figure, fully decided — what the cells draw. */
type Figure = {
  readonly id: string;
  /** The receiving list's heading (`drillTargetLabel`) — also the link's accessible name. */
  readonly label: string;
  readonly display: string;
  readonly href: string;
  readonly comparison: ComparisonLine | null;
  readonly movement?: Movement;
  readonly note?: string;
  /** red value, for the figures spec §4 paints red (overdue, late) — and only when non-zero */
  readonly alert?: boolean;
  /** the value is a server SENTENCE (fiscal reason), not a number */
  readonly sentence?: boolean;
  /** the call answered and the value is exactly 0 — "nothing pending" for the attention block */
  readonly zero?: boolean;
  readonly icon?: LucideIcon;
  /** Set on a RATE figure only: its percentage, or `null` for an empty sample. */
  readonly ratio?: number | null;
};

/**
 * "Nhiệm vụ" + "Quá hạn" → "Nhiệm vụ quá hạn". Used where figures of two modules sit side by side
 * and the list heading alone ("Quá hạn") would not say which register it counts. Built from the
 * receiving list's own heading, so the two still cannot drift; the link keeps the heading as name.
 */
function withModule(noun: string, label: string): string {
  return `${noun} ${label.charAt(0).toLocaleLowerCase("vi")}${label.slice(1)}`;
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * FIGURES — what is counted, compared and linked. Unchanged by the redesign.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * A count cell. NO `label`: a count cell is named by the list it opens (`drillTargetLabel`, from
 * the receiving side's table in `lib/drill-down.ts`), so the figure and the heading of its list
 * cannot drift apart.
 */
type CountSpec<T> = {
  readonly pick: (t: T) => number;
  readonly kind: FigureKind;
  readonly trend: Trend;
  readonly target: DrillTarget;
  readonly alert?: boolean;
  readonly icon: LucideIcon;
};

function countFigure<T>(
  blockId: string,
  pair: SummaryPair<T>,
  spec: CountSpec<T>,
  period: { from: string; to: string },
): Figure {
  const current = read(pair.current, spec.pick);
  const previous = read(pair.previous, spec.pick);
  return {
    id: `${blockId}-${spec.target.metric}`,
    label: drillTargetLabel(spec.target, period) ?? spec.target.metric,
    display: display(current, formatCount),
    href: drillHref(spec.target, period),
    comparison:
      current === undefined || previous === undefined
        ? null
        : comparisonLine(spec.kind, current, previous, spec.trend, formatCount),
    movement: movement(current, previous),
    alert: spec.alert === true && typeof current === "number" && current > 0,
    zero: current === 0,
    icon: spec.icon,
  };
}

type RatioSpec<T> = {
  readonly label: string;
  readonly numerator: (t: T) => number;
  readonly sample: (t: T) => number;
  /** `việc` / `phiếu` — what the sample counts */
  readonly unit: string;
  readonly target: DrillTarget;
};

function ratioFigure<T>(
  blockId: string,
  pair: SummaryPair<T>,
  spec: RatioSpec<T>,
  period: { from: string; to: string },
): Figure {
  const pick = (t: T) => ratioPercent(spec.numerator(t), spec.sample(t));
  const current = read(pair.current, pick);
  const previous = read(pair.previous, pick);
  const c = pair.current;
  return {
    id: `${blockId}-${spec.target.metric}`,
    label: spec.label,
    display: display(current, formatPercent),
    href: drillHref(spec.target, period),
    note:
      c !== null && c.ok
        ? `${formatCount(spec.numerator(c.duLieu))}/${formatCount(spec.sample(c.duLieu))} ${spec.unit} có hạn`
        : undefined,
    comparison:
      current === undefined || previous === undefined
        ? null
        : comparisonLine("period", current, previous, "higher-is-better", formatPercent),
    movement: movement(current, previous),
    icon: Target,
    ratio: typeof current === "number" ? current : null,
  };
}

/** Khối NHIỆM VỤ's figures — spec §4.1, labels per the user's decision. */
function taskFigures(pair: SummaryPair<petitions_taskSummaryOut>, windows: ComparedWindows) {
  const period = toQueryPeriod(windows.current);
  const id = "tasks";
  const count = (s: CountSpec<petitions_taskSummaryOut>) => countFigure(id, pair, s, period);
  return {
    inProgress: count({
      pick: (t) => t.in_progress,
      kind: "stock",
      trend: "neutral",
      target: { list: "tasks", metric: "in_progress" },
      icon: Loader,
    }),
    overdue: count({
      pick: (t) => t.overdue,
      kind: "stock",
      trend: "lower-is-better",
      target: { list: "tasks", metric: "overdue" },
      alert: true,
      icon: AlarmClock,
    }),
    suspended: count({
      pick: (t) => t.suspended,
      kind: "stock",
      trend: "neutral",
      target: { list: "tasks", metric: "suspended" },
      icon: CirclePause,
    }),
    completed: count({
      pick: (t) => t.completed,
      kind: "period",
      trend: "higher-is-better",
      target: { list: "tasks", metric: "completed" },
      icon: CircleCheck,
    }),
    onTime: ratioFigure(
      id,
      pair,
      {
        label: "Đúng hạn trong kỳ",
        numerator: (t) => t.on_time,
        sample: (t) => t.on_time_sample,
        unit: "việc",
        target: { list: "tasks", metric: "on_time" },
      },
      period,
    ),
  };
}

/**
 * Khối VĂN BẢN ĐẾN's figures — incoming documents only. The spec's block also counts citizen
 * letters (`don_thu`), which no service holds yet; that cell and the on-time rate (the register
 * stores no settled instant, `incoming_dashboard.go`) say so in `DocumentSourcesBlock`.
 */
function incomingDocumentFigures(
  pair: SummaryPair<documents_incomingSummaryOut>,
  windows: ComparedWindows,
) {
  const period = toQueryPeriod(windows.current);
  const id = "incoming-documents";
  const count = (s: CountSpec<documents_incomingSummaryOut>) => countFigure(id, pair, s, period);
  const c = pair.current;
  const asOf = c !== null && c.ok ? Date.parse(c.duLieu.as_of) : Number.NaN;
  const stockNote = Number.isNaN(asOf) ? undefined : `tính đến ${formatDateTime(asOf)}`;
  return {
    arrived: count({
      pick: (d) => d.arrived,
      kind: "period",
      trend: "neutral",
      target: { list: "incoming-documents", metric: "arrived" },
      icon: Inbox,
    }),
    open: {
      ...count({
        pick: (d) => d.open,
        kind: "stock",
        trend: "neutral",
        target: { list: "incoming-documents", metric: "open" },
        icon: Hourglass,
      }),
      note: stockNote,
    },
    overdue: {
      ...count({
        pick: (d) => d.overdue,
        kind: "stock",
        trend: "lower-is-better",
        target: { list: "incoming-documents", metric: "overdue" },
        alert: true,
        icon: AlarmClock,
      }),
      note: stockNote,
    },
  };
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * PRESENTATION PRIMITIVES
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

const TONE_TEXT: Readonly<Record<ComparisonLine["tone"], string>> = {
  better: "text-success-600",
  worse: "text-danger-600",
  neutral: "text-ink-500",
};

const MOVEMENT_ICON: Readonly<Record<Movement, LucideIcon>> = {
  up: TrendingUp,
  down: TrendingDown,
  same: Minus,
  none: History,
};

/** Module tiles: brand or neutral ONLY — red and orange are reserved for real states (spec §8.2). */
type TileTone = "brand" | "neutral";

const TILE: Readonly<Record<TileTone, string>> = {
  brand: "bg-brand-50 text-brand-600",
  neutral: "bg-[#f1f4f8] text-ink-500",
};

/**
 * A card with an icon tile + `<h2>`. The `<section aria-label>` is the landmark screen readers (and
 * the tests) find each block by.
 */
function SectionCard({
  title,
  icon: Icon,
  tile = "brand",
  aside,
  className,
  bodyClassName,
  children,
}: {
  title: string;
  icon: LucideIcon;
  tile?: TileTone;
  aside?: ReactNode;
  className?: string;
  bodyClassName?: string;
  children: ReactNode;
}) {
  return (
    <Card as="section" aria-label={title} className={cn("flex flex-col", className)}>
      <CardHeader>
        <span aria-hidden="true" className={cn("grid size-9 shrink-0 place-items-center rounded-lg", TILE[tile])}>
          <Icon className="size-[18px]" strokeWidth={1.8} focusable="false" />
        </span>
        <CardTitle className="min-w-0 flex-1">{title}</CardTitle>
        {aside}
      </CardHeader>
      <CardContent className={cn("flex flex-1 flex-col gap-3", bodyClassName)}>{children}</CardContent>
    </Card>
  );
}

/**
 * The value of a figure, inside the element `aria-describedby` points at. While loading, the
 * text stays "…" for assistive technology and a skeleton bar of the figure's size is drawn instead,
 * so the layout does not jump when the number arrives (spec §8b).
 */
function FigureValue({
  id,
  text,
  sentence = false,
  skeleton = "h-[26px] w-14",
}: {
  id: string;
  text: ReactNode;
  sentence?: boolean;
  skeleton?: string;
}) {
  if (text === LOADING) {
    return (
      <>
        <span
          aria-hidden="true"
          className={cn("inline-block animate-pulse rounded-md bg-[#eef1f5] align-middle motion-reduce:animate-none", skeleton)}
        />
        <span id={id} className="an-thi-giac">
          {LOADING}
        </span>
      </>
    );
  }
  return (
    <span
      id={id}
      className={cn(
        "[overflow-wrap:anywhere]",
        sentence && "block text-[13px] leading-snug font-medium text-ink-700",
      )}
    >
      {text}
    </span>
  );
}

function ComparisonCaption({ figure }: { figure: Figure }) {
  if (figure.comparison === null) return null;
  const Icon = MOVEMENT_ICON[figure.movement ?? "none"];
  return (
    <span className={cn("inline-flex items-center gap-1", TONE_TEXT[figure.comparison.tone])}>
      <Icon aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-3.5 shrink-0" />
      {figure.comparison.text}
    </span>
  );
}

/**
 * A load error — spec §8b: `CloudOff`, what failed, the server's sentence VERBATIM, and "Tải lại".
 *
 * "Tải lại" is the page's existing reload: the period handler called with the period already
 * selected — exactly what pressing the pressed period button does. No new mechanism.
 */
function LoadError({
  title,
  children,
  onReload,
}: {
  title?: string;
  children: ReactNode;
  onReload?: () => void;
}) {
  return (
    <div
      role="alert"
      className="flex flex-wrap items-start gap-3 rounded-xl border border-danger-200 bg-danger-50 px-3.5 py-3 text-[13px] leading-relaxed text-ink-700"
    >
      <CloudOff aria-hidden="true" focusable="false" strokeWidth={1.8} className="mt-0.5 size-[18px] shrink-0 text-danger-600" />
      <div className="min-w-0 flex-1 basis-48">
        {title !== undefined && <p className="m-0 font-semibold text-danger-600">{title}</p>}
        <p className="m-0">{children}</p>
      </div>
      {onReload !== undefined && (
        <Button
          type="button"
          size="sm"
          variant="secondary"
          onClick={onReload}
          icon={<RotateCw aria-hidden="true" focusable="false" strokeWidth={1.8} />}
        >
          Tải lại
        </Button>
      )}
    </div>
  );
}

/** The error lines of a pair — the current call's sentence, and the previous call's if it failed. */
function PairErrors<T>({
  pair,
  noun,
  onReload,
}: {
  pair: SummaryPair<T>;
  /** lower-case module noun: `nhiệm vụ`, `văn bản đến`, `phản ánh` */
  noun: string;
  onReload?: () => void;
}) {
  return (
    <>
      {pair.current !== null && !pair.current.ok && (
        <LoadError title={`Chưa tải được số liệu ${noun}`} onReload={onReload}>
          {pair.current.thongBao}
        </LoadError>
      )}
      {pair.current !== null && pair.current.ok && pair.previous !== null && !pair.previous.ok && (
        <LoadError title={`Chưa tải được số liệu kỳ trước — ${noun}`} onReload={onReload}>
          Không tải được số liệu kỳ trước: {pair.previous.thongBao}
        </LoadError>
      )}
    </>
  );
}

const LINK_FRAME =
  "block h-full rounded-xl text-inherit no-underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-500";

/**
 * One KPI: a LINK named `Xem danh sách đằng sau: {nhãn}` (spec §4, §9) to the list behind it.
 *
 * THE ACCESSIBLE NAME REPLACES THE VISIBLE TEXT, so the value is wired back with
 * `aria-describedby` — otherwise a screen-reader user hears where the link goes and never the
 * number it is about.
 */
function KpiCell({ figure, shown, tone = "brand" }: { figure: Figure; shown: string; tone?: StatTone }) {
  const valueId = `${figure.id}-value`;
  const hasCaption = figure.note !== undefined || figure.comparison !== null;
  return (
    <li className="min-w-0">
      <Link className={LINK_FRAME} href={figure.href} aria-label={drillLabel(figure.label)} aria-describedby={valueId}>
        <StatCard
          icon={figure.icon ?? Database}
          tone={tone}
          label={shown}
          alert={figure.alert === true}
          value={<FigureValue id={valueId} text={figure.display} />}
          caption={
            hasCaption ? (
              <span className="flex min-w-0 flex-col gap-0.5">
                {figure.note !== undefined && <span>{figure.note}</span>}
                <ComparisonCaption figure={figure} />
              </span>
            ) : undefined
          }
          className="shadow-none"
        />
      </Link>
    </li>
  );
}

/**
 * A RATE figure: label, the percentage with its sample `(x/y …)`, a thin bar, the comparison.
 * Same link, same name, same `-value` wiring as a KPI.
 *
 * The bar is a native `<progress>` hidden from assistive technology: the percentage beside it is
 * the accessible value, and a bar announced as "0%" for an EMPTY sample would say the opposite.
 */
function RatioRow({ figure, shown }: { figure: Figure; shown: string }) {
  const valueId = `${figure.id}-value`;
  const Icon = figure.icon ?? Target;
  const percent = figure.ratio ?? null;
  return (
    <Link
      className={cn(LINK_FRAME, "border border-line px-4 py-3 hover:border-line-strong")}
      href={figure.href}
      aria-label={drillLabel(figure.label)}
      aria-describedby={valueId}
    >
      <span className="flex flex-wrap items-center gap-x-2 gap-y-1 text-[13px] text-ink-700">
        <Icon aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-4 shrink-0 text-ink-500" />
        <span className="font-medium">{shown}</span>
        <span className="ml-auto inline-flex items-baseline gap-1.5 tabular-nums">
          <span className="font-bold text-ink-900">
            <FigureValue id={valueId} text={figure.display} skeleton="h-4 w-10" />
          </span>
          {figure.note !== undefined && <span className="text-ink-500">({figure.note})</span>}
        </span>
      </span>
      <progress
        aria-hidden="true"
        max={100}
        value={percent === null ? 0 : Math.min(100, Math.max(0, percent))}
        className={cn(
          "mt-2.5 block h-1.5 w-full appearance-none overflow-hidden rounded-full border-0 bg-[#eef1f5]",
          "[&::-webkit-progress-bar]:rounded-full [&::-webkit-progress-bar]:bg-[#eef1f5]",
          "[&::-webkit-progress-value]:rounded-full [&::-webkit-progress-value]:bg-brand-600",
          "[&::-moz-progress-bar]:rounded-full [&::-moz-progress-bar]:bg-brand-600",
        )}
      />
      {figure.comparison !== null && (
        <span className="mt-1.5 flex text-xs">
          <ComparisonCaption figure={figure} />
        </span>
      )}
    </Link>
  );
}

/**
 * One label–value row of a compact module card. The value is the link; an `::after` overlay
 * stretches it over the whole row, so the row is one click target and the `<dl>` stays valid.
 */
function MetricRow({ figure }: { figure: Figure }) {
  const valueId = `${figure.id}-value`;
  const caption =
    figure.note !== undefined || figure.comparison !== null ? (
      <dd className="m-0 flex flex-col gap-0.5 text-xs text-ink-500">
        {figure.note !== undefined && <span>{figure.note}</span>}
        <ComparisonCaption figure={figure} />
      </dd>
    ) : null;
  return (
    <div className="relative rounded-lg px-2 py-2 hover:bg-brand-50/60">
      <div className={cn("flex gap-3", figure.sentence === true ? "flex-col gap-1" : "items-baseline justify-between")}>
        <dt className="min-w-0 text-[13px] text-ink-700">{figure.label}</dt>
        <dd
          className={cn(
            "m-0 min-w-0 font-semibold tabular-nums",
            figure.sentence === true ? "" : "shrink-0 text-right text-[15px]",
            figure.alert === true ? "text-danger-600" : "text-ink-900",
          )}
        >
          <Link
            href={figure.href}
            aria-label={drillLabel(figure.label)}
            aria-describedby={valueId}
            className="text-inherit no-underline after:absolute after:inset-0 after:rounded-lg focus-visible:outline-none focus-visible:after:outline-2 focus-visible:after:outline-brand-500"
          >
            {figure.sentence === true && (
              <Database aria-hidden="true" focusable="false" strokeWidth={1.8} className="mr-1.5 inline size-3.5 text-ink-400" />
            )}
            <FigureValue id={valueId} text={figure.display} sentence={figure.sentence} skeleton="h-4 w-10" />
          </Link>
        </dd>
      </div>
      {caption}
    </div>
  );
}

/** A row with no source data in wave 1: "Chưa có dữ liệu", never a 0, never a link. */
function NoSourceRow({ label }: { label: string }) {
  return (
    <div className="flex items-baseline justify-between gap-3 px-2 py-2" title={NO_SOURCE_DATA}>
      <dt className="min-w-0 text-[13px] text-ink-700">{label}</dt>
      <dd className="m-0 inline-flex shrink-0 items-center gap-1 text-xs text-ink-400">
        <Database aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-3.5" />
        {NO_DATA_CAPTION}
      </dd>
    </div>
  );
}

function MetricList({ children }: { children: ReactNode }) {
  return <dl className="-mx-2 m-0 flex flex-col">{children}</dl>;
}

function SmallNote({ children }: { children: ReactNode }) {
  return <p className="m-0 text-xs leading-relaxed text-ink-500">{children}</p>;
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * BLOCKS
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * "CẦN XỬ LÝ" — the pending work, built ONLY from figures the page already reads: overdue tasks,
 * open and overdue incoming documents. "Đề nghị lùi hạn chờ duyệt" is in the spec but the page
 * reads no such figure, and the redesign adds no call.
 *
 * NO TOTAL BADGE: the rows overlap (an overdue document is also an open one), so their sum would
 * count it twice — a false figure on a public authority's screen.
 *
 * "Không có việc tồn đọng" ONLY when every row shown has ANSWERED with 0. A loading or failed row
 * keeps the list: an outage must never read as "nothing pending".
 */
export function AttentionBlock({
  tasks,
  incomingDocuments,
  windows,
  showTasks,
  showDocuments,
}: {
  tasks: SummaryPair<petitions_taskSummaryOut>;
  incomingDocuments: SummaryPair<documents_incomingSummaryOut>;
  windows: ComparedWindows;
  showTasks: boolean;
  showDocuments: boolean;
}) {
  const rows: { figure: Figure; title: string; tone: TodoItem["tone"] }[] = [];
  if (showTasks) {
    const t = taskFigures(tasks, windows);
    rows.push({
      figure: t.overdue,
      title: withModule("Nhiệm vụ", t.overdue.label),
      tone: t.overdue.alert === true ? "danger" : "neutral",
    });
  }
  if (showDocuments) {
    const d = incomingDocumentFigures(incomingDocuments, windows);
    rows.push({
      figure: d.open,
      title: withModule("Văn bản", d.open.label),
      // amber = "waiting", a real state — only for a number above 0, never for loading or "—"
      tone:
        d.open.zero === true || d.open.display === NO_VALUE || d.open.display === LOADING
          ? "neutral"
          : "warning",
    });
    rows.push({
      figure: d.overdue,
      title: withModule("Văn bản", d.overdue.label),
      tone: d.overdue.alert === true ? "danger" : "neutral",
    });
  }
  const nothingPending = rows.length > 0 && rows.every((r) => r.figure.zero === true);
  const items: TodoItem[] = rows.map(({ figure, title, tone }) => ({
    key: figure.id,
    icon: figure.icon ?? Database,
    tone,
    title,
    detail: figure.note,
    alert: figure.alert === true,
    value: <FigureValue id={`${figure.id}-value`} text={figure.display} skeleton="h-6 w-8" />,
    href: figure.href,
    linkLabel: drillLabel(figure.label),
    describedBy: `${figure.id}-value`,
  }));
  return (
    <SectionCard title="Cần xử lý" icon={BellRing} bodyClassName="p-0">
      {nothingPending ? (
        <EmptyState icon={CircleCheck} title="Không có việc tồn đọng" className="py-8" />
      ) : (
        <TodoList items={items} />
      )}
    </SectionCard>
  );
}

/**
 * "TÌNH HÌNH TRONG KỲ" — the period's figures: tasks in progress, completed, suspended, incoming
 * documents arrived, and the tasks' on-time rate as a bar. The previous-period sentence closes the
 * block as a small caption — it explains every "Kỳ trước" line above it.
 */
export function PeriodStatusBlock({
  tasks,
  incomingDocuments,
  windows,
  showTasks,
  showDocuments,
  comparisonText,
}: {
  tasks: SummaryPair<petitions_taskSummaryOut>;
  incomingDocuments: SummaryPair<documents_incomingSummaryOut>;
  windows: ComparedWindows;
  showTasks: boolean;
  showDocuments: boolean;
  /** Replaces the closing sentence — `/bao-cao`'s custom period compares differently (ADR 0053 B2). */
  comparisonText?: string;
}) {
  const t = showTasks ? taskFigures(tasks, windows) : null;
  const d = showDocuments ? incomingDocumentFigures(incomingDocuments, windows) : null;
  return (
    <SectionCard title="Tình hình trong kỳ" icon={Activity}>
      <ul className="m-0 grid list-none grid-cols-1 gap-3 p-0 min-[420px]:grid-cols-2">
        {t !== null && <KpiCell figure={t.inProgress} shown={withModule("Nhiệm vụ", t.inProgress.label)} />}
        {t !== null && <KpiCell figure={t.completed} shown={withModule("Nhiệm vụ", t.completed.label)} />}
        {d !== null && <KpiCell figure={d.arrived} shown={withModule("Văn bản", d.arrived.label)} />}
        {t !== null && (
          <KpiCell figure={t.suspended} shown={withModule("Nhiệm vụ", t.suspended.label)} tone="neutral" />
        )}
      </ul>
      {t !== null && <RatioRow figure={t.onTime} shown={withModule("Nhiệm vụ", t.onTime.label)} />}
      <p className="m-0 mt-auto flex items-start gap-1.5 pt-1 text-xs leading-relaxed text-ink-500">
        <History aria-hidden="true" focusable="false" strokeWidth={1.8} className="mt-px size-3.5 shrink-0" />
        <span className="min-w-0">{comparisonText ?? comparisonNote(windows)}</span>
      </p>
    </SectionCard>
  );
}

/**
 * "VĂN BẢN & ĐƠN THƯ" — the two figures of the spec's document block that are not drawn yet.
 *
 * The on-time rate is a NO-SOURCE row (the register stores no settled instant): a data gap, not an
 * unbuilt feature, so it keeps "Chưa có dữ liệu". "Đơn thư trong kỳ" is an unbuilt part — there is
 * no citizen-letter register to count — so it is the KPI card it will be, with the "?" of ADR 0068
 * §14 (`labels.ts`).
 */
export function DocumentSourcesBlock() {
  return (
    <SectionCard title="Văn bản & Đơn thư" icon={Mail}>
      <MetricList>
        <NoSourceRow label="Tỷ lệ đúng hạn văn bản" />
      </MetricList>
      <PendingStatCard info={pendingPart("Đơn thư trong kỳ")} icon={Inbox} />
    </SectionCard>
  );
}

/** Khối PHẢN ÁNH NGƯỜI DÂN — spec §4.5, labels per the user's decision. */
export function CitizenReportBlock({
  pair,
  windows,
  onReload,
}: {
  pair: SummaryPair<petitions_citizenReportSummaryOut>;
  windows: ComparedWindows;
  onReload?: () => void;
}) {
  const period = toQueryPeriod(windows.current);
  const id = "citizen-reports";
  const count = (s: CountSpec<petitions_citizenReportSummaryOut>) =>
    countFigure(id, pair, s, period);
  const figures: Figure[] = [
    count({
      pick: (r) => r.received,
      kind: "period",
      trend: "neutral",
      target: { list: "citizen-reports", metric: "received" },
      icon: Inbox,
    }),
    count({
      pick: (r) => r.in_progress,
      kind: "stock",
      trend: "neutral",
      target: { list: "citizen-reports", metric: "in_progress" },
      icon: Loader,
    }),
    ratioFigure(
      id,
      pair,
      {
        label: "Đúng hạn trong kỳ",
        numerator: (r) => r.on_time,
        sample: (r) => r.on_time_sample,
        unit: "phiếu",
        target: { list: "citizen-reports", metric: "on_time" },
      },
      period,
    ),
    count({
      pick: (r) => r.late,
      kind: "period",
      trend: "lower-is-better",
      target: { list: "citizen-reports", metric: "late" },
      alert: true,
      icon: AlarmClock,
    }),
    ratingFigure(id, pair.current, period),
  ];
  return (
    <SectionCard title="Phản ánh người dân" icon={MessageSquareWarning}>
      <PairErrors pair={pair} noun="phản ánh" onReload={onReload} />
      <MetricList>
        {figures.map((f) => (
          <MetricRow key={f.id} figure={f} />
        ))}
      </MetricList>
    </SectionCard>
  );
}

/**
 * Spec §4.5's fifth figure, "Điểm hài lòng" — the average star rating, from the SAME response the
 * block already reads (`rating_sum` / `rating_sample` of citizen-report-summary), divided by the
 * Phản ánh screen's own `ratingAverage`, so the two screens cannot show two averages.
 *
 * A STOCK figure (the register as it stands, `lib/drill-down.ts`): no comparison line, and the link
 * opens the rated petitions without a period. An empty sample is "—" plus the Phản ánh screen's
 * sentence, NEVER 0 or 0,0/5. A server that predates the two fields gives "—" with no sentence.
 */
function ratingFigure(
  blockId: string,
  current: Loaded<petitions_citizenReportSummaryOut>,
  period: { from: string; to: string },
): Figure {
  const r = current !== null && current.ok ? current.duLieu : null;
  const sample = r?.rating_sample;
  const known = typeof sample === "number" && typeof r?.rating_sum === "number";
  const average = r === null ? null : ratingAverage(r.rating_sum, sample);
  const target: DrillTarget = { list: "citizen-reports", metric: "rating_sample" };
  return {
    id: `${blockId}-rating`,
    label: "Điểm hài lòng",
    display: current === null ? LOADING : (average ?? NO_VALUE),
    href: drillHref(target, period),
    note: !known ? undefined : average === null ? KPI_NO_RATING : `${formatCount(sample)} phiếu được chấm`,
    comparison: null,
    icon: Star,
  };
}

/** Appends "đồng" after a number — never after `—` or a server sentence (as `bang-thu-chi.tsx`). */
function withUnit(text: string, hasNumber: boolean): string {
  return hasNumber && text !== "Không đọc được" ? `${text} đồng` : text;
}

function indicatorFigure(id: string, c: finance_chiSoRa): Figure {
  return {
    id,
    // THE NAME COMES FROM THE SERVER (`ChiSoDatDuToan`), as on the Thu - Chi screen's own card.
    label: c.name,
    display: nhanChiSo(c),
    href: FISCAL_SCREEN_PATH,
    comparison: null,
    sentence: c.basis_points === null,
  };
}

/**
 * Khối THU – CHI NGÂN SÁCH — year to date, NO comparison (user decision).
 *
 * EVERY WORD AND EVERY "NO VALUE" RULE IS THE THU - CHI SCREEN'S OWN (`features/thu-chi/
 * nhan-thu-chi.ts`, `bang-thu-chi.tsx` `TheChiSoNam`), reused, not re-written: a `null` with the
 * server's reason renders that SENTENCE instead of a figure (ADR 0035 §A), never `0`. The redesign
 * frames it as a muted "no data" line but does NOT replace it with a generic "Chưa có dữ liệu": the
 * sentence names what the commune has to do, which a sentence guessed by the web cannot.
 *
 * `NHAN_CHENH_LECH` IS THE TEMPORARY LABEL of the balance cell: open question #32 is disputed and
 * the user decided on 25/09/2026 to ask the customer and relabel the KPI meanwhile
 * (`kb/50-doi-chieu/2026-09-25-feat-m8-multitenant-foundation.md`, summary row 6). The note under
 * the block states how the figure is computed and that it awaits the customer.
 */
export function FiscalBlock({
  result,
  year,
  onReload,
}: {
  result: Loaded<finance_chiSoNamRa>;
  year: number;
  onReload?: () => void;
}) {
  const title = "Thu – Chi ngân sách";
  const scope = `Luỹ kế năm ${year}, không so với kỳ trước.`;

  if (result === null || !result.ok) {
    const shown = result === null ? LOADING : NO_VALUE;
    const fixed = ["Thu đạt dự toán", "Chi đạt dự toán", NHAN_CHENH_LECH];
    return (
      <SectionCard title={title} icon={Wallet}>
        {result !== null && (
          <LoadError title="Chưa tải được số liệu thu – chi" onReload={onReload}>
            {result.thongBao}
          </LoadError>
        )}
        <MetricList>
          {fixed.map((label, i) => (
            <MetricRow
              key={label}
              figure={{ id: `fiscal-${i}`, label, display: shown, href: FISCAL_SCREEN_PATH, comparison: null }}
            />
          ))}
        </MetricList>
        <SmallNote>{scope}</SmallNote>
      </SectionCard>
    );
  }

  const d = result.duLieu;
  const balanceText = nhanSoTienChiSo(d.balance, "dong");
  return (
    <SectionCard title={title} icon={Wallet}>
      <MetricList>
        <MetricRow figure={indicatorFigure("fiscal-revenue", d.revenue_achievement)} />
        <MetricRow figure={indicatorFigure("fiscal-expenditure", d.expenditure_achievement)} />
        <MetricRow
          figure={{
            id: "fiscal-balance",
            label: NHAN_CHENH_LECH,
            display: withUnit(balanceText, d.balance.amount !== null),
            href: FISCAL_SCREEN_PATH,
            comparison: null,
            sentence: d.balance.amount === null,
          }}
        />
        {d.revenue_totals.map((o) => {
          const reason = lyDoKhongTinh(o.unavailable_reason);
          const valueId = `fiscal-total-${o.column_id}-value`;
          return (
            <div key={o.column_id} className="relative flex flex-col gap-1 rounded-lg px-2 py-2 hover:bg-brand-50/60">
              <dt className="min-w-0 text-[13px] text-ink-700">{o.name}</dt>
              <dd className="m-0 min-w-0 text-[15px] font-semibold text-ink-900 tabular-nums">
                <Link
                  href={FISCAL_SCREEN_PATH}
                  aria-label={drillLabel(o.name)}
                  aria-describedby={valueId}
                  className="text-inherit no-underline after:absolute after:inset-0 after:rounded-lg focus-visible:outline-none focus-visible:after:outline-2 focus-visible:after:outline-brand-500"
                >
                  <span
                    id={valueId}
                    className={cn("[overflow-wrap:anywhere]", reason !== null && "text-[13px] font-medium text-ink-700")}
                  >
                    <OTien chu={withUnit(nhanSoTien(o.value, "dong"), o.value !== null)} lyDo={reason} hienLyDo />
                  </span>
                </Link>
              </dd>
            </div>
          );
        })}
      </MetricList>
      <div className="mt-auto flex flex-col gap-1">
        <SmallNote>{scope}</SmallNote>
        <SmallNote>
          {NHAN_CHENH_LECH}: {GHI_CHU_CHENH_LECH}
        </SmallNote>
      </div>
    </SectionCard>
  );
}

/** Task-type labels by code, or `null` when the catalogue did not load (rows then show the code). */
export type TaskTypeLabels = ReadonlyMap<string, string> | null;

/**
 * "CẦN XỬ LÝ NGAY" — spec §5, cut down by the user's decision to what may be shown on the most
 * widely seen screen of the product: module, code, which deadline, field/category, the deadline
 * itself, and the server's "critical" judgement. No title, content, unit or person (rule 3).
 *
 * A FAILED QUEUE IS AN ERROR LINE IN THE LIST, and the "nothing urgent" sentence appears ONLY when
 * every queue answered and none had a row. A 503 from the working calendar must never read as calm.
 */
export function UrgentPanel({
  queue,
  taskTypeLabels,
}: {
  queue: Loaded<MergedQueue>;
  taskTypeLabels: TaskTypeLabels;
}) {
  const title = "Cần xử lý ngay";
  if (queue === null || !queue.ok) {
    return (
      <SectionCard title={title} icon={Siren}>
        {queue === null ? (
          <div role="status" className="flex flex-col gap-2">
            <span className="an-thi-giac">Đang tải…</span>
            {[0, 1, 2].map((i) => (
              <span
                key={i}
                aria-hidden="true"
                className="block h-5 animate-pulse rounded-md bg-[#eef1f5] motion-reduce:animate-none"
              />
            ))}
          </div>
        ) : (
          <LoadError title="Chưa tải được danh sách cần xử lý ngay">{queue.thongBao}</LoadError>
        )}
      </SectionCard>
    );
  }
  const { rows, failures } = queue.duLieu;
  return (
    <SectionCard
      title={title}
      icon={Siren}
      aside={
        <Badge tone={rows.length > 0 ? "danger" : "neutral"} className="tabular-nums">
          {rows.length}
        </Badge>
      }
    >
      {failures.map((f) => (
        <LoadError key={f.module}>
          {MODULE_LABEL[f.module]}: {f.message}
        </LoadError>
      ))}
      {rows.length === 0 && failures.length === 0 && (
        <EmptyState icon={CircleCheck} title={NOTHING_URGENT} className="py-6" />
      )}
      {rows.length > 0 && (
        <ul className="-mx-4 m-0 list-none divide-y divide-line p-0">
          {rows.map((r) => {
            const category = categoryLabel(r, taskTypeLabels);
            return (
              <li
                key={`${r.module}|${r.code}|${r.kind}`}
                className="flex flex-wrap items-center gap-x-3 gap-y-1.5 px-4 py-2.5 text-[13px] text-ink-700"
              >
                <Badge tone="info">{MODULE_LABEL[r.module]}</Badge>
                <span className="font-semibold text-ink-900 tabular-nums [overflow-wrap:anywhere]">{r.code}</span>
                <span>{kindLabel(r.kind)}</span>
                {category !== null && <span className="text-ink-500">{category}</span>}
                <span className="inline-flex items-center gap-1 text-danger-600">
                  <AlarmClock aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-3.5 shrink-0" />
                  {missedSinceText(r.missedAt)}
                </span>
                {r.critical && (
                  <Badge tone="danger" className="sm:ml-auto">
                    Nghiêm trọng
                  </Badge>
                )}
              </li>
            );
          })}
        </ul>
      )}
    </SectionCard>
  );
}

/**
 * The period picker — four toggle buttons, one pressed, drawn as a segmented control. It emits the
 * very `PeriodKind` the buttons did: an unknown string (impossible — the options are `PERIOD_KINDS`)
 * is dropped rather than cast.
 */
export function PeriodPicker({
  kind,
  onChange,
}: {
  kind: PeriodKind;
  onChange: (k: PeriodKind) => void;
}) {
  return (
    <Segmented
      mode="buttons"
      legend="Kỳ báo cáo"
      name="dashboard-period"
      value={kind}
      options={PERIOD_KINDS.map((k) => ({ value: k, label: PERIOD_BUTTON_LABEL[k] }))}
      onChange={(v) => {
        const picked = PERIOD_KINDS.find((k) => k === v);
        if (picked !== undefined) onChange(picked);
      }}
    />
  );
}

/** Everything the page shows, already loaded (or loading) — the hook half assembles this. */
export type DashboardData = BlocksData & {
  readonly windows: PeriodWindows;
  /** The instant the windows were computed and the figures asked for. */
  readonly fetchedAt: number;
};

/** What the blocks under the context row draw — shared by `/tong-quan` and `/bao-cao`. */
export type BlocksData = {
  readonly windows: ComparedWindows;
  readonly tasks: SummaryPair<petitions_taskSummaryOut>;
  readonly incomingDocuments: SummaryPair<documents_incomingSummaryOut>;
  readonly citizenReports: SummaryPair<petitions_citizenReportSummaryOut>;
  readonly fiscal: Loaded<finance_chiSoNamRa>;
  readonly fiscalYear: number;
  /** `null` while loading; `{ok:false}` only if the merge itself could not run. */
  readonly queue: Loaded<MergedQueue>;
  readonly taskTypeLabels: TaskTypeLabels;
};

/** The module keys whose blocks exist on this page. `report.read` is the other half of each gate. */
export const BLOCK_KEYS = {
  tasks: QUYEN_XEM_NHIEM_VU,
  incomingDocuments: QUYEN_XEM_VAN_BAN,
  citizenReports: QUYEN_XEM_PHAN_ANH,
  budget: QUYEN_XEM_GIAI_NGAN,
} as const;

export type BlockVisibility = Readonly<Record<keyof typeof BLOCK_KEYS, boolean>>;

/** Which blocks the permissions open — BOTH keys each (`canSeeBlock`). */
export function blockVisibility(permissions: readonly string[]): BlockVisibility {
  return {
    tasks: canSeeBlock(permissions, BLOCK_KEYS.tasks),
    incomingDocuments: canSeeBlock(permissions, BLOCK_KEYS.incomingDocuments),
    citizenReports: canSeeBlock(permissions, BLOCK_KEYS.citizenReports),
    budget: canSeeBlock(permissions, BLOCK_KEYS.budget),
  };
}

/**
 * The page body. A figure whose keys the account lacks is NOT RENDERED — not drawn as 0, not drawn
 * as "—": a figure the account may not read must not be implied either way.
 *
 * `Kinh tế & Tài nguyên` has no source data and no read key of its own in wave 1; it opens nothing,
 * so it shows under the page gate (`report.read`) alone. It and `Giải ngân ngân sách` are unbuilt
 * parts, drawn as cards with the "?" of ADR 0068 §14 (`labels.ts`) — no figure, no link, no call.
 * `Giải ngân ngân sách` stays under `budget.read`: a placeholder must not reveal a block the account
 * would not see once it is built.
 *
 * THE PAGE TITLE IS NOT HERE: `app/tong-quan/page.tsx` draws the `PageHeader` outside the
 * `report.read` gate, so an account without the key still reads which page it is on. The context
 * row sits directly under it — period and "cập nhật" on the left, aligned with the title; the
 * picker on the right.
 */
export function DashboardView({
  data,
  visible,
  onPeriodChange,
}: {
  data: DashboardData;
  visible: BlockVisibility;
  onPeriodChange: (k: PeriodKind) => void;
}) {
  const reload = () => onPeriodChange(data.windows.kind);
  return (
    <>
      <div className="-mt-3 mb-5 flex flex-wrap items-center gap-x-4 gap-y-3 sm:pl-16">
        <p className="m-0 flex min-w-0 flex-1 basis-72 flex-wrap items-center gap-x-3 gap-y-1 text-[13px] text-ink-500 [&_svg]:size-3.5 [&_svg]:shrink-0">
          <span className="inline-flex items-center gap-1.5">
            <CalendarDays aria-hidden="true" focusable="false" strokeWidth={1.8} />
            {periodMetaLabel(data.windows)}
          </span>
          <span className="inline-flex items-center gap-1.5">
            <RefreshCw aria-hidden="true" focusable="false" strokeWidth={1.8} />
            Cập nhật {formatDateTime(data.fetchedAt)}
          </span>
        </p>
        <PeriodPicker kind={data.windows.kind} onChange={onPeriodChange} />
      </div>

      <DashboardBlocks data={data} visible={visible} onReload={reload} />
    </>
  );
}

/**
 * Every block under the context row, in `/tong-quan`'s order. `/bao-cao` draws the SAME blocks from
 * the same loader, so the two pages cannot show two numbers for one period (spec 13 §10) — minus
 * "Cần xử lý ngay" (`urgent={false}`, spec 13 §1).
 *
 * `onReload` re-asks the period on screen; `comparisonText` replaces the period block's closing
 * sentence when the comparison is not the named-period one.
 */
export function DashboardBlocks({
  data,
  visible,
  onReload,
  urgent = true,
  comparisonText,
}: {
  data: BlocksData;
  visible: BlockVisibility;
  onReload: () => void;
  urgent?: boolean;
  comparisonText?: string;
}) {
  const anyQueue = visible.tasks || visible.incomingDocuments || visible.citizenReports;
  const anyRow1 = visible.tasks || visible.incomingDocuments;
  const reload = onReload;
  return (
    <>
      {anyRow1 && (
        <div className="mb-4 flex flex-col gap-3">
          {visible.tasks && <PairErrors pair={data.tasks} noun="nhiệm vụ" onReload={reload} />}
          {visible.incomingDocuments && (
            <PairErrors pair={data.incomingDocuments} noun="văn bản đến" onReload={reload} />
          )}
        </div>
      )}

      {anyRow1 && (
        <div className="mb-4 grid min-w-0 grid-cols-1 gap-4 xl:grid-cols-[minmax(0,1.3fr)_minmax(0,1fr)]">
          <AttentionBlock
            tasks={data.tasks}
            incomingDocuments={data.incomingDocuments}
            windows={data.windows}
            showTasks={visible.tasks}
            showDocuments={visible.incomingDocuments}
          />
          <PeriodStatusBlock
            tasks={data.tasks}
            incomingDocuments={data.incomingDocuments}
            windows={data.windows}
            showTasks={visible.tasks}
            showDocuments={visible.incomingDocuments}
            comparisonText={comparisonText}
          />
        </div>
      )}

      {urgent && anyQueue && (
        <div className="mb-4">
          <UrgentPanel queue={data.queue} taskTypeLabels={data.taskTypeLabels} />
        </div>
      )}

      <div className="grid min-w-0 grid-cols-1 gap-4 sm:grid-cols-2 2xl:grid-cols-4">
        {visible.citizenReports && (
          <CitizenReportBlock pair={data.citizenReports} windows={data.windows} onReload={reload} />
        )}
        {visible.incomingDocuments && <DocumentSourcesBlock />}
        {visible.budget && <PendingSection info={pendingPart("Giải ngân ngân sách")} titleAs="h2" />}
        {visible.budget && <FiscalBlock result={data.fiscal} year={data.fiscalYear} onReload={reload} />}
        <PendingSection info={pendingPart("Kinh tế & Tài nguyên")} titleAs="h2" />
      </div>
    </>
  );
}
