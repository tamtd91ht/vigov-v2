import Link from "next/link";

import { OTien } from "@/features/budget/money-cell";
import {
  GHI_CHU_CHENH_LECH,
  lyDoKhongTinh,
  NHAN_CHENH_LECH,
  nhanChiSo,
  nhanSoTien,
  nhanSoTienChiSo,
} from "@/features/budget/budget-labels";
import type { KetQua } from "@/lib/api/request";
import type {
  documents_incomingSummaryOut,
  finance_chiSoNamRa,
  finance_chiSoRa,
  petitions_citizenReportSummaryOut,
  petitions_taskSummaryOut,
} from "@/lib/api/schema.gen";
import {
  QUYEN_XEM_GIAI_NGAN,
  QUYEN_XEM_NHIEM_VU,
  QUYEN_XEM_PHAN_ANH,
  QUYEN_XEM_VAN_BAN,
} from "@/lib/permissions";

import styles from "./dashboard.module.css";
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
 */

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

function display(v: Read, format: (n: number) => string): string {
  if (v === undefined) return "…";
  return v === null ? NO_VALUE : format(v);
}

/** One figure, fully decided — what `FigureCell` draws. */
type Figure = {
  readonly id: string;
  readonly label: string;
  readonly display: string;
  readonly href: string;
  readonly comparison: ComparisonLine | null;
  readonly note?: string;
  /** red value, for the figures spec §4 paints red (overdue, late) — and only when non-zero */
  readonly alert?: boolean;
  /** the value is a server SENTENCE (fiscal reason), not a number: normal size, not 2rem */
  readonly sentence?: boolean;
};

const TONE_CLASS: Readonly<Record<ComparisonLine["tone"], string | undefined>> = {
  better: styles.better,
  worse: styles.worse,
  neutral: styles.neutral,
};

/**
 * One figure: a LINK named `Xem danh sách đằng sau: {nhãn}` (spec §4, §9) to the list behind it.
 *
 * THE ACCESSIBLE NAME REPLACES THE VISIBLE TEXT, so the value is wired back with
 * `aria-describedby` — otherwise a screen-reader user hears where the link goes and never the
 * number it is about.
 */
export function FigureCell({ figure }: { figure: Figure }) {
  const valueId = `${figure.id}-value`;
  const valueClass = figure.sentence
    ? styles.sentence
    : figure.alert
      ? `${styles.value} ${styles.valueAlert}`
      : styles.value;
  return (
    <li className={styles.cell}>
      <Link
        className={styles.figureLink}
        href={figure.href}
        aria-label={drillLabel(figure.label)}
        aria-describedby={valueId}
      >
        <span id={valueId} className={valueClass}>
          {figure.display}
        </span>
        <span className={styles.label}>{figure.label}</span>
      </Link>
      {figure.note !== undefined && <span className={styles.note}>{figure.note}</span>}
      {figure.comparison !== null && (
        <span className={TONE_CLASS[figure.comparison.tone]}>{figure.comparison.text}</span>
      )}
    </li>
  );
}

/** A cell with no source data in wave 1: ONE muted line, never a 0, never a link. */
export function NoSourceCell({ label }: { label: string }) {
  return (
    <li className={styles.cell}>
      <span className={styles.label}>{label}</span>
      <span className={styles.muted}>{NO_SOURCE_DATA}</span>
    </li>
  );
}

/** A whole block with no source data in wave 1. */
export function NoSourceBlock({ title }: { title: string }) {
  return (
    <section className={styles.block} aria-label={title}>
      <h2 className={styles.blockTitle}>{title}</h2>
      <p className={styles.muted}>{NO_SOURCE_DATA}</p>
    </section>
  );
}

