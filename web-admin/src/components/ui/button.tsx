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
    "rounded-control border [font-family:inherit] text-sm font-semibold leading-tight no-underline",
    "cursor-pointer transition-[background-color,border-color,color,box-shadow,transform] duration-150 ease-out",
    "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-500",
    "active:not-disabled:translate-y-px",
    "disabled:cursor-not-allowed disabled:opacity-60 aria-busy:cursor-progress",
    "[&_svg]:size-[18px] [&_svg]:shrink-0",
  ],
  {
    variants: {
      variant: {
        primary: [
          "border-brand-600 bg-brand-600 text-white shadow-sm",
          "hover:not-disabled:border-brand-700 hover:not-disabled:bg-brand-700 hover:not-disabled:shadow-md",
        ],
        secondary: [
          "border-line-strong bg-surface text-ink-700",
          "hover:not-disabled:border-brand-100 hover:not-disabled:bg-brand-50 hover:not-disabled:text-brand-700",
        ],
        danger: [
          "border-danger-200 bg-surface text-danger-600",
          "hover:not-disabled:border-danger-600 hover:not-disabled:bg-danger-50",
        ],
        ghost: [
          "border-transparent bg-transparent text-ink-700",
          "hover:not-disabled:bg-brand-50 hover:not-disabled:text-brand-700",
        ],
        icon: [
          "border-transparent bg-transparent p-0 text-ink-500",
          "hover:not-disabled:bg-brand-50 hover:not-disabled:text-brand-700",
        ],
      },
      size: {
        sm: "h-[34px] px-3 text-[13px]",
        md: "h-10 px-4",
        lg: "h-[46px] px-5 text-[15px]",
      },
    },
    compoundVariants: [
      { variant: "icon", size: "sm", className: "size-[34px] px-0" },
      { variant: "icon", size: "md", className: "size-10 px-0" },
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
