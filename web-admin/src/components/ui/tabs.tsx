import type { LucideIcon } from "lucide-react";
import type { ComponentProps } from "react";

import { cn } from "@/lib/cn";

/**
 * Underline tabs — spec §7: 14px/500 + icon, a 2px `--brand-600` underline on the selected tab.
 *
 * PRIMITIVES, NOT A TABS WIDGET. The screens already own their tab state, their `id`s, their
 * `aria-controls` and their click handlers (`features/cau-hinh/khung-tab-cau-hinh.tsx`, the
 * Nhiệm vụ scope switch). `TabList` and `Tab` only draw: every native prop passes through, and
 * `selected` maps to `aria-selected`. A screen keeps its own keyboard handling exactly as it is.
 *
 * The selected tab differs by weight and colour AND the underline — never by colour alone.
 */
export function TabList({ className, ...props }: ComponentProps<"div">) {
  return (
    <div
      role="tablist"
      className={cn("flex min-w-0 flex-wrap gap-1 border-b border-line", className)}
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
        "-mb-px inline-flex min-h-11 cursor-pointer items-center gap-2 rounded-t-lg border-0 border-b-2 border-solid border-transparent bg-transparent px-3.5 [font-family:inherit] text-sm font-medium text-ink-500",
        "transition-[color,background-color,border-color] duration-150 hover:bg-surface-muted hover:text-ink-900",
        "focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-brand-500",
        "[&_svg]:size-[18px] [&_svg]:shrink-0",
        selected && "border-brand-600 font-semibold text-brand-700 hover:bg-transparent hover:text-brand-700",
        className,
      )}
      {...props}
    >
      {Icon !== undefined && <Icon aria-hidden="true" focusable="false" strokeWidth={1.8} />}
      {children}
    </button>
  );
}
