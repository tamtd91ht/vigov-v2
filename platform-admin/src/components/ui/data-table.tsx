import type { ComponentProps } from "react";

import { cn } from "@/lib/cn";

/**
 * Table helpers — copied from `web-admin/src/components/ui/data-table.tsx` (change both). ONLY the
 * table scrolls (never the page), header row sticky, body rows 48px. The CSS lives in
 * `globals.css`, layer `components`.
 *
 * CLASS NAMES, NOT A TABLE COMPONENT: each screen keeps its own `<table>`, `<caption>` and columns.
 * `TableScroll` is a focusable labelled region: a scroller a keyboard user cannot reach is a column
 * they cannot read.
 */
export const DATA_TABLE_CLASS = "data-table";

export type TableScrollProps = Omit<ComponentProps<"div">, "role" | "tabIndex"> & {
  "aria-label": string;
  sticky?: boolean;
};

export function TableScroll({ sticky = false, className, ...props }: TableScrollProps) {
  return <div role="region" tabIndex={0} className={cn("table-scroll", sticky && "table-scroll--sticky", className)} {...props} />;
}
