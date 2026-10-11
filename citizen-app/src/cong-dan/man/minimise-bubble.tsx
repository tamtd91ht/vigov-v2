/**
 * THE MINIMISE BUBBLE — the prototype's (`../vigov-require/apps/miniapp/src/components/layout/MinimiseBubble.tsx`,
 * owner 11/10/2026): a round 44px button, draggable, that minimises the Mini App into Zalo's floating bubble. A
 * citizen half-way through a petition who must open a chat to copy a number keeps the form as it was; closing the
 * app would lose the photos and the location.
 *
 * WHERE: above the bottom bar, at the right — never in the header, whose top-right corner is Zalo's own buttons.
 * DRAGGABLE, because a bubble that cannot move sooner or later covers exactly what the citizen needs to read.
 *
 * THE PLATFORM CALL IS THE SHELL'S: this half may not import zmp-sdk (`ranh-gioi-hai-nua.test.ts` §3a). `App.tsx`
 * injects `onMinimise` (`minimizeApp`) only inside Zalo — outside it there is nothing to minimise into, so no bubble.
 *
 * NOT REMEMBERED: the prototype keeps the dragged position in `localStorage`; this half writes nothing on the phone
 * (`ranh-gioi-hai-nua.test.ts` §3b, owner: no new localStorage), so the bubble starts at its place on each open.
 */
import { type PointerEvent as ReactPointerEvent, useCallback, useEffect, useRef, useState } from "react";

import { BieuTuong } from "./BieuTuong";
import { XA_TN } from "./noi-dung";

/** The bubble's side in CSS pixels — the 44px touch floor (`--tap-min`). */
const SIZE = 44;
/** Breathing room from the frame's edges. */
const EDGE = 10;
/** Below this many pixels of travel, a press is a TAP (a finger never stands perfectly still). */
const TAP_SLOP = 6;

type Point = { x: number; y: number };

/** `point` kept inside a `width`×`height` frame, `EDGE` from every side. PURE, exported for tests. */
export function clampToFrame(point: Point, width: number, height: number): Point {
  return {
    x: Math.min(Math.max(point.x, EDGE), Math.max(EDGE, width - SIZE - EDGE)),
    y: Math.min(Math.max(point.y, EDGE), Math.max(EDGE, height - SIZE - EDGE)),
  };
}

export function MinimiseBubble({ onMinimise }: { onMinimise: () => void }) {
  const ref = useRef<HTMLButtonElement>(null);
  /** `null` = the default place (CSS: right, above the tab bar); a point once the citizen dragged it. */
  const [pos, setPos] = useState<Point | null>(null);
  const [dragging, setDragging] = useState(false);
  const travel = useRef(0);
  const grab = useRef<Point>({ x: 0, y: 0 });

  const frameSize = useCallback(() => {
    const frame = ref.current?.offsetParent as HTMLElement | null;
    return { width: frame?.clientWidth ?? window.innerWidth, height: frame?.clientHeight ?? window.innerHeight };
  }, []);

  // The frame changed size (rotation, a larger text size): pull the bubble back inside rather than lose it.
  useEffect(() => {
    const onResize = () =>
      setPos((p) => {
        if (p === null) return p;
        const f = frameSize();
        return clampToFrame(p, f.width, f.height);
      });
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, [frameSize]);

  function onPointerDown(e: ReactPointerEvent<HTMLButtonElement>) {
    const node = ref.current;
    if (node === null) return;
    const frameBox = (node.offsetParent as HTMLElement | null)?.getBoundingClientRect();
    const box = node.getBoundingClientRect();
    // Where the finger holds the bubble, so it does not jump under the fingertip when the drag starts.
    grab.current = { x: e.clientX - box.left, y: e.clientY - box.top };
    const f = frameSize();
    setPos(clampToFrame({ x: box.left - (frameBox?.left ?? 0), y: box.top - (frameBox?.top ?? 0) }, f.width, f.height));
    travel.current = 0;
    setDragging(true);
    node.setPointerCapture?.(e.pointerId);
  }

  function onPointerMove(e: ReactPointerEvent<HTMLButtonElement>) {
    if (!dragging) return;
    const frameBox = (ref.current?.offsetParent as HTMLElement | null)?.getBoundingClientRect();
    const f = frameSize();
    const next = clampToFrame(
      { x: e.clientX - (frameBox?.left ?? 0) - grab.current.x, y: e.clientY - (frameBox?.top ?? 0) - grab.current.y },
      f.width,
      f.height,
    );
    setPos((p) => {
      if (p !== null) travel.current += Math.abs(next.x - p.x) + Math.abs(next.y - p.y);
      return next;
    });
  }

  function onPointerUp(e: ReactPointerEvent<HTMLButtonElement>) {
    if (!dragging) return;
    setDragging(false);
    ref.current?.releasePointerCapture?.(e.pointerId);
    if (travel.current < TAP_SLOP) onMinimise();
  }

  return (
    <button
      ref={ref}
      type="button"
      className={`xa-minimise${dragging ? " xa-minimise--dragging" : ""}`}
      aria-label={XA_TN.minimise_app}
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
      onPointerCancel={onPointerUp}
      // A keyboard cannot drag, so it gets its own way to the action.
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          onMinimise();
        }
      }}
      style={pos === null ? undefined : { left: pos.x, top: pos.y, right: "auto", bottom: "auto" }}
    >
      <BieuTuong ten="minus" co={22} stroke={3} />
    </button>
  );
}
