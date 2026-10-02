import { cn } from "@/lib/cn";

/**
 * Skeleton — spec v2 §7 / §8b "Đang tải lần đầu": light grey blocks in the SHAPE of the content
 * (table rows, KPI figures), so the layout does not jump when the data arrives. Never a page-wide
 * spinner.
 *
 * DECORATIVE ONLY: `aria-hidden`. The screen's own `role="status"` sentence (or `aria-busy`) is what
 * announces the load — a skeleton read aloud is a dozen empty items.
 *
 * Widths are FIXED by the caller or by the column pattern below, never random: `Math.random` is
 * banned here (rule 13), and a server render that differs from the client's breaks hydration.
 * The pulse stops under `prefers-reduced-motion` (`motion-safe:`).
 */
export function Skeleton({ className }: { className?: string }) {
  return <span aria-hidden="true" className={cn("block h-3 rounded bg-line motion-safe:animate-pulse", className)} />;
}

/** Column widths cycled across a row: a short code, a long title, a medium field, a badge. */
const ROW_PATTERN = ["w-14 shrink-0", "min-w-0 flex-1", "hidden w-28 shrink-0 sm:block", "h-[22px] w-24 shrink-0 rounded-full"];

export type SkeletonRowsProps = {
  /** How many placeholder rows. */
  rows?: number;
  /** Bars per row, taken from the fixed pattern above (1–4). */
  columns?: 1 | 2 | 3 | 4;
  className?: string;
};

/** Placeholder table/list rows, one `--row-h` (48px) each, hairlines between them. */
export function SkeletonRows({ rows = 5, columns = 4, className }: SkeletonRowsProps) {
  const pattern = ROW_PATTERN.slice(0, columns);
  return (
    <div aria-hidden="true" className={cn("skeleton-rows divide-y divide-line", className)}>
      {Array.from({ length: rows }, (_, i) => (
        <div key={i} className="flex h-[var(--row-h)] items-center gap-4 px-4">
          {pattern.map((w, j) => (
            <Skeleton key={j} className={w} />
          ))}
        </div>
      ))}
    </div>
  );
}
