import type { ComponentProps, ElementType, ReactNode } from "react";

import { cn } from "@/lib/cn";

/**
 * Card — copied from `web-admin/src/components/ui/card.tsx` (change both): the spec's section block
 * (spec 00 §4) — white, `line` hairline, `shadow-card`, `rounded-card` (12px). All parts share one
 * 16px horizontal padding. `data-slot="card"` is shadcn's marker: `globals.css` reads it so a table
 * scroller inside a card drops its own frame, and the card is not drawn twice.
 *
 * `as` picks the element (`section`, `form`…) so wrapping existing markup in a card never changes
 * its semantics: an existing `<form method="post" onSubmit>` stays the form, it just gains the frame.
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
      data-slot="card"
      className={cn("min-w-0 overflow-hidden rounded-card border border-solid border-line bg-surface shadow-card", className)}
      {...props}
    >
      {children}
    </Element>
  );
}

export function CardHeader({ className, ...props }: ComponentProps<"div">) {
  return <div className={cn("flex flex-wrap items-center gap-3 border-b border-line px-4 py-3.5", className)} {...props} />;
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

/** Footer: actions. */
export function CardFooter({ className, ...props }: ComponentProps<"div">) {
  return (
    <div
      className={cn("flex flex-wrap items-center gap-3 border-t border-line px-4 py-3 text-[13px] text-ink-muted", className)}
      {...props}
    />
  );
}
