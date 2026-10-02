import type { ComponentProps, ElementType, ReactNode } from "react";

import { cn } from "@/lib/cn";

/**
 * Card — copied from `web-admin/src/components/ui/card.tsx` (change both): white, `--line`
 * hairline, 16px radius, `--shadow-sm`. All parts share one 16px horizontal padding.
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
      className={cn("min-w-0 overflow-hidden rounded-card border border-line bg-surface shadow-sm", className)}
      {...props}
    >
      {children}
    </Element>
  );
}

export function CardHeader({ className, ...props }: ComponentProps<"div">) {
  return <div className={cn("flex flex-wrap items-center gap-3 border-b border-line px-4 py-3.5", className)} {...props} />;
}

/** Card title, 15px/600. An `<h2>` by default; pass `as` to match the page outline. */
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

/** Footer: actions. */
export function CardFooter({ className, ...props }: ComponentProps<"div">) {
  return (
    <div
      className={cn("flex flex-wrap items-center gap-3 border-t border-line px-4 py-3 text-[13px] text-ink-500", className)}
      {...props}
    />
  );
}
