"use client";

import { ChartLine, MessagesSquare, ReceiptText, TriangleAlert, Upload } from "lucide-react";
import { useState, type KeyboardEvent, type ReactNode } from "react";

import {
  PENDING_HOVER_TEXT,
  PendingButton,
  PendingField,
  PendingMarker,
  PendingTab,
} from "@/components/ui/pending-feature";
import { Tab, TabList } from "@/components/ui/tabs";

import { pendingPart } from "./nhan-ghi-giai-ngan";

/**
 * The "?" placeholders of the Giải ngân screens — ADR 0068 §14, positions approved 02/10/2026.
 *
 * Every part is drawn where `docs/ui-ux/06-giai-ngan.md` puts it, as the control it will be, DISABLED.
 * Nothing here fetches, stores or emits: a placeholder is presentation (ADR 0068 §1). The sentence
 * behind each "?" is the `PHAN_CHUA_DUNG_GHI` entry, looked up — never a second copy.
 *
 * In their own file because they use hooks (Radix): the screens' presentational parts stay
 * renderable on their own in tests.
 */

// Exact `ten` of each `PHAN_CHUA_DUNG_GHI` entry (spec sections in the entry's comment there).
const IMPORT_EXCEL = "Nhập giải ngân từ Excel"; // §10
// §5 `☰ Hạng mục` is LIVE since 06/10/2026 (`category-manager-dialog.tsx`).
// §3 KPI cards, §4 chart, §5 category table and the two §7.1 checkboxes are LIVE since 06/10/2026
// (`disbursement-overview.tsx`, `bang-du-an.tsx`); §8.3 `Biểu đồ` too (`project-curve.tsx`).
const ATTENTION_ISSUES = "Vướng mắc và nguy cơ không giải ngân hết"; // §3, fourth card's sub-line
/** Name of the list table's pending column, read by `bang-du-an.tsx`. */
export const LATEST_ISSUE_COLUMN = "Vướng mắc mới nhất"; // §7.2
/** Prototype list column "Đơn vị / phụ trách" and the detail figure "Đơn vị thực hiện". */
export const UNIT_OWNER = "Đơn vị và cán bộ phụ trách của dự án";
const ISSUES_TAB = "Vướng mắc"; // §8.1
const DISCUSSION_TAB = "Trao đổi"; // §8.4
/** Name used by the Thêm dự án form. §9 `Tự sinh mã` is LIVE since 9f0a0187 (`ghi-du-an.tsx`). */
export const UNIT_AND_OFFICER = "Đơn vị thực hiện và Cán bộ phụ trách"; // §9
// §7.2 funding chip, §8 per-source block and the §9 funding list are LIVE since 8245698b; the §8.2
// voucher list, its `NGUỒN VỐN` column and the voucher form's source select since db94b35c.

/**
 * PageHeader button `[Nhập giải ngân]`, prototype `BudgetWorkspace.tsx:154-168`, between the live
 * `[Hạng mục]` (`category-manager-dialog.tsx`) and `+ Thêm dự án`. The caller draws it only for an
 * account holding `budget.update`, as the prototype does (`canRecord`).
 */
export function DisbursementHeaderActions() {
  return (
    <PendingButton info={pendingPart(IMPORT_EXCEL)} side="bottom" icon={<Upload aria-hidden="true" />}>
      Nhập giải ngân
    </PendingButton>
  );
}

/**
 * Sub-line of §3's fourth card ("0 vướng mắc đang theo dõi · 0 nguy cơ không giải ngân hết"): the words
 * the figures would sit in, and the "?" — never a 0, which would read as "none" (the summary route
 * leaves both counts out on purpose).
 */
export function AttentionIssuesPending() {
  return (
    <span className="inline-flex min-w-0 flex-wrap items-center gap-1.5" data-pending="">
      <span className="text-ink-400">Vướng mắc · nguy cơ không giải ngân hết</span>
      <PendingMarker info={pendingPart(ATTENTION_ISSUES)} />
    </span>
  );
}

/**
 * The detail card's `Đơn vị thực hiện` figure (prototype `BudgetItemDetail.tsx:302`): its label, the
 * "?", and "—" — never the internal id the contract carries.
 */
export function ProjectUnitPending() {
  return (
    <div className="flex min-w-0 flex-col gap-0.5" data-pending="">
      <dt className="flex items-center gap-1.5 text-xs font-semibold text-ink-500">
        Đơn vị thực hiện
        <PendingMarker info={pendingPart(UNIT_OWNER)} />
      </dt>
      <dd className="m-0 text-sm font-medium text-ink-400">
        <span aria-hidden="true">—</span>
        <span className="sr-only">{PENDING_HOVER_TEXT}</span>
      </dd>
    </div>
  );
}

