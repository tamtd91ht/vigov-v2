import { ChevronLeft, ChevronRight } from "lucide-react";

import { cn } from "@/lib/cn";

import { Button } from "./button";

/**
 * Pagination — "‹ Trước / Sau ›", right-aligned in a card footer (spec v2 §8.1).
 *
 * CURSOR-BASED, AND THAT IS THE WHOLE SHAPE: the list contracts page by keyset and deliberately
 * return NO total count (ROADMAP_PHASE2 #7). So there are no page numbers and no "1–20 trên 235"
 * here — drawing one would mean inventing a figure the server never gave.
 *
 * `hasPrev` / `hasNext` come from the screen's own cursor state; `onPrev` / `onNext` are its
 * existing handlers. A side that cannot move is a DISABLED native button, still in the DOM and in
 * the reading order, so the control never jumps sideways between pages. `busy` disables both while
 * a page is loading, so a double click cannot fire two requests.
 */
export const PREV_LABEL = "Trước";
export const NEXT_LABEL = "Sau";

export type PaginationProps = {
  hasPrev: boolean;
  hasNext: boolean;
  onPrev: () => void;
  onNext: () => void;
  busy?: boolean;
  /** Accessible name of the `<nav>`. */
  label?: string;
  className?: string;
};

export function Pagination({ hasPrev, hasNext, onPrev, onNext, busy = false, label = "Phân trang", className }: PaginationProps) {
  return (
    <nav aria-label={label} className={cn("pagination ml-auto flex items-center gap-2", className)}>
      <Button
        type="button"
        variant="secondary"
        size="sm"
        onClick={onPrev}
        disabled={!hasPrev || busy}
        icon={<ChevronLeft aria-hidden="true" focusable="false" strokeWidth={1.8} />}
      >
        {PREV_LABEL}
      </Button>
      <Button type="button" variant="secondary" size="sm" onClick={onNext} disabled={!hasNext || busy}>
        {NEXT_LABEL}
        <ChevronRight aria-hidden="true" focusable="false" strokeWidth={1.8} />
      </Button>
    </nav>
  );
}
