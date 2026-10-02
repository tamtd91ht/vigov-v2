import { clsx, type ClassValue } from "clsx";
import { extendTailwindMerge } from "tailwind-merge";

/**
 * The radius names declared in `globals.css` (`@theme inline`, `--radius-*`). tailwind-merge
 * cannot read the CSS, so without this list `rounded-control` and `rounded-lg` would both survive
 * a merge and the winner would be whichever the stylesheet happened to emit last.
 */
const twMerge = extendTailwindMerge({
  extend: { theme: { radius: ["control", "card", "hero"] } },
});

/**
 * Join class names and let the LAST Tailwind utility win when two conflict (`px-4` then `px-2`
 * → `px-2`). A caller's `className` can therefore adjust a component without a second variant.
 *
 * Non-Tailwind names — the legacy hooks `nut-chinh`, `nut-phu`, `an-thi-giac` — pass through
 * untouched: `tailwind-merge` only dedupes classes it recognises as utilities.
 */
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}
