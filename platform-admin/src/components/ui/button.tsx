import { cva, type VariantProps } from "class-variance-authority";
import type { ComponentProps, ReactNode } from "react";

import { cn } from "@/lib/cn";

/**
 * Button — copied from `web-admin/src/components/ui/button.tsx` (ADR 0068; no shared package yet,
 * change both). PRESENTATIONAL ONLY: one native `<button>`, every prop forwarded untouched.
 *
 * `type` HAS NO DEFAULT, on purpose. A native `<button>` inside a `<form>` defaults to `submit`;
 * a wrapper that defaulted to `"button"` would silently stop a form submitting. Pass exactly the
 * `type` the original element had.
 *
 * DIFFERENCES FROM WEB-ADMIN: no legacy class (`nut-chinh`, `nut-phu`) is emitted — this console
 * never had those, and nothing keys on them here. Two variants added for flows web-admin draws
 * differently: `danger-solid` (the confirm button of a destructive action, which this console
 * already drew filled red) and `link` (the inline text actions of the sign-in steps).
 *
 * CSP: classes only, never a `style` attribute (`src/lib/csp.ts`).
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
          "border-brand-600 bg-brand-600 text-white",
          "hover:not-disabled:border-brand-700 hover:not-disabled:bg-brand-700",
        ],
        secondary: [
          "border-line-strong bg-surface text-ink-700",
          "hover:not-disabled:border-brand-100 hover:not-disabled:bg-brand-50 hover:not-disabled:text-brand-700",
        ],
        danger: [
          "border-danger-200 bg-surface text-danger-600",
          "hover:not-disabled:border-danger-600 hover:not-disabled:bg-danger-50",
        ],
        "danger-solid": [
          "border-danger-600 bg-danger-600 text-white",
          "hover:not-disabled:border-[#a32020] hover:not-disabled:bg-[#a32020]",
        ],
        ghost: [
          "border-transparent bg-transparent text-ink-700",
          "hover:not-disabled:bg-brand-50 hover:not-disabled:text-brand-700",
        ],
        link: [
          "border-transparent bg-transparent text-brand-700 underline underline-offset-2",
          "hover:not-disabled:text-brand-600",
        ],
      },
      size: {
        sm: "h-[34px] px-3 text-[13px]",
        md: "h-10 px-4",
        lg: "h-[46px] px-5 text-[15px]",
      },
    },
    compoundVariants: [{ variant: "link", className: "px-0" }],
    defaultVariants: { variant: "secondary", size: "md" },
  },
);

export type ButtonVariant = NonNullable<VariantProps<typeof buttonVariants>["variant"]>;
export type ButtonSize = NonNullable<VariantProps<typeof buttonVariants>["size"]>;

export type ButtonProps = ComponentProps<"button"> & {
  variant?: ButtonVariant;
  size?: ButtonSize;
  /** Leading icon, e.g. `<Plus aria-hidden="true" />`. Decorative: the label carries the meaning. */
  icon?: ReactNode;
};

export function Button({ variant = "secondary", size = "md", icon, className, children, ...props }: ButtonProps) {
  return (
    <button className={cn(buttonVariants({ variant, size }), className)} {...props}>
      {icon}
      {children}
    </button>
  );
}
