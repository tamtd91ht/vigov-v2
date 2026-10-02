import {
  ArrowUpRight,
  Ban,
  CircleCheck,
  ClipboardCheck,
  Forward,
  Hourglass,
  Inbox,
  Loader,
  LoaderCircle,
  Tags,
  type LucideIcon,
} from "lucide-react";
import type { ReactNode } from "react";

import { Badge, type BadgeTone } from "@/components/ui/badge";
import { buttonVariants, LEGACY_BUTTON_CLASS, type ButtonSize, type ButtonVariant } from "@/components/ui/button";
import { cn } from "@/lib/cn";

/**
 * Presentation shared by the Phản ánh screen (ADR 0068, spec v2). Nothing here reads data or decides
 * anything: it maps a status CODE the server already sent to an icon and a tone, and it draws the
 * screen's own native controls.
 *
 * NO HOOKS ANYWHERE IN THIS FILE: `chon-can-bo.test.tsx` calls `ChiTietPhieu` as a plain function
 * under a mocked React, and these pieces render inside it.
 *
 * LOCAL ON PURPOSE, to migrate later: `PetitionRowsSkeleton` and `LoadingBar` wait for the shared
 * `Skeleton` being built in `components/ui`; `toggleButtonClass` / `TOGGLE_TRACK` are the same
 * segmented look as `features/nhiem-vu/task-ui.tsx` (one feature does not import another's UI).
 */

/**
 * Icon and tone per lifecycle CODE (ADR 0027's nine codes) — never per label. ICON + WORD, never
 * colour alone (spec §7): every code has its own icon shape.
 *
 * Red is deliberately absent: overdue is not a status, it is DERIVED from the deadline (rule 10,
 * invariant 3) and drawn beside the deadline. An unknown code falls to the neutral pill with the
 * neutral icon — it still shows the label the screen computed for it.
 */
const STATUS_LOOK: Readonly<Record<string, { tone: BadgeTone; icon: LucideIcon }>> = {
  "da-tiep-nhan": { tone: "info", icon: Inbox },
  "dang-phan-loai": { tone: "info", icon: Tags },
  "da-chuyen-xu-ly": { tone: "info", icon: Forward },
  "dang-xu-ly": { tone: "info", icon: Loader },
  "da-xu-ly": { tone: "info", icon: ClipboardCheck },
  "cho-dan-xac-nhan": { tone: "warning", icon: Hourglass },
  "da-dong": { tone: "success", icon: CircleCheck },
  "khong-tiep-nhan": { tone: "neutral", icon: Ban },
  "chuyen-cap-tren": { tone: "neutral", icon: ArrowUpRight },
};

/** Status pill. `children` is the label for `status`, passed verbatim by the caller. */
export function PetitionStatusBadge({ status, children }: { status: string; children: ReactNode }) {
  const look = STATUS_LOOK[status];
  return (
    <Badge tone={look?.tone ?? "neutral"} icon={look?.icon}>
      {children}
    </Badge>
  );
}

/** Decorative icon inside a button or a line of text: hidden from assistive tech, never focusable. */
export function Glyph({ icon: Icon, className }: { icon: LucideIcon; className?: string }) {
  return <Icon aria-hidden="true" focusable="false" strokeWidth={1.8} className={className} />;
}

/**
 * Classes of a NATIVE `<button>` drawn like `Button`. Used where a test reads the raw element (its
 * `type` must be the FIRST attribute — `<button type="submit"…>` — or the tree walk must meet a
 * `"button"` element, not a component). The legacy class comes first, as in `Button`.
 */
export function buttonClass(variant: ButtonVariant = "secondary", size: ButtonSize = "md", extra?: string): string {
  return cn(LEGACY_BUTTON_CLASS[variant], buttonVariants({ variant, size }), extra);
}

/**
 * A busy submit label that KEEPS THE BUTTON'S WIDTH (spec v2 §8b): both labels sit in one grid
 * cell, the idle one turns invisible. The idle label stays in the markup, so the button never
 * resizes and the text tests read is still there.
 */
export function BusyLabel({ busy, children }: { busy: boolean; children: ReactNode }) {
  return (
    <span className="inline-grid [&>*]:[grid-area:1/1]">
      <span className={busy ? "invisible" : undefined}>{children}</span>
      {busy && (
        <span className="inline-flex items-center justify-center gap-1.5">
          <Glyph icon={LoaderCircle} className="motion-safe:animate-spin" />
          Đang lưu…
        </span>
      )}
    </span>
  );
}