type ProjectTab = "vouchers" | "chart";

/**
 * Detail page tabs, spec §8: `[Vướng mắc] [Chứng từ] [Biểu đồ] [Trao đổi]`. Chứng từ (the default) and
 * Biểu đồ are live; Vướng mắc and Trao đổi are disabled "?" tabs (`tabIndex={-1}`), which ←/→ skip.
 *
 * The voucher panel stays MOUNTED while the chart is shown (`hidden`), so a half-filled voucher form
 * survives a look at the chart. The chart panel mounts only while selected: `chart` reads the curve on
 * mount, so every opening shows the curve as it is after the last voucher write.
 *
 * `voucherCount` is the server's `count` (prototype `Chứng từ (N)`); `undefined` while the list is
 * loading or failed — no number rather than a "0" that would read as "nothing spent".
 */
export function ProjectRecordTabs({
  children,
  voucherCount,
  chart,
}: {
  children: ReactNode;
  voucherCount?: number;
  /** The §8.3 panel (`ProjectCurvePanel`), rendered only while `Biểu đồ` is selected. */
  chart: ReactNode;
}) {
  const [selected, setSelected] = useState<ProjectTab>("vouchers");

  function onKeyDown(e: KeyboardEvent<HTMLDivElement>): void {
    // Only from a tab: the "?" buttons of the pending tabs sit in this list too.
    if ((e.target as HTMLElement).getAttribute("role") !== "tab") return;
    if (e.key !== "ArrowLeft" && e.key !== "ArrowRight" && e.key !== "Home" && e.key !== "End") return;
    e.preventDefault();
    // Two live tabs: ←/→ land on the other one; Home/End name their end.
    const next: ProjectTab =
      e.key === "Home" ? "vouchers" : e.key === "End" ? "chart" : selected === "vouchers" ? "chart" : "vouchers";
    setSelected(next);
    document.getElementById(next === "vouchers" ? "tab-chung-tu-du-an" : "tab-bieu-do-du-an")?.focus();
  }

  return (
    <div className="flex min-w-0 flex-col gap-4">
      <TabList aria-label="Hồ sơ dự án" onKeyDown={onKeyDown}>
        <PendingTab info={pendingPart(ISSUES_TAB)} icon={TriangleAlert}>
          Vướng mắc
        </PendingTab>
        <Tab
          selected={selected === "vouchers"}
          id="tab-chung-tu-du-an"
          aria-controls="panel-chung-tu-du-an"
          tabIndex={selected === "vouchers" ? 0 : -1}
          icon={ReceiptText}
          onClick={() => setSelected("vouchers")}
        >
          {voucherCount === undefined ? "Chứng từ" : `Chứng từ (${voucherCount})`}
        </Tab>
        <Tab
          selected={selected === "chart"}
          id="tab-bieu-do-du-an"
          aria-controls="panel-bieu-do-du-an"
          tabIndex={selected === "chart" ? 0 : -1}
          icon={ChartLine}
          onClick={() => setSelected("chart")}
        >
          Biểu đồ
        </Tab>
        <PendingTab info={pendingPart(DISCUSSION_TAB)} icon={MessagesSquare}>
          Trao đổi
        </PendingTab>
      </TabList>
      <div
        role="tabpanel"
        id="panel-chung-tu-du-an"
        aria-labelledby="tab-chung-tu-du-an"
        hidden={selected !== "vouchers"}
        className="min-w-0"
      >
        {children}
      </div>
      {selected === "chart" && (
        <div role="tabpanel" id="panel-bieu-do-du-an" aria-labelledby="tab-bieu-do-du-an" className="min-w-0">
          {chart}
        </div>
      )}
    </div>
  );
}

/** The two `Thông tin thêm` selects of §9 not built yet, drawn disabled with their spec placeholders. */
export function UnitAndOfficerPending() {
  const info = pendingPart(UNIT_AND_OFFICER);
  return (
    <>
      <PendingField info={info} id="don-vi-du-an" label="Đơn vị thực hiện" kind="select" placeholder="— Chưa xác định —" className="min-w-0" />
      <PendingField info={info} id="can-bo-du-an" label="Cán bộ phụ trách" kind="select" placeholder="— Chưa phân công —" className="min-w-0" />
    </>
  );
}
