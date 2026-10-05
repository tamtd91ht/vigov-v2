import { cva, type VariantProps } from "class-variance-authority";
import type { ComponentProps, ReactNode } from "react";

import { cn } from "@/lib/cn";

/**
 * Button — spec §7. PRESENTATIONAL ONLY: it renders one native `<button>` and forwards every
 * prop (`type`, `onClick`, `disabled`, `aria-*`, `form`, `name`, `value`) untouched.
 *
 * `type` HAS NO DEFAULT, on purpose. A native `<button>` inside a `<form>` defaults to `submit`;
 * a wrapper that defaulted to `"button"` would silently stop a form submitting the day a screen
 * swaps its `<button>` for this one. Pass exactly the `type` the original element had.
 *
 * THE LEGACY CLASS IS EMITTED TOO (`nut-chinh`, `nut-phu`, `nut-xoa`): tests and contextual CSS
 * (`.o-thao-tac .nut-phu { min-height: 2.75rem }`, the 44px targets for older staff) key on those
 * names. Utilities sit in a later cascade layer than the legacy rules, so the new look wins where
 * both set the same property, while a legacy `min-height` the utilities do not set still applies.
 *
 * NO HOOKS, NO `forwardRef`: some tests call components as plain functions under a mocked React
 * that only provides useState/useEffect/useCallback/useMemo (`danh-ba-lien-he.luong.test.tsx`).
 * A plain function component is safe there; React 19 passes `ref` as an ordinary prop.
 */
export const buttonVariants = cva(
  [
    "inline-flex items-center justify-center gap-2 whitespace-nowrap select-none",
    "rounded-control border [font-family:inherit] text-[15px] font-normal leading-tight no-underline",
    "cursor-pointer transition-[background-color,border-color,color,box-shadow,transform] duration-(--dur-fast) ease-(--ease)",
    "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-500",
    "active:not-disabled:translate-y-px",
    "disabled:cursor-not-allowed disabled:opacity-50 aria-busy:cursor-progress",
    "[&_svg]:size-5 [&_svg]:shrink-0",
  ],
  {
    /*
     * OMICALL guide §8.1 / §9 (05/10/2026):
     *   primary    the main call to action — solid green, white text 4.70:1. ONE per region.
     *   outline    its quieter sibling (guide "Tải lên"): green word (5.87:1) on a soft green
     *              border; hover fills solid.
     *   dark       navy solid (white 13.04:1) — the active tab / segment look, a dark action.
     *   secondary  navy 5% fill, navy text; hover keeps the fill and adds a 1px navy border.
     *   danger     destructive: red word on the red tint (5.01:1); hover adds the red border.
     * Weight 400 (guide §3); hierarchy comes from the fill, never from a bigger size.
     */
    variants: {
      variant: {
        primary: [
          "border-cta bg-cta text-white",
          "hover:not-disabled:border-cta-hover hover:not-disabled:bg-cta-hover",
        ],
        outline: [
          "border-cta-outline bg-transparent text-cta-text",
          "hover:not-disabled:border-cta-hover hover:not-disabled:bg-cta-hover hover:not-disabled:text-white",
        ],
        dark: [
          "border-brand-600 bg-brand-600 text-white",
          "hover:not-disabled:border-brand-700 hover:not-disabled:bg-brand-700",
        ],
        secondary: [
          "border-transparent bg-surface-subtle text-ink-900",
          "hover:not-disabled:border-ink-900",
        ],
        danger: [
          "border-transparent bg-danger-50 text-danger-600",
          "hover:not-disabled:border-danger-600",
        ],
        ghost: [
          "border-transparent bg-transparent text-ink-700",
          "hover:not-disabled:bg-surface-subtle hover:not-disabled:text-ink-900",
        ],
        icon: [
          "rounded-popover border-transparent bg-transparent p-0 text-ink-500",
          "hover:not-disabled:bg-surface-subtle hover:not-disabled:text-ink-900",
        ],
      },
      size: {
        sm: "h-[34px] px-3 text-[13px]",
        md: "h-9 px-4",
        lg: "h-[46px] px-5",
      },
    },
    compoundVariants: [
      { variant: "icon", size: "sm", className: "size-[34px] px-0" },
      { variant: "icon", size: "md", className: "size-9 px-0" },
      { variant: "icon", size: "lg", className: "size-[46px] px-0" },
    ],
    defaultVariants: { variant: "secondary", size: "md" },
  },
);

export type ButtonVariant = NonNullable<VariantProps<typeof buttonVariants>["variant"]>;
export type ButtonSize = NonNullable<VariantProps<typeof buttonVariants>["size"]>;

/** Legacy class each variant carries — see the block comment above for why. */
export const LEGACY_BUTTON_CLASS: Record<ButtonVariant, string> = {
  primary: "nut-chinh",
  outline: "nut-phu",
  dark: "nut-phu",
  secondary: "nut-phu",
  danger: "nut-phu nut-xoa",
  ghost: "nut-phu",
  icon: "nut-phu",
};

export type ButtonProps = ComponentProps<"button"> & {
  variant?: ButtonVariant;
  size?: ButtonSize;
  /** Leading icon, e.g. `<Plus aria-hidden />`. Decorative: the label carries the meaning. */
  icon?: ReactNode;
};

export function Button({
  variant = "secondary",
  size = "md",
  icon,
  className,
  children,
  ...props
}: ButtonProps) {
  return (
    <button className={cn(LEGACY_BUTTON_CLASS[variant], buttonVariants({ variant, size }), className)} {...props}>
      {icon}
      {children}
    </button>
  );
}
