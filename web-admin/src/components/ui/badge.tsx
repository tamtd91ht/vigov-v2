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
 * Status tag — shadcn's Badge with the spec's chip colours (spec 00 §4–§5, ADR 0068 lần 6): a 20px
 * pill, 12px/500 word, 12px icon, the tone at 12% fill / full word / 25% border
 * (`bg-{c}/12 text-{c} border-{c}/25`). The tone colours are the spec's as written — their contrast
 * is the owner-accepted debt recorded beside the tokens in `globals.css`.
 *
 * ICON + WORD, NEVER COLOUR ALONE. The spec measured orange vs green at ΔE 4.2 for red-blind
 * readers, so every tone carries a default icon of its own shape and the caller supplies the word.
 * Pass `icon` to pick a more specific one (`EyeOff` for "Chưa hiện", `AlarmClock` for "Quá hạn").
 *
 *   success  leaf       — đang hiện, hoàn thành
 *   neutral  ink-muted  — chưa hiện, ngừng (the spec's `draft` chip)
 *   warning  tangerine  — chờ, sắp hạn
 *   danger   danger     — quá hạn (only for something actually late or wrong)
 *   info     brand      — đơn vị, thông tin
 */
export type BadgeTone = "success" | "neutral" | "warning" | "danger" | "info";

const TONE_CLASS: Record<BadgeTone, string> = {
  success: "border-leaf/25 bg-leaf/12 text-leaf",
  neutral: "border-line bg-ink-muted/12 text-ink",
  warning: "border-tangerine/25 bg-tangerine/12 text-tangerine",
  danger: "border-danger/25 bg-danger/12 text-danger",
  info: "border-brand/25 bg-brand/12 text-brand",
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
  /**
   * Overrides the tone's default icon. `null` draws NO icon — the prototype's text-only chip, for a
   * screen whose owner chose it (`/nguoi-dung`, user 09/10/2026). Absent keeps the icon, so every
   * other screen is unchanged; the word still carries the meaning, never the colour alone.
   */
  icon?: LucideIcon | null;
  children: ReactNode;
};

export function Badge({ tone = "neutral", icon, className, children, ...props }: BadgeProps) {
  const Icon = icon === null ? null : (icon ?? TONE_ICON[tone]);
  return (
    <span
      className={cn(
        "inline-flex h-5 w-fit shrink-0 items-center justify-center gap-1 overflow-hidden rounded-4xl border border-solid px-2 py-0.5 text-xs leading-none font-medium whitespace-nowrap",
        TONE_CLASS[tone],
        className,
      )}
      {...props}
    >
      {Icon !== null && <Icon aria-hidden="true" focusable="false" className="size-3 shrink-0" strokeWidth={2} />}
      {children}
    </span>
  );
}
