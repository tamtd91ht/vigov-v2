import type { LucideIcon } from "lucide-react";

import { cn } from "@/lib/cn";

import { PendingMarker, type PendingFeatureInfo } from "./pending-feature";

/**
 * Segmented control — spec §7: a select with ≤ 4 options drawn as one row of segments. Look:
 * guide §7 (05/10/2026), the same as `Tab` — 36px segments on the navy 5% fill, the selected one
 * navy solid with white text and weight 500; disabled = opacity .5 (guide §9).
 *
 * PRESENTATIONAL ONLY, AND IT EMITS EXACTLY THE VALUES IT IS GIVEN. Each option's `value` is the
 * very string the screen's `<select>` used to emit, and `onChange(value)` is the screen's existing
 * setter. Nothing is translated, defaulted or reordered here — so the state and the query a screen
 * sends stay byte-for-byte what they were (spec §0.4).
 *
 * Two renderings:
 *   `radio`   (default) native `<input type="radio">` in a `<fieldset>` with a `<legend>`: arrow
 *             keys move between options, the group has a name, the selection is announced.
 *   `buttons` native `<button type="button" aria-pressed>` — for a screen whose current control is
 *             already a pair of toggle buttons, so its accessible roles stay the same.
 *
 * No hooks: safe under the mocked-React test runners (see `button.tsx`). An option with `pending`
 * adds a `PendingMarker` element, which does use hooks — fine as an element in the tree, but a
 * runner that renders the tree by calling every component must not be given such an option.
 */
export type SegmentedOption = {
  value: string;
  label: string;
  icon?: LucideIcon;
  /**
   * An unbuilt segment (ADR 0068 §14): drawn DISABLED at its spec position with a "?" beside it, and
   * never emitted — `onChange` cannot fire for it. Leave it unset for every real option.
   */
  pending?: PendingFeatureInfo;
  /** With `pending`: the description says the segment belongs to Phase 2. */
  phase2?: boolean;
};

export type SegmentedProps = {
  /** Group name: `<legend>` text in radio mode, `aria-label` of the group in buttons mode. */
  legend: string;
  /** Show the legend visually (12px/600 above, like a Field label). Hidden by default. */
  showLegend?: boolean;
  /** Radio `name` and the id prefix of each option. Must be unique on the page. */
  name: string;
  value: string;
  options: readonly SegmentedOption[];
  onChange: (value: string) => void;
  mode?: "radio" | "buttons";
  disabled?: boolean;
  className?: string;
};

const TRACK = "inline-flex max-w-full flex-wrap gap-1";

// The hover border is an INSET SHADOW, not a border: the radio-mode label and the button-mode
// button then keep one box size, and a 1px border appearing on hover shifts nothing.
const SEGMENT = cn(
  "inline-flex h-9 items-center gap-1.5 rounded-control bg-surface-subtle px-4 text-[15px] font-normal whitespace-nowrap text-ink-900",
  "cursor-pointer transition-[background-color,color,box-shadow] duration-(--dur-fast) ease-(--ease) hover:shadow-[inset_0_0_0_1px_var(--ink-900)]",
  "[&_svg]:size-5 [&_svg]:shrink-0",
);

const SEGMENT_ON = "bg-brand-600 font-medium text-white hover:shadow-none";

function optionId(name: string, value: string): string {
  // Values may hold characters an `id` should not ("", spaces); an index-free, readable id is
  // not needed — only uniqueness within the group.
  return `${name}--${encodeURIComponent(value === "" ? "_" : value)}`;
}

export function Segmented({
  legend,
  showLegend = false,
  name,
  value,
  options,
  onChange,
  mode = "radio",
  disabled = false,
  className,
}: SegmentedProps) {
  if (mode === "buttons") {
    return (
      <div className={cn("flex flex-col gap-1.5", className)}>
        {showLegend && <span className="text-xs leading-tight font-semibold text-ink-700">{legend}</span>}
        <div role="group" aria-label={legend} className={TRACK}>
          {options.map((o) => {
            const on = o.value === value;
            const Icon = o.icon;
            if (o.pending !== undefined) {
              return (
                <span key={o.value} className="relative inline-flex">
                  <button
                    type="button"
                    aria-pressed={false}
                    disabled
                    className={cn(
                      SEGMENT,
                      "cursor-not-allowed border-0 pr-8 [font-family:inherit] opacity-50 hover:shadow-none",
                    )}
                  >
                    {Icon !== undefined && <Icon aria-hidden="true" focusable="false" strokeWidth={1.8} />}
                    {o.label}
                  </button>
                  <PendingMarker info={o.pending} phase2={o.phase2} side="bottom" placement="end" />
                </span>
              );
            }
            return (
              <button
                key={o.value}
                type="button"
                aria-pressed={on}
                disabled={disabled}
                onClick={() => onChange(o.value)}
                className={cn(
                  SEGMENT,
                  "border-0 [font-family:inherit]",
                  on && SEGMENT_ON,
                  "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-500",
                  "disabled:cursor-not-allowed disabled:opacity-50",
                )}
              >
                {Icon !== undefined && <Icon aria-hidden="true" focusable="false" strokeWidth={1.8} />}
                {o.label}
              </button>
            );
          })}
        </div>
      </div>
    );
  }

  return (
    <fieldset className={cn("m-0 flex min-w-0 flex-col gap-1.5 border-0 p-0", className)} disabled={disabled}>
      <legend className={showLegend ? "mb-1.5 p-0 text-xs leading-tight font-semibold text-ink-700" : "an-thi-giac"}>
        {legend}
      </legend>
      <div className={TRACK}>
        {options.map((o) => {
          const on = o.value === value;
          const id = optionId(name, o.value);
          const Icon = o.icon;
          if (o.pending !== undefined) {
            // Disabled and UNCONTROLLED (no `checked`, no `onChange`): it can never be selected, so it
            // can never put a value into the screen's state.
            return (
              <span key={o.value} className="relative inline-flex">
                <input
                  id={id}
                  type="radio"
                  name={name}
                  value={o.value}
                  disabled
                  className="peer absolute inset-0 m-0 appearance-none rounded-control opacity-0"
                />
                <label
                  htmlFor={id}
                  className={cn(SEGMENT, "pointer-events-none cursor-not-allowed pr-8 opacity-50 hover:shadow-none")}
                >
                  {Icon !== undefined && <Icon aria-hidden="true" focusable="false" strokeWidth={1.8} />}
                  {o.label}
                </label>
                <PendingMarker info={o.pending} phase2={o.phase2} side="bottom" placement="end" />
              </span>
            );
          }
          return (
            <span key={o.value} className="relative inline-flex">
              <input
                id={id}
                type="radio"
                name={name}
                value={o.value}
                checked={on}
                onChange={(e) => onChange(e.target.value)}
                className="peer absolute inset-0 m-0 cursor-pointer appearance-none rounded-control opacity-0"
              />
              <label
                htmlFor={id}
                className={cn(
                  SEGMENT,
                  on && SEGMENT_ON,
                  !on && "peer-hover:shadow-[inset_0_0_0_1px_var(--ink-900)]",
                  "pointer-events-none peer-focus-visible:outline-2 peer-focus-visible:outline-offset-2 peer-focus-visible:outline-brand-500",
                  "peer-disabled:cursor-not-allowed peer-disabled:opacity-50",
                )}
              >
                {Icon !== undefined && <Icon aria-hidden="true" focusable="false" strokeWidth={1.8} />}
                {o.label}
              </label>
            </span>
          );
        })}
      </div>
    </fieldset>
  );
}
