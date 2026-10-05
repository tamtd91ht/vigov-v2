"use client";

import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import type { ReactElement, ReactNode } from "react";

import { cn } from "@/lib/cn";

/**
 * Tooltip — Radix, styled with utilities. Radix positions the bubble through React's `style`
 * prop (CSSOM), never an injected `<style>` element, so a future CSP without `'unsafe-inline'`
 * for style elements does not break it.
 *
 * A TOOLTIP NEVER CARRIES THE ONLY COPY OF SOMETHING: hover does not exist on a phone and Radix
 * does not open on touch. The trigger must already have its accessible name (visible text,
 * visually-hidden text or `aria-label`); the tooltip repeats it for a sighted mouse user.
 *
 * USES HOOKS (inside Radix): do not put it in a component that tests call as a plain function
 * under the mocked React (`danh-ba-lien-he.luong.test.tsx`). `Button` / `IconButton` already
 * show their label through the native `title` attribute and need no Tooltip.
 *
 * `children` must be ONE element that accepts a ref and event handlers (a `<button>`, a `<Link>`,
 * a `<span>`): Radix merges its trigger props into it (`asChild`).
 */
export type TooltipProps = {
  content: ReactNode;
  children: ReactElement;
  side?: "top" | "right" | "bottom" | "left";
  /** `false` renders `children` alone — handy when a tooltip is only wanted in one state. */
  enabled?: boolean;
  className?: string;
};

export function Tooltip({ content, children, side = "top", enabled = true, className }: TooltipProps) {
  if (!enabled) return children;
  return (
    <TooltipPrimitive.Provider delayDuration={200} skipDelayDuration={100}>
      <TooltipPrimitive.Root>
        <TooltipPrimitive.Trigger asChild>{children}</TooltipPrimitive.Trigger>
        <TooltipPrimitive.Portal>
          <TooltipPrimitive.Content
            side={side}
            sideOffset={8}
            collisionPadding={8}
            className={cn(
              "z-[60] max-w-xs rounded-control bg-ink-900 px-2.5 py-1.5 text-xs leading-snug font-medium text-white shadow-md",
              "origin-(--radix-tooltip-content-transform-origin) animate-[menu-in_160ms_ease-out]",
              className,
            )}
          >
            {content}
          </TooltipPrimitive.Content>
        </TooltipPrimitive.Portal>
      </TooltipPrimitive.Root>
    </TooltipPrimitive.Provider>
  );
}
