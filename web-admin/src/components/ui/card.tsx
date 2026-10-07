import type { ComponentProps, ElementType, ReactNode } from "react";

import { cn } from "@/lib/cn";

/**
 * Card — the spec's section block (spec 00 §4, ADR 0068 lần 6): white, `line` hairline, `shadow-card`,
 * `rounded-card` (12px). All four parts share one horizontal padding (16px) so a page title, the card
 * edge and the card's content line up on the same left edge (spec v2 §6.8). The padding sits on the
 * parts, not on the frame, so a header's rule can run edge to edge; a card with no parts takes
 * `p-4` from its caller, exactly as the spec's `<section … p-4>`.
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
      // shadcn's own marker. `globals.css` reads it: a table scroller (`.bang-cuon`) inside a card drops
      // its frame, so the card is not drawn twice.
      data-slot="card"
      className={cn("min-w-0 overflow-hidden rounded-card border border-solid border-line bg-surface shadow-card", className)}
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

/** Card title, the spec's section title: navy 13px/700 (spec 00 §3). An `<h2>` by default; pass `as`
 * to match the page outline. */
export function CardTitle({
  as: Heading = "h2",
  className,
  ...props
}: ComponentProps<"h2"> & { as?: "h2" | "h3" | "h4" }) {
  return <Heading className={cn("m-0 text-[13px] leading-snug font-bold text-navy", className)} {...props} />;
}

export function CardContent({ className, ...props }: ComponentProps<"div">) {
  return <div className={cn("p-4", className)} {...props} />;
}

/** Footer: pagination and summary text, right-aligned actions (spec §8.1). */
export function CardFooter({ className, ...props }: ComponentProps<"div">) {
  return (
    <div
      className={cn("flex flex-wrap items-center gap-3 border-t border-line px-4 py-3 text-[13px] text-ink-muted", className)}
      {...props}
    />
  );
}
