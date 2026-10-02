import { History, type LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

import { cn } from "@/lib/cn";

/**
 * Page header — spec §5, "áp dụng cho MỌI màn hình": a 48px icon tile, the page `<h1>` (24px/700),
 * ONE line of subtitle, and the actions on the right of the same row.
 *
 * The tile is FLAT — `--brand-50` with a `--brand-600` icon, no gradient, no shadow (spec v2,
 * ADR 0068 §11). A solid brand tile on every page competed with the page's one primary button.
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
  icon: LucideIcon;
  title: ReactNode;
  subtitle?: ReactNode;
  actions?: ReactNode;
  /** Small metadata line under the subtitle — usually a `PageHeaderMeta`. */
  meta?: ReactNode;
  /** `id` of the `<h1>`, for `aria-labelledby` on the page's main region. */
  titleId?: string;
  className?: string;
};

export function PageHeader({ icon: Icon, title, subtitle, actions, meta, titleId, className }: PageHeaderProps) {
  return (
    <header className={cn("mb-5 flex flex-wrap items-center gap-x-4 gap-y-3", className)}>
      <span
        aria-hidden="true"
        className="grid size-12 shrink-0 place-items-center rounded-xl border border-brand-100 bg-brand-50 text-brand-600"
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
        {meta !== undefined && (
          <div className="page-header-meta mt-1 flex flex-wrap items-center gap-x-2 text-xs text-ink-500 [&_svg]:size-3.5 [&_svg]:shrink-0">
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
