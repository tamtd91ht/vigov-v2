import { cn } from "@/lib/cn";

/**
 * Skeleton — copied from `web-admin/src/components/ui/skeleton.tsx` (change both): grey blocks in
 * the SHAPE of the content, so the layout does not jump when the data arrives.
 *
 * DECORATIVE ONLY (`aria-hidden`): the screen's own `role="status"` sentence announces the load.
 * Widths are FIXED, never random — `Math.random` is banned (rule 13) and would break hydration.
 */
export function Skeleton({ className }: { className?: string }) {
  return <span aria-hidden="true" className={cn("block h-3 rounded bg-line motion-safe:animate-pulse", className)} />;
}

/** Column widths cycled across a row: a long name, a medium field, a badge, a domain. */
const ROW_PATTERN = [
  "min-w-0 flex-1",
  "hidden w-28 shrink-0 sm:block",
  "h-[22px] w-24 shrink-0 rounded-full",
  "hidden w-36 shrink-0 md:block",
];

export type SkeletonRowsProps = {
  rows?: number;
  columns?: 1 | 2 | 3 | 4;
  className?: string;
};

/** Placeholder table/list rows, one `--row-h` (48px) each, hairlines between them. */
export function SkeletonRows({ rows = 5, columns = 4, className }: SkeletonRowsProps) {
  const pattern = ROW_PATTERN.slice(0, columns);
  return (
    <div aria-hidden="true" className={cn("divide-y divide-line", className)}>
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
