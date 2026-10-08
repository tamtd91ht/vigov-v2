"use client";

import { useState, type KeyboardEvent, type ReactNode } from "react";

import { PendingMarker } from "@/components/ui/pending-feature";
import { Tab, TabList } from "@/components/ui/tabs";

import { pendingPart } from "./nhan-ghi-giai-ngan";
import { atRiskLabel, openIssuesLabel } from "./project-discussion-labels";

/**
 * The "?" placeholders of the Giải ngân screens — ADR 0068 §14, positions approved 02/10/2026 — and the
 * project page's four tabs, which used to hold two of them.
 *
 * Every part is drawn where `docs/ui-ux/06-giai-ngan.md` puts it, as the control it will be, DISABLED.
 * Nothing here fetches, stores or emits: a placeholder is presentation (ADR 0068 §1). The sentence
 * behind each "?" is the `PHAN_CHUA_DUNG_GHI` entry, looked up — never a second copy.
 *
 * In their own file because they use hooks (Radix): the screens' presentational parts stay
 * renderable on their own in tests.
 */

// Exact `ten` of each `PHAN_CHUA_DUNG_GHI` entry (spec sections in the entry's comment there).
// §10 `Nhập giải ngân` is LIVE (`disbursement-import-dialog.tsx`, server f181bb76).
// §5 `☰ Hạng mục` is LIVE since 06/10/2026 (`category-manager-dialog.tsx`).
// §3 KPI cards, §4 chart, §5 category table and the two §7.1 checkboxes are LIVE since 06/10/2026
// (`disbursement-overview.tsx`, `bang-du-an.tsx`); §8.3 `Biểu đồ` too (`project-curve.tsx`).
// §3 fourth card's sub-line, second half (`Nguy cơ không giải ngân hết`) is LIVE since 81533bb3.
// §7.2 `Vướng mắc mới nhất`, §8.1 `Vướng mắc` and §8.4 `Trao đổi` are LIVE since 889d4598.
const TRACKING_TASK = "Tự sinh nhiệm vụ theo dõi"; // §8.1 note, §13 rule 4
// §8.4 `Thông báo cho người được nhắc tên` is LIVE since dda12fa4 (bell notice from the server).
// §7.2 `Đơn vị / phụ trách`, §8 `Đơn vị thực hiện` and the two §9 selects are LIVE (`project-people.ts`).
// §9 `Tự sinh mã` is LIVE since 9f0a0187 (`ghi-du-an.tsx`).
// §7.2 funding chip, §8 per-source block and the §9 funding list are LIVE since 8245698b; the §8.2
// voucher list, its `NGUỒN VỐN` column and the voucher form's source select since db94b35c.

/**
 * Sub-line of §3's fourth card, verbatim from the prototype (`BudgetWorkspace.tsx:221`): "3 vướng mắc
 * đang theo dõi · 1 nguy cơ không giải ngân hết". Both counts are the summary's (`open_issue_count`,
 * `at_risk_count`); an absent count is said as not read, never as 0.
 */
export function AttentionCaption({
  openIssueCount,
  atRiskCount,
}: {
  openIssueCount: number | null | undefined;
  atRiskCount: number | null | undefined;
}) {
  return (
    <span className="inline-flex min-w-0 flex-wrap items-center gap-x-1.5 gap-y-0.5" data-attention-caption="">
      <span data-open-issues="">{openIssuesLabel(openIssueCount)}</span>
      <span aria-hidden="true">·</span>
      <span data-at-risk-count="">{atRiskLabel(atRiskCount)}</span>
    </span>
  );
}

/**
 * §8.1's note, as what it is: NOT BUILT. The spec's sentence ("…hệ thống tự sinh một nhiệm vụ theo dõi")
 * is not printed as a fact — no task is created yet — so the part is named greyed, with its "?".
 */
export function TrackingTaskPending() {
  return (
    <span className="inline-flex min-w-0 items-center gap-1.5 text-[11px]" data-pending="">
      <span className="text-ink-muted">{TRACKING_TASK}</span>
      <PendingMarker info={pendingPart(TRACKING_TASK)} />
    </span>
  );
}

type ProjectTab = "issues" | "vouchers" | "chart" | "discussion";

