import { Info, LockKeyhole, type LucideIcon } from "lucide-react";
import type { ComponentProps, ReactNode } from "react";

import { cn } from "@/lib/cn";

/**
 * Notice — spec §7 "Ghi chú pháp lý" and the informational boxes around it.
 *
 *   info     brand-blue tint, `Info` icon — a neutral explanation
 *   legal    gold-50 + `#F5E1A4` border, `LockKeyhole` icon — Decree 13/2023 and similar legal
 *            reminders, 1–2 lines (spec §8.1)
 *   neutral  grey surface — scope notes ("không phải phần mềm kế toán")
 *
 * There is deliberately NO red tone: red is for an error or an overdue item, and a red note that is
 * neither is the "báo động giả" the spec removes. Errors keep their own `role="alert"` markup.
 *
 * `role`, `aria-*` and `id` pass through, so a screen keeps whatever announcement it had.
 */
export type NoticeTone = "info" | "legal" | "neutral";

const TONE: Record<NoticeTone, { box: string; icon: string; Icon: LucideIcon }> = {
  info: { box: "border-brand-100 bg-brand-50 text-ink-700", icon: "text-brand-600", Icon: Info },
  legal: { box: "border-legal-200 bg-legal-50 text-legal-800", icon: "text-warning-600", Icon: LockKeyhole },
  neutral: { box: "border-line bg-surface-muted text-ink-700", icon: "text-ink-500", Icon: Info },
};

export type NoticeProps = Omit<ComponentProps<"div">, "title"> & {
  tone?: NoticeTone;
  /** Overrides the tone's icon. */
  icon?: LucideIcon;
  /** Optional bold lead-in, e.g. "Dữ liệu cá nhân – NĐ 13/2023/NĐ-CP." */
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
      <div className="min-w-0">
        {title !== undefined && <strong className="font-semibold">{title} </strong>}
        {children}
      </div>
    </div>
  );
}
