import {
  AlertTriangle,
  ArrowDownRight,
  ArrowUpRight,
  Clock,
  CloudOff,
  Database,
  History,
  LayoutDashboard,
  Minus,
  RotateCw,
  type LucideIcon,
} from "lucide-react";
import Link from "next/link";
import { createContext, useContext } from "react";
import type { ReactNode } from "react";

import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { PageHeader } from "@/components/ui/page-header";
import { PENDING_HOVER_TEXT, PendingMarker } from "@/components/ui/pending-feature";
import type { PendingFeatureInfo } from "@/components/ui/pending-feature";
import { NO_DATA_CAPTION } from "@/components/ui/stat-card";
import { taskDetailHref } from "@/features/nhiem-vu/task-link";
import { KPI_NO_RATING, ratingAverage } from "@/features/phan-anh/nhan-phieu";
import { reportPendingPart } from "@/features/report/labels";
import { OTien } from "@/features/thu-chi/o-tien";
import {
  GHI_CHU_CHENH_LECH,
  lyDoKhongTinh,
  NHAN_CHENH_LECH,
  nhanChiSo,
  nhanSoTien,
  nhanSoTienChiSo,
  O_KHONG_TINH_DUOC,
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
import { compactDong, compactDongReport } from "@/lib/compact-dong";
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
import type { ComparisonLine, DrillTarget, FigureKind, MergedQueue, Trend, UrgentRow } from "./figures";
import { buildDashboardExport, EXPORT_NO_FIGURE, urgentSummary } from "./export-model";
import type { DashboardExport, ExportBlock, ExportCommune, ExportFigure } from "./export-model";
import { DashboardHeaderActions } from "./header-actions";
import type { ExportSource } from "./header-actions";
import { PENDING_BLOCK_METRICS, pendingPart } from "./labels";
import { PresentationContext, PresentationFrame } from "./presentation";
import type { PresentationControl, PresentationState } from "./presentation";
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
 *   header  title · "Kỳ …: … · tính đến …" · period buttons · PDF/XLSX/PPTX · Trình chiếu (both live)
 *   body    ONE grid — 1 column, 2 from 1024px, 3 from 1536px — of six blocks in the prototype's
 *           order (Nhiệm vụ · Văn bản & Đơn thư · Giải ngân ngân sách · Thu – Chi ngân sách ·
 *           Phản ánh người dân · Kinh tế & Tài nguyên), then "Cần xử lý ngay" as the 7th cell.
 * The old "Cần xử lý" / "Tình hình trong kỳ" rows are gone as BLOCKS, not as figures: every figure
 * they drew is a tile of its module's block now. EVERY FIGURE THE PAGE SHOWED BEFORE IS STILL SHOWN
 * EXACTLY ONCE, each still a link to its list, under the same read keys (ADR 0068 §1).
 *
 * COMPOSITION (ADR 0068 lần 5, 06/10/2026 — "giống prototype nhất"): the block is the prototype's
 * `Panel` (small upper-case title row, red count pill and red frame only when "Cần xử lý ngay" has
 * rows) and each figure its `MetricTile` (value first, large and bold; label; delta line with an
 * arrow). Colours and fonts stay the tokens of lần 2.
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
  /** the full value when `display` is shortened ("9,64 tỷ đồng" ← "9.640.000.000 đồng"), shown on hover */
  readonly exact?: string;
  /** the server's own count behind `display` — for the exported workbook only, never drawn */
  readonly raw?: number;
  /** stock (the register now) or period (counted in the window) — what `/bao-cao`'s chart may plot */
  readonly kind?: FigureKind;
  /** which way is good — colours `/bao-cao`'s delta line and chart bar */
  readonly trend?: Trend;
  /**
   * A PERIOD figure's two numbers, set only when BOTH windows answered with a value. `/bao-cao` reads
   * them for its delta line and its comparison chart; Tổng quan never does (its line is `comparison`).
   */
  readonly values?: { readonly current: number; readonly previous: number; readonly previousText: string };
};

