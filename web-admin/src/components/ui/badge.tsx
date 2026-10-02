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
 * Status pill — spec §7: 26px, fully rounded, 14px icon + 12px/600 word.
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
  neutral: "bg-[#f1f4f8] text-ink-500",
  warning: "bg-warning-50 text-warning-600",
  danger: "bg-danger-50 text-danger-600",
  info: "bg-brand-50 text-brand-700",
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
        "inline-flex h-[26px] items-center gap-[5px] rounded-full px-2.5 text-xs leading-none font-semibold whitespace-nowrap",
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
