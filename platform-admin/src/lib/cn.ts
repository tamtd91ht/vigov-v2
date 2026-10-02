import { clsx, type ClassValue } from "clsx";
import { extendTailwindMerge } from "tailwind-merge";

/**
 * Copied from `web-admin/src/lib/cn.ts` (ADR 0068). No shared package exists yet, so the two
 * copies must be changed together.
 *
 * The radius names declared in `globals.css` (`@theme inline`, `--radius-*`). tailwind-merge
 * cannot read the CSS, so without this list `rounded-control` and `rounded-lg` would both survive
 * a merge and the winner would be whichever the stylesheet happened to emit last.
 */
const twMerge = extendTailwindMerge({
  extend: { theme: { radius: ["control", "card", "hero"] } },
});

/** Join class names and let the LAST Tailwind utility win when two conflict (`px-4` then `px-2`). */
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}
