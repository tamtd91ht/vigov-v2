"use client";

import { useCallback, useSyncExternalStore } from "react";

/**
 * The left sidebar can be collapsed to icons only, and the choice is remembered (owner, 05/10/2026:
 * module navigation back on the left, shaped like the prototype's `SidebarState`).
 *
 * REMEMBERED, NOT ASKED AGAIN: a staff member works on one machine all day; collapsing is done to give a
 * wide register its room, not to be redone every morning. Stored in `localStorage`, which the browser
 * already scopes to this commune's own host — so one commune's preference never reaches another's page.
 * It is a layout preference: no identity, no permission, nothing the server reads.
 *
 * `useSyncExternalStore`, not state + effect: the server renders without `localStorage`, so it needs its
 * own snapshot (always expanded); and a change in one tab follows in the others through `storage`.
 *
 * EVERY STORAGE ACCESS IS IN try/catch: a browser with storage blocked (private mode, a policy) must still
 * draw the menu — it just forgets the choice at the next load.
 */
export const SIDEBAR_STORAGE_KEY = "vigov.sidebar-collapsed";

let cached = false;
const listeners = new Set<() => void>();

function read(): boolean {
  try {
    return window.localStorage.getItem(SIDEBAR_STORAGE_KEY) === "1";
  } catch {
    return false;
  }
}

function announce(): void {
  for (const listener of listeners) listener();
}

function subscribe(listener: () => void): () => void {
  if (listeners.size === 0) cached = read();
  listeners.add(listener);
  const onStorage = (event: StorageEvent) => {
    if (event.key !== SIDEBAR_STORAGE_KEY) return;
    cached = read();
    announce();
  };
  window.addEventListener("storage", onStorage);
  return () => {
    listeners.delete(listener);
    window.removeEventListener("storage", onStorage);
  };
}

function getSnapshot(): boolean {
  return cached;
}

/** The server has no storage to read: it always draws the sidebar expanded. */
function getServerSnapshot(): boolean {
  return false;
}

export function useSidebarCollapsed(): { collapsed: boolean; toggle: () => void } {
  const collapsed = useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
  const toggle = useCallback(() => {
    cached = !cached;
    try {
      window.localStorage.setItem(SIDEBAR_STORAGE_KEY, cached ? "1" : "0");
    } catch {
      // Storage blocked: this page still collapses, the choice is just not remembered.
    }
    announce();
  }, []);
  return { collapsed, toggle };
}
