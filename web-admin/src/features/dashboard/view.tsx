import {
  AlarmClock,
  Banknote,
  CircleCheck,
  CirclePause,
  Clock,
  CloudOff,
  Database,
  History,
  Hourglass,
  Inbox,
  LayoutDashboard,
  ListTodo,
  Loader,
  Mail,
  MessageSquareWarning,
  Minus,
  RotateCw,
  Siren,
  Star,
  Store,
  Target,
  TrendingDown,
  TrendingUp,
  Wallet,
  type LucideIcon,
} from "lucide-react";
import Link from "next/link";
import type { ReactNode } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { PageHeader } from "@/components/ui/page-header";
import { PENDING_HOVER_TEXT, PendingMarker } from "@/components/ui/pending-feature";
import type { PendingFeatureInfo } from "@/components/ui/pending-feature";
import { Segmented } from "@/components/ui/segmented";
import { NO_DATA_CAPTION } from "@/components/ui/stat-card";
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
import { DashboardHeaderActions } from "./header-actions";
import { PENDING_BLOCK_METRICS, pendingPart } from "./labels";
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
 * LAYOUT (owner, 05/10/2026 — the prototype's `DashboardWorkspace` frame, new UI language of
 * ADR 0068 §"Sửa đổi 05/10/2026 (lần 2)"):
 *   header  title · "Kỳ …: … · tính đến …" · period buttons · PDF/XLSX/PPTX · Trình chiếu
 *   body    ONE grid — 1 column, 2 from 1024px, 3 from 1536px — of six blocks in the prototype's
 *           order (Nhiệm vụ · Văn bản & Đơn thư · Giải ngân ngân sách · Thu – Chi ngân sách ·
 *           Phản ánh người dân · Kinh tế & Tài nguyên), then "Cần xử lý ngay" as the 7th cell.
 * The old "Cần xử lý" / "Tình hình trong kỳ" rows are gone as BLOCKS, not as figures: every figure
 * they drew is a tile of its module's block now. EVERY FIGURE THE PAGE SHOWED BEFORE IS STILL SHOWN
 * EXACTLY ONCE, each still a link to its list, under the same read keys (ADR 0068 §1).
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

/** One figure, fully decided — what the tiles draw. */
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
  readonly icon?: LucideIcon;
};

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
    icon: spec.icon,
  };
}

