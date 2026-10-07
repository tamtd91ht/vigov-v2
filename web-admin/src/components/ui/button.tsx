import { cva, type VariantProps } from "class-variance-authority";
import type { ComponentProps, ReactNode } from "react";

import { cn } from "@/lib/cn";

/**
 * Button — spec 00 §5 (shadcn `h-8 rounded-lg text-sm font-medium`, icon 16px). PRESENTATIONAL ONLY: it renders one native `<button>` and forwards every
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
    "inline-flex shrink-0 items-center justify-center gap-1.5 whitespace-nowrap select-none",
    "rounded-lg border border-solid border-transparent bg-clip-padding [font-family:inherit] text-sm font-medium leading-tight no-underline",
    "cursor-pointer transition-all duration-(--dur-fast) ease-(--ease) outline-none",
    "focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50",
    "active:not-disabled:translate-y-px",
    "disabled:pointer-events-none disabled:opacity-50 aria-busy:cursor-progress",
    "[&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
  ],
  {
    /*
     * shadcn/ui's current defaults (spec 00 §5, ADR 0068 lần 6). This app's variant NAMES are kept
     * (≈300 call sites); each maps to the prototype variant the call sites were ported from:
     *   primary    shadcn `default` — navy solid, white text, hover navy/80. ONE per region.
     *   secondary  shadcn `outline` — hairline border, page-colour fill, `muted` on hover. The
     *              prototype's `variant="outline"` was ported as `secondary` (ADR 0068 lần 5), so
     *              this is the prototype's usual non-primary button.
     *   outline    the same shadcn `outline` (it used to be the green outline of lần 2).
     *   dark       navy solid, as `primary` — the active segment / dark action.
     *   danger     shadcn `destructive` — red word on the 10% red tint, 20% on hover.
     *   ghost      shadcn `ghost` — no fill until hover.
     *   icon       shadcn `ghost` at the square icon sizes.
     * NO TEXT COLOUR IS INHERITED: Tailwind's preflight is off (globals.css), so a `<button>` would
     * otherwise draw in the browser's `buttontext` black — every variant names its colour.
     */
    variants: {
      variant: {
        primary: "bg-primary text-primary-foreground hover:not-disabled:bg-primary/80",
        outline: [
          "border-border bg-background text-foreground",
          "hover:not-disabled:bg-muted hover:not-disabled:text-foreground aria-expanded:bg-muted",
        ],
        dark: "bg-primary text-primary-foreground hover:not-disabled:bg-primary/80",
        secondary: [
          "border-border bg-background text-foreground",
          "hover:not-disabled:bg-muted hover:not-disabled:text-foreground aria-expanded:bg-muted",
        ],
        danger: [
          "bg-destructive/10 text-destructive hover:not-disabled:bg-destructive/20",
          "focus-visible:border-destructive/40 focus-visible:ring-destructive/20",
        ],
        ghost: "bg-transparent text-foreground hover:not-disabled:bg-muted hover:not-disabled:text-foreground aria-expanded:bg-muted",
        icon: "bg-transparent p-0 text-foreground hover:not-disabled:bg-muted hover:not-disabled:text-foreground aria-expanded:bg-muted",
      },
      size: {
        sm: "h-7 gap-1 rounded-md px-2.5 text-[0.8rem] [&_svg:not([class*='size-'])]:size-3.5",
        md: "h-8 px-2.5",
        lg: "h-9 px-2.5",
      },
    },
    compoundVariants: [
      { variant: "icon", size: "sm", className: "size-7 px-0" },
      { variant: "icon", size: "md", className: "size-8 px-0" },
      { variant: "icon", size: "lg", className: "size-9 px-0" },
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
