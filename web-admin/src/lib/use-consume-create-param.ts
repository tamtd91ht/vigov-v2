"use client";

import { useEffect, useRef } from "react";

import { OPEN_CREATE_PARAM, parseOpenCreate } from "@/lib/create-link";

/**
 * The address without `?create=` — every other parameter kept in order. Exported for the tests;
 * screens call `useConsumeCreateParam`.
 */
export function searchWithoutCreate(search: string): string {
  const params = new URLSearchParams(search);
  params.delete(OPEN_CREATE_PARAM);
  const rest = params.toString();
  return rest === "" ? "" : `?${rest}`;
}

/** `?create=1` still in the address bar, read under the server's rule (`parseOpenCreate`). */
function createStillInAddress(search: string): boolean {
  const values = new URLSearchParams(search).getAll(OPEN_CREATE_PARAM);
  return parseOpenCreate({ [OPEN_CREATE_PARAM]: values.length > 1 ? values : values[0] });
}

/**
 * `?create=1` (`create-link.ts`) honoured ONCE: open the screen's create dialog, then drop the
 * parameter from the address bar.
 *
 * `requested` is the server-read prop (`parseOpenCreate`). `allowed` is the SAME condition the screen
 * uses to draw its create button, and `null` while that cannot be told yet (session or catalogue still
 * being read): the hook waits rather than deciding on a half-loaded screen — deciding `false` early
 * would drop the link for an account that does hold the right, deciding `true` would open a form the
 * button would not offer. Either way the server checks the permission on save (rule 5, forbidden #1).
 *
 * THE ADDRESS BAR IS CHECKED TOO, not only the prop: a tab switch remounts the screen with the same
 * server prop, and Next's router cache can replay it on Back. Once the parameter is gone from the
 * address, nothing reopens the dialog.
 *
 * Once decided, the parameter goes whichever way it went: left in place, closing the dialog and
 * pressing F5 opens it again. `history.replaceState` keeps Next's own `history.state`, every other
 * parameter (`?task=`, `?id=`, `?tab=`) and the hash — same idiom as `useZaloOA`. The ref guards
 * React's development double-invoke of effects.
 */
export function useConsumeCreateParam(requested: boolean, allowed: boolean | null, open: () => void): void {
  const consumed = useRef(false);
  useEffect(() => {
    if (!requested || consumed.current || allowed === null) return;
    consumed.current = true;
    const { pathname, search, hash } = window.location;
    if (!createStillInAddress(search)) return;
    if (allowed) open();
    window.history.replaceState(window.history.state, "", `${pathname}${searchWithoutCreate(search)}${hash}`);
  }, [requested, allowed, open]);
}
