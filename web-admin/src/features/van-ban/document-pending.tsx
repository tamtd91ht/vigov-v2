"use client";

import { Download, FileSpreadsheet, ListChecks } from "lucide-react";
import type { KeyboardEvent, ReactNode } from "react";

import { PendingButton, PendingMarker } from "@/components/ui/pending-feature";
import { Tab, TabList } from "@/components/ui/tabs";
import { cn } from "@/lib/cn";

import { INCOMING_STATUS_STEPS, PHAN_CHUA_DUNG, type PendingPart } from "./nhan-van-ban";

/**
 * The parts of the Văn bản & Đơn thư screen that cannot be built yet, drawn at the PROTOTYPE's
 * position (`vigov-require/apps/admin/src/components/documents/DocumentWorkspace.tsx`, ADR 0068 lần 5)
 * as the control they will be, disabled, with a "?" (ADR 0068 §14). NOTHING HERE CALLS A SERVER:
 * there is no handler to pass, on purpose — and no figure, row or count is drawn where the data does
 * not exist (never fake data, lần 5 #5).
 *
 * Descriptions come from `PHAN_CHUA_DUNG` by name, never a second sentence written here: two copies
 * of one reason drift, and the stale one is what a staff member reads.
 *
 * The two tabs `Đơn thư công dân` and `Báo cáo` were placeholders here until 08/10/2026; they are
 * built now (`letter-register.tsx`, `letter-report.tsx`). Only their Excel buttons stay "?".
 */
export function pendingPart(name: string): PendingPart {
  const found = PHAN_CHUA_DUNG.find((p) => p.ten === name);
  // A renamed entry must fail loudly in tests, not draw a "?" that explains nothing.
  if (found === undefined) throw new Error(`PHAN_CHUA_DUNG has no entry named "${name}"`);
  return found;
}

/* ---- the tab bar ------------------------------------------------------------------------------ */

/**
 * The prototype's four tabs, in its order: the registers first, then `Đơn thư công dân`, then
 * `Báo cáo`. The prototype HIDES its `Văn bản đến` tab because ITS demo commune asked (17/09/2026);
 * that request does not apply here, and our two working registers each get their tab. The prototype's
 * counts in brackets are not drawn: every register is paged, so the count of the loaded page is not
 * the register's count.
 */
export const DOCUMENT_TABS = [
  { id: "incoming", label: "Văn bản đến" },
  { id: "outgoing", label: "Văn bản đi" },
  { id: "petitions", label: "Đơn thư công dân" },
  { id: "report", label: "Báo cáo" },
] as const;

export type DocumentTabId = (typeof DOCUMENT_TABS)[number]["id"];

export const DOCUMENT_PANEL_ID = "khung-tab-van-ban";

export function documentTabDomId(id: DocumentTabId): string {
  return `tab-van-ban-${id}`;
}

/** The tab bar under the page header. Roving focus: Left/Right/Home/End. */
export function DocumentTabBar({
  selected,
  onSelect,
}: {
  selected: DocumentTabId;
  onSelect: (id: DocumentTabId) => void;
}) {
  function onKeyDown(e: KeyboardEvent<HTMLDivElement>) {
    const at = DOCUMENT_TABS.findIndex((t) => t.id === selected);
    const last = DOCUMENT_TABS.length - 1;
    const next =
      e.key === "ArrowRight" ? (at === last ? 0 : at + 1)
      : e.key === "ArrowLeft" ? (at === 0 ? last : at - 1)
      : e.key === "Home" ? 0
      : e.key === "End" ? last
      : null;
    if (next === null) return;
    e.preventDefault();
    // Focus follows selection; the page moves it once the selected tab is rendered.
    onSelect(DOCUMENT_TABS[next]!.id);
  }

  return (
    <TabList aria-label="Phần của màn Văn bản & đơn thư" onKeyDown={onKeyDown}>
      {DOCUMENT_TABS.map((t) => (
        <Tab
          key={t.id}
          id={documentTabDomId(t.id)}
          selected={t.id === selected}
          aria-controls={DOCUMENT_PANEL_ID}
          tabIndex={t.id === selected ? 0 : -1}
          onClick={() => onSelect(t.id)}
        >
          {t.label}
        </Tab>
      ))}
    </TabList>
  );
}

/* ---- header actions --------------------------------------------------------------------------- */

/** The prototype's "or" between two ways of entering one register (`DocumentWorkspace.tsx:134`). */
export function HeaderOr() {
  return <span className="text-[12px] text-ink-muted">hoặc</span>;
}

/**
 * The prototype's second header button of the petition register — `[Nhập từ Excel]`
 * (`DocumentWorkspace.tsx:135-142`) — disabled with its "?": no import route exists (ADR 0078 #6).
 */
export function LetterImportButton() {
  return (
    <PendingButton
      info={pendingPart("Nhập đơn thư từ Excel")}
      variant="outline"
      icon={<FileSpreadsheet aria-hidden="true" />}
    >
      Nhập từ Excel
    </PendingButton>
  );
}

/* ---- pieces of the incoming register ---------------------------------------------------------- */

/**
 * The prototype's `ScopeFilter` (`components/common/ScopeFilter.tsx`), first in the incoming filter
 * row, drawn with its own classes: `Toàn xã` is what the register shows today (pressed, nothing to
 * change); the two personal scopes are disabled, with ONE "?" after the group — both wait on the same
 * missing route parameter, and two markers would say the same sentence twice.
 */
