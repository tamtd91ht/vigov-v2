import type { ComponentProps } from "react";

import { cn } from "@/lib/cn";

/**
 * Switch — shadcn's Switch look (the prototype's `components/ui/switch.tsx`, used by Tự động hoá and
 * Kênh Zalo): an 18.4×32px pill, navy (`primary`) when on, the `input` hairline colour when off, a
 * 16px page-colour thumb. Small size 14×24 with a 12px thumb.
 *
 * A NATIVE `<button role="switch" aria-checked>`, not Radix: the app ships no Radix switch, and the
 * ARIA switch pattern is exactly a toggle button — Space and Enter press it, the state is
 * `aria-checked`. CONTROLLED ONLY, no hooks: the caller owns `checked` and answers
 * `onCheckedChange(next)`, so a write the server refuses never leaves the switch showing a state that
 * was not saved.
 *
 * The invisible `after:` box widens the hit area past the 18px pill, as the prototype does.
 */
export type SwitchProps = Omit<ComponentProps<"button">, "type" | "role" | "aria-checked" | "onChange"> & {
  checked: boolean;
  onCheckedChange?: (checked: boolean) => void;
  size?: "sm" | "default";
};

export function Switch({ checked, onCheckedChange, size = "default", className, onClick, ...props }: SwitchProps) {
  return (
    <button
      {...props}
      type="button"
      role="switch"
      aria-checked={checked}
      data-state={checked ? "checked" : "unchecked"}
      data-size={size}
      onClick={(e) => {
        onClick?.(e);
        if (!e.defaultPrevented) onCheckedChange?.(!checked);
      }}
      className={cn(
        "peer relative inline-flex shrink-0 cursor-pointer items-center rounded-full border border-solid border-transparent p-0 transition-all outline-none",
        "after:absolute after:-inset-x-3 after:-inset-y-2",
        "focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50",
        "disabled:cursor-not-allowed disabled:opacity-50",
        size === "sm" ? "h-[14px] w-[24px]" : "h-[18.4px] w-[32px]",
        checked ? "bg-primary" : "bg-input",
        className,
      )}
    >
      <span
        aria-hidden="true"
        className={cn(
          "bg-background pointer-events-none block rounded-full ring-0 transition-transform",
          size === "sm" ? "size-3" : "size-4",
          checked ? "translate-x-[calc(100%-2px)]" : "translate-x-0",
        )}
      />
    </button>
  );
}
