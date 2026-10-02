import { Inbox, type LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

import { cn } from "@/lib/cn";

/**
 * Empty state — spec §7: a 72px `--brand-50` circle with an icon, a 16px/600 title, ONE line of
 * guidance, and an optional action. For an empty table, "no request waiting", "no source data".
 *
 * The SENTENCES stay the screen's own: pass the screen's existing empty-state sentence as `title`
 * (or `description`) verbatim. This component changes how "nothing here" looks, never when the
 * screen decides there is nothing — nor what it says about why (spec §8.2).
 *
 * Renders a `<div>`; give it `role="status"` from the caller only where the screen already
 * announced the empty state.
 */
export type EmptyStateProps = {
  icon?: LucideIcon;
  title: ReactNode;
  description?: ReactNode;
  action?: ReactNode;
  className?: string;
  role?: "status";
};

export function EmptyState({ icon: Icon = Inbox, title, description, action, className, role }: EmptyStateProps) {
  return (
    <div role={role} className={cn("flex flex-col items-center gap-2 px-4 py-10 text-center", className)}>
      <span aria-hidden="true" className="mb-2 grid size-[72px] place-items-center rounded-full bg-brand-50 text-brand-600">
        <Icon className="size-8" strokeWidth={1.6} focusable="false" />
      </span>
      <p className="m-0 text-base font-semibold text-ink-900">{title}</p>
      {description !== undefined && <p className="m-0 max-w-md text-[13px] text-ink-500">{description}</p>}
      {action !== undefined && <div className="mt-2">{action}</div>}
    </div>
  );
}
