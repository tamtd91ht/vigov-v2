"use client";

import {
  Banknote,
  ChartLine,
  Hourglass,
  Layers,
  MessagesSquare,
  Plus,
  ReceiptText,
  TriangleAlert,
  Upload,
  Wallet,
} from "lucide-react";
import type { ReactNode } from "react";

import { Button } from "@/components/ui/button";
import {
  PENDING_HOVER_TEXT,
  PendingButton,
  PendingFeature,
  PendingField,
  PendingMarker,
  PendingSection,
  PendingStatCard,
  PendingTab,
} from "@/components/ui/pending-feature";
import { Tab, TabList } from "@/components/ui/tabs";
import { cn } from "@/lib/cn";

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
const CATEGORIES = "Hạng mục"; // §5
const KPI_CARDS = "Số liệu tổng hợp của năm"; // §3
const CUMULATIVE_CHART = "Luỹ kế giải ngân so với kế hoạch"; // §4
const CATEGORY_PROGRESS = "Tiến độ theo hạng mục"; // §5
const ONLY_DELAYED = "Chỉ dự án chậm"; // §7.1
const GROUP_BY_CATEGORY = "Gộp theo hạng mục"; // §7.1
/** Names of the list table's two pending columns, read by `bang-du-an.tsx`. */
export const FUNDING_COLUMN = "Nguồn vốn của dự án"; // §7.2
export const LATEST_ISSUE_COLUMN = "Vướng mắc mới nhất"; // §7.2
/** Prototype list column "Đơn vị / phụ trách" and the detail figure "Đơn vị thực hiện". */
export const UNIT_OWNER = "Đơn vị và cán bộ phụ trách của dự án";
const PROJECT_FUNDING = "Giải ngân theo nguồn vốn"; // §8
const ISSUES_TAB = "Vướng mắc"; // §8.1
const CHART_TAB = "Biểu đồ"; // §8.3
const DISCUSSION_TAB = "Trao đổi"; // §8.4
/** Names used by the Thêm dự án form and the voucher table. */
export const AUTO_CODE = "Tự sinh mã"; // §9
export const UNIT_AND_OFFICER = "Đơn vị thực hiện và Cán bộ phụ trách"; // §9
export const FUNDING_LIST = "Thêm nguồn vốn cho dự án"; // §9, §8
export const VOUCHER_FUNDING_COLUMN = "Nguồn vốn của chứng từ"; // §8.2, §6

/**
 * PageHeader buttons, prototype `BudgetWorkspace.tsx:154-168`: `[Hạng mục] [Nhập giải ngân]`, before
 * `+ Thêm dự án`. The caller draws them only for an account holding `budget.update`, as the prototype
 * does (`canRecord`).
 */
export function DisbursementHeaderActions() {
  return (
    <>
      <PendingButton info={pendingPart(CATEGORIES)} side="bottom" icon={<Layers aria-hidden="true" />}>
        Hạng mục
      </PendingButton>
      <PendingButton info={pendingPart(IMPORT_EXCEL)} side="bottom" icon={<Upload aria-hidden="true" />}>
        Nhập giải ngân
      </PendingButton>
    </>
  );
}

/**
 * Body of the list screen between the scope banner and the live funding-source block, in the
 * prototype's order and spacing (`BudgetWorkspace.tsx:198-251`): four KPI cards, the cumulative chart,
 * the per-category table.
 */
export function DisbursementOverviewPending() {
  const kpi = pendingPart(KPI_CARDS);
  return (
    <>
      <div className="mb-5 grid min-w-0 gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <PendingStatCard info={kpi} icon={Wallet} label="Kế hoạch vốn năm" />
        <PendingStatCard info={kpi} icon={Banknote} label="Đã giải ngân" />
        <PendingStatCard info={kpi} icon={Hourglass} label="Còn phải giải ngân" />
        <PendingStatCard info={kpi} icon={TriangleAlert} label="Cần chú ý" />
      </div>
      <PendingSection
        info={pendingPart(CUMULATIVE_CHART)}
        title="Luỹ kế giải ngân so với kế hoạch"
        className="mb-4"
      />
      <PendingSection info={pendingPart(CATEGORY_PROGRESS)} title="Tiến độ theo hạng mục" className="mb-5" />
      {/* §6 "Tiến độ theo nguồn vốn" is LIVE since migration 0013 (`funding-source-progress.tsx`); the
          register draws it right after this block, in the prototype's order. */}
    </>
  );
}

/**
 * The two checkboxes of the project filter row, spec §7.1. Disabled and UNCHECKED — the spec has
 * `Gộp theo hạng mục` on by default, but a ticked box that does nothing would claim the table is
 * grouped when it is not.
 */
