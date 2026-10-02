import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

import { cn } from "@/lib/cn";

/**
 * Page header — copied from `web-admin/src/components/ui/page-header.tsx` (change both): a flat
 * 48px icon tile (no gradient, no shadow — ADR 0068 §11), the page `<h1>`, one optional subtitle
 * line, and the actions on the right of the same row. Actions wrap under the title at narrow widths.
 *
 * DIFFERENCE FROM WEB-ADMIN: no `meta` slot — nothing on this console carries "updated at / by".
 */
export type PageHeaderProps = {
  icon: LucideIcon;
  title: ReactNode;
  subtitle?: ReactNode;
  actions?: ReactNode;
  titleId?: string;
  className?: string;
};

export function PageHeader({ icon: Icon, title, subtitle, actions, titleId, className }: PageHeaderProps) {
  return (
    <header className={cn("mb-5 flex flex-wrap items-center gap-x-4 gap-y-3", className)}>
      <span
        aria-hidden="true"
        className="grid size-12 shrink-0 place-items-center rounded-xl border border-brand-100 bg-brand-50 text-brand-600"
      >
        <Icon className="size-6" strokeWidth={1.8} focusable="false" />
      </span>
      <div className="min-w-0 flex-1 basis-60">
        <h1 id={titleId} className="m-0 text-2xl leading-tight font-bold tracking-[-0.01em] [overflow-wrap:anywhere] text-ink-900">
          {title}
        </h1>
        {subtitle !== undefined && (
          <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-[13px] text-ink-500">{subtitle}</div>
        )}
      </div>
      {actions !== undefined && <div className="flex flex-wrap items-center gap-2 sm:ml-auto">{actions}</div>}
    </header>
  );
}
