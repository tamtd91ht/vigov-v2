"use client";

import { Eraser, Plus, Undo2, X } from "lucide-react";
import { useRef, useState, type PointerEvent as ReactPointerEvent } from "react";

import { Button } from "@/components/ui/button";
import { IconButton } from "@/components/ui/icon-button";
import type { comms_DangerZonePoint } from "@/lib/api/schema.gen";

import {
  clampCoord,
  COORD_MAX,
  COORD_MIN,
  polygonPoints,
  REMOVE_ROW,
  toGridPoint,
  ZONE_ADD_POINT,
  ZONE_CANVAS_LABEL,
  ZONE_CLEAR,
  ZONE_MAX_POINTS,
  ZONE_POINTS_LEGEND,
  ZONE_REMOVE_LAST,
} from "./alert-form";

/** Grid lines every 10 units — light, as a frame to place points by, not a map. */
const GRID_LINES = [10, 20, 30, 40, 50, 60, 70, 80, 90];

/**
 * The danger-zone editor (ADR 0091 §2): a square SVG canvas over a 0..100 grid. A press on the canvas adds
 * a vertex; dragging a vertex moves it. The polygon is drawn as the Mini App draws it (`AlertPage.tsx:
 * 180-187`: red fill at 30%, dashed red outline), so staff see what residents will see.
 *
 * THE KEYBOARD PATH IS THE LIST UNDER THE CANVAS: every point has an X and a Y number box, `Thêm điểm`
 * appends one, each row has its own remove. The canvas itself is a pointer convenience over the same state.
 *
 * ADD ON POINTER-DOWN OF THE BACKGROUND, NOT ON CLICK: a click also fires at the end of a vertex drag, and
 * would add a stray point wherever the drag ended. Vertices stop the pointer-down from reaching the
 * background.
 */
