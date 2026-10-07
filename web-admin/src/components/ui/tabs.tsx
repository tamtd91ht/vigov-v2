import type { LucideIcon } from "lucide-react";
import type { ComponentProps } from "react";

import { cn } from "@/lib/cn";

/**
 * Toolbar tabs — shadcn's Tabs look (spec 00 §5, ADR 0068 lần 6): a `muted` track 32px tall with 3px
 * padding; the selected segment is the page colour with the foreground word and a small shadow, the
 * others a 60% foreground word that darkens on hover. (Lần 2's navy solid segment is gone.)
 *
 * PRIMITIVES, NOT A TABS WIDGET. The screens already own their tab state, their `id`s, their
 * `aria-controls` and their click handlers (the Nhiệm vụ scope switch). Cấu hình draws its own
 * segmented strip since ADR 0068 lần 5 (the prototype's `TabsList`, `khung-tab-cau-hinh.tsx`). `TabList` and `Tab` only draw: every native prop passes through, and
 * `selected` maps to `aria-selected`. A screen keeps its own keyboard handling exactly as it is.
 *
 * The selected tab differs by fill, shadow and word colour together — never by hue alone. The track
 * scrolls sideways rather than wrapping: a wrapped second row would break the fixed 32px track, and
 * at 320px the segments stay one row the thumb can swipe.
 */
export function TabList({ className, ...props }: ComponentProps<"div">) {
  return (
    <div
      role="tablist"
      className={cn(
        "inline-flex h-8 w-fit max-w-full min-w-0 items-center overflow-x-auto rounded-lg bg-muted p-[3px] text-muted-foreground",
        className,
      )}
      {...props}
    />
  );
}

export type TabProps = Omit<ComponentProps<"button">, "role" | "aria-selected"> & {
  selected: boolean;
  icon?: LucideIcon;
};

export function Tab({ selected, icon: Icon, className, children, type = "button", ...props }: TabProps) {
  return (
    <button
      type={type}
      role="tab"
      aria-selected={selected}
      className={cn(
        "relative inline-flex h-[calc(100%-1px)] shrink-0 cursor-pointer items-center justify-center gap-1.5 rounded-md border border-solid border-transparent bg-transparent px-1.5 py-0.5 [font-family:inherit] text-sm font-medium whitespace-nowrap text-foreground/60",
        "transition-all duration-(--dur-fast) ease-(--ease) hover:text-foreground",
        "outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50",
        // No `disabled:` styling: a tab here is never disabled (the "?" placeholder tab dims itself,
        // `pending-feature.tsx`), and tests read a tab's markup for the word "disabled".
        "[&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
        selected && "bg-background text-foreground shadow-sm",
        className,
      )}
      {...props}
    >
      {Icon !== undefined && <Icon aria-hidden="true" focusable="false" strokeWidth={1.8} />}
      {children}
    </button>
  );
}
