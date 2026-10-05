import {
  CircleAlert,
  CircleCheck,
  CircleMinus,
  Clock,
  Info,
  type LucideIcon,
} from "lucide-react";
import type { ComponentProps, ReactNode } from "react";

import { cn } from "@/lib/cn";

/**
 * Status tag — guide §8.6 (05/10/2026): a 6px-radius tag on the tone's 10% tint, 14px icon +
 * 12px/500 word (never below 11px). Each tone's word is its TEXT-safe shade on its own tint:
 * green 5.54:1, amber 5.52:1, red 5.01:1, cyan 5.40:1, navy-muted ≥4.62:1.
 *
 * ICON + WORD, NEVER COLOUR ALONE. The spec measured orange vs green at ΔE 4.2 for red-blind
 * readers, so every tone carries a default icon of its own shape and the caller supplies the word.
 * Pass `icon` to pick a more specific one (`EyeOff` for "Chưa hiện", `AlarmClock` for "Quá hạn").
 *
 *   success  green  — đang hiện, hoàn thành
 *   neutral  grey   — chưa hiện, ngừng
 *   warning  amber  — chờ, sắp hạn
 *   danger   red    — quá hạn (only for something actually late or wrong)
 *   info     blue   — đơn vị, thông tin
 */
export type BadgeTone = "success" | "neutral" | "warning" | "danger" | "info";

const TONE_CLASS: Record<BadgeTone, string> = {
  success: "bg-success-50 text-success-600",
  neutral: "bg-surface-subtle text-ink-500",
  warning: "bg-warning-50 text-warning-600",
  danger: "bg-danger-50 text-danger-600",
  info: "bg-accent-50 text-accent-700",
};

const TONE_ICON: Record<BadgeTone, LucideIcon> = {
  success: CircleCheck,
  neutral: CircleMinus,
  warning: Clock,
  danger: CircleAlert,
  info: Info,
};

export type BadgeProps = Omit<ComponentProps<"span">, "children"> & {
  tone?: BadgeTone;
  /** Overrides the tone's default icon. */
  icon?: LucideIcon;
  children: ReactNode;
};

export function Badge({ tone = "neutral", icon, className, children, ...props }: BadgeProps) {
  const Icon = icon ?? TONE_ICON[tone];
  return (
    <span
      className={cn(
        "inline-flex h-6 items-center gap-[5px] rounded-xs px-2 text-xs leading-none font-medium whitespace-nowrap",
        TONE_CLASS[tone],
        className,
      )}
      {...props}
    >
      <Icon aria-hidden="true" focusable="false" className="size-3.5 shrink-0" strokeWidth={2} />
      {children}
    </span>
  );
}
