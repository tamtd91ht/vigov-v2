import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

import { cn } from "@/lib/cn";

/**
 * Page header — copied from `web-admin/src/components/ui/page-header.tsx` (change both; spec 02 §1 /
 * 00 §3, ADR 0068 lần 6): the page `<h1>` (navy 22px/700), one optional subtitle line (ink-muted
 * 13px), and the actions on the right of the same row, bottoms aligned. Actions wrap under the title
 * at narrow widths.
 *
 * `icon` IS STILL ACCEPTED AND IGNORED, as in web-admin: the spec's header has no icon tile any more,
 * and every screen here passes one — dropping it from each is a markup change in every screen for no
 * visible gain.
 *
 * DIFFERENCE FROM WEB-ADMIN: no `meta` slot — nothing on this console carries "updated at / by".
 */
export type PageHeaderProps = {
  /** Ignored since ADR 0068 lần 6 (no icon tile) — kept so existing callers compile. */
  icon?: LucideIcon;
  title: ReactNode;
  subtitle?: ReactNode;
  actions?: ReactNode;
  titleId?: string;
  className?: string;
};

export function PageHeader({ title, subtitle, actions, titleId, className }: PageHeaderProps) {
  return (
    <header className={cn("mb-3 flex flex-wrap items-end gap-4", className)}>
      <div className="min-w-0 flex-1 basis-60">
        <h1 id={titleId} className="m-0 text-[22px] leading-tight font-bold [overflow-wrap:anywhere] text-navy">
          {title}
        </h1>
        {subtitle !== undefined && (
          <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-[13px] text-ink-muted [&_svg]:size-3.5 [&_svg]:shrink-0">
            {subtitle}
          </div>
        )}
      </div>
      {actions !== undefined && <div className="flex flex-wrap items-center gap-2 sm:ml-auto">{actions}</div>}
    </header>
  );
}
