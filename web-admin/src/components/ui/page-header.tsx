import { History, type LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

import { cn } from "@/lib/cn";

/**
 * Page header — spec 02 §1 / 00 §3 (ADR 0068 lần 6): the page `<h1>` (navy 22px/700), ONE line of
 * subtitle (ink-muted 13px), and the actions on the right of the same row, bottoms aligned. NO icon
 * tile any more — the prototype's header has none.
 *
 * `icon` IS STILL ACCEPTED AND IGNORED: eighteen screens pass one, and dropping it from each is a
 * markup change in eighteen files for no visible gain. It is optional, so a new screen need not
 * pass it.
 *
 * `meta` is the slot for "History Cập nhật 02/10/2026 10:32 — Nguyễn Văn A" (spec v2 §8b
 * "Minh bạch & tin cậy"): pass it ONLY where the screen's data already carries the time and the
 * person. Nothing here fetches or invents it (ROADMAP_PHASE2 "chừa sẵn"). `PageHeaderMeta` draws
 * the line.
 *
 * Actions: at most one primary button plus one or two secondary ones, never full width. They
 * wrap under the title at narrow widths instead of squeezing it.
 *
 * The subtitle is a slot, not a string: a screen may put a `Badge` or an icon + short phrase there.
 * Long explanations belong in an `Info` tooltip, not here (spec §7, "Quy tắc câu chữ").
 */
export type PageHeaderProps = {
  /** Ignored since ADR 0068 lần 6 (no icon tile) — kept so existing callers compile. */
  icon?: LucideIcon;
  title: ReactNode;
  subtitle?: ReactNode;
  actions?: ReactNode;
  /** Small metadata line under the subtitle — usually a `PageHeaderMeta`. */
  meta?: ReactNode;
  /** `id` of the `<h1>`, for `aria-labelledby` on the page's main region. */
  titleId?: string;
  className?: string;
};

export function PageHeader({ title, subtitle, actions, meta, titleId, className }: PageHeaderProps) {
  return (
    <header className={cn("mb-3 flex flex-wrap items-end gap-4", className)}>
      <div className="min-w-0 flex-1 basis-60">
        <h1 id={titleId} className="m-0 text-[22px] leading-tight font-bold text-navy">
          {title}
        </h1>
        {subtitle !== undefined && (
          <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-[13px] text-ink-muted [&_svg]:size-3.5 [&_svg]:shrink-0">
            {subtitle}
          </div>
        )}
        {meta !== undefined && (
          <div className="page-header-meta mt-1 flex flex-wrap items-center gap-x-2 text-xs text-ink-muted [&_svg]:size-3.5 [&_svg]:shrink-0">
            {meta}
          </div>
        )}
      </div>
      {actions !== undefined && <div className="flex flex-wrap items-center gap-2 sm:ml-auto">{actions}</div>}
    </header>
  );
}

/**
 * "History Cập nhật <at> — <by>". Both values come from the screen's own data, already formatted
 * (date format is the screen's call, and the person is printed as the data names them). `by`
 * omitted = the data carries no person: the dash and the name are left out, never guessed.
 */
export function PageHeaderMeta({ at, by }: { at: ReactNode; by?: ReactNode }) {
  return (
    <span className="inline-flex flex-wrap items-center gap-1">
      <History aria-hidden="true" focusable="false" strokeWidth={1.8} />
      <span>Cập nhật {at}</span>
      {by !== undefined && <span>— {by}</span>}
    </span>
  );
}