/** `values` of a period figure, or nothing — never a pair with a missing half. */
function pairValues(
  kind: FigureKind,
  current: Read,
  previous: Read,
  format: (n: number) => string,
): Figure["values"] {
  if (kind !== "period" || typeof current !== "number" || typeof previous !== "number") return undefined;
  return { current, previous, previousText: format(previous) };
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
    raw: typeof current === "number" ? current : undefined,
    kind: spec.kind,
    trend: spec.trend,
    values: pairValues(spec.kind, current, previous, formatCount),
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
    kind: "period",
    trend: "higher-is-better",
    values: pairValues("period", current, previous, formatPercent),
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
      target: { list: "tasks", metric: "in_progress" },    }),
    overdue: count({
      pick: (t) => t.overdue,
      kind: "stock",
      trend: "lower-is-better",
      target: { list: "tasks", metric: "overdue" },
      alert: true,    }),
    suspended: count({
      pick: (t) => t.suspended,
      kind: "stock",
      trend: "neutral",
      target: { list: "tasks", metric: "suspended" },    }),
    completed: count({
      pick: (t) => t.completed,
      kind: "period",
      trend: "higher-is-better",
      target: { list: "tasks", metric: "completed" },    }),
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
      target: { list: "incoming-documents", metric: "arrived" },    }),
    open: {
      ...count({
        pick: (d) => d.open,
        kind: "stock",
        trend: "neutral",
        target: { list: "incoming-documents", metric: "open" },      }),
      note: stockNote,
    },
    overdue: {
      ...count({
        pick: (d) => d.overdue,
        kind: "stock",
        trend: "lower-is-better",
        target: { list: "incoming-documents", metric: "overdue" },
        alert: true,      }),
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

/** The delta line's arrow — the prototype's `MetricTile`. "none" (no percentage to show) has none. */
const MOVEMENT_ICON: Readonly<Record<Movement, LucideIcon | null>> = {
  up: ArrowUpRight,
  down: ArrowDownRight,
  same: Minus,
  none: null,
};

/** The stable key of each block — `data-block` on its `<section>`, what the order tests read. */
export type BlockKey = "tasks" | "documents" | "budget" | "fiscal" | "citizen-reports" | "economy" | "urgent";

/**
 * Which page draws the blocks. The prototype spaces them differently on its two screens: Tổng quan
 * (`DashboardWorkspace`) is the dense glance — `p-3.5` cards, tiles 3 across when a block has more
 * than four — and Báo cáo (`ReportWorkspace`) the roomier read — `p-4` cards, tiles always 2 across.
 * A context rather than a prop through six block components: only the frame differs, never a figure.
 *
 * Since 09/10/2026 (owner, ADR 0053 §Sửa đổi 09/10/2026 lần 2, D4/D6) `"report"` also draws the
 * prototype's report TILE (not a link, never red, `1.7vw` value, delta line + sub-lines), the report
 * fiscal order and sums, and the restyled "?". Every such branch reads this context; `"dashboard"` —
 * Tổng quan — is unchanged except the fiscal block's title, which both pages share.
 */
export type BlockLayout = "dashboard" | "report";

const BlockLayoutContext = createContext<BlockLayout>("dashboard");

const NOT_PRESENTING: PresentationState = { on: false, current: null };

/**
 * Trình chiếu as the blocks see it (`presentation.tsx`). A context READ, not state: this file stays
 * props-only. `layout="report"` always reads "off" — `/bao-cao` has no Trình chiếu (spec 13 §1), and a
 * report grid rendered under a presenting frame by mistake must still not take its type sizes.
 */
function usePresenting(): PresentationState {
  const layout = useContext(BlockLayoutContext);
  const presentation = useContext(PresentationContext);
  return layout === "dashboard" && presentation.on ? presentation : NOT_PRESENTING;
}

/**
 * One block of the grid — the prototype's `Panel`: a white card (no shadow — ADR 0068 lần 2 #4) with a
 * small upper-case muted title row and its body under it. `count` given and above 0 = the alarm
 * look of "Cần xử lý ngay": a red count pill after the title and a red frame. At 0 neither is drawn,
 * as in the prototype — an all-clear cell must not look like an alarm.
 *
 * The `<section aria-label>` is the landmark screen readers (and the tests) find each block by. The
 * body is a size CONTAINER: the tile grid picks its column count from the width of THE CARD, not of
 * the window — the same card is a third of a 1536px screen and the whole of a phone.
 */
function Panel({
  blockKey,
  title,
  aside,
  count,
  children,
}: {
  blockKey: BlockKey;
  title: string;
  aside?: ReactNode;
  count?: number;
  children: ReactNode;
}) {
  const layout = useContext(BlockLayoutContext);
  const presenting = usePresenting();
  const alarm = count !== undefined && count > 0;
  const current = presenting.current === blockKey;
  return (
    <Card
      as="section"
      aria-label={title}
      data-block={blockKey}
      // Trình chiếu: each block is a keyboard stop (`presentation.tsx` moves focus here). The ring, not
      // the browser outline, marks the current one — it stays while Tab walks the block's own tiles.
      tabIndex={presenting.on ? -1 : undefined}
      data-current={current ? "" : undefined}
      className={cn(
        "flex min-h-0 flex-col",
        layout === "report" ? "p-4" : presenting.on ? "p-5" : "p-3.5",
        alarm && "border border-danger-200",
        presenting.on && "scroll-m-6 outline-none",
        current && "ring-2 ring-brand-500 ring-offset-2 ring-offset-canvas",
      )}
    >
      <div
        className={cn(
          "flex shrink-0 items-center",
          // Báo cáo: the "?" sits right after the title, `gap-1` (spec 05 B, owner 09/10/2026 D4).
          layout === "report" ? "gap-1" : "gap-2",
          layout === "report" || presenting.on ? "mb-3" : "mb-2",
        )}
      >
        <h2
          className={cn(
            "m-0 min-w-0 leading-snug font-bold tracking-wide text-ink-500 uppercase",
            presenting.on ? "text-[15px]" : "text-[11.5px]",
          )}
        >
          {title}
        </h2>
        {alarm && (
          // `bg-danger-solid` + 11px, not the prototype's `bg-danger` + 10px: white on the lighter red
          // fails AA (ADR 0068 lần 2 #7, "Huy hiệu thông báo").
          <span className="grid h-4 min-w-4 place-items-center rounded-full bg-danger-solid px-1 text-[11px] leading-none font-bold text-white tabular-nums">
            {count}
          </span>
        )}
        {aside}
      </div>
      <div className="@container flex min-h-0 flex-1 flex-col gap-3">{children}</div>
    </Card>
  );
}

/**
 * The tiles of a block. Tổng quan: 2 columns, 3 when the block has more than four figures AND its card
 * is wide enough (`@md` = 28rem of card body) — three columns of a 288px phone card would cut
 * "91,30%" in half, so the prototype's rule ("3 when > 4") holds wherever it fits. Báo cáo: always 2.
 * Trình chiếu: always 2 — the type is half as large again, and three columns of it run "108,1%" into
 * "4.317 tỷ" as one number that does not exist (prototype `DashboardWorkspace.tsx:226-236`).
 */
function TileGrid({ count, children }: { count: number; children: ReactNode }) {
  const layout = useContext(BlockLayoutContext);
  const presenting = usePresenting().on;
  return (
    <ul
      className={cn(
        "m-0 grid list-none grid-cols-2 p-0",
        layout === "report" ? "gap-3" : presenting ? "gap-x-5 gap-y-3" : "gap-x-4 gap-y-2",
        layout === "dashboard" && !presenting && count > 4 && "@md:grid-cols-3",
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

/**
 * The prototype's delta line: arrow + "+12,5% so với kỳ trước", coloured by which way is good. A line
 * with no percentage ("Kỳ trước: 0") has no arrow — there is no direction to point.
 */
function ComparisonCaption({ figure }: { figure: Figure }) {
  const presenting = usePresenting().on;
  if (figure.comparison === null) return null;
  // Trình chiếu drops the lines with NOTHING TO COMPARE — "Kỳ trước: —" and "Kỳ trước: 0", the only
  // ones with no movement (`comparisonLine`, figures.ts). On a projector a dozen of them only thin out
  // the figures (prototype `MetricTile.tsx:81-87`). A failed previous call is still said by
  // `PairErrors`; a real change, "không đổi" included, stays.
  if (presenting && (figure.movement ?? "none") === "none") return null;
  const Icon = MOVEMENT_ICON[figure.movement ?? "none"];
  return (
    <span
      className={cn(
        "mt-1 flex items-center gap-1 leading-snug",
        presenting ? "text-[13px]" : "text-[11px]",
        TONE_TEXT[figure.comparison.tone],
      )}
    >
      {Icon !== null && (
        <Icon
          aria-hidden="true"
          focusable="false"
          strokeWidth={2}
          className={cn("shrink-0", presenting ? "size-3.5" : "size-3")}
        />
      )}
      <span className="min-w-0">{figure.comparison.text}</span>
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

/**
 * The prototype `MetricTile`'s frame: no fixed height — a tile is as tall as its own lines — and a
 * small inner padding so the hover fill reads as the tile, not as the cell around it.
 */
const TILE_BODY = "block h-full min-w-0 overflow-hidden rounded-[10px] px-1 py-0.5 text-left";

/**
 * The value size: 16–26px, scaled to the TILE's own width (`cqi` of the `@container` tile), not the
 * window's. The prototype's `1.7vw` grew with the window even when the block was one of three narrow
 * columns, so a sum of money outran its tile — clipped ("690.000.000 đồn") or broken onto a second line
 * (tester screenshot 06/10/2026).
 */
const VALUE_TEXT = "text-[clamp(16px,12cqi,26px)] leading-tight font-bold tabular-nums";

/**
 * Trình chiếu's value size: the SAME 12cqi of the tile, with a higher floor and ceiling. Not the
 * prototype's `clamp(24px,2.1vw,40px)`: a window-based size is exactly what outran the tile before
 * (412409a1). The figure grows because the tile does — always 2 tiles across here — up to 40px.
 */
const VALUE_TEXT_PRESENTING = "text-[clamp(18px,12cqi,40px)] leading-[1.1] font-bold tabular-nums";

/** The label under the value, then the extra lines (note, delta) under it. */
const LABEL_TEXT = "mt-0.5 block text-[12px] leading-snug text-ink-500";
const LABEL_TEXT_PRESENTING = "mt-0.5 block text-[15px] leading-snug text-ink-500";

function tileText(presenting: boolean): { value: string; label: string; note: string } {
  return presenting
    ? { value: VALUE_TEXT_PRESENTING, label: LABEL_TEXT_PRESENTING, note: "text-[13px]" }
    : { value: VALUE_TEXT, label: LABEL_TEXT, note: "text-[11px]" };
}

/**
 * One figure, in the order of the prototype's `MetricTile`: VALUE (large, bold) · label · note ·
 * delta line. A LINK named `Xem danh sách đằng sau: {nhãn}` (spec §4, §9) to the pre-filtered list
 * behind it (`?metric=…`) — never a dialog (ADR 0053 §7); the prototype's tile opened a dialog.
 *
 * THE ACCESSIBLE NAME REPLACES THE VISIBLE TEXT, so the value is wired back with
 * `aria-describedby` — otherwise a screen-reader user hears where the link goes and never the
 * number it is about.
 *
 * Red is never alone: an alarming figure is red AND its label says "Quá hạn" / "Trễ hạn" (ADR 0068
 * lần 2 #8b). Hover tints with the accent (red tint on an alarming figure, as the prototype) — a
 * FILL, never text. A server SENTENCE (fiscal reason) takes the whole row: squeezed into one of three
 * columns it would read one word per line. A count never wraps (`whitespace-nowrap`, the prototype's
 * rule — "1.234" broken over two lines reads as two numbers); an amount of money may.
 */
function FigureTile({ figure, value }: { figure: Figure; value?: ReactNode }) {
  const layout = useContext(BlockLayoutContext);
  const valueId = `${figure.id}-value`;
  const alert = figure.alert === true;
  const sentence = figure.sentence === true;
  const text = tileText(usePresenting().on);
  if (layout === "report") return <ReportFigureTile figure={figure} value={value} />;
  return (
    <li className={cn("@container min-w-0", sentence && "col-span-full")}>
      <Link
        href={figure.href}
        title={figure.exact}
        aria-label={drillLabel(figure.label)}
        aria-describedby={valueId}
        className={cn(
          TILE_BODY,
          "text-inherit no-underline transition-colors",
          alert ? "hover:bg-danger-50" : "hover:bg-accent-50",
          "focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-brand-500",
        )}
      >
        <span
          className={cn(
            "block",
            sentence ? "text-[13px] leading-snug font-medium" : text.value,
            // A figure never wraps — a sum is shortened (`compactDong`) so it fits instead. Only a
            // server SENTENCE wraps.
            !sentence && "whitespace-nowrap",
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
        <span className={text.label}>{figure.label}</span>
        {figure.note !== undefined && (
          <span className={cn("mt-1 block leading-snug text-ink-500", text.note)}>{figure.note}</span>
        )}
        <ComparisonCaption figure={figure} />
      </Link>
    </li>
  );
}

/* ── `/bao-cao`'s tile — the prototype's `MetricTile` as `ReportWorkspace` draws it ─────────────────
 *
 * Owner decision 09/10/2026 (ADR 0053 §Sửa đổi 09/10/2026 lần 2, D4), REPORT ONLY — Tổng quan's tile
 * above is untouched:
 *   · NOT a link (the prototype passes no `onOpen` on this page) and never red (no `emphasis`);
 *   · the value at the prototype's `clamp(19px,1.7vw,26px)`, navy, bold, one line;
 *   · the delta line: arrow + "+12,5% so với kỳ trước" / "không đổi" under 0,05%, or — when there is
 *     no percentage (stock figure, previous 0 or missing) — "chưa có kỳ trước để so";
 *   · what our page says beyond the prototype (README "giữ"): "Kỳ trước: 0", "tính đến …", "1/1 việc có
 *     hạn"… as small sub-lines AFTER the delta line, two lines at most, the full text on hover. */

/** The prototype's sentence for a figure with no percentage to show (`MetricTile.tsx`). */
export const NO_PREVIOUS_PERIOD = "chưa có kỳ trước để so";

const REPORT_TILE = "min-w-0 overflow-hidden px-1 py-0.5";
const REPORT_VALUE = "block text-[clamp(19px,1.7vw,26px)] font-bold whitespace-nowrap text-navy tabular-nums";
const REPORT_LABEL = "mt-0.5 block text-[12px] text-ink-muted";
const REPORT_QUIET = "mt-1 block text-[11px] text-ink-muted/60";
const REPORT_SUB = "mt-0.5 line-clamp-2 text-[10.5px] leading-snug text-ink-muted/70";

/**
 * The "?" of an unbuilt part on `/bao-cao`: the prototype's `HelpCircle size-3.5 text-ink-muted/70`,
 * drawn by RESTYLING `PendingMarker` (a 14px muted ring with "?"), not by changing it — its default
 * look, behaviour (tooltip, description, 42px hit area) and every other screen stay as they are.
 */
export const REPORT_PENDING_MARKER = "size-3.5 border-ink-muted/70 bg-transparent text-[9px] text-ink-muted/70";

type ReportDelta = {
  readonly text: string;
  readonly tone: "good" | "bad" | "flat";
  readonly icon: LucideIcon;
};

/**
 * `/bao-cao`'s delta line for a figure — the prototype's `movement()`: `null` when there is no
 * percentage (a stock figure, a previous value of 0 or missing), "không đổi" under 0,05%, otherwise the
 * signed change. A volume whose rise is neither good nor bad (`neutral`) stays muted, as on Tổng quan.
 */
export function reportDelta(f: Pick<Figure, "values" | "trend">): ReportDelta | null {
  const v = f.values;
  if (v === undefined || v.previous === 0) return null;
  const change = ((v.current - v.previous) / v.previous) * 100;
  if (Math.abs(change) < 0.05) return { text: "không đổi", tone: "flat", icon: Minus };
  const up = change > 0;
  const tone =
    f.trend === undefined || f.trend === "neutral" ? "flat" : (f.trend === "higher-is-better") === up ? "good" : "bad";
  return {
    text: `${up ? "+" : "-"}${formatPercent(Math.abs(change))} so với kỳ trước`,
    tone,
    icon: up ? ArrowUpRight : ArrowDownRight,
  };
}

const REPORT_TONE: Readonly<Record<ReportDelta["tone"], string>> = {
  good: "text-leaf",
  bad: "text-danger",
  flat: "text-ink-muted",
};

function ReportDeltaLine({ delta }: { delta: ReportDelta | null }) {
  if (delta === null) return <span className={REPORT_QUIET}>{NO_PREVIOUS_PERIOD}</span>;
  const Icon = delta.icon;
  return (
    <span className={cn("mt-1 flex items-center gap-1 text-[11px]", REPORT_TONE[delta.tone])}>
      <Icon aria-hidden="true" focusable="false" strokeWidth={2} className="size-3 shrink-0" />
      <span className="min-w-0">{delta.text}</span>
    </span>
  );
}

function ReportSubLine({ text }: { text: string }) {
  return (
    <span title={text} className={REPORT_SUB}>
      {text}
    </span>
  );
}

function ReportFigureTile({ figure, value }: { figure: Figure; value?: ReactNode }) {
  const valueId = `${figure.id}-value`;
  const sentence = figure.sentence === true;
  const loading = value === undefined && figure.display === LOADING;
  const delta = reportDelta(figure);
  // "Kỳ trước: 0" / "Kỳ trước: —" is said only when no percentage could be drawn (README "giữ").
  const subLines = [figure.note, delta === null ? figure.comparison?.text : undefined].filter(
    (s): s is string => s !== undefined && s !== "",
  );
  return (
    <li className={cn(REPORT_TILE, sentence && "col-span-full")} title={figure.exact}>
      <span className={sentence ? "block text-[13px] leading-snug font-medium text-ink-700" : REPORT_VALUE}>
        {value !== undefined ? (
          <span id={valueId} className="[overflow-wrap:anywhere]">
            {value}
          </span>
        ) : (
          <FigureValue id={valueId} text={figure.display} sentence={sentence} />
        )}
      </span>
      <span className={REPORT_LABEL}>{figure.label}</span>
      {/* No line while loading (it would claim "no previous period" before the answer), and none
          under a server SENTENCE — there is no figure there to compare. */}
      {!loading && !sentence && <ReportDeltaLine delta={delta} />}
      {subLines.map((s) => (
        <ReportSubLine key={s} text={s} />
      ))}
    </li>
  );
}

/** A figure with no source data in wave 1: "Chưa có dữ liệu", never a 0, never a link. */
function NoSourceTile({ label }: { label: string }) {
  const layout = useContext(BlockLayoutContext);
  const text = tileText(usePresenting().on);
  if (layout === "report") {
    // Báo cáo (D4): "—" in the figure's own place and size, the caption as the quiet line — no
    // Database icon, so the tile lines up with its neighbours.
    return (
      <li className={REPORT_TILE} title={NO_SOURCE_DATA}>
        <span aria-hidden="true" className={REPORT_VALUE}>
          —
        </span>
        <span className={REPORT_LABEL}>{label}</span>
        <span className={REPORT_QUIET}>{NO_DATA_CAPTION}</span>
      </li>
    );
  }
  return (
    <li className={TILE_BODY} title={NO_SOURCE_DATA}>
      <span className="flex min-h-[26px] items-center gap-1 text-[13px] text-ink-500">
        <Database aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-3.5 shrink-0" />
        {NO_DATA_CAPTION}
      </span>
      <span className={text.label}>{label}</span>
    </li>
  );
}

/**
 * A figure that is an UNBUILT part (ADR 0068 §14): the tile it will be, "—" for the eye and the
 * reason for a screen reader, never a link. `info` given = the tile carries its own "?"; without it
 * the "?" is the block's (a whole unbuilt block has one "?" in its header, not one per tile).
 */
function PendingTile({ label, info }: { label: string; info?: PendingFeatureInfo }) {
  const layout = useContext(BlockLayoutContext);
  const text = tileText(usePresenting().on);
  if (layout === "report") {
    // Báo cáo (D4): "—" navy bold like any figure, and "Tính năng đang phát triển" SAID on the tile
    // (the quiet line), not only to a screen reader.
    return (
      <li className={REPORT_TILE} data-pending="">
        <span className="flex items-center gap-1">
          <span aria-hidden="true" className={REPORT_VALUE}>
            —
          </span>
          {info !== undefined && <PendingMarker info={info} className={REPORT_PENDING_MARKER} />}
        </span>
        <span className={REPORT_LABEL}>{label}</span>
        <span className={REPORT_QUIET}>{PENDING_HOVER_TEXT}</span>
      </li>
    );
  }
  return (
    <li className={TILE_BODY} data-pending="">
      <span className="flex items-center gap-2">
        <span aria-hidden="true" className={cn(text.value, "text-ink-400")}>
          —
        </span>
        <span className="sr-only">{PENDING_HOVER_TEXT}</span>
        {info !== undefined && <PendingMarker info={info} />}
      </span>
      <span className={text.label}>{label}</span>
    </li>
  );
}

function SmallNote({ children }: { children: ReactNode }) {
  const presenting = usePresenting().on;
  return (
    <p className={cn("m-0 leading-relaxed text-ink-500", presenting ? "text-sm" : "text-xs")}>{children}</p>
  );
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
    <Panel blockKey="tasks" title="Nhiệm vụ">
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
const DOCUMENT_ON_TIME_LABEL = "Tỷ lệ đúng hạn văn bản";
const LETTERS_LABEL = "Đơn thư trong kỳ";

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
    <Panel blockKey="documents" title="Văn bản & Đơn thư">
      <PairErrors pair={pair} noun="văn bản đến" onReload={onReload} />
      <TileGrid count={figures.length + 2}>
        {figures.map((f) => (
          <FigureTile key={f.id} figure={f} />
        ))}
        <NoSourceTile label={DOCUMENT_ON_TIME_LABEL} />
        <PendingTile label={LETTERS_LABEL} info={pendingPart(LETTERS_LABEL)} />
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
}: {
  blockKey: BlockKey;
  name: keyof typeof PENDING_BLOCK_METRICS;
}) {
  const layout = useContext(BlockLayoutContext);
  const metrics = PENDING_BLOCK_METRICS[name];
  return (
    <Panel
      blockKey={blockKey}
      title={name}
      aside={
        <PendingMarker
          info={pendingPart(name)}
          className={layout === "report" ? REPORT_PENDING_MARKER : undefined}
        />
      }
    >
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
  const layout = useContext(BlockLayoutContext);
  const figures = citizenReportFigures(pair, windows).map((f) => figureFor(layout, f));
  return (
    <Panel blockKey="citizen-reports" title="Phản ánh người dân">
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
 * `/bao-cao`'s own label for a figure, where the prototype names it differently from Tổng quan's list
 * heading (D4: "Tiếp nhận trong kỳ"). Tổng quan keeps the heading of the list the tile opens.
 */
const REPORT_LABELS: Readonly<Record<string, string>> = {
  "citizen-reports-received": "Tiếp nhận trong kỳ",
};

/** A figure as `layout` draws it: on `/bao-cao` the report label and never the red (`alert`). */
function figureFor(layout: BlockLayout, f: Figure): Figure {
  if (layout !== "report") return f;
  return { ...f, label: REPORT_LABELS[f.id] ?? f.label, alert: false };
}

/** Khối PHẢN ÁNH NGƯỜI DÂN's figures — shared by the block and the export (`exportBlocks`). */
function citizenReportFigures(
  pair: SummaryPair<petitions_citizenReportSummaryOut>,
  windows: ComparedWindows,
): Figure[] {
  const period = toQueryPeriod(windows.current);
  const id = "citizen-reports";
  const count = (s: CountSpec<petitions_citizenReportSummaryOut>) =>
    countFigure(id, pair, s, period);
  return [
    count({
      pick: (r) => r.received,
      kind: "period",
      trend: "neutral",
      target: { list: "citizen-reports", metric: "received" },    }),
    count({
      pick: (r) => r.in_progress,
      kind: "stock",
      trend: "neutral",
      target: { list: "citizen-reports", metric: "in_progress" },    }),
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
      alert: true,    }),
    ratingFigure(id, pair.current, period),
  ];
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
    comparison: null,  };
}

/** A real amount that may be shortened — not null, not past the safe-integer range ("Không đọc được"). */
function safeAmount(amount: number | null | undefined): amount is number {
  return typeof amount === "number" && Number.isSafeInteger(amount);
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
    sentence: c.basis_points === null,  };
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
  const layout = useContext(BlockLayoutContext);
  const report = layout === "report";
  const scope = fiscalScope(year);

  if (result === null || !result.ok) {
    const shown = result === null ? LOADING : NO_VALUE;
    const fixed = ["Thu đạt dự toán", "Chi đạt dự toán", NHAN_CHENH_LECH];
    const tile = (label: string, i: number) => (
      <FigureTile
        key={label}
        figure={{ id: `fiscal-${i}`, label, display: shown, href: FISCAL_SCREEN_PATH, comparison: null }}
      />
    );
    return (
      <Panel blockKey="fiscal" title={FISCAL_TITLE}>
        {result !== null && (
          <LoadError title="Chưa tải được số liệu thu – chi" onReload={onReload}>
            {result.thongBao}
          </LoadError>
        )}
        <TileGrid count={fixed.length}>
          {fixed.slice(0, 2).map(tile)}
          {report && <PendingTile label={EXPENDITURE_TOTAL_LABEL} info={reportPendingPart(EXPENDITURE_TOTAL_LABEL)} />}
          {tile(NHAN_CHENH_LECH, 2)}
        </TileGrid>
        {report ? <ReportFootnotes notes={[scope]} /> : <SmallNote>{scope}</SmallNote>}
      </Panel>
    );
  }

  const d = result.duLieu;
  const totals = d.revenue_totals.map((o) => {
    const total = revenueTotalText(o, layout);
    return (
      <FigureTile
        key={o.column_id}
        figure={{
          id: `fiscal-total-${o.column_id}`,
          label: o.name,
          display: "",
          exact: total.exact,
          href: FISCAL_SCREEN_PATH,
          comparison: null,
          sentence: total.reason !== null,        }}
        value={<OTien chu={total.shown} lyDo={total.reason} hienLyDo />}
      />
    );
  });
  const revenue = <FigureTile figure={indicatorFigure("fiscal-revenue", d.revenue_achievement)} />;
  const expenditure = <FigureTile figure={indicatorFigure("fiscal-expenditure", d.expenditure_achievement)} />;
  const balance = <FigureTile figure={balanceFigure(d, layout)} />;

  if (report) {
    // The prototype's order (D6): Thu đạt dự toán · Tổng thu · Chi đạt dự toán · Tổng chi · Cân đối.
    // "Tổng thu" is each revenue total the server sends, under the server's own column name; "Tổng
    // chi" has no field in `finance_chiSoNamRa`, so it is the "?" tile.
    return (
      <Panel blockKey="fiscal" title={FISCAL_TITLE}>
        <TileGrid count={4 + d.revenue_totals.length}>
          {revenue}
          {totals}
          {expenditure}
          <PendingTile label={EXPENDITURE_TOTAL_LABEL} info={reportPendingPart(EXPENDITURE_TOTAL_LABEL)} />
          {balance}
        </TileGrid>
        <ReportFootnotes notes={[scope, balanceNote()]} />
      </Panel>
    );
  }

  return (
    <Panel blockKey="fiscal" title={FISCAL_TITLE}>
      <TileGrid count={3 + d.revenue_totals.length}>
        {revenue}
        {expenditure}
        {balance}
        {totals}
      </TileGrid>
      <div className="mt-auto flex flex-col gap-1">
        <SmallNote>{scope}</SmallNote>
        <SmallNote>{balanceNote()}</SmallNote>
      </div>
    </Panel>
  );
}

/**
 * The fiscal block's title, on BOTH pages and in the files — the prototype's `blockLabel("fiscal")`
 * (owner 09/10/2026, D4: the one change Tổng quan takes from this round).
 */
export const FISCAL_TITLE = "Thu - Chi ngân sách xã";

/** The prototype's tile between "Chi đạt dự toán" and the balance — unbuilt (`features/report/labels.ts`). */
const EXPENDITURE_TOTAL_LABEL = "Tổng chi";

/**
 * A block's closing notes on `/bao-cao` — spec 05 B: small, muted, under a hairline. Tổng quan keeps
 * its `SmallNote`s.
 */
function ReportFootnotes({ notes }: { notes: readonly string[] }) {
  return (
    <div className="mt-auto border-t border-line pt-2">
      {notes.map((n) => (
        <p key={n} className="m-0 text-[11px] leading-snug text-ink-muted">
          {n}
        </p>
      ))}
    </div>
  );
}

/** The scope line under the fiscal block — on screen and in the files. */
function fiscalScope(year: number): string {
  return `Luỹ kế năm ${year}, không so với kỳ trước.`;
}

/** The balance cell's temporary label and how it is computed (open question #32). */
function balanceNote(): string {
  return `${NHAN_CHENH_LECH}: ${GHI_CHU_CHENH_LECH}`;
}

/**
 * A tile's short sum: Tổng quan's `compactDong` ("9,64 tỷ đồng"), or on `/bao-cao` the prototype's
 * `compactDongReport` ("9,6 tỷ", D6). The exact amount is the same on both.
 */
function shortDong(amount: number, layout: BlockLayout): string {
  return layout === "report" ? compactDongReport(amount) : compactDong(amount);
}

/** The balance tile — the tile and the export read this one object. */
function balanceFigure(d: finance_chiSoNamRa, layout: BlockLayout = "dashboard"): Figure {
  const balanceText = nhanSoTienChiSo(d.balance, "dong");
  return {
    id: "fiscal-balance",
    label: NHAN_CHENH_LECH,
    display: safeAmount(d.balance.amount)
      ? shortDong(d.balance.amount, layout)
      : withUnit(balanceText, d.balance.amount !== null),
    exact: withUnit(balanceText, d.balance.amount !== null),
    href: FISCAL_SCREEN_PATH,
    comparison: null,
    sentence: d.balance.amount === null,
    raw: safeAmount(d.balance.amount) ? d.balance.amount : undefined,
  };
}

/** One revenue total's text: shortened for the tile, exact for hover, and the "not computed" reason. */
function revenueTotalText(
  o: finance_chiSoNamRa["revenue_totals"][number],
  layout: BlockLayout = "dashboard",
): {
  shown: string;
  exact: string;
  reason: string | null;
  raw: number | undefined;
} {
  const exact = withUnit(nhanSoTien(o.value, "dong"), o.value !== null);
  return {
    shown: safeAmount(o.value) ? shortDong(o.value, layout) : exact,
    exact,
    reason: lyDoKhongTinh(o.unavailable_reason),
    raw: safeAmount(o.value) ? o.value : undefined,
  };
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
      <Panel blockKey="urgent" title={URGENT_TITLE}>
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
    <Panel blockKey="urgent" title={URGENT_TITLE} count={rows.length}>
      {failures.map((f) => (
        <LoadError key={f.module}>
          {MODULE_LABEL[f.module]}: {f.message}
        </LoadError>
      ))}
      {rows.length === 0 && failures.length === 0 && (
        <p role="status" className="m-0 py-6 text-center text-[13px] text-success-600">
          {NOTHING_URGENT}
        </p>
      )}
      {rows.length > 0 && (
        <div
          role="region"
          aria-label="Danh sách cần xử lý ngay"
          tabIndex={0}
          className="max-h-64 overflow-auto pr-1 focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-brand-500"
        >
          <ul className="m-0 list-none space-y-1.5 p-0">
            {rows.map((r) => (
              <li key={`${r.module}|${r.code}|${r.kind}`}>
                <UrgentRowView row={r} category={categoryLabel(r, taskTypeLabels)} />
              </li>
            ))}
          </ul>
        </div>
      )}
    </Panel>
  );
}

const URGENT_ROW_FRAME = "block rounded-[9px] border border-line px-2.5 py-1.5";

/**
 * One row in the prototype's shape: a bordered row, a warning triangle (red when the server judged it
 * critical, amber otherwise), a first line and a second line led by the deadline pill.
 *
 * THE FIRST LINE IS MODULE + CODE, NOT THE PROTOTYPE'S TITLE: a row may not show title, content, unit
 * or person on the most widely seen screen (user decision; rule 3). "Nghiêm trọng" is written out on
 * a critical row so the red is never the only sign (ADR 0068 lần 2 #8b).
 *
 * Only a TASK row is a link (`/nhiem-vu?task=<code>` opens its detail, `task-link.ts`): the citizen-
 * report and document registers have no "open one record" link to land on, and a row that links to a
 * whole register would promise a record it does not open.
 */
function UrgentRowView({ row, category }: { row: UrgentRow; category: string | null }) {
  // Trình chiếu: the prototype's 15px / 13px rows (`DashboardWorkspace.tsx:301`).
  const presenting = usePresenting().on;
  const body = (
    <span className="flex items-start gap-2">
      <AlertTriangle
        aria-hidden="true"
        focusable="false"
        strokeWidth={1.8}
        className={cn("mt-0.5 size-3.5 shrink-0", row.critical ? "text-danger-600" : "text-warning-600")}
      />
      <span className="block min-w-0 flex-1">
        <span
          className={cn("line-clamp-1 font-medium text-ink-900", presenting ? "text-[15px]" : "text-[12.5px]")}
        >
          {MODULE_LABEL[row.module]} <span className="tabular-nums">{row.code}</span>
        </span>
        <span
          className={cn(
            "flex flex-wrap items-center gap-x-2 gap-y-0.5 text-ink-500",
            presenting ? "text-[13px]" : "text-[11px]",
          )}
        >
          <span
            className={cn(
              "rounded border px-1.5",
              row.critical
                ? "border-danger-200 bg-danger-50 text-danger-600"
                : "border-warning-500/25 bg-warning-50 text-warning-600",
            )}
          >
            {missedSinceText(row.missedAt)}
          </span>
          <span>{kindLabel(row.kind)}</span>
          {category !== null && <span className="min-w-0 truncate">{category}</span>}
          {row.critical && <span className="font-semibold text-danger-600">Nghiêm trọng</span>}
        </span>
      </span>
    </span>
  );
  if (row.module !== "task") return <div className={URGENT_ROW_FRAME}>{body}</div>;
  return (
    <Link
      href={taskDetailHref(row.code)}
      className={cn(
        URGENT_ROW_FRAME,
        "text-inherit no-underline transition-colors hover:border-danger-200 hover:bg-danger-50",
        "focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-brand-500",
      )}
    >
      {body}
    </Link>
  );
}

/**
 * The prototype's period buttons: separate small buttons in a row, the chosen one solid navy
 * (`dark`), the rest the quiet `secondary` — `aria-pressed` says which, since the fill alone would
 * not. Shared with `/bao-cao`, which adds "Tuỳ chọn". An unknown value (impossible — the options are
 * the caller's own list) is dropped by the caller rather than cast here.
 */
export function PeriodButtons<V extends string>({
  options,
  value,
  onChange,
}: {
  options: readonly { value: V; label: string }[];
  value: V;
  onChange: (v: V) => void;
}) {
  return (
    <div role="group" aria-label="Kỳ báo cáo" className="flex flex-wrap items-center gap-2">
      {options.map((o) => (
        <Button
          key={o.value}
          type="button"
          size="sm"
          variant={o.value === value ? "dark" : "secondary"}
          aria-pressed={o.value === value}
          onClick={() => onChange(o.value)}
        >
          {o.label}
        </Button>
      ))}
    </div>
  );
}

/** Tổng quan's four named periods. */
export function PeriodPicker({
  kind,
  onChange,
}: {
  kind: PeriodKind;
  onChange: (k: PeriodKind) => void;
}) {
  return (
    <PeriodButtons
      options={PERIOD_KINDS.map((k) => ({ value: k, label: PERIOD_BUTTON_LABEL[k] }))}
      value={kind}
      onChange={onChange}
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
 * buttons, then PDF/XLSX/PPTX (built 06/10/2026, `export-actions.tsx`) and the Trình chiếu toggle
 * (built 07/10/2026, `presentation.tsx`). In Trình chiếu the header stays — inside the overlay, with
 * the toggle that leaves it — and its title grows to the prototype's 28px.
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
  context?: {
    windows: PeriodWindows;
    fetchedAt: number;
    onPeriodChange: (k: PeriodKind) => void;
    /** absent = no export possible (no commune configuration passed) — the buttons stay disabled */
    exportSource?: ExportSource;
    /** absent = the toggle stays disabled */
    presentation?: PresentationControl;
  };
}) {
  const presenting = usePresenting().on;
  return (
    <PageHeader
      icon={LayoutDashboard}
      title={DASHBOARD_TITLE}
      // The prototype's header and grid sit `gap-3` apart (`DashboardWorkspace` "flex flex-col gap-3").
      className={cn("mb-3", presenting && "[&_h1]:text-[28px]")}
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
          <DashboardHeaderActions exportSource={context?.exportSource} presentation={context?.presentation} />
        </>
      }
    />
  );
}

/** Why the export buttons are disabled — said on the buttons' group, never only greyed. */
export const EXPORT_NEEDS_PERMISSION =
  "Tài khoản của bạn chưa có quyền Xuất báo cáo (report.export), nên chưa xuất được tệp.";
export const EXPORT_WAITING = "Đang tải số liệu — xuất được khi mọi khối đã tải xong.";

/**
 * What the export needs from the hook half: the commune's DISPLAY configuration (runtime, from
 * `Host` — rule 1 inv. 10) and whether the account holds `report.export`. Absent = no export.
 */
export type ExportAccess = {
  readonly commune: ExportCommune;
  readonly canExport: boolean;
};

function exportSourceOf(data: DashboardData, visible: BlockVisibility, access: ExportAccess): ExportSource {
  return {
    blockedReason: !access.canExport
      ? EXPORT_NEEDS_PERMISSION
      : !figuresSettled(data, visible)
        ? EXPORT_WAITING
        : null,
    build: (generatedAt) => dashboardExport(data, visible, access.commune, generatedAt),
  };
}

/**
 * The page body. A figure whose keys the account lacks is NOT RENDERED — not drawn as 0, not drawn
 * as "—": a figure the account may not read must not be implied either way.
 *
 * Under the grid, ONE line says what "kỳ trước" means — it explains every comparison caption of
 * every block above, so it belongs to none of them. Drawn only when a compared block is shown.
 *
 * `presentation` = Trình chiếu's on/off, owned by the hook half; the whole body sits in its frame
 * (`presentation.tsx`) so header, grid and note go full-screen together. Absent = toggle disabled.
 */
export function DashboardView({
  data,
  visible,
  onPeriodChange,
  exportAccess,
  presentation,
}: {
  data: DashboardData;
  visible: BlockVisibility;
  onPeriodChange: (k: PeriodKind) => void;
  exportAccess?: ExportAccess;
  presentation?: PresentationControl;
}) {
  const reload = () => onPeriodChange(data.windows.kind);
  const compared = visible.tasks || visible.incomingDocuments || visible.citizenReports;
  return (
    <PresentationFrame control={presentation}>
      <DashboardHeader
        context={{
          windows: data.windows,
          fetchedAt: data.fetchedAt,
          onPeriodChange,
          exportSource: exportAccess === undefined ? undefined : exportSourceOf(data, visible, exportAccess),
          presentation,
        }}
      />
      <DashboardBlocks data={data} visible={visible} onReload={reload} />
      {compared && (
        <p className="m-0 mt-3 flex items-start gap-1.5 text-xs leading-relaxed text-ink-500">
          <History aria-hidden="true" focusable="false" strokeWidth={1.8} className="mt-px size-3.5 shrink-0" />
          <span className="min-w-0">{comparisonNote(data.windows)}</span>
        </p>
      )}
    </PresentationFrame>
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
 * one period (spec 13 §10) — minus "Cần xử lý ngay" (spec 13 §1), in the prototype's `ReportWorkspace`
 * spacing (`layout="report"`): `gap-4`, 2 columns from 768px, 3 from 1280px. The prototype starts at 2
 * columns on any width; 1 column under 768px is kept so a 320px screen does not halve every card.
 *
 * `Kinh tế & Tài nguyên` has no source data and no read key of its own in wave 1; it opens nothing,
 * so it shows under the page gate (`report.read`) alone. `Giải ngân ngân sách` stays under
 * `budget.read`: a placeholder must not reveal a block the account would not see once it is built.
 */
export function DashboardBlocks({
  data,
  visible,
  onReload,
  layout = "dashboard",
}: {
  data: BlocksData;
  visible: BlockVisibility;
  onReload: () => void;
  layout?: BlockLayout;
}) {
  const anyQueue = visible.tasks || visible.incomingDocuments || visible.citizenReports;
  return (
    <BlockLayoutContext.Provider value={layout}>
      <div
        data-dashboard-grid=""
        className={cn(
          "grid min-w-0 grid-cols-1",
          layout === "report" ? "gap-4 md:grid-cols-2 xl:grid-cols-3" : "gap-3 lg:grid-cols-2 2xl:grid-cols-3",
        )}
      >
        {visible.tasks && <TaskBlock pair={data.tasks} windows={data.windows} onReload={onReload} />}
        {visible.incomingDocuments && (
          <DocumentBlock pair={data.incomingDocuments} windows={data.windows} onReload={onReload} />
        )}
        {visible.budget && <PendingBlock blockKey="budget" name="Giải ngân ngân sách" />}
        {visible.budget && <FiscalBlock result={data.fiscal} year={data.fiscalYear} onReload={onReload} />}
        {visible.citizenReports && (
          <CitizenReportBlock pair={data.citizenReports} windows={data.windows} onReload={onReload} />
        )}
        <PendingBlock blockKey="economy" name="Kinh tế & Tài nguyên" />
        {layout === "dashboard" && anyQueue && (
          <UrgentPanel queue={data.queue} taskTypeLabels={data.taskTypeLabels} />
        )}
      </div>
    </BlockLayoutContext.Provider>
  );
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * EXPORT — the blocks above, as the files carry them (`export-model.ts`)
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * Has every block the account sees ANSWERED — success or failure, but no longer loading? The export
 * buttons wait for it: a file written while a tile still says "…" would carry a figure nobody saw.
 * At least one figure block must be visible; a file of placeholders only is not a report.
 */
export function figuresSettled(data: BlocksData, visible: BlockVisibility, withQueue = true): boolean {
  const settled = <T,>(p: SummaryPair<T>) => p.current !== null && p.previous !== null;
  if (!(visible.tasks || visible.incomingDocuments || visible.citizenReports || visible.budget)) return false;
  if (visible.tasks && !settled(data.tasks)) return false;
  if (visible.incomingDocuments && !settled(data.incomingDocuments)) return false;
  if (visible.citizenReports && !settled(data.citizenReports)) return false;
  if (visible.budget && data.fiscal === null) return false;
  // `/bao-cao` never reads the queues (no "Cần xử lý ngay"), so it passes `withQueue = false`.
  const anyQueue = visible.tasks || visible.incomingDocuments || visible.citizenReports;
  if (withQueue && anyQueue && data.queue === null) return false;
  return true;
}

function toExportFigure(f: Figure, unit: "count" | "dong" = "count", layout: BlockLayout = "dashboard"): ExportFigure {
  const shown = figureFor(layout, f);
  return {
    label: shown.label,
    value: shown.display,
    exact: shown.exact,
    number: shown.raw,
    unit: shown.raw === undefined ? undefined : unit,
    note: shown.note,
    // `/bao-cao`'s delta line ("không đổi" under 0,05%) where it has one; else the line both pages
    // carry ("Kỳ trước: 0"). Tổng quan: its own line, unchanged.
    comparison: (layout === "report" ? reportDelta(shown)?.text : undefined) ?? shown.comparison?.text,
    alert: shown.alert === true,
  };
}

/** `PairErrors` in words — the same two sentences the block draws. */
function pairErrors<T>(pair: SummaryPair<T>, noun: string): string[] {
  if (pair.current !== null && !pair.current.ok) {
    return [`Chưa tải được số liệu ${noun}: ${pair.current.thongBao}`];
  }
  if (pair.current !== null && pair.current.ok && pair.previous !== null && !pair.previous.ok) {
    return [`Không tải được số liệu kỳ trước: ${pair.previous.thongBao}`];
  }
  return [];
}

function pendingExportBlock(name: keyof typeof PENDING_BLOCK_METRICS): ExportBlock {
  return {
    title: name,
    figures: PENDING_BLOCK_METRICS[name].map((label) => ({ label, value: EXPORT_NO_FIGURE, noFigure: true })),
    notes: [],
    errors: [],
    pending: pendingPart(name).viSao,
  };
}

/** The "Tổng chi" tile of `/bao-cao`, as the files carry it: no number, the reason. */
function expenditureTotalExport(): ExportFigure {
  return {
    label: EXPENDITURE_TOTAL_LABEL,
    value: EXPORT_NO_FIGURE,
    note: reportPendingPart(EXPENDITURE_TOTAL_LABEL).viSao,
    noFigure: true,
  };
}

function fiscalExportBlock(
  result: Loaded<finance_chiSoNamRa>,
  year: number,
  layout: BlockLayout = "dashboard",
): ExportBlock {
  const title = FISCAL_TITLE;
  const report = layout === "report";
  if (result === null || !result.ok) {
    const none = (label: string): ExportFigure => ({ label, value: NO_VALUE });
    return {
      title,
      figures: report
        ? [none("Thu đạt dự toán"), none("Chi đạt dự toán"), expenditureTotalExport(), none(NHAN_CHENH_LECH)]
        : ["Thu đạt dự toán", "Chi đạt dự toán", NHAN_CHENH_LECH].map(none),
      notes: [fiscalScope(year)],
      errors: result === null ? [] : [`Chưa tải được số liệu thu – chi: ${result.thongBao}`],
    };
  }
  const d = result.duLieu;
  const totals: ExportFigure[] = d.revenue_totals.map((o) => {
    const t = revenueTotalText(o, layout);
    return t.reason === null
      ? { label: o.name, value: t.shown, exact: t.exact, number: t.raw, unit: t.raw === undefined ? undefined : "dong" }
      : { label: o.name, value: `${O_KHONG_TINH_DUOC} — ${t.reason}` };
  });
  const revenue = toExportFigure(indicatorFigure("fiscal-revenue", d.revenue_achievement), "count", layout);
  const expenditure = toExportFigure(indicatorFigure("fiscal-expenditure", d.expenditure_achievement), "count", layout);
  const balance = toExportFigure(balanceFigure(d, layout), "dong", layout);
  return {
    title,
    // The order each page draws (`FiscalBlock`).
    figures: report
      ? [revenue, ...totals, expenditure, expenditureTotalExport(), balance]
      : [revenue, expenditure, balance, ...totals],
    notes: [fiscalScope(year), balanceNote()],
    errors: [],
  };
}

/**
 * The blocks the account sees, in the grid's order, as the files carry them. A block whose keys the
 * account lacks is absent here exactly as on screen — a file must not reveal what the page hides.
 * "Cần xử lý ngay" is not a block here: it goes in as an aggregate (`urgentSummary`).
 *
 * `layout = "report"` = `/bao-cao`'s files: its labels, its fiscal order and sums, no red — what that
 * page draws. Tổng quan's files are the default and do not change.
 */
export function exportBlocks(
  data: BlocksData,
  visible: BlockVisibility,
  layout: BlockLayout = "dashboard",
): ExportBlock[] {
  const out = (f: Figure) => toExportFigure(f, "count", layout);
  const blocks: ExportBlock[] = [];
  if (visible.tasks) {
    const t = taskFigures(data.tasks, data.windows);
    blocks.push({
      title: "Nhiệm vụ",
      figures: [t.inProgress, t.overdue, t.completed, t.suspended, t.onTime].map(out),
      notes: [],
      errors: pairErrors(data.tasks, "nhiệm vụ"),
    });
  }
  if (visible.incomingDocuments) {
    const d = incomingDocumentFigures(data.incomingDocuments, data.windows);
    blocks.push({
      title: "Văn bản & Đơn thư",
      figures: [
        ...[d.arrived, d.open, d.overdue].map(out),
        { label: DOCUMENT_ON_TIME_LABEL, value: NO_DATA_CAPTION, note: NO_SOURCE_DATA, noFigure: true },
        { label: LETTERS_LABEL, value: EXPORT_NO_FIGURE, note: pendingPart(LETTERS_LABEL).viSao, noFigure: true },
      ],
      notes: [],
      errors: pairErrors(data.incomingDocuments, "văn bản đến"),
    });
  }
  if (visible.budget) {
    blocks.push(pendingExportBlock("Giải ngân ngân sách"));
    blocks.push(fiscalExportBlock(data.fiscal, data.fiscalYear, layout));
  }
  if (visible.citizenReports) {
    blocks.push({
      title: "Phản ánh người dân",
      figures: citizenReportFigures(data.citizenReports, data.windows).map(out),
      notes: [],
      errors: pairErrors(data.citizenReports, "phản ánh"),
    });
  }
  blocks.push(pendingExportBlock("Kinh tế & Tài nguyên"));
  return blocks;
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * `/bao-cao`'s "So sánh với kỳ trước" — the figures its chart plots
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/** One bar of the comparison chart: a PERIOD figure whose two windows both answered. */
export type ComparedFigure = {
  readonly id: string;
  /** the tile's label, with the block named when two blocks share it ("Đúng hạn trong kỳ") */
  readonly label: string;
  readonly currentText: string;
  readonly previousText: string;
  /** change against the previous window, in percent — never computed from a previous of 0 */
  readonly delta: number;
  readonly trend: Trend;
};

/**
 * What the chart may plot, in the grid's order. PERIOD FIGURES ONLY (owner 09/10/2026, D2): a stock
 * figure has no previous period, Thu – Chi is year to date with no comparison, and "Điểm hài lòng" is a
 * stock. A figure is left out when either window failed, the sample was empty, or the previous value
 * is 0 — "from 0 to 3" is not a percentage (`comparisonLine`, user decision), and the tile says so.
 *
 * `null` = some visible compared block is still loading: the chart waits rather than draw a partial
 * picture as the whole one.
 */
export function comparedFigures(data: BlocksData, visible: BlockVisibility): ComparedFigure[] | null {
  const settled = <T,>(p: SummaryPair<T>) => p.current !== null && p.previous !== null;
  if (visible.tasks && !settled(data.tasks)) return null;
  if (visible.incomingDocuments && !settled(data.incomingDocuments)) return null;
  if (visible.citizenReports && !settled(data.citizenReports)) return null;

  const groups: { figures: Figure[]; block: string }[] = [];
  if (visible.tasks) {
    const t = taskFigures(data.tasks, data.windows);
    groups.push({ figures: [t.completed, t.onTime], block: "nhiệm vụ" });
  }
  if (visible.incomingDocuments) {
    groups.push({ figures: [incomingDocumentFigures(data.incomingDocuments, data.windows).arrived], block: "văn bản" });
  }
  if (visible.citizenReports) {
    groups.push({ figures: citizenReportFigures(data.citizenReports, data.windows), block: "phản ánh" });
  }
  const rows: { f: Figure; v: NonNullable<Figure["values"]>; block: string }[] = [];
  for (const { figures, block } of groups) {
    for (const raw of figures) {
      const f = figureFor("report", raw);
      if (f.kind === "period" && f.values !== undefined && f.values.previous !== 0) rows.push({ f, v: f.values, block });
    }
  }
  const count = new Map<string, number>();
  for (const { f } of rows) count.set(f.label, (count.get(f.label) ?? 0) + 1);
  return rows.map(({ f, v, block }) => ({
    id: f.id,
    label: (count.get(f.label) ?? 0) > 1 ? `${f.label} (${block})` : f.label,
    currentText: f.display,
    previousText: v.previousText,
    delta: ((v.current - v.previous) / v.previous) * 100,
    trend: f.trend ?? "neutral",
  }));
}

/**
 * The whole file content at the instant of the click, or throws — `buildDashboardExport` refuses a
 * commune with no name. `null` source = nothing exportable (gate closed, commune unknown).
 */
export function dashboardExport(
  data: DashboardData,
  visible: BlockVisibility,
  commune: ExportCommune,
  generatedAt: number,
): DashboardExport {
  const anyQueue = visible.tasks || visible.incomingDocuments || visible.citizenReports;
  const compared = anyQueue;
  return buildDashboardExport({
    commune,
    title: DASHBOARD_TITLE,
    windows: data.windows,
    fetchedAt: data.fetchedAt,
    generatedAt,
    blocks: exportBlocks(data, visible),
    urgent:
      !anyQueue || data.queue === null
        ? null
        : data.queue.ok
          ? urgentSummary(data.queue.duLieu)
          : { title: "Cần xử lý ngay", lines: [`Chưa tải được danh sách cần xử lý ngay: ${data.queue.thongBao}`] },
    comparisonNote: compared ? comparisonNote(data.windows) : null,
  });
}