/** The error lines of a pair — the current call's sentence, and the previous call's if it failed. */
function PairErrors<T>({ pair }: { pair: SummaryPair<T> }) {
  return (
    <>
      {pair.current !== null && !pair.current.ok && (
        <p className="thong-bao-loi" role="alert">
          {pair.current.thongBao}
        </p>
      )}
      {pair.current !== null && pair.current.ok && pair.previous !== null && !pair.previous.ok && (
        <p className="thong-bao-loi" role="alert">
          Không tải được số liệu kỳ trước: {pair.previous.thongBao}
        </p>
      )}
    </>
  );
}

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
    alert: spec.alert === true && typeof current === "number" && current > 0,
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
  };
}

/** Khối NHIỆM VỤ — spec §4.1, labels per the user's decision. */
export function TaskBlock({
  pair,
  windows,
}: {
  pair: SummaryPair<petitions_taskSummaryOut>;
  windows: PeriodWindows;
}) {
  const period = toQueryPeriod(windows.current);
  const id = "tasks";
  const count = (s: CountSpec<petitions_taskSummaryOut>) => countFigure(id, pair, s, period);
  const figures: Figure[] = [
    count({
      pick: (t) => t.in_progress,
      kind: "stock",
      trend: "neutral",
      target: { list: "tasks", metric: "in_progress" },
    }),
    count({
      pick: (t) => t.overdue,
      kind: "stock",
      trend: "lower-is-better",
      target: { list: "tasks", metric: "overdue" },
      alert: true,
    }),
    count({
      pick: (t) => t.suspended,
      kind: "stock",
      trend: "neutral",
      target: { list: "tasks", metric: "suspended" },
    }),
    count({
      pick: (t) => t.completed,
      kind: "period",
      trend: "higher-is-better",
      target: { list: "tasks", metric: "completed" },
    }),
    ratioFigure(
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
  ];
  return (
    <section className={styles.block} aria-label="Nhiệm vụ">
      <h2 className={styles.blockTitle}>Nhiệm vụ</h2>
      <PairErrors pair={pair} />
      <ul className={styles.cells}>
        {figures.map((f) => (
          <FigureCell key={f.id} figure={f} />
        ))}
      </ul>
    </section>
  );
}

/**
 * Khối VĂN BẢN ĐẾN — incoming documents only. The spec's block also counts citizen letters
 * (`don_thu`), which no service holds yet; that cell and the on-time rate (the register stores no
 * settled instant, `incoming_dashboard.go`) say so rather than show a number.
 */
export function IncomingDocumentBlock({
  pair,
  windows,
}: {
  pair: SummaryPair<documents_incomingSummaryOut>;
  windows: PeriodWindows;
}) {
  const period = toQueryPeriod(windows.current);
  const id = "incoming-documents";
  const count = (s: CountSpec<documents_incomingSummaryOut>) => countFigure(id, pair, s, period);
  const c = pair.current;
  const asOf = c !== null && c.ok ? Date.parse(c.duLieu.as_of) : Number.NaN;
  const stockNote = Number.isNaN(asOf) ? undefined : `tính đến ${formatDateTime(asOf)}`;
  const figures: Figure[] = [
    count({
      pick: (d) => d.arrived,
      kind: "period",
      trend: "neutral",
      target: { list: "incoming-documents", metric: "arrived" },
    }),
    {
      ...count({
        pick: (d) => d.open,
        kind: "stock",
        trend: "neutral",
        target: { list: "incoming-documents", metric: "open" },
      }),
      note: stockNote,
    },
    {
      ...count({
        pick: (d) => d.overdue,
        kind: "stock",
        trend: "lower-is-better",
        target: { list: "incoming-documents", metric: "overdue" },
        alert: true,
      }),
      note: stockNote,
    },
  ];
  return (
    <section className={styles.block} aria-label="Văn bản đến">
      <h2 className={styles.blockTitle}>Văn bản đến</h2>
      <PairErrors pair={pair} />
      <ul className={styles.cells}>
        {figures.map((f) => (
          <FigureCell key={f.id} figure={f} />
        ))}
        <NoSourceCell label="Tỷ lệ đúng hạn văn bản" />
        <NoSourceCell label="Đơn thư trong kỳ" />
      </ul>
    </section>
  );
}

/** Khối PHẢN ÁNH NGƯỜI DÂN — spec §4.5, labels per the user's decision. */
export function CitizenReportBlock({
  pair,
  windows,
}: {
  pair: SummaryPair<petitions_citizenReportSummaryOut>;
  windows: PeriodWindows;
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
    }),
    count({
      pick: (r) => r.in_progress,
      kind: "stock",
      trend: "neutral",
      target: { list: "citizen-reports", metric: "in_progress" },
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
    }),
  ];
  return (
    <section className={styles.block} aria-label="Phản ánh người dân">
      <h2 className={styles.blockTitle}>Phản ánh người dân</h2>
      <PairErrors pair={pair} />
      <ul className={styles.cells}>
        {figures.map((f) => (
          <FigureCell key={f.id} figure={f} />
        ))}
        <NoSourceCell label="Điểm hài lòng" />
      </ul>
    </section>
  );
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
 * server's reason renders that SENTENCE instead of a figure (ADR 0035 §A), never `0`.
 *
 * `NHAN_CHENH_LECH` IS THE TEMPORARY LABEL of the balance cell: open question #32 is disputed and
 * the user decided on 25/09/2026 to ask the customer and relabel the KPI meanwhile
 * (`kb/50-doi-chieu/2026-09-25-feat-m8-multitenant-foundation.md`, summary row 6). The note under
 * the block states how the figure is computed and that it awaits the customer.
 */
