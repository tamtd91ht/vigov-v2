"use client";

import { Landmark, Plus, Settings2 } from "lucide-react";
import { useEffect, useState } from "react";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { Skeleton } from "@/components/ui/skeleton";
import { listFundingSources } from "@/lib/api/funding-sources";
import type { KetQua } from "@/lib/api/goi"; // vi-name-ok: existing result type of goi.ts (rule 12 invariant 3)
import type { finance_fundingSourceOut, finance_fundingSourcesOut } from "@/lib/api/schema.gen";
import { cn } from "@/lib/cn";
import { compactDong } from "@/lib/compact-dong";

import { FundingSourceProjectsDialog, ManageFundingSourcesDialog } from "./funding-source-dialogs";
// vi-name-ok: existing formatters of nhan-du-an.ts, imported unchanged (rule 12, invariant 3)
import { nhanTien, nhanTyLeGiaiNgan } from "./nhan-du-an";
import { Glyph } from "./project-ui";

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
    <Card as="section" className="mb-5" aria-labelledby="tieu-de-tien-do-nguon-von">
      <CardHeader>
        <CardTitle id="tieu-de-tien-do-nguon-von">Tiến độ theo nguồn vốn</CardTitle>
        {/* Prototype `SourceReportPanel.tsx:75-82`: the way back to a source's yearly figure, drawn
            once the catalogue has a source (the empty state carries `+ Thêm nguồn vốn` instead). */}
        {canManage && data !== null && data.items.length > 0 && (
          <Button
            type="button"
            variant="secondary"
            size="sm"
            className="ml-auto"
            icon={<Glyph icon={Settings2} />}
            aria-haspopup="dialog"
            onClick={() => setManaging(true)}
          >
            Quản lý nguồn vốn
          </Button>
        )}
      </CardHeader>
      <CardContent>
        {current === null && (
          <>
            <p role="status" className="an-thi-giac">
              Đang tải tiến độ theo nguồn vốn…
            </p>
            <Skeleton className="h-32 w-full" />
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
      </CardContent>

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
    </Card>
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
  return (
    <div className="flex min-w-0 flex-col gap-4">
      {data.items.length === 0 ? (
        <EmptyState
          icon={Landmark}
          title="Chưa khai báo nguồn vốn nào"
          description="Khai các nguồn vốn của xã để theo dõi số được giao, số đã phân bổ vào dự án và số đã giải ngân theo từng nguồn."
          action={
            canManage ? (
              <Button
                type="button"
                variant="secondary"
                icon={<Glyph icon={Plus} />}
                aria-haspopup="dialog"
                onClick={onManage}
              >
                Thêm nguồn vốn
              </Button>
            ) : undefined
          }
          className="py-6"
        />
      ) : (
        <ul className="m-0 grid min-w-0 list-none gap-4 p-0 lg:grid-cols-2" aria-label={`Nguồn vốn năm ${data.year}`}>
          {data.items.map((source) => (
            <SourceCard key={source.id} source={source} onOpen={onOpenSource} />
          ))}
        </ul>
      )}

      {/* §13 rule 6, said and not hidden: the cards alone do not add up to the year's disbursed total,
          and a reader who finds that out alone concludes one of the two figures is wrong. */}
      {data.unattributed_disbursed_amount > 0 && (
        <p className="m-0 text-[13px] text-warning-600" data-unattributed="">
          Còn <Money amount={data.unattributed_disbursed_amount} /> đã chi nhưng chưa ghi rút từ nguồn nào — thuộc các
          dự án chưa khai phân bổ nguồn vốn.
        </p>
      )}
    </div>
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
    <li className="min-w-0 rounded-[10px] border border-line p-3" data-funding-source={source.id}>
      <div className="flex items-baseline justify-between gap-2">
        <span className="min-w-0 text-[14px] font-semibold text-ink-900">{source.name}</span>
        <span className="shrink-0 text-xs text-ink-500">{source.project_count} dự án</span>
      </div>

      {/* 1 — allocated into projects. The allocated figure opens the projects behind it. */}
      <p className="m-0 mt-1.5 text-[13px] text-ink-700">
        Đã phân bổ{" "}
        <button
          type="button"
          aria-haspopup="dialog"
          title="Xem các dự án đang lấy vốn từ nguồn này"
          onClick={() => onOpen(source)}
          className="cursor-pointer border-0 bg-transparent p-0 font-semibold text-ink-900 underline decoration-dotted underline-offset-2 hover:text-brand-600"
        >
          <Money amount={source.allocated_amount} />
        </button>
        {knowsGrant ? (
          <>
            {" / tổng "}
            <b className="text-ink-900">
              <Money amount={source.granted_amount} />
            </b>
            {over ? (
              <span className="font-semibold text-danger-600" data-overallocated="">
                {" · vượt nguồn "}
                <Money amount={source.overallocated_amount} />
              </span>
            ) : (
              <span className={cn(source.unallocated_amount > 0 ? "font-semibold text-warning-600" : "text-ink-500")}>
                {" · còn "}
                <Money amount={source.unallocated_amount} /> chưa phân bổ
              </span>
            )}
          </>
        ) : (
          <span className="text-ink-500"> · chưa nhập số được giao</span>
        )}
      </p>
      {knowsGrant && <Meter ratio={source.allocated_ratio} label="Tỷ lệ đã phân bổ trên tổng nguồn" danger={over} muted />}

      {/* 2 — disbursed over what was allocated: are the projects moving. */}
      <p className="m-0 mt-2.5 text-[13px] text-ink-700">
        Đã giải ngân{" "}
        <b className="text-ink-900">
          <Money amount={source.disbursed_amount} />
        </b>
        {" / "}
        <b className="text-ink-900">
          <Money amount={source.allocated_amount} />
        </b>{" "}
        đã phân bổ
      </p>
      <Meter ratio={source.disbursed_of_allocated_ratio} label="Tỷ lệ đã giải ngân trên phần đã phân bổ" />

      {/* 3 — disbursed over the whole grant: has the money actually gone out. */}
      {knowsGrant && (
        <>
          <p className="m-0 mt-2 text-[13px] text-ink-700">
            Đã giải ngân{" "}
            <b className="text-ink-900">
              <Money amount={source.disbursed_amount} />
            </b>
            {" / "}
            <b className="text-ink-900">
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

/** A short amount ("9,2 tỷ đồng", `compactDong`), the exact đồng on hover — the tile convention. */
function Money({ amount }: { amount: number }) {
  return <span title={nhanTien(amount)}>{compactDong(amount)}</span>;
}

/**
 * Bar + percent. The bar is capped at full width; the WORDS keep the real figure, above 100% included
 * (§13 rule 2). `null` (no denominator) is "—" over an empty track, never "0%": 0% claims a
 * denominator existed and nothing moved.
 */
function Meter({
  ratio,
  label,
  danger = false,
  muted = false,
}: {
  ratio: number | null;
  label: string;
  danger?: boolean;
  muted?: boolean;
}) {
  const known = ratio !== null && Number.isFinite(ratio);
  const width = known ? Math.min(100, Math.max(0, ratio / 100)) : 0;
  return (
    <div className="mt-1 flex items-center gap-2" data-meter="">
      <div aria-hidden="true" className="h-1.5 min-w-0 flex-1 overflow-hidden rounded-full bg-surface-subtle-2">
        <div
          className={cn("h-full rounded-full", danger ? "bg-danger-500" : muted ? "bg-ink-400" : "bg-brand-500")}
          style={{ width: `${width}%` }}
        />
      </div>
      <span title={label} className="w-16 shrink-0 text-right text-xs font-semibold text-ink-900 tabular-nums">
        <span className="an-thi-giac">{label}: </span>
        {known ? nhanTyLeGiaiNgan(ratio) : "—"}
      </span>
    </div>
  );
}
