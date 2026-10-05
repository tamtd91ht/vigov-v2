import { ChevronDown, type LucideIcon } from "lucide-react";
import type { ComponentProps, ReactNode } from "react";

import { cn } from "@/lib/cn";

/**
 * Toolbar + Field — spec §6.4: ONE filter row, label above every control, every control 36px
 * (guide §6, 05/10/2026),
 * bottoms aligned, wrapping onto further rows at narrow widths with the left edges still straight.
 *
 * FIELD WRAPS THE SCREEN'S OWN NATIVE CONTROL, IT NEVER REPLACES IT. The `<input>` / `<select>` the
 * screen already renders — with its `id`, `name`, `value`, `onChange` — is passed as `children`
 * unchanged; Field only draws the label, the optional icon and the frame (via descendant
 * selectors). So no field name, id, handler or emitted value can change by adopting it.
 */

/**
 * Frame shared by every native control inside a Field; exported for a control outside one.
 *
 * Focus is the guide's §8.4 look — a pale cyan fill plus a 1px inset ring on top of the border —
 * drawn in `brand-500` (#0284c7, ≥3:1 on white and on the tint), not the guide's #00B1FF, which
 * at 2.40:1 would leave a keyboard user without a visible indicator. Text stays 16px below 768px:
 * iOS zooms the page on focus into anything smaller; 15px (the guide's base) from 768px.
 */
export const controlClass = cn(
  "h-9 w-full min-w-0 rounded-control border border-line-strong bg-surface px-3 [font-family:inherit] text-base text-ink-900 md:text-[15px]",
  "transition-[border-color,box-shadow,background-color] duration-(--dur-fast) ease-(--ease)",
  "placeholder:text-ink-500 hover:not-disabled:border-line-hover",
  "focus-visible:border-brand-500 focus-visible:bg-accent-50 focus-visible:shadow-[inset_0_0_0_1px_var(--brand-500)] focus-visible:outline-none",
  "disabled:cursor-not-allowed disabled:bg-surface-muted disabled:text-ink-500",
);

/**
 * `end` is the right-hand slot (ROADMAP_PHASE2 "chừa sẵn": the future "Cột" / "Xuất" buttons). It is
 * drawn as a `ToolbarActions` after the filters, so it stays at the right end and wraps last. Pass
 * only controls the screen already has — the slot existing is not a reason to add one.
 */
export function Toolbar({ className, children, end, ...props }: ComponentProps<"div"> & { end?: ReactNode }) {
  return (
    <div
      className={cn("flex min-w-0 flex-wrap items-end gap-3 border-b border-line px-4 py-3.5", className)}
      {...props}
    >
      {children}
      {end !== undefined && <ToolbarActions className="toolbar-end">{end}</ToolbarActions>}
    </div>
  );
}

/** Pushes its children to the right end of a Toolbar row (spec §6.4 `.actions`). */
export function ToolbarActions({ className, ...props }: ComponentProps<"div">) {
  return <div className={cn("ml-auto flex flex-wrap items-end gap-2", className)} {...props} />;
}

export type FieldProps = {
  /** The visible label. Rendered as a real `<label htmlFor>` — always present for assistive tech. */
  label: ReactNode;
  /** `id` of the native control passed as `children`. */
  htmlFor: string;
  /**
   * Hide the label visually (class `an-thi-giac`) when an icon and placeholder already say it
   * (spec §6.4). The `<label>` stays in the DOM, so the control keeps its accessible name.
   */
  hideLabel?: boolean;
  /** Leading decorative icon drawn inside the control. */
  icon?: LucideIcon;
  /** `select` draws a ChevronDown and hides the native arrow so all selects look alike. */
  kind?: "input" | "select";
  /**
   * Width behaviour inside a Toolbar: `search` grows (320–420px), `field` is a 180–220px column,
   * `auto` takes its content width.
   */
  grow?: "search" | "field" | "auto";
  /** One short helper line under the control. */
  hint?: ReactNode;
  className?: string;
  /** The screen's own native `<input>`, `<select>` or `<textarea>`, unchanged. */
  children: ReactNode;
};

const GROW_CLASS = {
  search: "min-w-0 flex-[1_1_320px] max-w-[420px]",
  field: "min-w-[180px] flex-[0_1_220px]",
  auto: "flex-none",
} as const;