export function FiscalBlock({ result, year }: { result: Loaded<finance_chiSoNamRa>; year: number }) {
  const title = "Thu – Chi ngân sách";
  const scope = `Luỹ kế năm ${year}, không so với kỳ trước.`;

  if (result === null || !result.ok) {
    const shown = result === null ? "…" : NO_VALUE;
    const fixed = ["Thu đạt dự toán", "Chi đạt dự toán", NHAN_CHENH_LECH];
    return (
      <section className={styles.block} aria-label={title}>
        <h2 className={styles.blockTitle}>{title}</h2>
        {result !== null && (
          <p className="thong-bao-loi" role="alert">
            {result.thongBao}
          </p>
        )}
        <ul className={styles.cells}>
          {fixed.map((label, i) => (
            <FigureCell
              key={label}
              figure={{
                id: `fiscal-${i}`,
                label,
                display: shown,
                href: FISCAL_SCREEN_PATH,
                comparison: null,
              }}
            />
          ))}
        </ul>
        <p className={styles.note}>{scope}</p>
      </section>
    );
  }

  const d = result.duLieu;
  const balanceText = nhanSoTienChiSo(d.balance, "dong");
  return (
    <section className={styles.block} aria-label={title}>
      <h2 className={styles.blockTitle}>{title}</h2>
      <ul className={styles.cells}>
        <FigureCell figure={indicatorFigure("fiscal-revenue", d.revenue_achievement)} />
        <FigureCell figure={indicatorFigure("fiscal-expenditure", d.expenditure_achievement)} />
        <FigureCell
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
            <li key={o.column_id} className={styles.cell}>
              <Link
                className={styles.figureLink}
                href={FISCAL_SCREEN_PATH}
                aria-label={drillLabel(o.name)}
                aria-describedby={valueId}
              >
                <span id={valueId} className={reason === null ? styles.value : styles.sentence}>
                  <OTien
                    chu={withUnit(nhanSoTien(o.value, "dong"), o.value !== null)}
                    lyDo={reason}
                    hienLyDo
                  />
                </span>
                <span className={styles.label}>{o.name}</span>
              </Link>
            </li>
          );
        })}
      </ul>
      <p className={styles.note}>{scope}</p>
      <p className={styles.note}>
        {NHAN_CHENH_LECH}: {GHI_CHU_CHENH_LECH}
      </p>
    </section>
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
      <section className={styles.urgent} aria-label={title}>
        <h2 className={styles.blockTitle}>{title}</h2>
        {queue === null ? (
          <p role="status">Đang tải…</p>
        ) : (
          <p className="thong-bao-loi" role="alert">
            {queue.thongBao}
          </p>
        )}
      </section>
    );
  }
  const { rows, failures } = queue.duLieu;
  return (
    <section className={styles.urgent} aria-label={title}>
      <h2 className={styles.blockTitle}>
        {title}
        <span className={styles.count}>{rows.length}</span>
      </h2>
      {failures.map((f) => (
        <p key={f.module} className="thong-bao-loi" role="alert">
          {MODULE_LABEL[f.module]}: {f.message}
        </p>
      ))}
      {rows.length === 0 && failures.length === 0 && (
        <p className="trang-thai-rong">{NOTHING_URGENT}</p>
      )}
      {rows.length > 0 && (
        <ul className={styles.urgentList}>
          {rows.map((r) => {
            const category = categoryLabel(r, taskTypeLabels);
            return (
              <li key={`${r.module}|${r.code}|${r.kind}`} className={styles.urgentRow}>
                <span className="chip chip-ngung">{MODULE_LABEL[r.module]}</span>
                <span className={styles.urgentCode}>{r.code}</span>
                <span>{kindLabel(r.kind)}</span>
                {category !== null && <span className={styles.note}>{category}</span>}
                <span className={styles.missed}>{missedSinceText(r.missedAt)}</span>
                {r.critical && <span className={styles.critical}>Nghiêm trọng</span>}
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}

/** The period picker — a segmented control of four buttons, one pressed. */
export function PeriodPicker({
  kind,
  onChange,
}: {
  kind: PeriodKind;
  onChange: (k: PeriodKind) => void;
}) {
  return (
    <div className={styles.periodBar} role="group" aria-label="Kỳ báo cáo">
      {PERIOD_KINDS.map((k) => (
        <button
          key={k}
          type="button"
          className={styles.periodButton}
          aria-pressed={k === kind}
          onClick={() => onChange(k)}
        >
          {PERIOD_BUTTON_LABEL[k]}
        </button>
      ))}
    </div>
  );
}

/** Everything the page shows, already loaded (or loading) — the hook half assembles this. */
export type DashboardData = {
  readonly windows: PeriodWindows;
  /** The instant the windows were computed and the figures asked for. */
  readonly fetchedAt: number;
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
 * The page body. A block whose keys the account lacks is NOT RENDERED — not drawn as 0, not drawn
 * as "—": a figure the account may not read must not be implied either way.
 *
 * `Kinh tế & Tài nguyên` has no source data and no read key of its own in wave 1; it opens nothing,
 * so it shows under the page gate (`report.read`) alone.
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
  const anyQueue = visible.tasks || visible.incomingDocuments || visible.citizenReports;
  return (
    <>
      <p className={styles.meta}>
        {periodMetaLabel(data.windows)} · tính đến {formatDateTime(data.fetchedAt)}
      </p>
      <PeriodPicker kind={data.windows.kind} onChange={onPeriodChange} />
      <p className={styles.meta}>{comparisonNote(data.windows)}</p>
      <div className={styles.grid}>
        {visible.tasks && <TaskBlock pair={data.tasks} windows={data.windows} />}
        {visible.incomingDocuments && (
          <IncomingDocumentBlock pair={data.incomingDocuments} windows={data.windows} />
        )}
        {visible.budget && <NoSourceBlock title="Giải ngân ngân sách" />}
        {visible.budget && <FiscalBlock result={data.fiscal} year={data.fiscalYear} />}
        {visible.citizenReports && (
          <CitizenReportBlock pair={data.citizenReports} windows={data.windows} />
        )}
        <NoSourceBlock title="Kinh tế & Tài nguyên" />
      </div>
      {anyQueue && <UrgentPanel queue={data.queue} taskTypeLabels={data.taskTypeLabels} />}
    </>
  );
}
