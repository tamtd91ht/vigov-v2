"use client";

import { ChevronDown, ChevronUp } from "lucide-react";
import { useSyncExternalStore, type ComponentProps } from "react";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/cn";

/**
 * A card of Cấu hình → Kênh Zalo with a chevron in its top-right corner that folds it down to its
 * heading (user request 17/10/2026: the tab is long enough that finding one section meant scrolling
 * past all the others).
 *
 * WHAT STAYS VISIBLE: the toggle and the section's FIRST child — in every section of the tab that is
 * the heading (an `<h3>`, or the row holding it). Everything after it is hidden with CSS, NOT unmounted:
 * a section's in-flight read, typed draft or open confirmation must survive a fold, and an unmount
 * would throw them away.
 *
 * The fold is a per-viewer convenience kept in `localStorage`, keyed by the section's `aria-labelledby`
 * id — never anything about the commune or the user. Every access is guarded and mirrored in memory: a
 * private window or blocked storage still folds, it just does not remember across reloads. Read through
 * `useSyncExternalStore` with an "open" server snapshot, so the server render and the first client
 * render agree and a remembered fold applies right after hydration.
 *
 * `padRight` reserves room for the chevron in the section's own padding; a section whose header row
 * has its own padding (Ai đã ghép nối) turns it off and pads that row instead.
 */
export function CollapsibleSection({
  className,
  children,
  padRight = true,
  toggleClassName,
  ...props
}: ComponentProps<"section"> & { "aria-labelledby": string; padRight?: boolean; toggleClassName?: string }) {
  const titleId = props["aria-labelledby"];
  const storageKey = `${STORAGE_PREFIX}${titleId}`;
  const collapsed = useSyncExternalStore(
    subscribe,
    () => readCollapsed(storageKey),
    () => false,
  );

  return (
    <section
      {...props}
      data-collapsed={collapsed ? "" : undefined}
      className={cn(
        className,
        "relative",
        padRight && "pr-12",
        // Child 1 is the toggle, child 2 the heading; the rest folds away.
        "data-[collapsed]:[&>*:nth-child(n+3)]:hidden",
      )}
    >
      <Button
        type="button"
        variant="icon"
        size="sm"
        className={cn("text-ink-muted hover:text-navy absolute top-4 right-3 min-h-0", toggleClassName)}
        aria-expanded={!collapsed}
        aria-label={collapsed ? "Mở rộng mục" : "Thu gọn mục"}
        aria-describedby={titleId}
        title={collapsed ? "Mở rộng" : "Thu gọn"}
        data-section-toggle=""
        onClick={() => writeCollapsed(storageKey, !collapsed)}
      >
        {collapsed ? (
          <ChevronDown aria-hidden="true" focusable="false" />
        ) : (
          <ChevronUp aria-hidden="true" focusable="false" />
        )}
      </Button>
      {children}
    </section>
  );
}

const STORAGE_PREFIX = "zalo-section-collapsed:";
const CHANGE_EVENT = "zalo-section-collapsed";
/** What this page load decided, so a fold works even where storage throws. */
const memory = new Map<string, boolean>();

function subscribe(onChange: () => void): () => void {
  window.addEventListener("storage", onChange);
  window.addEventListener(CHANGE_EVENT, onChange);
  return () => {
    window.removeEventListener("storage", onChange);
    window.removeEventListener(CHANGE_EVENT, onChange);
  };
}

function readCollapsed(key: string): boolean {
  const known = memory.get(key);
  if (known !== undefined) return known;
  try {
    return window.localStorage.getItem(key) === "1";
  } catch {
    return false;
  }
}

function writeCollapsed(key: string, collapsed: boolean): void {
  memory.set(key, collapsed);
  try {
    if (collapsed) window.localStorage.setItem(key, "1");
    else window.localStorage.removeItem(key);
  } catch {
    // Not remembered across reloads; `memory` still carries the fold for this page load.
  }
  window.dispatchEvent(new Event(CHANGE_EVENT));
}
