import { Inbox, type LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

import { cn } from "@/lib/cn";

/**
 * Empty state — copied from `web-admin/src/components/ui/empty-state.tsx` (change both).
 *
 * The SENTENCE stays the screen's own, verbatim. This changes how "nothing here" looks, never when
 * the screen decides there is nothing.
 */
export type EmptyStateTone = "brand" | "neutral" | "danger";

const TONE_CIRCLE: Record<EmptyStateTone, string> = {
  brand: "bg-brand-50 text-brand-600",
  neutral: "bg-surface-muted text-ink-500",
  danger: "bg-danger-50 text-danger-600",
};

export type EmptyStateProps = {
  icon?: LucideIcon;
  title: ReactNode;
  description?: ReactNode;
  action?: ReactNode;
  className?: string;
  role?: "status";
  tone?: EmptyStateTone;
};

export function EmptyState({ icon: Icon = Inbox, title, description, action, className, role, tone = "brand" }: EmptyStateProps) {
  return (
    <div role={role} className={cn("flex flex-col items-center gap-2 px-4 py-10 text-center", className)}>
      <span aria-hidden="true" className={cn("mb-2 grid size-[72px] place-items-center rounded-full", TONE_CIRCLE[tone])}>
        <Icon className="size-8" strokeWidth={1.6} focusable="false" />
      </span>
      <p className="m-0 text-base font-semibold text-ink-900">{title}</p>
      {description !== undefined && <p className="m-0 max-w-md text-[13px] text-ink-500">{description}</p>}
      {action !== undefined && <div className="mt-2">{action}</div>}
    </div>
  );
}
