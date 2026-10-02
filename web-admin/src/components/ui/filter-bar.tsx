"use client";

import { ChevronDown, ChevronUp, SlidersHorizontal } from "lucide-react";
import { useState, type ReactNode } from "react";

import { cn } from "@/lib/cn";

import { Button } from "./button";

/**
 * FilterBar — one filter row with the decisive controls, everything else behind one "Bộ lọc"
 * button (owner feedback 02/10/2026: "chỉ show các chức năng chính, còn lại ẩn vào 1 nút mở rộng";
 * "ô tìm kiếm nên nằm ở bên trái").
 *
 * Layout (CSS in `globals.css`, `.filter-bar*`, because it also has to normalise the legacy
 * `.form-tra-cuu` / `.chon-hang-muc` / `.o-chon` children every older screen still renders):
 *   row 1   `primary` — the search box FIRST (leftmost, grows), then at most two decisive filters —
 *           then the toggle at the right end. Bottom-aligned, every control 40px, labels above.
 *   panel   `more` — a grid of equal columns, labels above.
 *   < 768px everything stacks full width, in DOM order (so the search box is first there too).
 *
 * PRESENTATIONAL ONLY. The screen's own controls are passed in unchanged — ids, names, values and
 * handlers stay where they were (ADR 0068 §1).
 *
 * THE PANEL IS HIDDEN WITH THE `hidden` ATTRIBUTE, NEVER UNMOUNTED: a closed panel keeps its
 * controls (and their state, ids and labels) in the document, so a filter set before closing still
 * applies and is still findable by a test or a screen reader's form list.
 *
 * `moreActiveCount` > 0 opens the panel on first render and stays on the button ("Bộ lọc · 2"), so
 * a filter hidden behind the button is never a filter nobody can see is on. The user may still
 * close the panel; the count keeps saying it.
 *
 * ONLY `useState`, AND THE PANEL ID COMES FROM THE REQUIRED `id` PROP: some tests render screens
 * under a mocked React that provides only useState/useEffect/useCallback/useMemo — no useId, no
 * useRef, no context (`danh-ba-lien-he.luong.test.tsx`).
 */
export type FilterBarProps = {
  /** Unique on the page; the panel is `${id}-more`, the toggle `${id}-toggle`. */
  id: string;
  /** Row 1: the search box first, then at most two decisive filters. */
  primary: ReactNode;
  /** The panel behind the toggle. Omit for a row with nothing to hide (no toggle is drawn). */
  more?: ReactNode;
  /** How many `more` filters are not at their default value. */
  moreActiveCount: number;
  /** Toggle text. */
  label?: string;
  className?: string;
};

export function FilterBar({
  id,
  primary,
  more,
  moreActiveCount,
  label = "Bộ lọc",
  className,
}: FilterBarProps) {
  const [open, setOpen] = useState(moreActiveCount > 0);
  const hasMore = more !== undefined && more !== null && more !== false;
  const panelId = `${id}-more`;
  const toggleId = `${id}-toggle`;

  return (
    <div className={cn("filter-bar", className)}>
      <div className="filter-bar-row">
        {primary}
        {hasMore && (
          <Button
            id={toggleId}
            type="button"
            variant="secondary"
            className="filter-bar-toggle"
            aria-expanded={open}
            aria-controls={panelId}
            onClick={() => setOpen((o) => !o)}
            icon={<SlidersHorizontal aria-hidden="true" focusable="false" strokeWidth={1.8} />}
          >
            {moreActiveCount > 0 ? `${label} · ${moreActiveCount}` : label}
            {open ? (
              <ChevronUp aria-hidden="true" focusable="false" strokeWidth={1.8} />
            ) : (
              <ChevronDown aria-hidden="true" focusable="false" strokeWidth={1.8} />
            )}
          </Button>
        )}
      </div>
      {hasMore && (
        <div id={panelId} className="filter-bar-panel" role="group" aria-labelledby={toggleId} hidden={!open}>
          {more}
        </div>
      )}
    </div>
  );
}