/** Segmented track around native `aria-pressed` toggle buttons (spec §7). */
export const TOGGLE_TRACK = "inline-flex max-w-full flex-wrap gap-0.5 rounded-control bg-[#f1f4f8] p-[3px]";

export function toggleButtonClass(on: boolean): string {
  return cn(
    "inline-flex h-[34px] cursor-pointer items-center gap-1.5 rounded-lg border-0 bg-transparent px-3 [font-family:inherit] text-[13px] font-medium whitespace-nowrap text-ink-500",
    "transition-[background-color,color,box-shadow] duration-150 hover:text-ink-900",
    "focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-brand-500",
    on && "bg-surface font-semibold text-ink-900 shadow-sm",
  );
}

/**
 * First-load placeholder for the register (spec §8b): grey bars in the shape of table rows, so the
 * layout does not jump when the page arrives. Decorative: the screen's own `role="status"` sentence
 * announces the load.
 */
export function PetitionRowsSkeleton({ rows = 5 }: { rows?: number }) {
  return (
    <div aria-hidden="true" className="divide-y divide-line">
      {Array.from({ length: rows }, (_, i) => (
        <div key={i} className="flex h-12 items-center gap-4 px-4">
          <span className="h-3 w-24 shrink-0 rounded bg-line motion-safe:animate-pulse" />
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

/** Section heading inside a drawer card: icon + 15px/600 words (spec §3). */
export function SectionTitle({
  icon,
  id,
  as: Heading = "h4",
  children,
}: {
  icon: LucideIcon;
  id?: string;
  as?: "h3" | "h4" | "h5";
  children: ReactNode;
}) {
  return (
    <Heading id={id} className="m-0 flex items-center gap-2 text-[15px] leading-snug font-semibold text-ink-900">
      <Glyph icon={icon} className="size-[18px] shrink-0 text-brand-600" />
      {children}
    </Heading>
  );
}

/**
 * The frame of every textarea on the screen — the Field frame without its fixed 40px height. The
 * screen's own `<textarea>` keeps its id, name, value and handler.
 */
export const TEXTAREA_CLASS = cn(
  "block w-full min-w-0 rounded-control border border-line-strong bg-surface px-3 py-2 [font-family:inherit] text-base text-ink-900 md:text-sm",
  "transition-[border-color,box-shadow] duration-150 placeholder:text-ink-500",
  "focus-visible:border-brand-500 focus-visible:shadow-[0_0_0_3px_var(--brand-100)] focus-visible:outline-none",
  "disabled:cursor-not-allowed disabled:bg-surface-muted",
);

/**
 * One processing act inside the `Xử lý phiếu` card. Keeps the legacy `form-danh-muc` class (its
 * 44px button floor for older staff still applies) but drops its own frame: the card is the frame,
 * a hairline divides the acts.
 */
export const ACT_CLASS = "form-danh-muc m-0 flex flex-col gap-3 rounded-none border-0 bg-transparent p-4";

/** Label above a control, 12px/600 (spec §6.3). */
export const LABEL_CLASS = "mb-1.5 block text-xs leading-tight font-semibold text-ink-700";

/** Small helper / counter line under a control. */
export const HINT_CLASS = "mt-1 mb-0 text-xs text-ink-500";

/** `dl` of label–value pairs: label column + value column from 768px, stacked below. */
export const FIELD_LIST_CLASS = cn(
  "m-0 grid grid-cols-1 gap-x-6 gap-y-0 text-sm md:grid-cols-[12rem_minmax(0,1fr)]",
  "[&>dt]:pt-3 [&>dt]:text-xs [&>dt]:font-semibold [&>dt]:text-ink-500 md:[&>dt]:border-t md:[&>dt]:border-line md:[&>dt]:pb-3",
  "[&>dd]:m-0 [&>dd]:min-w-0 [&>dd]:pb-3 [&>dd]:text-ink-900 md:[&>dd]:border-t md:[&>dd]:border-line md:[&>dd]:pt-3",
  "md:[&>dt:first-of-type]:border-t-0 md:[&>dt:first-of-type+dd]:border-t-0",
  "[&_dd_p]:my-0 [&_dd_p+p]:mt-1",
);
