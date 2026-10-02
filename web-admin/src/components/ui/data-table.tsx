import type { ComponentProps } from "react";

import { cn } from "@/lib/cn";

/**
 * Table helpers — spec v2 §6.7 / §8.1: ONLY the table scrolls (never the page), header row sticky
 * while scrolling, body rows `--row-h` (48px).
 *
 * CLASS NAMES, NOT A TABLE COMPONENT: every screen keeps its own `<table>` with its own
 * `<caption class="an-thi-giac">`, columns and legacy hook class (`bang-can-bo`…), because those
 * are what its tests and screen readers rely on. It only adds `DATA_TABLE_CLASS` and wraps the
 * table in `TableScroll`.
 *
 * `TableScroll` is a focusable labelled region (`tabIndex=0`, `role="region"`, `aria-label`
 * required): a scroller a keyboard user cannot reach is a column they cannot read — the pattern
 * `danh-ba-can-bo.tsx` already uses. `sticky` caps its height so the header row can stick INSIDE
 * it; without a height the page scrolls instead and a sticky header has nothing to stick to.
 */
export const DATA_TABLE_CLASS = "data-table";

export type TableScrollProps = Omit<ComponentProps<"div">, "role" | "tabIndex"> & {
  "aria-label": string;
  sticky?: boolean;
};

export function TableScroll({ sticky = false, className, ...props }: TableScrollProps) {
  return (
    <div
      role="region"
      tabIndex={0}
      className={cn("bang-cuon table-scroll", sticky && "table-scroll--sticky", className)}
      {...props}
    />
  );
}