const TAB_ORDER: readonly ProjectTab[] = ["issues", "vouchers", "chart", "discussion"];
/** Exported for the dev-only preview (`dev-preview/`), which opens a tab by these ids for a screenshot. */
export const TAB_ID: Record<ProjectTab, string> = {
  issues: "tab-vuong-mac-du-an",
  vouchers: "tab-chung-tu-du-an",
  chart: "tab-bieu-do-du-an",
  discussion: "tab-trao-doi-du-an",
};
const PANEL_ID: Record<ProjectTab, string> = {
  issues: "panel-vuong-mac-du-an",
  vouchers: "panel-chung-tu-du-an",
  chart: "panel-bieu-do-du-an",
  discussion: "panel-trao-doi-du-an",
};

/** `Vướng mắc (2)` once the server's count is read; no number before — a "(0)" would read as "none". */
function counted(label: string, n: number | undefined): string {
  return n === undefined ? label : `${label} (${n})`;
}

/**
 * Detail page tabs, spec §8: `[Vướng mắc (N)] [Chứng từ (N)] [Biểu đồ] [Trao đổi]` — all four live.
 * `Vướng mắc` is the default, as in the prototype (`BudgetItemDetail.tsx:379`); it was `Chứng từ` only
 * while the issue tab had no route behind it.
 *
 * Issues, vouchers and discussion stay MOUNTED while hidden, so a half-typed issue, voucher or message
 * survives a look at another tab. The chart mounts only while selected: it reads the curve on mount,
 * so every opening shows the curve as it is after the last voucher write.
 *
 * `issueCount` is the server's `open_count` (the prototype's `Vướng mắc (open_issue_count)`);
 * `voucherCount` its `count`. `undefined` while loading or failed.
 */
export function ProjectRecordTabs({
  children,
  voucherCount,
  chart,
  issues,
  issueCount,
  discussion,
}: {
  /** The §8.2 voucher panel. */
  children: ReactNode;
  voucherCount?: number;
  /** The §8.3 panel (`ProjectCurvePanel`), rendered only while `Biểu đồ` is selected. */
  chart: ReactNode;
  /** The §8.1 panel (`ProjectIssuesPanel`). */
  issues: ReactNode;
  issueCount?: number;
  /** The §8.4 panel (`ProjectCommentsPanel`). */
  discussion: ReactNode;
}) {
  const [selected, setSelected] = useState<ProjectTab>("issues");

  function onKeyDown(e: KeyboardEvent<HTMLDivElement>): void {
    if ((e.target as HTMLElement).getAttribute("role") !== "tab") return;
    const at = TAB_ORDER.indexOf(selected);
    const last = TAB_ORDER.length - 1;
    let next: number;
    if (e.key === "ArrowRight") next = at === last ? 0 : at + 1;
    else if (e.key === "ArrowLeft") next = at === 0 ? last : at - 1;
    else if (e.key === "Home") next = 0;
    else if (e.key === "End") next = last;
    else return;
    e.preventDefault();
    const tab = TAB_ORDER[next]!;
    setSelected(tab);
    document.getElementById(TAB_ID[tab])?.focus();
  }

  // No icons (spec 07 §Body 5, shadcn TabsTrigger with words only).
  function tab(id: ProjectTab, label: string) {
    return (
      <Tab
        selected={selected === id}
        id={TAB_ID[id]}
        aria-controls={PANEL_ID[id]}
        tabIndex={selected === id ? 0 : -1}
        onClick={() => setSelected(id)}
      >
        {label}
      </Tab>
    );
  }

  function kept(id: ProjectTab, content: ReactNode) {
    return (
      <div role="tabpanel" id={PANEL_ID[id]} aria-labelledby={TAB_ID[id]} hidden={selected !== id} className="min-w-0">
        {content}
      </div>
    );
  }

  return (
    <div className="flex min-w-0 flex-col gap-4">
      <TabList aria-label="Hồ sơ dự án" onKeyDown={onKeyDown}>
        {tab("issues", counted("Vướng mắc", issueCount))}
        {tab("vouchers", counted("Chứng từ", voucherCount))}
        {tab("chart", "Biểu đồ")}
        {tab("discussion", "Trao đổi")}
      </TabList>
      {kept("issues", issues)}
      {kept("vouchers", children)}
      {selected === "chart" && (
        <div role="tabpanel" id={PANEL_ID.chart} aria-labelledby={TAB_ID.chart} className="min-w-0">
          {chart}
        </div>
      )}
      {kept("discussion", discussion)}
    </div>
  );
}