export function IncomingScopeFilter() {
  const choices = [
    { label: "Toàn xã", active: true },
    { label: "Giao cho tôi", active: false },
    { label: "Liên quan đến tôi", active: false },
  ] as const;
  return (
    <span className="inline-flex items-center gap-1.5">
      <span
        role="group"
        aria-label="Lọc nhanh theo người xử lý"
        className="inline-flex overflow-hidden rounded-md border border-solid border-line bg-white"
      >
        {choices.map((c) => (
          <button
            key={c.label}
            type="button"
            aria-pressed={c.active}
            disabled={!c.active}
            className={cn(
              "h-9 border-0 border-r border-solid border-line px-3 [font-family:inherit] text-[12.5px] font-semibold last:border-r-0",
              c.active ? "bg-navy text-white" : "cursor-not-allowed bg-white text-ink opacity-50",
            )}
          >
            {c.label}
          </button>
        ))}
      </span>
      <PendingMarker info={pendingPart("Lọc Giao cho tôi / Liên quan đến tôi")} />
    </span>
  );
}

/**
 * A link-shaped control of the prototype's filter row (`DocumentWorkspace.tsx:225-244`), disabled,
 * with its "?" beside it — a `<button>`, since a disabled `<a>` does not exist.
 */
function PendingRowLink({
  info,
  icon,
  tone,
  className,
  children,
}: {
  info: PendingPart;
  icon: ReactNode;
  tone: "brand" | "muted";
  className?: string;
  children?: ReactNode;
}) {
  return (
    <span className={cn("inline-flex items-center gap-1.5", className)} data-pending="">
      <button
        type="button"
        disabled
        className={cn(
          "inline-flex cursor-not-allowed items-center gap-1.5 border-0 bg-transparent p-0 [font-family:inherit] text-[12.5px] opacity-60",
          "[&_svg]:size-4 [&_svg]:shrink-0",
          tone === "brand" ? "text-brand" : "text-ink-muted",
        )}
      >
        {icon}
        {children ?? info.ten}
      </button>
      <PendingMarker info={info} />
    </span>
  );
}

/**
 * Right end of the incoming filter row: `Nhập hàng loạt từ Excel` (brand link, `ml-auto`) and
 * `Xuất sổ {năm}` (muted link) — both disabled with "?" (ADR 0078 #6).
 */
export function IncomingRowLinks({ year }: { year: number }) {
  return (
    <>
      <PendingRowLink
        info={pendingPart("Nhập hàng loạt từ Excel")}
        tone="brand"
        className="ml-auto"
        icon={<FileSpreadsheet aria-hidden="true" />}
      />
      <PendingRowLink info={pendingPart("Xuất sổ văn bản đến")} tone="muted" icon={<Download aria-hidden="true" />}>
        Xuất sổ {year}
      </PendingRowLink>
    </>
  );
}

/**
 * The detail's `Chuyển thành nhiệm vụ` (prototype `DocumentDetailDrawer.tsx:404-418`,
 * `PetitionDetailDrawer.tsx:590-602`), disabled.
 */
export function RaiseTaskButton() {
  return (
    <PendingButton info={pendingPart("Chuyển thành nhiệm vụ")} variant="primary" icon={<ListChecks aria-hidden="true" />} />
  );
}

/**
 * The prototype's chip colour of the CURRENT step (`DOCUMENT_ACTIVE_TONE`, `lib/document-display.ts`),
 * by step index of `INCOMING_STATUS_STEPS`.
 */
const STEP_ACTIVE_TONE = ["bg-ink-muted", "bg-tangerine", "bg-brand", "bg-teal", "bg-leaf"] as const;

/**
 * The prototype's status strip (`DocumentDetailDrawer.tsx:239-264`, `StatusChip` :543-590): one chip per
 * C2 step, `flex-1` with a 6.5rem floor, the current one filled with "đang ở đây" under it, the others
 * white with "—". EVERY CHIP IS DISABLED and the row carries ONE "?": the server has no route that
 * moves an incoming document between these steps yet (ADR 0078, Hệ quả). `current` comes from
 * `incomingStatusStep` — `null` lights no chip (see there for why).
 *
 * The prototype's second row ("Kết thúc khác:") is not drawn: C2 lists no other ending.
 */
export function StatusChangeRow({ current }: { current: number | null }) {
  return (
    <div role="group" aria-labelledby="nhan-chuyen-trang-thai" className="flex min-w-0 flex-col" data-pending="">
      <span id="nhan-chuyen-trang-thai" className="an-thi-giac">
        Các bước của văn bản đến
      </span>
      <div className="flex min-w-0 flex-wrap items-stretch gap-1.5">
        {INCOMING_STATUS_STEPS.map((step, i) => {
          const here = i === current;
          return (
            <button
              key={step}
              type="button"
              disabled
              aria-current={here ? "step" : undefined}
              aria-label={here ? `${step}, đang ở đây` : `Chuyển sang ${step}`}
              className={cn(
                "min-w-[6.5rem] flex-1 cursor-not-allowed rounded-[8px] border border-solid px-2.5 py-1.5 text-left [font-family:inherit]",
                here ? cn("border-transparent text-white", STEP_ACTIVE_TONE[i]) : "border-line/60 bg-white text-ink-muted/60",
              )}
            >
              <span className="block text-[12px] font-semibold">{step}</span>
              <span className={cn("block text-[10.5px]", here ? "text-white/80" : "text-ink-muted/70")}>
                {here ? "đang ở đây" : "—"}
              </span>
            </button>
          );
        })}
        <span className="inline-flex items-center self-center px-1">
          <PendingMarker info={pendingPart("Chuyển trạng thái văn bản đến")} />
        </span>
      </div>
    </div>
  );
}
