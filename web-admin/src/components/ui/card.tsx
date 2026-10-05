import type { ComponentProps, ElementType, ReactNode } from "react";

import { cn } from "@/lib/cn";

/**
 * Card — guide §5 / §8.7: white, 16px radius, NO border and NO shadow — on `--bg` the white alone
 * separates it (tint separation, guide §1). All four parts share one
 * horizontal padding (16px) so a page title, the card edge and the card's content line up on the
 * same left edge (spec §6.8).
 *
 * `as` picks the element (`section`, `aside`, `form`…) so wrapping existing markup in a card never
 * changes its semantics: an existing `<form onSubmit>` stays the form, it just gains the frame.
 */
type CardElement = "div" | "section" | "article" | "aside" | "form";

export type CardProps<E extends CardElement = "div"> = { as?: E; className?: string; children?: ReactNode } & Omit<
  ComponentProps<E>,
  "className" | "children"
>;

export function Card<E extends CardElement = "div">({ as, className, children, ...props }: CardProps<E>) {
  const Element = (as ?? "div") as ElementType;
  return (
    <Element
      className={cn("min-w-0 overflow-hidden rounded-card bg-surface", className)}
      {...props}
    >
      {children}
    </Element>
  );
}

export function CardHeader({ className, ...props }: ComponentProps<"div">) {
  return (
    <div
      className={cn("flex flex-wrap items-center gap-3 border-b border-line px-4 py-3.5", className)}
      {...props}
    />
  );
}

/** Card title, 15px/600 (spec §3). An `<h2>` by default; pass `as` to match the page outline. */
export function CardTitle({
  as: Heading = "h2",
  className,
  ...props
}: ComponentProps<"h2"> & { as?: "h2" | "h3" | "h4" }) {
  return <Heading className={cn("m-0 text-[15px] leading-snug font-semibold text-ink-900", className)} {...props} />;
}

export function CardContent({ className, ...props }: ComponentProps<"div">) {
  return <div className={cn("p-4", className)} {...props} />;
}

/** Footer: pagination and summary text, right-aligned actions (spec §8.1). */
export function CardFooter({ className, ...props }: ComponentProps<"div">) {
  return (
    <div
      className={cn("flex flex-wrap items-center gap-3 border-t border-line px-4 py-3 text-[13px] text-ink-500", className)}
      {...props}
    />
  );
}
