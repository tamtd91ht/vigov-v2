import { Info, LockKeyhole, type LucideIcon } from "lucide-react";
import type { ComponentProps, ReactNode } from "react";

import { cn } from "@/lib/cn";

/**
 * Notice — copied from `web-admin/src/components/ui/notice.tsx` (change both).
 *
 *   info     brand-blue tint — a neutral explanation
 *   legal    gold tint, `LockKeyhole` — a reminder with consequences (a secret shown once, a batch
 *            of codes invalidated, a commune's sites stopping)
 *   neutral  grey surface
 *
 * Deliberately NO red tone: red is for an error. Errors keep their own `role="alert"` markup.
 * `role`, `aria-*` and `id` pass through, so a screen keeps whatever announcement it had.
 *
 * DIFFERENCE FROM WEB-ADMIN: the body is a column with an 8px gap and `<p>` margins reset, because
 * several notices here hold two paragraphs and this app has no legacy paragraph rules.
 */
export type NoticeTone = "info" | "legal" | "neutral";

const TONE: Record<NoticeTone, { box: string; icon: string; Icon: LucideIcon }> = {
  info: { box: "border-brand-100 bg-brand-50 text-ink-700", icon: "text-brand-600", Icon: Info },
  legal: { box: "border-legal-200 bg-legal-50 text-legal-800", icon: "text-warning-600", Icon: LockKeyhole },
  neutral: { box: "border-line bg-surface-muted text-ink-700", icon: "text-ink-500", Icon: Info },
};

export type NoticeProps = Omit<ComponentProps<"div">, "title"> & {
  tone?: NoticeTone;
  icon?: LucideIcon;
  /** Optional bold lead-in. */
  title?: ReactNode;
};

export function Notice({ tone = "info", icon, title, className, children, ...props }: NoticeProps) {
  const t = TONE[tone];
  const Icon = icon ?? t.Icon;
  return (
    <div
      className={cn("flex items-start gap-3 rounded-xl border px-3.5 py-3 text-[13px] leading-relaxed", t.box, className)}
      {...props}
    >
      <Icon aria-hidden="true" focusable="false" strokeWidth={1.8} className={cn("mt-0.5 size-[18px] shrink-0", t.icon)} />
      <div className="flex min-w-0 flex-col gap-2 [&_p]:m-0">
        {title !== undefined && <strong className="font-semibold">{title}</strong>}
        {children}
      </div>
    </div>
  );
}
