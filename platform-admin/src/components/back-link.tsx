import { ChevronLeft } from "lucide-react";
import Link from "next/link";
import type { ReactNode } from "react";

/**
 * The way back to the list above a page header — the old `.breadcrumb` paragraph, same link, same
 * words, drawn as a small icon link. A `<p>` so the page outline is unchanged.
 */
export function BackLink({ href, children }: { href: string; children: ReactNode }) {
  return (
    <p className="m-0 mb-3 text-[13px]">
      <Link
        href={href}
        className="inline-flex min-h-8 items-center gap-1 font-medium text-brand-700 no-underline hover:underline focus-visible:underline"
      >
        <ChevronLeft aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-4 shrink-0" />
        {children}
      </Link>
    </p>
  );
}
