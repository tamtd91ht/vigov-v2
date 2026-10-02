import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

import { cn } from "@/lib/cn";

/**
 * Page header — spec §5, "áp dụng cho MỌI màn hình": a 48px brand tile with a white 24px icon,
 * the page `<h1>` (24px/700), ONE line of subtitle, and the actions on the right of the same row.
 *
 * Actions: at most one primary button plus one or two secondary ones, never full width. They
 * wrap under the title at narrow widths instead of squeezing it.
 *
 * The subtitle is a slot, not a string: a screen may put a `Badge` or an icon + short phrase there.
 * Long explanations belong in an `Info` tooltip, not here (spec §7, "Quy tắc câu chữ").
 */
export type PageHeaderProps = {
  icon: LucideIcon;
  title: ReactNode;
  subtitle?: ReactNode;
  actions?: ReactNode;
  /** `id` of the `<h1>`, for `aria-labelledby` on the page's main region. */
  titleId?: string;
  className?: string;
};

export function PageHeader({ icon: Icon, title, subtitle, actions, titleId, className }: PageHeaderProps) {
  return (
    <header className={cn("mb-5 flex flex-wrap items-center gap-x-4 gap-y-3", className)}>
      <span
        aria-hidden="true"
        className="grid size-12 shrink-0 place-items-center rounded-xl bg-linear-to-br from-brand-500 to-brand-700 text-white shadow-[0_6px_16px_-6px_rgba(21,101,192,0.55)]"
      >
        <Icon className="size-6" strokeWidth={1.8} focusable="false" />
      </span>
      <div className="min-w-0 flex-1 basis-60">
        <h1 id={titleId} className="m-0 text-2xl leading-tight font-bold tracking-[-0.01em] text-ink-900">
          {title}
        </h1>
        {subtitle !== undefined && (
          <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-[13px] text-ink-500 [&_svg]:size-3.5 [&_svg]:shrink-0">
            {subtitle}
          </div>
        )}
      </div>
      {actions !== undefined && <div className="flex flex-wrap items-center gap-2 sm:ml-auto">{actions}</div>}
    </header>
  );
}