export function ProjectFilterPending() {
  return (
    <>
      <PendingCheckbox id="loc-chi-du-an-cham" name={ONLY_DELAYED} label="Chỉ dự án chậm" />
      <PendingCheckbox id="loc-gop-hang-muc" name={GROUP_BY_CATEGORY} label="Gộp theo hạng mục" />
    </>
  );
}

/**
 * The voucher form's `Rút từ nguồn vốn` select (prototype `DisbursementForm.tsx:158-175`), full width
 * above the date — disabled with the "?" of the voucher funding column (same missing catalogue).
 */
export function VoucherFundingPending() {
  return (
    <PendingField
      info={pendingPart(VOUCHER_FUNDING_COLUMN)}
      id="nguon-von-chung-tu"
      label="Rút từ nguồn vốn"
      kind="select"
      placeholder="— Chọn nguồn vốn —"
      className="min-w-0 sm:col-span-2"
    />
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

/** A disabled checkbox + its label, with the "?" BESIDE the label (never inside it — ADR 0068 §14). */
export function PendingCheckbox({
  id,
  name,
  label,
  className,
}: {
  id: string;
  name: string;
  label: ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("flex h-10 items-center gap-2", className)} data-pending="">
      <input id={id} type="checkbox" disabled className="size-4 cursor-not-allowed" />
      <label htmlFor={id} className="text-sm text-ink-500">
        {label}
      </label>
      <PendingMarker info={pendingPart(name)} />
    </div>
  );
}

/**
 * Detail page: the `GIẢI NGÂN THEO NGUỒN VỐN` box inside the project card, under the progress bar
 * (prototype `BudgetItemDetail.tsx:333-372`) — bordered, as it sits on the card's own white.
 */
export function ProjectFundingPending() {
  return (
    <PendingSection
      info={pendingPart(PROJECT_FUNDING)}
      title="Giải ngân theo nguồn vốn"
      titleAs="h3"
      className="rounded-xl border border-line"
    />
  );
}

/**
 * Detail page tabs, spec §8: `[Vướng mắc] [Chứng từ] [Biểu đồ] [Trao đổi]`. Chứng từ is the only
 * live tab, always selected, and its panel is `children`. No arrow-key handling: with one enabled tab
 * there is nowhere to move, and the three pending tabs are `disabled` + `tabIndex={-1}`.
 */
export function ProjectRecordTabs({ children }: { children: ReactNode }) {
  return (
    <div className="flex min-w-0 flex-col gap-4">
      <TabList aria-label="Hồ sơ dự án">
        <PendingTab info={pendingPart(ISSUES_TAB)} icon={TriangleAlert}>
          Vướng mắc
        </PendingTab>
        <Tab selected id="tab-chung-tu-du-an" aria-controls="panel-chung-tu-du-an" icon={ReceiptText}>
          Chứng từ
        </Tab>
        <PendingTab info={pendingPart(CHART_TAB)} icon={ChartLine}>
          Biểu đồ
        </PendingTab>
        <PendingTab info={pendingPart(DISCUSSION_TAB)} icon={MessagesSquare}>
          Trao đổi
        </PendingTab>
      </TabList>
      <div role="tabpanel" id="panel-chung-tu-du-an" aria-labelledby="tab-chung-tu-du-an" className="min-w-0">
        {children}
      </div>
    </div>
  );
}

/**
 * The three spec §9 fields of `Thêm dự án` that are not built: `☑ Tự sinh mã`, the `Đơn vị thực
 * hiện` / `Cán bộ phụ trách` selects, and the dynamic `Nguồn vốn` list.
 */
export function AutoCodePending() {
  return <PendingCheckbox id="tu-sinh-ma-du-an" name={AUTO_CODE} label="Tự sinh mã" />;
}

/** The two `Thông tin thêm` selects of §9, drawn disabled with their spec placeholders. */
export function UnitAndOfficerPending() {
  const info = pendingPart(UNIT_AND_OFFICER);
  return (
    <>
      <PendingField info={info} id="don-vi-du-an" label="Đơn vị thực hiện" kind="select" placeholder="— Chưa xác định —" className="min-w-0" />
      <PendingField info={info} id="can-bo-du-an" label="Cán bộ phụ trách" kind="select" placeholder="— Chưa phân công —" className="min-w-0" />
    </>
  );
}

export function FundingListPending() {
  return (
    <fieldset className="m-0 flex min-w-0 flex-col gap-1.5 border-0 p-0 sm:col-span-2" data-pending="">
      <legend className="mb-1.5 p-0 text-xs leading-tight font-semibold text-ink-500">Nguồn vốn</legend>
      <PendingFeature info={pendingPart(FUNDING_LIST)} className="self-start">
        <Button type="button" variant="secondary" size="sm" icon={<Plus aria-hidden="true" />} disabled>
          Thêm nguồn vốn
        </Button>
      </PendingFeature>
    </fieldset>
  );
}
