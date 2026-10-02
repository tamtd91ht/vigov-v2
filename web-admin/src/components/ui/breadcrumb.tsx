import { ChevronRight } from "lucide-react";
import Link from "next/link";
import type { ReactNode } from "react";

import { cn } from "@/lib/cn";

/**
 * Breadcrumb — spec v2 §5 app shell, above the page header: where am I.
 *
 * The trail is the screen's own: it passes the labels and the routes that ALREADY exist. An item
 * with no `href` is plain text (the current page, or a level with no screen of its own) — never a
 * link to a route that does not exist. The last item is the current page (`aria-current="page"`).
 */
export type BreadcrumbItem = { label: ReactNode; href?: string };

export function Breadcrumb({ items, className }: { items: readonly BreadcrumbItem[]; className?: string }) {
  return (
    <nav aria-label="Đường dẫn trang" className={cn("breadcrumb mb-2 text-xs text-ink-500", className)}>
      <ol className="m-0 flex list-none flex-wrap items-center gap-1 p-0">
        {items.map((item, i) => {
          const last = i === items.length - 1;
          return (
            <li key={i} className="inline-flex min-w-0 items-center gap-1">
              {i > 0 && <ChevronRight aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-3.5 shrink-0 text-ink-400" />}
              {item.href !== undefined && !last ? (
                <Link href={item.href} className="text-ink-500 no-underline hover:text-brand-700 hover:underline">
                  {item.label}
                </Link>
              ) : (
                <span aria-current={last ? "page" : undefined} className={last ? "font-medium text-ink-700" : undefined}>
                  {item.label}
                </span>
              )}
            </li>
          );
        })}
      </ol>
    </nav>
  );
}
