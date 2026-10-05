"use client";

import { X, XCircle, type LucideIcon } from "lucide-react";
import { useEffect, type KeyboardEvent } from "react";

import { cn } from "@/lib/cn";

/**
 * The strip of RECORD TABS on top of a detail panel (owner 05/10/2026; state in
 * `record-tabs-state.ts`). Navy band; each tab = icon · title (truncated) · one secondary line · ✕;
 * the ACTIVE tab is white and runs into the panel below it; `Đóng tất cả` sits at the right.
 *
 * ARIA tabs pattern: `role="tablist"`, ROVING TABINDEX (only the active tab is in the Tab order),
 * ←/→ move and activate, Home/End jump, Delete closes the focused tab. Automatic activation, as the
 * detail's own tabs do: a tab switch is one read, and a read overtaken by the next switch is
 * dropped by the caller.
 *
 * The ✕ is a SIBLING of the tab, never inside it: a button inside a button is invalid HTML and a
 * screen reader would read the tab's name as "… Đóng tab NV19". Only the active tab's ✕ is in the
 * Tab order; the others are reached by Delete on their tab — so Tab goes tab → its ✕ → Đóng tất cả
 * → the panel, never through eight ✕ in a row.
 *
 * Overflow scrolls horizontally at every width (below 768px too); the active tab is scrolled into
 * view when it changes. Colour transitions only, 150ms, under `motion-safe` (ADR 0068 §11).
 */
/**
 * The DOM id of the tab for record `id`. A business code is typed by a clerk and may hold a space
 * or a slash, which an `id` referenced by `aria-labelledby` cannot; every other character is
 * escaped to its code point, so two codes never share an id.
 */
export function recordTabDomId(idPrefix: string, id: string): string {
  return `${idPrefix}-${id.replace(/[^A-Za-z0-9-]/g, (c) => `_${c.codePointAt(0)!.toString(16)}_`)}`;
}

export type RecordTabItem = {
  readonly id: string;
  readonly title: string;
  /** One line under the title (status, …). Never personal data beyond what the list shows. */
  readonly secondary?: string;
  readonly icon?: LucideIcon;
};