type RatioSpec<T> = {
  readonly label: string;
  readonly numerator: (t: T) => number;
  readonly sample: (t: T) => number;
  /** `việc` / `phiếu` — what the sample counts */
  readonly unit: string;
  /**
   * The note under the figure, when "x/y <unit> có hạn" does not say what the sample is. Must describe
   * the server's own denominator, never a nicer one (TQ-05).
   */
  readonly note?: (numerator: string, sample: string) => string;
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
      c === null || !c.ok
        ? undefined
        : spec.note !== undefined
          ? spec.note(formatCount(spec.numerator(c.duLieu)), formatCount(spec.sample(c.duLieu)))
          : `${formatCount(spec.numerator(c.duLieu))}/${formatCount(spec.sample(c.duLieu))} ${spec.unit} có hạn`,
    comparison:
      current === undefined || previous === undefined
        ? null
        : comparisonLine("period", current, previous, "higher-is-better", formatPercent),
    movement: movement(current, previous),
    icon: Target,
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
 * letters (`don_thu`), which no service holds yet; that tile and the on-time rate (the register
 * stores no settled instant, `incoming_dashboard.go`) say so in `DocumentBlock`.
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

/** The stable key of each block — `data-block` on its `<section>`, what the order tests read. */
export type BlockKey = "tasks" | "documents" | "budget" | "fiscal" | "citizen-reports" | "economy" | "urgent";

/**
 * One block of the grid: a white card (no border, no shadow — ADR 0068 lần 2 #4) with a header row
 * (icon · `<h2>` · an optional aside: a count, or the "?" of an unbuilt block) and its body.
 *
 * The `<section aria-label>` is the landmark screen readers (and the tests) find each block by. The
 * body is a size CONTAINER: the tile grid picks its column count from the width of THE CARD, not of
 * the window — the same card is a third of a 1536px screen and the whole of a phone.
 */
function Panel({
  blockKey,
  title,
  icon: Icon,
  aside,
  children,
}: {
  blockKey: BlockKey;
  title: string;
  icon: LucideIcon;
  aside?: ReactNode;
  children: ReactNode;
}) {
  return (
    <Card as="section" aria-label={title} data-block={blockKey} className="flex flex-col">
      <CardHeader className="gap-2">
        <Icon aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-[18px] shrink-0 text-ink-500" />
        <CardTitle className="min-w-0 flex-1">{title}</CardTitle>
        {aside}
      </CardHeader>
      <CardContent className="@container flex flex-1 flex-col gap-3">{children}</CardContent>
    </Card>
  );
}

/**
 * The tiles of a block: 2 columns, 3 when the block has more than four figures AND its card is wide
 * enough (`@md` = 28rem of card body). Three columns of a 288px phone card would cut "91,30%" in
 * half; the prototype's rule ("3 when > 4") holds wherever it fits.
 */
function TileGrid({ count, children }: { count: number; children: ReactNode }) {
  return (
    <ul
      className={cn(
        "-mx-2.5 m-0 grid list-none grid-cols-2 gap-1 p-0",
        count > 4 && "@md:grid-cols-3",
      )}
    >
      {children}
    </ul>
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
      className="flex flex-wrap items-start gap-3 rounded-lg border border-danger-200 bg-danger-50 px-3.5 py-3 text-[13px] leading-relaxed text-ink-700"
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

/** Value · label · caption — the order of the prototype's `MetricTile`. */
const TILE_BODY = "flex h-full min-w-0 flex-col gap-1 rounded-lg px-2.5 py-2";

/**
 * One figure: a LINK named `Xem danh sách đằng sau: {nhãn}` (spec §4, §9) to the pre-filtered list
 * behind it (`?metric=…`) — never a dialog (ADR 0053 §7).
 *
 * THE ACCESSIBLE NAME REPLACES THE VISIBLE TEXT, so the value is wired back with
 * `aria-describedby` — otherwise a screen-reader user hears where the link goes and never the
 * number it is about.
 *
 * Red is never alone: an alarming figure is red AND carries its clock icon AND its label says
 * "Quá hạn" / "Trễ hạn" (ADR 0068 lần 2 #8b). Hover tints with the accent — a FILL, never text.
 * A server SENTENCE (fiscal reason) takes the whole row: squeezed into one of three columns it
 * would read one word per line.
 */
function FigureTile({ figure, value }: { figure: Figure; value?: ReactNode }) {
  const valueId = `${figure.id}-value`;
  const Icon = figure.icon ?? Database;
  const alert = figure.alert === true;
  const sentence = figure.sentence === true;
  const hasCaption = figure.note !== undefined || figure.comparison !== null;
  return (
    <li className={cn("min-w-0", sentence && "col-span-full")}>
      <Link
        href={figure.href}
        aria-label={drillLabel(figure.label)}
        aria-describedby={valueId}
        className={cn(
          TILE_BODY,
          "text-inherit no-underline transition-colors hover:bg-accent-50",
          "focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-brand-500",
        )}
      >
        <span
          className={cn(
            "font-semibold tabular-nums",
            sentence ? "text-[13px] leading-snug" : "text-[22px] leading-tight",
            alert ? "text-danger-600" : "text-ink-900",
          )}
        >
          {value !== undefined ? (
            <span id={valueId} className="[overflow-wrap:anywhere]">
              {value}
            </span>
          ) : (
            <FigureValue id={valueId} text={figure.display} sentence={sentence} />
          )}
        </span>
        <span className="flex min-w-0 items-start gap-1.5 text-[13px] text-ink-700">
          <Icon
            aria-hidden="true"
            focusable="false"
            strokeWidth={1.8}
            className={cn("mt-px size-3.5 shrink-0", alert ? "text-danger-600" : "text-ink-500")}
          />
          <span className="min-w-0">{figure.label}</span>
        </span>
        {hasCaption && (
          <span className="flex min-w-0 flex-col gap-0.5 text-xs leading-snug text-ink-500">
            {figure.note !== undefined && <span>{figure.note}</span>}
            <ComparisonCaption figure={figure} />
          </span>
        )}
      </Link>
    </li>
  );
}

/** A figure with no source data in wave 1: "Chưa có dữ liệu", never a 0, never a link. */
function NoSourceTile({ label, icon: Icon }: { label: string; icon: LucideIcon }) {
  return (
    <li className={TILE_BODY} title={NO_SOURCE_DATA}>
      <span className="inline-flex items-center gap-1 text-[13px] leading-[26px] text-ink-500">
        <Database aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-3.5 shrink-0" />
        {NO_DATA_CAPTION}
      </span>
      <span className="flex min-w-0 items-start gap-1.5 text-[13px] text-ink-700">
        <Icon aria-hidden="true" focusable="false" strokeWidth={1.8} className="mt-px size-3.5 shrink-0 text-ink-500" />
        <span className="min-w-0">{label}</span>
      </span>
    </li>
  );
}

/**
 * A figure that is an UNBUILT part (ADR 0068 §14): the tile it will be, "—" for the eye and the
 * reason for a screen reader, never a link. `info` given = the tile carries its own "?"; without it
 * the "?" is the block's (a whole unbuilt block has one "?" in its header, not one per tile).
 */
function PendingTile({ label, info }: { label: string; info?: PendingFeatureInfo }) {
  return (
    <li className={TILE_BODY} data-pending="">
      <span className="flex items-center gap-2">
        <span aria-hidden="true" className="text-[22px] leading-tight font-semibold text-ink-400">
          —
        </span>
        <span className="sr-only">{PENDING_HOVER_TEXT}</span>
        {info !== undefined && <PendingMarker info={info} />}
      </span>
      <span className="text-[13px] text-ink-500">{label}</span>
    </li>
  );
}

function SmallNote({ children }: { children: ReactNode }) {
  return <p className="m-0 text-xs leading-relaxed text-ink-500">{children}</p>;
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * BLOCKS — the grid's order: tasks · documents · budget · fiscal · citizen reports · economy
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * Khối NHIỆM VỤ — what the old "Cần xử lý" and "Tình hình trong kỳ" showed of tasks: in progress,
 * overdue, completed, suspended, and the on-time rate. The block's title names the module, so each
 * tile is named by its list's heading alone ("Quá hạn"), as on the receiving list.
 */
export function TaskBlock({
  pair,
  windows,
  onReload,
}: {
  pair: SummaryPair<petitions_taskSummaryOut>;
  windows: ComparedWindows;
  onReload?: () => void;
}) {
  const t = taskFigures(pair, windows);
  const figures = [t.inProgress, t.overdue, t.completed, t.suspended, t.onTime];
  return (
    <Panel blockKey="tasks" title="Nhiệm vụ" icon={ListTodo}>
      <PairErrors pair={pair} noun="nhiệm vụ" onReload={onReload} />
      <TileGrid count={figures.length}>
        {figures.map((f) => (
          <FigureTile key={f.id} figure={f} />
        ))}
      </TileGrid>
    </Panel>
  );
}

/**
 * Khối VĂN BẢN & ĐƠN THƯ — the prototype's second block ("register"). The incoming-document counts
 * of the old "Cần xử lý" / "Tình hình trong kỳ" rows live here now, beside the two figures that are
 * not drawn yet:
 *   · "Tỷ lệ đúng hạn văn bản" is a NO-SOURCE tile (the register stores no settled instant): a data
 *     gap, not an unbuilt feature, so it keeps "Chưa có dữ liệu";
 *   · "Đơn thư trong kỳ" is an unbuilt part — there is no citizen-letter register to count — so it
 *     is the tile it will be, with the "?" of ADR 0068 §14 (`labels.ts`).
 */
export function DocumentBlock({
  pair,
  windows,
  onReload,
}: {
  pair: SummaryPair<documents_incomingSummaryOut>;
  windows: ComparedWindows;
  onReload?: () => void;
}) {
  const d = incomingDocumentFigures(pair, windows);
  const figures = [d.arrived, d.open, d.overdue];
  return (
    <Panel blockKey="documents" title="Văn bản & Đơn thư" icon={Mail}>
      <PairErrors pair={pair} noun="văn bản đến" onReload={onReload} />
      <TileGrid count={figures.length + 2}>
        {figures.map((f) => (
          <FigureTile key={f.id} figure={f} />
        ))}
        <NoSourceTile label="Tỷ lệ đúng hạn văn bản" icon={Target} />
        <PendingTile label="Đơn thư trong kỳ" info={pendingPart("Đơn thư trong kỳ")} />
      </TileGrid>
    </Panel>
  );
}

/**
 * A whole block that is an unbuilt part (Giải ngân ngân sách, Kinh tế & Tài nguyên): it stays at its
 * place in the grid, with the tiles it will have — "—", no figure, no link, no call — and ONE "?" in
 * its header that reads the reason from `labels.ts`.
 */
function PendingBlock({
  blockKey,
  name,
  icon,
}: {
  blockKey: BlockKey;
  name: keyof typeof PENDING_BLOCK_METRICS;
  icon: LucideIcon;
}) {
  const metrics = PENDING_BLOCK_METRICS[name];
  return (
    <Panel blockKey={blockKey} title={name} icon={icon} aside={<PendingMarker info={pendingPart(name)} />}>
      <TileGrid count={metrics.length}>
        {metrics.map((label) => (
          <PendingTile key={label} label={label} />
        ))}
      </TileGrid>
    </Panel>
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
        // The sample is NOT "petitions with a deadline": it is A ∪ B of
        // `service-petitions/internal/store/citizen_report_summary.go:40-84` — settled in the period
        // against a stored deadline, plus those whose classification ceiling fell in the period unmet.
        note: (onTime, sample) =>
          `${onTime}/${sample} phiếu đúng hạn, trên số phiếu xử lý xong hoặc quá hạn phân loại trong kỳ`,
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
    <Panel blockKey="citizen-reports" title="Phản ánh người dân" icon={MessageSquareWarning}>
      <PairErrors pair={pair} noun="phản ánh" onReload={onReload} />
      <TileGrid count={figures.length}>
        {figures.map((f) => (
          <FigureTile key={f.id} figure={f} />
        ))}
      </TileGrid>
    </Panel>
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
    icon: Target,
  };
}

/**
 * Khối THU – CHI NGÂN SÁCH — year to date, NO comparison (user decision).
 *
 * EVERY WORD AND EVERY "NO VALUE" RULE IS THE THU - CHI SCREEN'S OWN (`features/thu-chi/
 * nhan-thu-chi.ts`, `bang-thu-chi.tsx` `TheChiSoNam`), reused, not re-written: a `null` with the
 * server's reason renders that SENTENCE instead of a figure (ADR 0035 §A), never `0`. The redesign
 * frames it as a full-width tile but does NOT replace it with a generic "Chưa có dữ liệu": the
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
      <Panel blockKey="fiscal" title={title} icon={Wallet}>
        {result !== null && (
          <LoadError title="Chưa tải được số liệu thu – chi" onReload={onReload}>
            {result.thongBao}
          </LoadError>
        )}
        <TileGrid count={fixed.length}>
          {fixed.map((label, i) => (
            <FigureTile
              key={label}
              figure={{ id: `fiscal-${i}`, label, display: shown, href: FISCAL_SCREEN_PATH, comparison: null, icon: Target }}
            />
          ))}
        </TileGrid>
        <SmallNote>{scope}</SmallNote>
      </Panel>
    );
  }

  const d = result.duLieu;
  const balanceText = nhanSoTienChiSo(d.balance, "dong");
  return (
    <Panel blockKey="fiscal" title={title} icon={Wallet}>
      <TileGrid count={3 + d.revenue_totals.length}>
        <FigureTile figure={indicatorFigure("fiscal-revenue", d.revenue_achievement)} />
        <FigureTile figure={indicatorFigure("fiscal-expenditure", d.expenditure_achievement)} />
        <FigureTile
          figure={{
            id: "fiscal-balance",
            label: NHAN_CHENH_LECH,
            display: withUnit(balanceText, d.balance.amount !== null),
            href: FISCAL_SCREEN_PATH,
            comparison: null,
            sentence: d.balance.amount === null,
            icon: Banknote,
          }}
        />
        {d.revenue_totals.map((o) => {
          const reason = lyDoKhongTinh(o.unavailable_reason);
          return (
            <FigureTile
              key={o.column_id}
              figure={{
                id: `fiscal-total-${o.column_id}`,
                label: o.name,
                display: "",
                href: FISCAL_SCREEN_PATH,
                comparison: null,
                sentence: reason !== null,
                icon: Banknote,
              }}
              value={<OTien chu={withUnit(nhanSoTien(o.value, "dong"), o.value !== null)} lyDo={reason} hienLyDo />}
            />
          );
        })}
      </TileGrid>
      <div className="mt-auto flex flex-col gap-1">
        <SmallNote>{scope}</SmallNote>
        <SmallNote>
          {NHAN_CHENH_LECH}: {GHI_CHU_CHENH_LECH}
        </SmallNote>
      </div>
    </Panel>
  );
}

/** Task-type labels by code, or `null` when the catalogue did not load (rows then show the code). */
export type TaskTypeLabels = ReadonlyMap<string, string> | null;

const URGENT_TITLE = "Cần xử lý ngay";

/**
 * "CẦN XỬ LÝ NGAY" — spec §5, cut down by the user's decision to what may be shown on the most
 * widely seen screen of the product: module, code, which deadline, field/category, the deadline
 * itself, and the server's "critical" judgement. No title, content, unit or person (rule 3).
 *
 * A FAILED QUEUE IS AN ERROR LINE IN THE LIST, and the "nothing urgent" sentence appears ONLY when
 * every queue answered and none had a row. A 503 from the working calendar must never read as calm.
 *
 * THE LIST IS CAPPED IN HEIGHT AND SCROLLS INSIDE THE CELL: dozens of overdue rows must not stretch
 * the grid's last row into a column of white. The scroller is a labelled, focusable region so a
 * keyboard can scroll it (the `TableScroll` pattern).
 */
export function UrgentPanel({
  queue,
  taskTypeLabels,
}: {
  queue: Loaded<MergedQueue>;
  taskTypeLabels: TaskTypeLabels;
}) {
  if (queue === null || !queue.ok) {
    return (
      <Panel blockKey="urgent" title={URGENT_TITLE} icon={Siren}>
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
      </Panel>
    );
  }
  const { rows, failures } = queue.duLieu;
  return (
    <Panel
      blockKey="urgent"
      title={URGENT_TITLE}
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
        <div
          role="region"
          aria-label="Danh sách cần xử lý ngay"
          tabIndex={0}
          className="-mx-4 max-h-80 overflow-y-auto focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-brand-500"
        >
          <ul className="m-0 list-none divide-y divide-line p-0">
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
        </div>
      )}
    </Panel>
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

/** What the blocks draw — shared by `/tong-quan` and `/bao-cao`. */
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

export const DASHBOARD_TITLE = "Tổng quan điều hành";

/**
 * The page header, prototype frame: title; "Kỳ …: … · tính đến …" under it; on the right the period
 * buttons, then the unbuilt PDF/XLSX/PPTX and Trình chiếu (disabled, "?").
 *
 * `context` absent = the account cannot read the page (`report.read`), or its session is still being
 * read: the title and the disabled actions only — a disabled control with no data behind it reveals
 * nothing — and no period, because there is no figure for it to describe.
 *
 * NO "Tính lại ngay", NO "số liệu cũ" badge (ADR 0053): the counts are live at the owning service.
 */
export function DashboardHeader({
  context,
}: {
  context?: { windows: PeriodWindows; fetchedAt: number; onPeriodChange: (k: PeriodKind) => void };
}) {
  return (
    <PageHeader
      icon={LayoutDashboard}
      title={DASHBOARD_TITLE}
      subtitle={
        context === undefined ? undefined : (
          <span className="inline-flex items-center gap-1.5">
            <Clock aria-hidden="true" focusable="false" strokeWidth={1.8} />
            {`${periodMetaLabel(context.windows)} · tính đến ${formatDateTime(context.fetchedAt)}`}
          </span>
        )
      }
      actions={
        <>
          {context !== undefined && (
            <PeriodPicker kind={context.windows.kind} onChange={context.onPeriodChange} />
          )}
          <DashboardHeaderActions />
        </>
      }
    />
  );
}

/**
 * The page body. A figure whose keys the account lacks is NOT RENDERED — not drawn as 0, not drawn
 * as "—": a figure the account may not read must not be implied either way.
 *
 * Under the grid, ONE line says what "kỳ trước" means — it explains every comparison caption of
 * every block above, so it belongs to none of them. Drawn only when a compared block is shown.
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
  const compared = visible.tasks || visible.incomingDocuments || visible.citizenReports;
  return (
    <>
      <DashboardHeader context={{ windows: data.windows, fetchedAt: data.fetchedAt, onPeriodChange }} />
      <DashboardBlocks data={data} visible={visible} onReload={reload} />
      {compared && (
        <p className="m-0 mt-4 flex items-start gap-1.5 text-xs leading-relaxed text-ink-500">
          <History aria-hidden="true" focusable="false" strokeWidth={1.8} className="mt-px size-3.5 shrink-0" />
          <span className="min-w-0">{comparisonNote(data.windows)}</span>
        </p>
      )}
    </>
  );
}

/**
 * The grid — ONE grid, 1 column, 2 from 1024px, 3 from 1536px (the prototype's `lg` / `2xl`), the six
 * blocks in the prototype's fixed order, then "Cần xử lý ngay". A block whose key the account lacks
 * is not rendered and the rest close up; the ORDER of what remains never changes.
 *
 * NO `auto-rows-fr`: the cells of one row stretch to that row's tallest, as a grid does, but a row
 * of short blocks is not stretched to the tallest row of the page.
 *
 * `/bao-cao` draws the SAME blocks from the same loader, so the two pages cannot show two numbers for
 * one period (spec 13 §10) — minus "Cần xử lý ngay" (`urgent={false}`, spec 13 §1).
 *
 * `Kinh tế & Tài nguyên` has no source data and no read key of its own in wave 1; it opens nothing,
 * so it shows under the page gate (`report.read`) alone. `Giải ngân ngân sách` stays under
 * `budget.read`: a placeholder must not reveal a block the account would not see once it is built.
 */
export function DashboardBlocks({
  data,
  visible,
  onReload,
  urgent = true,
}: {
  data: BlocksData;
  visible: BlockVisibility;
  onReload: () => void;
  urgent?: boolean;
}) {
  const anyQueue = visible.tasks || visible.incomingDocuments || visible.citizenReports;
  return (
    <div data-dashboard-grid="" className="grid min-w-0 grid-cols-1 gap-4 lg:grid-cols-2 2xl:grid-cols-3">
      {visible.tasks && <TaskBlock pair={data.tasks} windows={data.windows} onReload={onReload} />}
      {visible.incomingDocuments && (
        <DocumentBlock pair={data.incomingDocuments} windows={data.windows} onReload={onReload} />
      )}
      {visible.budget && <PendingBlock blockKey="budget" name="Giải ngân ngân sách" icon={Banknote} />}
      {visible.budget && <FiscalBlock result={data.fiscal} year={data.fiscalYear} onReload={onReload} />}
      {visible.citizenReports && (
        <CitizenReportBlock pair={data.citizenReports} windows={data.windows} onReload={onReload} />
      )}
      <PendingBlock blockKey="economy" name="Kinh tế & Tài nguyên" icon={Store} />
      {urgent && anyQueue && <UrgentPanel queue={data.queue} taskTypeLabels={data.taskTypeLabels} />}
    </div>
  );
}
