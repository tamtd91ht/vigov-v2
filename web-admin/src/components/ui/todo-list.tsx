import { ChevronRight, type LucideIcon } from "lucide-react";
import Link from "next/link";
import type { ReactNode } from "react";

import { cn } from "@/lib/cn";

/**
 * "Việc cần xử lý" list — spec §8.2: one row per kind of pending work, each row = status icon +
 * title + one detail line + a BIG number + `ChevronRight`, the whole row a link to the list behind
 * the number.
 *
 * PRESENTATIONAL ONLY, NO HOOKS. The caller decides every word, every number and whether a row
 * links anywhere: a row with no `href` renders without the chevron, because a chevron that leads
 * nowhere promises a screen that does not exist (ADR 0068 §1).
 *
 * THE NUMBER CARRIES ITS OWN ID. `value` is the caller's node — typically a `<span id>` that the
 * link's `aria-describedby` points at — so a link whose accessible name replaces its text still
 * lets a screen-reader user hear the number.
 *
 * `tone` colours the STATUS ICON only, and only for a real state (`danger` = something late);
 * `alert` paints the number red and is for the same single case as `StatCard` — late and above 0.
 */
export type TodoTone = "danger" | "warning" | "brand" | "neutral";

const TONE_TILE: Record<TodoTone, string> = {
  danger: "bg-danger-50 text-danger-600",
  warning: "bg-warning-50 text-warning-600",
  brand: "bg-brand-50 text-brand-600",
  neutral: "bg-[#f1f4f8] text-ink-500",
};

export type TodoItem = {
  key: string;
  icon: LucideIcon;
  tone?: TodoTone;
  title: ReactNode;
  detail?: ReactNode;
  value: ReactNode;
  alert?: boolean;
  /** The list behind the number. Omit when no such route exists. */
  href?: string;
  /** Accessible name of the row link (replaces the visible text). */
  linkLabel?: string;
  /** `id` of the element holding the number, for `aria-describedby`. */
  describedBy?: string;
};

export type TodoListProps = {
  items: readonly TodoItem[];
  className?: string;
};

function Row({ item }: { item: TodoItem }) {
  const Icon = item.icon;
  return (
    <>
      <span
        aria-hidden="true"
        className={cn("grid size-10 shrink-0 place-items-center rounded-xl", TONE_TILE[item.tone ?? "neutral"])}
      >
        <Icon className="size-5" strokeWidth={1.8} focusable="false" />
      </span>
      <span className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="text-[15px] leading-snug font-semibold text-ink-900">{item.title}</span>
        {item.detail !== undefined && (
          <span className="text-[13px] leading-snug text-ink-500">{item.detail}</span>
        )}
      </span>
      <span
        className={cn(
          "shrink-0 text-2xl leading-none font-bold tabular-nums",
          item.alert === true ? "text-danger-600" : "text-ink-900",
        )}
      >
        {item.value}
      </span>
      {item.href !== undefined && (
        <ChevronRight aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-5 shrink-0 text-ink-400" />
      )}
    </>
  );
}

const ROW = "flex min-h-[72px] items-center gap-3 px-4 py-3";

export function TodoList({ items, className }: TodoListProps) {
  return (
    <ul className={cn("m-0 list-none divide-y divide-line p-0", className)}>
      {items.map((item) => (
        <li key={item.key} className="min-w-0">
          {item.href !== undefined ? (
            <Link
              href={item.href}
              aria-label={item.linkLabel}
              aria-describedby={item.describedBy}
              className={cn(
                ROW,
                "text-inherit no-underline transition-colors duration-150 ease-out hover:bg-brand-50/60",
                "focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-brand-500",
              )}
            >
              <Row item={item} />
            </Link>
          ) : (
            <div className={ROW}>
              <Row item={item} />
            </div>
          )}
        </li>
      ))}
    </ul>
  );
}