export function DangerZoneEditor({
  value,
  onChange,
  disabled = false,
  error,
}: {
  value: readonly comms_DangerZonePoint[];
  onChange: (next: comms_DangerZonePoint[]) => void;
  disabled?: boolean;
  error?: string;
}) {
  const svgRef = useRef<SVGSVGElement>(null);
  const [dragging, setDragging] = useState<number | null>(null);
  const full = value.length >= ZONE_MAX_POINTS;

  function pointAt(e: ReactPointerEvent): comms_DangerZonePoint | null {
    const svg = svgRef.current;
    if (svg === null) return null;
    return toGridPoint(e.clientX, e.clientY, svg.getBoundingClientRect());
  }

  function addAt(e: ReactPointerEvent<SVGRectElement>): void {
    if (disabled || full) return;
    const p = pointAt(e);
    if (p !== null) onChange([...value, p]);
  }

  function startDrag(i: number, e: ReactPointerEvent<SVGCircleElement>): void {
    e.stopPropagation();
    if (disabled) return;
    // jsdom has no pointer capture; a real browser keeps the drag even when the pointer leaves the vertex.
    svgRef.current?.setPointerCapture?.(e.pointerId);
    setDragging(i);
  }

  function moveDrag(e: ReactPointerEvent<SVGSVGElement>): void {
    if (dragging === null) return;
    const p = pointAt(e);
    if (p === null) return;
    onChange(value.map((q, j) => (j === dragging ? p : q)));
  }

  function endDrag(): void {
    setDragging(null);
  }

  function setCoord(i: number, axis: "x" | "y", raw: string): void {
    const n = Number(raw);
    if (raw.trim() === "" || !Number.isFinite(n)) return;
    onChange(value.map((q, j) => (j === i ? { ...q, [axis]: clampCoord(n) } : q)));
  }

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start">
        <svg
          ref={svgRef}
          viewBox="0 0 100 100"
          role="img"
          aria-label={ZONE_CANVAS_LABEL}
          data-testid="danger-zone-canvas"
          className="aspect-square w-full max-w-[22rem] shrink-0 touch-none rounded-lg border border-line bg-[#E8F0F4] select-none"
          onPointerMove={moveDrag}
          onPointerUp={endDrag}
          onPointerCancel={endDrag}
          onPointerLeave={endDrag}
        >
          <rect
            width="100"
            height="100"
            fill="#E8F0F4"
            data-testid="danger-zone-background"
            className={disabled || full ? "cursor-not-allowed" : "cursor-crosshair"}
            onPointerDown={addAt}
          />
          {GRID_LINES.map((g) => (
            <g key={g} aria-hidden="true" pointerEvents="none">
              <line x1={g} y1="0" x2={g} y2="100" stroke="#CBD9E2" strokeWidth={g === 50 ? 0.6 : 0.3} />
              <line x1="0" y1={g} x2="100" y2={g} stroke="#CBD9E2" strokeWidth={g === 50 ? 0.6 : 0.3} />
            </g>
          ))}
          {value.length >= 2 && (
            <polygon
              points={polygonPoints(value)}
              pointerEvents="none"
              data-testid="danger-zone-polygon"
              className="fill-danger stroke-danger"
              fillOpacity={value.length >= 3 ? 0.3 : 0}
              strokeWidth="1.6"
              strokeDasharray="3 2"
            />
          )}
          {value.map((p, i) => (
            <circle
              key={i}
              cx={p.x}
              cy={p.y}
              r={dragging === i ? 2.6 : 2}
              data-testid="danger-zone-vertex"
              className={disabled ? "fill-danger" : "cursor-grab fill-danger stroke-white"}
              strokeWidth="0.6"
              onPointerDown={(e) => startDrag(i, e)}
            />
          ))}
        </svg>

        <div className="flex flex-wrap gap-2 sm:flex-col">
          <Button
            type="button"
            size="sm"
            disabled={disabled || value.length === 0}
            onClick={() => onChange(value.slice(0, -1))}
            icon={<Undo2 aria-hidden="true" focusable="false" />}
          >
            {ZONE_REMOVE_LAST}
          </Button>
          <Button
            type="button"
            size="sm"
            variant="danger"
            disabled={disabled || value.length === 0}
            onClick={() => onChange([])}
            icon={<Eraser aria-hidden="true" focusable="false" />}
          >
            {ZONE_CLEAR}
          </Button>
          <p className="m-0 text-[12px] text-ink-muted" aria-live="polite">
            {value.length} / {ZONE_MAX_POINTS} điểm
          </p>
        </div>
      </div>

      {error !== undefined && (
        <p role="alert" className="m-0 text-[12px] font-medium text-danger">
          {error}
        </p>
      )}

      <fieldset className="m-0 flex flex-col gap-2 border-0 p-0">
        <legend className="mb-1 p-0 text-[12px] font-semibold text-ink-muted">{ZONE_POINTS_LEGEND}</legend>
        {value.length > 0 && (
          <ol className="m-0 flex list-none flex-col gap-1.5 p-0" data-testid="danger-zone-points">
            {value.map((p, i) => (
              <li key={i} className="flex flex-wrap items-center gap-2 text-[13px]">
                <span className="w-14 shrink-0 text-ink-muted">Điểm {i + 1}</span>
                <label className="flex items-center gap-1">
                  X
                  <input
                    type="number"
                    min={COORD_MIN}
                    max={COORD_MAX}
                    step={0.1}
                    value={p.x}
                    disabled={disabled}
                    aria-label={`Điểm ${i + 1}, X`}
                    className="h-8 w-20 rounded-lg border border-input bg-surface px-2 text-base text-navy md:text-sm"
                    onChange={(e) => setCoord(i, "x", e.target.value)}
                  />
                </label>
                <label className="flex items-center gap-1">
                  Y
                  <input
                    type="number"
                    min={COORD_MIN}
                    max={COORD_MAX}
                    step={0.1}
                    value={p.y}
                    disabled={disabled}
                    aria-label={`Điểm ${i + 1}, Y`}
                    className="h-8 w-20 rounded-lg border border-input bg-surface px-2 text-base text-navy md:text-sm"
                    onChange={(e) => setCoord(i, "y", e.target.value)}
                  />
                </label>
                <IconButton
                  type="button"
                  label={`${REMOVE_ROW} — điểm ${i + 1}`}
                  disabled={disabled}
                  onClick={() => onChange(value.filter((_, j) => j !== i))}
                >
                  <X aria-hidden="true" focusable="false" />
                </IconButton>
              </li>
            ))}
          </ol>
        )}
        <div>
          <Button
            type="button"
            size="sm"
            disabled={disabled || full}
            onClick={() => onChange([...value, { x: 50, y: 50 }])}
            icon={<Plus aria-hidden="true" focusable="false" />}
          >
            {ZONE_ADD_POINT}
          </Button>
        </div>
      </fieldset>
    </div>
  );
}
