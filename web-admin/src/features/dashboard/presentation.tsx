"use client";

import { Maximize2, Minimize2 } from "lucide-react";
import { createContext, useEffect, useRef, useState } from "react";
import type { ReactNode } from "react";

import { Button } from "@/components/ui/button";

/**
 * "Trình chiếu" — the meeting-room mode of Tổng quan (spec 01 §2: "Ẩn sidebar + header, phóng to chữ,
 * nền tối, điều hướng bằng phím"; prototype `DashboardWorkspace.tsx:110-130,196-208`). BUILT on the
 * user's decision of 07/10/2026, overriding ADR 0053 / ADR 0068 "giai đoạn 2" for this one control —
 * the same precedent as the export (ae894ce4).
 *
 * A PAGE OVERLAY, NOT THE BROWSER'S FULLSCREEN API — as `/ban-do`'s map (`map-expand.tsx`): the "?"
 * descriptions open in portals on `<body>`, which a fullscreen element would hide. The projector's
 * own F11 still gives the whole screen.
 *
 * NOT IN THE URL: a drill-down from a tile navigates away, and coming back opens the normal page.
 * The mode is the meeting's, not the link's.
 */

/** The accessible name the spec gives the control (spec 01 §2, spec 15 §4). */
export const PRESENTATION_LABEL = "Chế độ trình chiếu phòng họp";

/** On/off, owned by the hook half (`overview.tsx`). Absent = the toggle is drawn disabled. */
export type PresentationControl = {
  readonly on: boolean;
  readonly onChange: (on: boolean) => void;
};

/**
 * What the blocks read: whether the mode is on and which block is the current keyboard stop (its
 * `data-block` key). A context, as `BlockLayoutContext`: six block components and their tiles change
 * only their type size, never a figure. `/bao-cao` never mounts the frame, and `view.tsx` also ignores
 * this context under `layout="report"`.
 */
export type PresentationState = { readonly on: boolean; readonly current: string | null };

export const PresentationContext = createContext<PresentationState>({ on: false, current: null });

/**
 * The toggle — the shape of `MapExpandToggle`: `aria-pressed`, Maximize2 / Minimize2. The visible word
 * "Trình chiếu" is contained in the accessible name, so speech input saying what it sees still works
 * (WCAG 2.5.3). Solid navy while on, as the prototype's `variant="default"`.
 */
export function PresentationToggle({ control }: { control?: PresentationControl }) {
  const on = control?.on === true;
  const Icon = on ? Minimize2 : Maximize2;
  return (
    <Button
      type="button"
      size="sm"
      variant={on ? "dark" : "secondary"}
      aria-label={PRESENTATION_LABEL}
      aria-pressed={on}
      title={PRESENTATION_LABEL}
      disabled={control === undefined}
      data-presentation-toggle=""
      icon={<Icon aria-hidden="true" focusable="false" strokeWidth={1.8} />}
      onClick={() => control?.onChange(!on)}
    >
      Trình chiếu
    </Button>
  );
}

/** Keys that walk the blocks. Anything else is left to the browser. */
const STEP: Readonly<Record<string, (at: number, last: number) => number>> = {
  ArrowRight: (at, last) => (at < 0 ? 0 : Math.min(at + 1, last)),
  PageDown: (at, last) => (at < 0 ? 0 : Math.min(at + 1, last)),
  ArrowLeft: (at) => (at < 0 ? 0 : Math.max(at - 1, 0)),
  PageUp: (at) => (at < 0 ? 0 : Math.max(at - 1, 0)),
  Home: () => 0,
  End: (_at, last) => last,
};

/** A field where the arrows and Home/End belong to the text or the list, never to the slides. */
function isTypingTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  return target.isContentEditable || ["INPUT", "SELECT", "TEXTAREA"].includes(target.tagName);
}

/**
 * Something above the overlay owns the keyboard: a "?" description (Radix popover, `role="dialog"`)
 * or a native modal. Its Esc closes IT; the mode stays.
 */
function dialogOpen(): boolean {
  return document.querySelector('[role="dialog"], dialog[open]') !== null;
}

/**
 * Makes everything OUTSIDE the overlay `inert` — the siblings of the overlay and of each of its
 * ancestors up to `<body>`: the shell's navy header and its sidebar are siblings of `<main>`. Without it
 * Tab walks off the overlay into links nobody can see. Returns exactly the elements it changed, so
 * closing gives back only those (one already inert for its own reason stays inert).
 *
 * Portals opened LATER (a "?" description) are appended to `<body>` after this ran, so they stay live.
 */
