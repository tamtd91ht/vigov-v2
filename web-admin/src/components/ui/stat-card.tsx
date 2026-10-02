import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

import { cn } from "@/lib/cn";

/**
 * KPI card — spec §7 "Thẻ số liệu": icon + one-line 12px label → 26px/700 tabular figure → 11–12px
 * comparison line. Cards in one row are the same height and their figures share a baseline
 * (the label is clipped to one line, never wraps).
 *
 * The figure is drawn in TEXT colour, never a decorative colour. `alert` paints it red and is for
 * one case only — an overdue count above zero (spec §7). The caller decides that from data it
 * already has; this component never inspects the number.
 *
 * `value === null` draws "—" with "Chưa có dữ liệu" (spec: never an empty cell, never a raw error).
 * Pass `null` only where the screen ALREADY shows "no data" today — this changes the look of that
 * state, not when it happens.
 */
export type StatTone = "brand" | "success" | "warning" | "danger" | "neutral";

const TONE_TILE: Record<StatTone, string> = {
  brand: "bg-brand-50 text-brand-600",
  success: "bg-success-50 text-success-600",
  warning: "bg-warning-50 text-warning-600",
  danger: "bg-danger-50 text-danger-600",
  neutral: "bg-[#f1f4f8] text-ink-500",
};

export const NO_DATA_CAPTION = "Chưa có dữ liệu";

export type StatCardProps = {
  icon: LucideIcon;
  label: string;
  value: ReactNode | null;
  caption?: ReactNode;
  tone?: StatTone;
  alert?: boolean;
  className?: string;
};

export function StatCard({ icon: Icon, label, value, caption, tone = "brand", alert = false, className }: StatCardProps) {
  const empty = value === null;
  return (
    <div
      className={cn(
        // No hover lift: the card is not clickable, and a card that rises under the pointer
        // promises an action it does not have (spec v2: flat, very light shadow).
        "flex h-full min-w-0 flex-col gap-2 rounded-xl border border-line bg-surface p-4 shadow-sm",
        className,
      )}
    >
      <div className="flex min-w-0 items-center gap-2">
        <span aria-hidden="true" className={cn("grid size-8 shrink-0 place-items-center rounded-lg", TONE_TILE[tone])}>
          <Icon className="size-4" strokeWidth={1.8} focusable="false" />
        </span>
        <span className="min-w-0 truncate text-xs font-semibold text-ink-500" title={label}>
          {label}
        </span>
      </div>
      <p
        className={cn(
          "m-0 text-[26px] leading-none font-bold tabular-nums",
          empty ? "text-ink-400" : alert ? "text-danger-600" : "text-ink-900",
        )}
      >
        {empty ? "—" : value}
      </p>
      {(empty || caption !== undefined) && (
        <p className="m-0 flex items-center gap-1 text-xs text-ink-500 [&_svg]:size-3.5 [&_svg]:shrink-0">
          {empty ? NO_DATA_CAPTION : caption}
        </p>
      )}
    </div>
  );
}
