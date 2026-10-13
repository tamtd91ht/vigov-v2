import { cva, type VariantProps } from "class-variance-authority";
import type { ComponentProps, ReactNode } from "react";

import { cn } from "@/lib/cn";

/**
 * Button — ported from `web-admin/src/components/ui/button.tsx` (ADR 0068 lần 6, spec 00 §5: shadcn
 * `h-8 rounded-lg text-sm font-medium`, icon 16px; no shared package yet, change both). PRESENTATIONAL
 * ONLY: one native `<button>`, every prop forwarded untouched.
 *
 * `type` HAS NO DEFAULT, on purpose. A native `<button>` inside a `<form>` defaults to `submit`;
 * a wrapper that defaulted to `"button"` would silently stop a form submitting. Pass exactly the
 * `type` the original element had.
 *
 * DIFFERENCES FROM WEB-ADMIN: no legacy class (`nut-chinh`, `nut-phu`) is emitted — this console
 * never had those, and nothing keys on them here. Two variants added for flows web-admin draws
 * differently: `danger-solid` (the confirm button of a destructive action, which this console
 * already drew filled red) and `link` (the inline text actions of the sign-in steps, in the spec's
 * link colour `brand`, as web-admin writes its inline links). web-admin's `outline` / `dark` / `icon`
 * are not ported: no screen here asks for them.
 *
 * NO TEXT COLOUR IS INHERITED: preflight is off (`globals.css`), so a `<button>` would otherwise draw
 * in the browser's `buttontext` black — every variant names its colour.
 *
 * CSP: classes only, never a `style` attribute (`src/lib/csp.ts`).
 */
export const buttonVariants = cva(
  [
    "inline-flex shrink-0 items-center justify-center gap-1.5 whitespace-nowrap select-none",
    "rounded-lg border border-solid border-transparent bg-clip-padding [font-family:inherit] text-sm font-medium leading-tight no-underline",
    "cursor-pointer transition-all duration-(--dur-fast) ease-(--ease) outline-none",
    "focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50",
    "active:not-disabled:translate-y-px",
    "disabled:pointer-events-none disabled:opacity-50 aria-busy:cursor-progress",
    "[&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
  ],
  {
    variants: {
      variant: {
        // shadcn `default`: navy solid, white text, hover navy/80. ONE per region.
        primary: "bg-primary text-primary-foreground hover:not-disabled:bg-primary/80",
        // shadcn `outline`: hairline border, page-colour fill, `muted` on hover.
        secondary: [
          "border-border bg-background text-foreground",
          "hover:not-disabled:bg-muted hover:not-disabled:text-foreground aria-expanded:bg-muted",
        ],
        // shadcn `destructive`: red word on the 10% red tint, 20% on hover.
        danger: [
          "bg-destructive/10 text-destructive hover:not-disabled:bg-destructive/20",
          "focus-visible:border-destructive/40 focus-visible:ring-destructive/20",
        ],
        "danger-solid": [
          "bg-danger-solid text-white hover:not-disabled:bg-danger-solid/80",
          "focus-visible:border-destructive/40 focus-visible:ring-destructive/20",
        ],
        ghost: "bg-transparent text-foreground hover:not-disabled:bg-muted hover:not-disabled:text-foreground aria-expanded:bg-muted",
        link: "bg-transparent text-brand underline underline-offset-2 hover:not-disabled:text-brand-600",
      },
      size: {
        sm: "h-7 gap-1 rounded-md px-2.5 text-[0.8rem] [&_svg:not([class*='size-'])]:size-3.5",
        md: "h-8 px-2.5",
        lg: "h-9 px-2.5",
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
