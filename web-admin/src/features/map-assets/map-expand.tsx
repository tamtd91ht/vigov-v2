"use client";

import { Maximize, Maximize2, Minimize, Minimize2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/cn";

import { MAP_COLLAPSE, MAP_EXPAND } from "./labels";

/**
 * "Mở rộng bản đồ" / "Thu gọn" — the main map of `/ban-do` (owner, 04/10/2026: the map window was too
 * small to work in). The form's pin map no longer has it (owner, 11/10/2026: prototype `LocationPicker`).
 *
 * NOT the browser's Fullscreen API: a fullscreen MAP element would hide the React detail panel and
 * the dialogs that open from it, i.e. exactly what staff work with. The main map instead grows its
 * whole region (map + detail panel) into a page overlay. It collapses back with this button or Esc;
 * the map keeps its frame (maxBounds) either way.
 *
 * `floating`: the main map's form (spec 08 B) — icon only, in the group of buttons floating on the map's
 * top-left corner; the word moves to `aria-label` + `title`, so it is still named for everyone.
 */
export function MapExpandToggle({
  expanded,
  onToggle,
  controls,
  floating = false,
}: {
  expanded: boolean;
  onToggle: () => void;
  /** id of the region this button resizes (`aria-controls`). */
  controls: string;
  floating?: boolean;
}) {
  const label = expanded ? MAP_COLLAPSE : MAP_EXPAND;
  if (floating) {
    const Icon = expanded ? Minimize : Maximize;
    return (
      <Button
        type="button"
        variant="outline"
        size="sm"
        aria-pressed={expanded}
        aria-controls={controls}
        aria-label={label}
        title={label}
        className={cn(FLOATING_BUTTON, "w-8 px-0")}
        icon={<Icon aria-hidden="true" focusable="false" className="size-3.5" />}
        onClick={onToggle}
      />
    );
  }
  const Icon = expanded ? Minimize2 : Maximize2;
  return (
    <Button
      type="button"
      variant="secondary"
      size="sm"
      aria-pressed={expanded}
      aria-controls={controls}
      icon={<Icon aria-hidden="true" focusable="false" strokeWidth={1.8} />}
      onClick={onToggle}
    >
      {label}
    </Button>
  );
}

/** Spec 08 B: the buttons floating on the map — white at 95%, 32px, 12px text, the card shadow. */
export const FLOATING_BUTTON = "h-8 bg-white/95 text-[12px] shadow-card";
