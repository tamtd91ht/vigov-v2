"use client";

import { Maximize2, Minimize2 } from "lucide-react";

import { Button } from "@/components/ui/button";

import { MAP_COLLAPSE, MAP_EXPAND } from "./labels";

/**
 * "Mở rộng bản đồ" / "Thu gọn" — one toggle for both maps of `/ban-do` (owner, 04/10/2026: the map
 * window was too small to work in).
 *
 * NOT the browser's Fullscreen API: a fullscreen MAP element would hide the React detail panel and
 * the dialogs that open from it, i.e. exactly what staff work with. The main map instead grows its
 * whole region (map + detail panel) into a page overlay; the form's picker grows inside its dialog.
 * Both collapse back with this button or Esc; the map keeps its frame (maxBounds) either way.
 */
export function MapExpandToggle({
  expanded,
  onToggle,
  controls,
}: {
  expanded: boolean;
  onToggle: () => void;
  /** id of the region this button resizes (`aria-controls`). */
  controls: string;
}) {
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
      {expanded ? MAP_COLLAPSE : MAP_EXPAND}
    </Button>
  );
}