function inertOutside(frame: HTMLElement): HTMLElement[] {
  const changed: HTMLElement[] = [];
  for (let node: HTMLElement = frame; node.parentElement !== null && node !== document.body; node = node.parentElement) {
    for (const sibling of node.parentElement.children) {
      if (sibling === node || !(sibling instanceof HTMLElement) || sibling.hasAttribute("inert")) continue;
      sibling.setAttribute("inert", "");
      changed.push(sibling);
    }
  }
  return changed;
}

/**
 * The frame around the WHOLE page body — header, grid, the "kỳ trước" note. Off: a plain wrapper,
 * nothing else. On: `fixed inset-0` over the shell, dark tokens scoped to it (`[data-presentation]` in
 * `globals.css` — the app-wide dark theme stays OFF, owner 02/10/2026), the page behind locked and
 * inert, and the keyboard stops.
 *
 * THE SAME ELEMENT IN BOTH STATES: switching only its attributes keeps the toggle — which lives inside
 * it, in the page header — the very element that had focus, so a mouse click never loses focus.
 *
 * WRAPS AT PAGE LEVEL, NEVER INSIDE A BLOCK: each block's body is a size `@container`, and a container
 * is the containing block of its `fixed` descendants — a fixed overlay inside one would cover one card.
 * z-40 per the documented scale (`globals.css` `.dau-trang`): header 30 < full-screen 40 < popovers 60.
 */
export function PresentationFrame({
  control,
  children,
}: {
  control?: PresentationControl;
  children: ReactNode;
}) {
  const on = control?.on === true;
  const frameRef = useRef<HTMLDivElement>(null);
  const [current, setCurrent] = useState<string | null>(null);

  // The latest handler, read by the key listener without re-running the effect: a caller passing a
  // new arrow each render must not re-lock the body and re-steal focus on every render.
  const onChangeRef = useRef<PresentationControl["onChange"] | undefined>(undefined);
  useEffect(() => {
    onChangeRef.current = control?.onChange;
  });

  useEffect(() => {
    const frame = frameRef.current;
    if (!on || frame === null) return;
    // The page behind the overlay must not scroll under it (`economic-map-screen.tsx` pattern).
    const overflowBefore = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    const madeInert = inertOutside(frame);
    // Focus INTO the overlay: a click on the toggle already left it there; anything else lands on
    // the frame itself, from where Tab walks the header and the blocks.
    if (!frame.contains(document.activeElement)) frame.focus({ preventScroll: true });

    const onKey = (e: KeyboardEvent) => {
      if (e.defaultPrevented || e.altKey || e.ctrlKey || e.metaKey || dialogOpen()) return;
      if (e.key === "Escape") {
        onChangeRef.current?.(false);
        return;
      }
      const step = STEP[e.key];
      if (step === undefined || isTypingTarget(e.target)) return;
      const stops = [...frame.querySelectorAll<HTMLElement>("[data-block]")].map((s) => s.dataset.block ?? "");
      if (stops.length === 0) return;
      e.preventDefault();
      setCurrent((prev) => stops[step(prev === null ? -1 : stops.indexOf(prev), stops.length - 1)] ?? null);
    };
    window.addEventListener("keydown", onKey);

    return () => {
      window.removeEventListener("keydown", onKey);
      document.body.style.overflow = overflowBefore;
      for (const el of madeInert) el.removeAttribute("inert");
      // A fresh meeting starts with no block singled out.
      setCurrent(null);
      // Back to the control that opened it — after Esc, focus would otherwise sit on a block (or the
      // frame) that is no longer a stop. On unmount the toggle is gone too, and this is a no-op.
      frame.querySelector<HTMLElement>("[data-presentation-toggle]")?.focus({ preventScroll: true });
    };
  }, [on]);

  // The current block takes focus — a screen reader then announces the block it landed on — and is
  // scrolled into view; `block: "nearest"` moves the overlay only when the block is not on screen.
  useEffect(() => {
    if (!on || current === null) return;
    const block = frameRef.current?.querySelector<HTMLElement>(`[data-block="${current}"]`);
    if (block === null || block === undefined) return;
    block.focus({ preventScroll: true });
    block.scrollIntoView?.({ block: "nearest" });
  }, [on, current]);

  return (
    <PresentationContext.Provider value={{ on, current: on ? current : null }}>
      <div
        ref={frameRef}
        data-presentation={on ? "on" : undefined}
        tabIndex={on ? -1 : undefined}
        className={on ? "fixed inset-0 z-40 overflow-auto bg-canvas p-4 text-ink-900 outline-none sm:p-6" : undefined}
      >
        {children}
      </div>
    </PresentationContext.Provider>
  );
}
