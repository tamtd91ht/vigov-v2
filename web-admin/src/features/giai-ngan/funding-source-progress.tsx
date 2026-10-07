"use client";

import { Plus, Settings2 } from "lucide-react";
import { useEffect, useState } from "react";

import { Button } from "@/components/ui/button";
import { ErrorState } from "@/components/ui/error-state";
import { Skeleton } from "@/components/ui/skeleton";
import { listFundingSources } from "@/lib/api/funding-sources";
import type { KetQua } from "@/lib/api/goi"; // vi-name-ok: existing result type of goi.ts (rule 12 invariant 3)
import type { finance_fundingSourceOut, finance_fundingSourcesOut } from "@/lib/api/schema.gen";
import { cn } from "@/lib/cn";

import { Panel } from "./disbursement-overview";
import { FundingSourceProjectsDialog, ManageFundingSourcesDialog } from "./funding-source-dialogs";
// vi-name-ok: existing formatters of nhan-du-an.ts, imported unchanged (rule 12, invariant 3)
import { nhanTien, shortDongLabel } from "./nhan-du-an";
import { PROGRESS_BAR_CLASS, progressTone } from "./progress-tone";
import { Glyph } from "./project-ui";
import { TRACK_CLASS } from "./spec-classes";

/**
 * §6 "Tiến độ theo nguồn vốn" for the selected budget year, in the prototype's composition
 * (`SourceReportPanel.tsx`, `BudgetWorkspace.tsx:260-269, 446-456`): one card per source with three
 * bars, the §13 rule 6 warning under them, `Quản lý nguồn vốn` and the per-source project list.
 *
 * ⚠ THE PROTOTYPE'S FIELD NAMES ARE INVERTED against the contract: its `allocated_amount` is our
 * `granted_amount` (vốn được giao), its `annual_plan_amount` is our `allocated_amount` (đã phân bổ vào
 * dự án). Read the CONTRACT names below, never the prototype's.
 *
 * EVERY FIGURE IS THE SERVER'S. The three ratios arrive in hundredths of a percent, `null` when the
 * denominator is 0, NOT clamped; `unallocated_amount` / `overallocated_amount` arrive computed. This
 * file only draws them — a sum or a division here is a second answer to a question `finance` already
 * answered, and it would drift the day a voucher changes state.
 *
 * READ UNDER `budget.read` (the caller mounts it inside `CongQuyen`); the two write entries are drawn
 * only with `budget.update` (`canManage`). Hiding is UX only — `finance` checks both keys (rule 5).
 *
 * NO SCOPE BANNER HERE: the same `scope_notice` is already drawn once above the screen by the project
 * register (`bang-du-an.tsx`). Two copies of one banner is noise, not emphasis.
 *
 * LOOK = spec 06 §A (ADR 0068 lần 6): `Quản lý nguồn vốn` in the BODY above the grid (small outline),
 * the empty state ONE line + `+ Thêm nguồn vốn`, short amounts (`shortDongLabel`, the spec's
 * `formatDongShort`), the allocation meter #8AA2B8 and the two disbursed meters in the 80/50/30 tier.
 */
export function FundingSourceProgress({ year, canManage }: { year: number; canManage: boolean }) {
  /** Bumped after every write and by "Tải lại": the cards re-read, never patched from a write reply. */
  const [reloads, setReloads] = useState(0);
  const key = `${year}|${reloads}`;
  // The result is kept WITH the key that produced it, and "loading" is derived from a mismatch — so a
  // previous year's money never stands under a select already showing the new year (`bang-du-an.tsx`).
  const [loaded, setLoaded] = useState<{ key: string; result: KetQua<finance_fundingSourcesOut> } | null>(null);
  const [drill, setDrill] = useState<finance_fundingSourceOut | null>(null);
  const [managing, setManaging] = useState(false);

  useEffect(() => {
    let dropped = false;
    listFundingSources(year).then((result) => {
      if (!dropped) setLoaded({ key, result });
    });
    return () => {
      dropped = true;
    };
  }, [year, key]);

  const current = loaded !== null && loaded.key === key ? loaded.result : null;
  const data = current !== null && current.ok ? current.duLieu : null;
  const reload = () => setReloads((n) => n + 1);

  return (
    <Panel title="Tiến độ theo nguồn vốn" titleId="tieu-de-tien-do-nguon-von" className="mb-5">
      {current === null && (
        <>
          <p role="status" className="an-thi-giac">
            Đang tải tiến độ theo nguồn vốn…
          </p>
          <Skeleton className="h-64 w-full" />
        </>
      )}
      {current !== null && !current.ok && (
        <ErrorState
          title="Chưa tải được tiến độ theo nguồn vốn"
          message={<span role="alert">{current.thongBao}</span>}
          onRetry={reload}
        />
      )}
      {data !== null && (
        <FundingSourceCards
          data={data}
          canManage={canManage}
          onOpenSource={setDrill}
          onManage={() => setManaging(true)}
        />
      )}

      {drill !== null && data !== null && (
        <FundingSourceProjectsDialog source={drill} year={data.year} onClose={() => setDrill(null)} />
      )}
      {managing && canManage && (
        <ManageFundingSourcesDialog
          year={year}
          sources={data?.items ?? null}
          onChanged={reload}
          onClose={() => setManaging(false)}
        />
      )}
    </Panel>
  );
}