export function RecordTabStrip({
  label,
  idPrefix,
  panelId,
  tabs,
  activeId,
  onSelect,
  onClose,
  onCloseAll,
  closeTabLabel,
  closeAllLabel = "Đóng tất cả",
  className,
}: {
  /** The tablist's accessible name, e.g. "Nhiệm vụ đang mở". */
  label: string;
  /** Prefix of every tab's `id` — unique on the page. */
  idPrefix: string;
  /** `id` of the panel the tabs control. */
  panelId: string;
  tabs: readonly RecordTabItem[];
  activeId: string | null;
  onSelect: (id: string) => void;
  onClose: (id: string) => void;
  onCloseAll: () => void;
  /** e.g. `(id) => \`Đóng tab ${id}\`` — names the record, so eight ✕ are told apart. */
  closeTabLabel: (id: string) => string;
  closeAllLabel?: string;
  className?: string;
}) {
  const tabId = (id: string) => recordTabDomId(idPrefix, id);

  useEffect(() => {
    if (activeId === null) return;
    const el = document.getElementById(recordTabDomId(idPrefix, activeId));
    // jsdom has no `scrollIntoView`.
    if (typeof el?.scrollIntoView === "function") el.scrollIntoView({ block: "nearest", inline: "nearest" });
  }, [activeId, idPrefix]);

  function focusTab(id: string): void {
    document.getElementById(tabId(id))?.focus();
  }

  function onKeyDown(e: KeyboardEvent<HTMLDivElement>): void {
    const target = e.target as HTMLElement;
    if (target.getAttribute("role") !== "tab") return;
    const i = tabs.findIndex((t) => tabId(t.id) === target.id);
    if (i < 0) return;
    if (e.key === "Delete") {
      e.preventDefault();
      const closing = tabs[i]!.id;
      // Focus goes where the active tab will be: the neighbour when the active one closes (the
      // caller activates it), else the active tab as it was.
      const next = closing === activeId ? (tabs[i + 1] ?? tabs[i - 1])?.id : activeId;
      onClose(closing);
      if (next !== undefined && next !== null) requestAnimationFrame(() => focusTab(next));
      return;
    }
    const n = tabs.length;
    const next =
      e.key === "ArrowRight" ? (i + 1) % n
      : e.key === "ArrowLeft" ? (i - 1 + n) % n
      : e.key === "Home" ? 0
      : e.key === "End" ? n - 1
      : -1;
    if (next < 0) return;
    e.preventDefault();
    const id = tabs[next]!.id;
    onSelect(id);
    focusTab(id);
  }

  if (tabs.length === 0) return null;

  return (
    <div className={cn("flex shrink-0 items-end gap-2 bg-brand-600 pt-2 pr-2 pl-2 md:pl-3", className)}>
      <div
        role="tablist"
        aria-label={label}
        aria-orientation="horizontal"
        className="flex min-w-0 flex-1 items-end gap-1 overflow-x-auto overflow-y-hidden [scrollbar-width:thin]"
        onKeyDown={onKeyDown}
      >
        {tabs.map((t) => {
          const selected = t.id === activeId;
          const Icon = t.icon;
          return (
            <div
              key={t.id}
              role="presentation"
              className={cn(
                "group flex w-56 max-w-[70vw] shrink-0 items-center rounded-t-popover",
                "motion-safe:transition-colors motion-safe:duration-150",
                selected ? "bg-surface text-ink-900" : "text-white hover:bg-white/10",
              )}
            >
              <button
                type="button"
                role="tab"
                id={tabId(t.id)}
                aria-selected={selected}
                aria-controls={panelId}
                tabIndex={selected ? 0 : -1}
                className={cn(
                  "flex min-w-0 flex-1 cursor-pointer items-center gap-2 border-0 bg-transparent py-2 pr-1 pl-3 text-left [font-family:inherit] text-inherit",
                  "focus-visible:outline-2 focus-visible:-outline-offset-2",
                  selected ? "focus-visible:outline-brand-500" : "focus-visible:outline-white",
                )}
                onClick={() => onSelect(t.id)}
              >
                {Icon !== undefined && (
                  <span
                    aria-hidden="true"
                    className={cn(
                      "grid size-8 shrink-0 place-items-center rounded-full",
                      selected ? "bg-accent-50 text-ink-900" : "bg-white/10 text-white",
                    )}
                  >
                    <Icon focusable="false" strokeWidth={1.8} className="size-4" />
                  </span>
                )}
                <span className="flex min-w-0 flex-col">
                  <span className={cn("truncate text-[14px] leading-5", selected ? "font-medium" : "font-normal")}>
                    {t.title}
                  </span>
                  {t.secondary !== undefined && t.secondary !== "" && (
                    <span className={cn("truncate text-[12px] leading-4", selected ? "text-ink-500" : "text-white/75")}>
                      {t.secondary}
                    </span>
                  )}
                </span>
              </button>
              <button
                type="button"
                aria-label={closeTabLabel(t.id)}
                title={closeTabLabel(t.id)}
                tabIndex={selected ? 0 : -1}
                className={cn(
                  "mr-1 grid size-7 shrink-0 cursor-pointer place-items-center rounded-control border-0 bg-transparent p-0 text-inherit",
                  "motion-safe:transition-colors motion-safe:duration-150",
                  "focus-visible:outline-2 focus-visible:-outline-offset-2",
                  selected
                    ? "hover:bg-surface-subtle focus-visible:outline-brand-500"
                    : "hover:bg-white/15 focus-visible:outline-white",
                )}
                onClick={() => onClose(t.id)}
              >
                <X aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-4" />
              </button>
            </div>
          );
        })}
      </div>
      <button
        type="button"
        className={cn(
          "mb-1.5 inline-flex h-9 shrink-0 cursor-pointer items-center gap-2 rounded-control border-0 bg-transparent px-3 [font-family:inherit] text-[14px] whitespace-nowrap text-white",
          "motion-safe:transition-colors motion-safe:duration-150 hover:bg-white/10",
          "focus-visible:outline-2 focus-visible:outline-offset-0 focus-visible:outline-white",
        )}
        onClick={onCloseAll}
      >
        {closeAllLabel}
        <XCircle aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-4" />
      </button>
    </div>
  );
}