export function Field({
  label,
  htmlFor,
  hideLabel = false,
  icon: Icon,
  kind = "input",
  grow = "field",
  hint,
  className,
  children,
}: FieldProps) {
  return (
    <div className={cn("flex flex-col gap-1.5", GROW_CLASS[grow], className)}>
      <label
        htmlFor={htmlFor}
        className={hideLabel ? "an-thi-giac" : "text-xs leading-tight font-medium text-ink-700"}
      >
        {label}
      </label>
      <div
        className={cn(
          "relative",
          "[&_input:not([type=checkbox]):not([type=radio])]:h-9 [&_select]:h-9",
          "[&_:is(input,select,textarea)]:w-full [&_:is(input,select,textarea)]:min-w-0",
          "[&_:is(input,select,textarea)]:rounded-control [&_:is(input,select,textarea)]:border",
          "[&_:is(input,select,textarea)]:border-line-strong [&_:is(input,select,textarea)]:bg-surface",
          "[&_:is(input,select,textarea)]:px-3 [&_:is(input,select,textarea)]:[font-family:inherit]",
          "[&_:is(input,select,textarea)]:text-base md:[&_:is(input,select,textarea)]:text-[15px]",
          "[&_:is(input,select,textarea)]:text-ink-900 [&_:is(input,select,textarea)]:transition-[border-color,box-shadow,background-color]",
          "[&_:is(input,select,textarea):focus-visible]:border-brand-500 [&_:is(input,select,textarea):focus-visible]:outline-none",
          "[&_:is(input,select,textarea):focus-visible]:bg-accent-50",
          "[&_:is(input,select,textarea):focus-visible]:shadow-[inset_0_0_0_1px_var(--brand-500)]",
          "[&_:is(input,select,textarea):disabled]:cursor-not-allowed [&_:is(input,select,textarea):disabled]:bg-surface-muted",
          "[&_::placeholder]:text-ink-500",
          // SAME VARIANT AS `px-3` ON PURPOSE. Tailwind orders rules by variant first, property
          // second: written as `[&_:is(input,select)]:pl-10` it was emitted BEFORE the
          // `[&_:is(input,select,textarea)]:px-3` rule, which then reset the left padding and the
          // icon sat on top of the text (owner screenshot 02/10/2026). Within one variant
          // `padding-left`/`padding-right` sort after `padding-inline`, so these win.
          Icon !== undefined && "[&_:is(input,select,textarea)]:pl-10",
          kind === "select" && "[&_select]:cursor-pointer [&_select]:appearance-none [&_:is(input,select,textarea)]:pr-9",
          // Field draws its own ChevronDown icon below; the global select frame's background-image
          // chevron (`globals.css`) would be a second one.
          kind === "select" && "[&_select]:bg-none",
          // The search box is the guide's §8.4 pill: 32px radius, `--bg` fill, no visible border
          // until focus (the transparent border keeps the 36px box the same size). SAME VARIANT as
          // the frame above on purpose: `cn` (tailwind-merge) then DROPS the frame's radius, border
          // colour, fill and padding, rather than leaving two rules whose winner is emit order.
          grow === "search" &&
            "[&_:is(input,select,textarea)]:rounded-pill [&_:is(input,select,textarea)]:border-transparent [&_:is(input,select,textarea)]:bg-canvas [&_:is(input,select,textarea)]:px-4",
          grow === "search" && Icon !== undefined && "[&_:is(input,select,textarea)]:pl-10",
        )}
      >
        {Icon !== undefined && (
          <Icon
            aria-hidden="true"
            focusable="false"
            strokeWidth={1.8}
            className="pointer-events-none absolute top-1/2 left-3 size-[18px] -translate-y-1/2 text-ink-500"
          />
        )}
        {children}
        {kind === "select" && (
          <ChevronDown
            aria-hidden="true"
            focusable="false"
            strokeWidth={1.8}
            className="pointer-events-none absolute top-1/2 right-3 size-4 -translate-y-1/2 text-ink-500"
          />
        )}
      </div>
      {hint !== undefined && <p className="m-0 text-xs text-ink-500">{hint}</p>}
    </div>
  );
}