/**
 * The cards and the warning, from one server reply. Hook-free, so it renders on its own in tests.
 *
 * `year` comes from the REPLY, not from the select: a reply that does not say which year it was read
 * for cannot be told apart from another year's.
 */
export function FundingSourceCards({
  data,
  canManage,
  onOpenSource,
  onManage,
}: {
  data: finance_fundingSourcesOut;
  canManage: boolean;
  onOpenSource: (source: finance_fundingSourceOut) => void;
  onManage: () => void;
}) {
  if (data.items.length === 0) {
    // Spec 06 §A "Chưa có nguồn": one line and one button, no explanation box.
    return (
      <div>
        <p className="text-ink-muted m-0 text-[12.5px]">Chưa khai báo nguồn vốn nào.</p>
        {canManage && (
          <Button
            type="button"
            variant="outline"
            className="mt-3"
            icon={<Glyph icon={Plus} />}
            aria-haspopup="dialog"
            onClick={onManage}
          >
            Thêm nguồn vốn
          </Button>
        )}
        <UnattributedLine amount={data.unattributed_disbursed_amount} />
      </div>
    );
  }
  return (
    <div className="flex min-w-0 flex-col gap-4">
      {canManage && (
        <div className="flex justify-end">
          <Button
            type="button"
            variant="outline"
            size="sm"
            icon={<Glyph icon={Settings2} className="size-3.5" />}
            aria-haspopup="dialog"
            onClick={onManage}
          >
            Quản lý nguồn vốn
          </Button>
        </div>
      )}
      <ul className="m-0 grid min-w-0 list-none gap-4 p-0 lg:grid-cols-2" aria-label={`Nguồn vốn năm ${data.year}`}>
        {data.items.map((source) => (
          <SourceCard key={source.id} source={source} onOpen={onOpenSource} />
        ))}
      </ul>
      <UnattributedLine amount={data.unattributed_disbursed_amount} />
    </div>
  );
}

/**
 * §13 rule 6, said and not hidden (spec 06 §A): the cards alone do not add up to the year's disbursed
 * total, and a reader who finds that out alone concludes one of the two figures is wrong.
 */
function UnattributedLine({ amount }: { amount: number }) {
  if (!(amount > 0)) return null;
  return (
    <p className="text-tangerine m-0 mt-3 text-[12px]" data-unattributed="">
      Còn <Money amount={amount} /> đã chi nhưng chưa ghi rút từ nguồn nào — thuộc các dự án chưa khai phân bổ
      nguồn vốn.
    </p>
  );
}

/**
 * One source: (1) allocated to projects out of what was granted, (2) disbursed out of what was
 * allocated, (3) disbursed out of what was granted. (2) and (3) differ by exactly the money not yet
 * allocated — the gap that is leadership's next question, which is why they are two bars, not one.
 *
 * GRANTED 0 = NOTHING ENTERED FOR THE YEAR: every ratio over it is meaningless (`null` from the
 * server), so only bar (2) is drawn and the card says the figure is missing (prototype `:114-119`).
 */
