import { Info, TriangleAlert, WifiOff, type LucideIcon } from "lucide-react";
import type { ComponentProps, ReactNode } from "react";

import { cn } from "@/lib/cn";

/**
 * Banner — spec v2 §7 / §8b: a LONG-LIVED system condition shown across the top of the content
 * area — offline, "dữ liệu chưa đủ nguồn". Not a toast (short result of an action), not a Notice
 * (a fixed explanatory note), not a form error.
 *
 *   info     light blue — data incomplete, a source not connected yet
 *   warning  amber — something the person should know before acting
 *   offline  amber + `WifiOff` — the network is gone
 *
 * Shown and hidden by the screen's OWN condition; nothing here detects the network or the data.
 * `role` passes through (`status` for a polite announcement) — none by default, so adopting the
 * banner never adds an announcement the screen did not make. Every tone carries an icon and words,
 * never colour alone.
 */
export type BannerTone = "info" | "warning" | "offline";

const TONE: Record<BannerTone, { box: string; icon: string; Icon: LucideIcon }> = {
  info: { box: "border-brand-100 bg-brand-50 text-ink-700", icon: "text-brand-600", Icon: Info },
  warning: { box: "border-legal-200 bg-warning-50 text-ink-900", icon: "text-warning-600", Icon: TriangleAlert },
  offline: { box: "border-legal-200 bg-warning-50 text-ink-900", icon: "text-warning-600", Icon: WifiOff },
};

export type BannerProps = Omit<ComponentProps<"div">, "title"> & {
  tone?: BannerTone;
  icon?: LucideIcon;
  title?: ReactNode;
  /** Right-hand slot for an action the screen already has. */
  action?: ReactNode;
};

export function Banner({ tone = "info", icon, title, action, className, children, ...props }: BannerProps) {
  const t = TONE[tone];
  const Icon = icon ?? t.Icon;
  return (
    <div
      className={cn("banner flex flex-wrap items-center gap-x-3 gap-y-2 rounded-xl border px-4 py-3 text-[13px]", t.box, className)}
      {...props}
    >
      <Icon aria-hidden="true" focusable="false" strokeWidth={1.8} className={cn("size-[18px] shrink-0", t.icon)} />
      <div className="min-w-0 flex-1">
        {title !== undefined && <strong className="font-semibold">{title} </strong>}
        {children}
      </div>
      {action !== undefined && <div className="flex shrink-0 items-center gap-2">{action}</div>}
    </div>
  );
}
