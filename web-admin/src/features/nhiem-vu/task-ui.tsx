import {
  CircleCheck,
  CircleDot,
  CirclePause,
  ClipboardCheck,
  Forward,
  Hourglass,
  Loader,
  type LucideIcon,
} from "lucide-react";
import type { ReactNode } from "react";

import { Badge, type BadgeTone } from "@/components/ui/badge";
import { cn } from "@/lib/cn";

/**
 * Presentation shared by the Nhiệm vụ screen (ADR 0068, spec §8.3). Nothing here reads data or
 * decides anything: it maps a status CODE the server already sent to an icon and a tone, and it
 * draws native toggle buttons the screen already owns.
 */

/**
 * Icon and tone per lifecycle CODE — never per label: labels are the commune's own words
 * (`nhanTT`), and a commune renaming `tam-dung` must not change which icon it gets.
 *
 * ICON + WORD, never colour alone (spec §7). Red is deliberately absent: overdue is not a status,
 * it is derived from `due_at` (rule 10, invariant 3) and drawn in the `Hạn` column. An unknown
 * code falls to the neutral pill — it still shows the server's label, only without a specific icon.
 */
const STATUS_LOOK: Readonly<Record<string, { tone: BadgeTone; icon: LucideIcon }>> = {
  "moi-giao": { tone: "info", icon: CircleDot },
  "da-tiep-nhan": { tone: "info", icon: ClipboardCheck },
  "dang-thuc-hien": { tone: "info", icon: Loader },
  "cho-duyet": { tone: "warning", icon: Hourglass },
  "hoan-thanh": { tone: "success", icon: CircleCheck },
  "tam-dung": { tone: "neutral", icon: CirclePause },
  "chuyen-tiep": { tone: "neutral", icon: Forward },
};

/** The icon of a status CODE — the detail dialog's large status pill uses the same map as the list. */
export function taskStatusIcon(status: string): LucideIcon {
  return STATUS_LOOK[status]?.icon ?? CircleDot;
}

/**
 * The 8px dot before a Kanban column title — the prototype's `TASK_STATUS_META[...].dot`
 * (`vigov-require/.../lib/task-display.ts:8-31`), its hex values verbatim because this palette has no
 * teal or violet token. Decorative only: the column title beside it is the word that carries meaning.
 */
const KANBAN_DOT: Readonly<Record<string, string>> = {
  "moi-giao": "bg-[#8aa2b8]",
  "da-tiep-nhan": "bg-[#12b5c9]",
  "dang-thuc-hien": "bg-[#2fb1f9]",
  "cho-duyet": "bg-[#6e59e8]",
  "hoan-thanh": "bg-[#86b940]",
};

export function kanbanDotClass(status: string): string {
  return KANBAN_DOT[status] ?? "bg-ink-400";
}

/** Status pill for the list table. `label` is the commune's label for `status`, passed verbatim. */
export function TaskStatusBadge({ status, children }: { status: string; children: ReactNode }) {
  const look = STATUS_LOOK[status];
  return (
    <Badge tone={look?.tone ?? "neutral"} icon={look?.icon}>
      {children}
    </Badge>
  );
}

/**
 * A row of NATIVE `aria-pressed` toggle buttons drawn as a segmented control (spec §7). Not the
 * `Segmented` component: the screen's buttons — their `onClick`, their `aria-pressed`, their
 * source text that tests read — stay exactly where they are; only the classes change.
 */
export const TOGGLE_TRACK = "inline-flex max-w-full flex-wrap gap-0.5 rounded-control bg-[#f1f4f8] p-[3px]";

export function toggleButtonClass(on: boolean): string {
  return cn(
    "inline-flex h-[34px] cursor-pointer items-center gap-1.5 rounded-lg border-0 bg-transparent px-3 [font-family:inherit] text-[13px] font-medium whitespace-nowrap text-ink-500",
    "transition-[background-color,color,box-shadow] duration-150 hover:text-ink-900",
    "focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-brand-500",
    "disabled:cursor-not-allowed disabled:opacity-60 [&_svg]:size-4 [&_svg]:shrink-0",
    on && "bg-surface font-semibold text-ink-900 shadow-sm",
  );
}

/**
 * First-load placeholder for the list (spec §8b): grey bars in the shape of table rows, so the
 * layout does not jump when the page arrives. LOCAL ON PURPOSE — a shared `Skeleton` is owned by
 * another work item (F2); this one is replaced by it then. Decorative: the screen's own
 * `role="status"` sentence is what announces the load.
 */
export function TaskRowsSkeleton({ rows = 5 }: { rows?: number }) {
  return (
    <div aria-hidden="true" className="divide-y divide-line">
      {Array.from({ length: rows }, (_, i) => (
        <div key={i} className="flex h-12 items-center gap-4 px-4">
          <span className="h-3 w-14 shrink-0 rounded bg-line motion-safe:animate-pulse" />
          <span className="h-3 min-w-0 flex-1 rounded bg-line motion-safe:animate-pulse" />
          <span className="hidden h-3 w-28 shrink-0 rounded bg-line motion-safe:animate-pulse sm:block" />
          <span className="h-[22px] w-24 shrink-0 rounded-full bg-line motion-safe:animate-pulse" />
        </div>
      ))}
    </div>
  );
}

/** 2px bar at the head of a card while it re-reads (spec §8b). Decorative, like the skeleton. */
export function LoadingBar() {
  return (
    <div aria-hidden="true" className="h-0.5 w-full overflow-hidden bg-brand-50">
      <div className="h-full w-1/3 bg-brand-500 motion-safe:animate-pulse" />
    </div>
  );
}

/** Decorative icon inside a button or a line of text: hidden from assistive tech, never focusable. */
export function Glyph({ icon: Icon, className }: { icon: LucideIcon; className?: string }) {
  return <Icon aria-hidden="true" focusable="false" strokeWidth={1.8} className={className} />;
}