function SourceCard({
  source,
  onOpen,
}: {
  source: finance_fundingSourceOut;
  onOpen: (source: finance_fundingSourceOut) => void;
}) {
  const knowsGrant = source.granted_amount > 0;
  const over = source.overallocated_amount > 0;

  return (
    <li className="border-line min-w-0 rounded-[10px] border border-solid p-3" data-funding-source={source.id}>
      <div className="flex items-baseline justify-between gap-2">
        <span className="text-navy min-w-0 text-[13px] font-semibold">{source.name}</span>
        <span className="text-ink-muted shrink-0 text-[11px]">{source.project_count} dự án</span>
      </div>

      {/* 1 — allocated into projects. The allocated figure opens the projects behind it. */}
      <p className="text-ink m-0 mt-1.5 text-[12px]">
        Đã phân bổ{" "}
        <button
          type="button"
          aria-haspopup="dialog"
          title="Xem các dự án đang lấy vốn từ nguồn này"
          onClick={() => onOpen(source)}
          className="text-navy decoration-brand/50 cursor-pointer border-0 bg-transparent p-0 [font:inherit] font-bold underline decoration-dotted underline-offset-2"
        >
          <Money amount={source.allocated_amount} />
        </button>
        {knowsGrant ? (
          <>
            {" / "}
            <b className="text-navy">
              <Money amount={source.granted_amount} />
            </b>
            {over ? (
              <span className="text-danger font-semibold" data-overallocated="">
                {" · vượt nguồn "}
                <Money amount={source.overallocated_amount} />
              </span>
            ) : (
              <span className={cn(source.unallocated_amount > 0 ? "text-tangerine font-semibold" : "text-ink-muted")}>
                {" · còn "}
                <Money amount={source.unallocated_amount} /> chưa phân bổ
              </span>
            )}
          </>
        ) : (
          <span className="text-ink-muted"> · chưa nhập số được giao</span>
        )}
      </p>
      {knowsGrant && <Meter ratio={source.allocated_ratio} label="Tỷ lệ đã phân bổ trên tổng nguồn" allocation />}

      {/* 2 — disbursed over what was allocated: are the projects moving. */}
      <p className="text-ink m-0 mt-2.5 text-[12px]">
        Đã giải ngân{" "}
        <b className="text-navy">
          <Money amount={source.disbursed_amount} />
        </b>
        {" / "}
        <b className="text-navy">
          <Money amount={source.allocated_amount} />
        </b>{" "}
        đã phân bổ
      </p>
      <Meter ratio={source.disbursed_of_allocated_ratio} label="Tỷ lệ đã giải ngân trên phần đã phân bổ" />

      {/* 3 — disbursed over the whole grant: has the money actually gone out. */}
      {knowsGrant && (
        <>
          <p className="text-ink m-0 mt-2 text-[12px]">
            Đã giải ngân{" "}
            <b className="text-navy">
              <Money amount={source.disbursed_amount} />
            </b>
            {" / "}
            <b className="text-navy">
              <Money amount={source.granted_amount} />
            </b>{" "}
            tổng nguồn
          </p>
          <Meter ratio={source.disbursed_of_granted_ratio} label="Tỷ lệ đã giải ngân trên tổng nguồn" />
        </>
      )}
    </li>
  );
}

/** A short amount ("9,2 tỷ", `shortDongLabel` = the spec's `formatDongShort`), the exact đồng on hover. */
function Money({ amount }: { amount: number }) {
  return <span title={nhanTien(amount)}>{shortDongLabel(amount)}</span>;
}

/** Spec 06 §A `Bar`'s percent: at most one decimal, `vi-VN`. */
const METER_PERCENT = new Intl.NumberFormat("vi-VN", { maximumFractionDigits: 1 });

/**
 * Spec 06 §A `Bar`: an `h-1.5` track, the percent `w-11` 11.5px bold. The allocation bar is #8AA2B8;
 * the disbursed bars take the 80/50/30 tier. The bar is capped at full width; the WORDS keep the real
 * figure, above 100% included (§13 rule 2). `null` (no denominator) is "—" over an empty track, never
 * "0%": 0% claims a denominator existed and nothing moved.
 */
function Meter({
  ratio,
  label,
  allocation = false,
}: {
  ratio: number | null;
  label: string;
  /** The "đã phân bổ / được giao" bar: the spec's fixed #8AA2B8, not a progress tier. */
  allocation?: boolean;
}) {
  const known = ratio !== null && Number.isFinite(ratio);
  const width = known ? Math.min(100, Math.max(0, ratio / 100)) : 0;
  const fill = allocation ? "bg-[#8AA2B8]" : known ? PROGRESS_BAR_CLASS[progressTone(ratio)] : "";
  return (
    <div className="mt-1 flex items-center gap-2" data-meter="">
      <div aria-hidden="true" className={cn("h-1.5 min-w-0 flex-1 overflow-hidden rounded-full", TRACK_CLASS)}>
        <div className={cn("h-full rounded-full", fill)} style={{ width: `${width}%` }} />
      </div>
      <span title={label} className="text-navy w-11 shrink-0 text-right text-[11.5px] font-bold tabular-nums">
        <span className="an-thi-giac">{label}: </span>
        {known ? `${METER_PERCENT.format(ratio / 100)}%` : "—"}
      </span>
    </div>
  );
}
