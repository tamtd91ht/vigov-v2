import { ChevronDown, type LucideIcon } from "lucide-react";
import type { ComponentProps, ReactNode } from "react";

import { cn } from "@/lib/cn";

/**
 * Toolbar + Field — spec §6.4: ONE filter row, label above every control, every control 32px
 * (spec 00 §5, ADR 0068 lần 6), bottoms aligned, wrapping onto further rows at narrow widths with the left edges still straight.
 *
 * FIELD WRAPS THE SCREEN'S OWN NATIVE CONTROL, IT NEVER REPLACES IT. The `<input>` / `<select>` the
 * screen already renders — with its `id`, `name`, `value`, `onChange` — is passed as `children`
 * unchanged; Field only draws the label, the optional icon and the frame (via descendant
 * selectors). So no field name, id, handler or emitted value can change by adopting it.
 */

/**
 * Frame shared by every native control inside a Field; exported for a control outside one.
 *
 * shadcn's Input (spec 00 §5): 32px, `rounded-lg`, the `input` hairline, 10px side padding, focus =
 * the `ring` border plus a 3px `ring/50` halo. Text stays 16px below 768px: iOS zooms the page on
 * focus into anything smaller; 14px from 768px (shadcn `md:text-sm`). Screens that want the spec's
 * page size write `h-9 text-[12.5px]` after it.
 */
export const controlClass = cn(
  "h-8 w-full min-w-0 rounded-lg border border-solid border-input bg-surface px-2.5 py-1 [font-family:inherit] text-base text-navy md:text-sm",
  "transition-[border-color,box-shadow,background-color] duration-(--dur-fast) ease-(--ease) outline-none",
  "placeholder:text-muted-foreground",
  "focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50",
  "disabled:cursor-not-allowed disabled:bg-input/50 disabled:opacity-50",
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
  /** One short helper line under the control. Hidden while `error` is shown (spec 00 §5). */
  hint?: ReactNode;
  /** Draws the spec's red `*` after the label. The control keeps its own `required` attribute. */
  required?: boolean;
  /** Error under the control, announced (`role="alert"`). Replaces `hint` while present. */
  error?: ReactNode;
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
  required = false,
  error,
  className,
  children,
}: FieldProps) {
  return (
    <div className={cn("flex flex-col gap-1.5", GROW_CLASS[grow], className)}>
      <label
        htmlFor={htmlFor}
        className={hideLabel ? "an-thi-giac" : "block text-[13px] leading-tight font-semibold text-navy"}
      >
        {label}
        {required && <span className="ml-1 text-danger">*</span>}
      </label>
      <div
        className={cn(
          "relative",
          "[&_input:not([type=checkbox]):not([type=radio])]:h-8 [&_select]:h-8",
          "[&_:is(input,select,textarea)]:w-full [&_:is(input,select,textarea)]:min-w-0",
          "[&_:is(input,select,textarea)]:rounded-lg [&_:is(input,select,textarea)]:border",
          "[&_:is(input,select,textarea)]:border-input [&_:is(input,select,textarea)]:bg-surface",
          "[&_:is(input,select,textarea)]:px-2.5 [&_:is(input,select,textarea)]:[font-family:inherit]",
          "[&_:is(input,select,textarea)]:text-base md:[&_:is(input,select,textarea)]:text-sm",
          "[&_:is(input,select,textarea)]:text-navy [&_:is(input,select,textarea)]:transition-[border-color,box-shadow,background-color]",
          "[&_:is(input,select,textarea):focus-visible]:border-ring [&_:is(input,select,textarea):focus-visible]:outline-none",
          "[&_:is(input,select,textarea):focus-visible]:ring-3 [&_:is(input,select,textarea):focus-visible]:ring-ring/50",
          "[&_:is(input,select,textarea):disabled]:cursor-not-allowed [&_:is(input,select,textarea):disabled]:bg-input/50",
          "[&_::placeholder]:text-muted-foreground",
          // SAME VARIANT AS `px-2.5` ON PURPOSE. Tailwind orders rules by variant first, property
          // second: written as `[&_:is(input,select)]:pl-8` it was emitted BEFORE the
          // `[&_:is(input,select,textarea)]:px-*` rule, which then reset the left padding and the
          // icon sat on top of the text (owner screenshot 02/10/2026). Within one variant
          // `padding-left`/`padding-right` sort after `padding-inline`, so these win.
          Icon !== undefined && "[&_:is(input,select,textarea)]:pl-8",
          kind === "select" && "[&_select]:cursor-pointer [&_select]:appearance-none [&_:is(input,select,textarea)]:pr-8",
          // Field draws its own ChevronDown icon below; the global select frame's background-image
          // chevron (`globals.css`) would be a second one.
          kind === "select" && "[&_select]:bg-none",
          // `search` used to be the OMICALL pill (lần 2); since lần 6 it is the same frame as every
          // other control — the prototype's search box is a plain shadcn Input with a leading icon.
        )}
      >
        {Icon !== undefined && (
          <Icon
            aria-hidden="true"
            focusable="false"
            strokeWidth={1.8}
            className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-ink-muted"
          />
        )}
        {children}
        {kind === "select" && (
          <ChevronDown
            aria-hidden="true"
            focusable="false"
            strokeWidth={1.8}
            className="pointer-events-none absolute top-1/2 right-2.5 size-4 -translate-y-1/2 text-ink-muted"
          />
        )}
      </div>
      {hint !== undefined && error === undefined && <p className="m-0 text-[12px] text-ink-muted">{hint}</p>}
      {error !== undefined && (
        <p role="alert" className="m-0 text-[12px] font-medium text-danger">
          {error}
        </p>
      )}
    </div>
  );
}
