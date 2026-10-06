import type { LucideIcon } from "lucide-react";
import type { ComponentProps } from "react";

import { cn } from "@/lib/cn";

/**
 * Toolbar tabs — guide §7 / §8.1 (05/10/2026): a row of 36px, 8px-radius segments; the others on
 * the navy 5% fill, the selected one NAVY SOLID with white text (13.04:1). Hover on an unselected
 * tab keeps the fill and adds a 1px navy border (guide §9).
 *
 * PRIMITIVES, NOT A TABS WIDGET. The screens already own their tab state, their `id`s, their
 * `aria-controls` and their click handlers (the Nhiệm vụ scope switch). Cấu hình draws its own
 * segmented strip since ADR 0068 lần 5 (the prototype's `TabsList`, `khung-tab-cau-hinh.tsx`). `TabList` and `Tab` only draw: every native prop passes through, and
 * `selected` maps to `aria-selected`. A screen keeps its own keyboard handling exactly as it is.
 *
 * The selected tab differs by weight (500 vs 400) and by a fill whose luminance is far from its
 * neighbours' — never by hue alone.
 */
export function TabList({ className, ...props }: ComponentProps<"div">) {
  return (
    <div
      role="tablist"
      className={cn("flex min-w-0 flex-wrap items-center gap-2", className)}
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
        "inline-flex h-9 cursor-pointer items-center gap-2 rounded-control border border-solid border-transparent bg-surface-subtle px-4 [font-family:inherit] text-[15px] font-normal text-ink-900",
        "transition-[color,background-color,border-color] duration-(--dur-fast) ease-(--ease) hover:border-ink-900",
        "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-500",
        "[&_svg]:size-5 [&_svg]:shrink-0",
        selected && "border-brand-600 bg-brand-600 font-medium text-white hover:border-brand-600",
        className,
      )}
      {...props}
    >
      {Icon !== undefined && <Icon aria-hidden="true" focusable="false" strokeWidth={1.8} />}
      {children}
    </button>
  );
}
