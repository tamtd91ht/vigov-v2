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

/*
 * WHAT SURVIVES A MOVE BETWEEN SCREENS (bug sheet 15/10/2026: after opening "Phản ánh người dân" the
 * menu jumped back to its top and the items below had to be scrolled to again).
 *
 * WHY A MOVE LOSES THEM AT ALL: every screen draws its own shell (`page.tsx` → `.khung-trang` →
 * `DauTrang`), because each page resolves the commune from `Host` itself and 404s there (see
 * `app/layout.tsx`). A `<Link>` therefore swaps the WHOLE page subtree, sidebar included: the new
 * sidebar is a new element at scrollTop 0, and the new page's `PhienProvider` starts unread, so for a
 * moment only the items needing no key are listed — the menu visibly empties and refills.
 *
 * The two values below carry the previous screen's sidebar into the next one. MODULE state, so it lives
 * exactly as long as this tab's JavaScript: a full reload (sign-in, sign-out, forced password change all
 * navigate with `location.assign`) starts from nothing. Each commune is its own host, hence its own
 * JavaScript realm — one commune's values never reach another's page. Both are written ONLY from the
 * browser (a scroll event, an effect): this module is also evaluated on the server, where a write would
 * be shared by every request of every commune.
 */
let navScrollTop = 0;
let menuPermissions: readonly string[] | null = null;

/** `onScroll` of the sidebar's list: remember how far down it is. */
export function rememberNavScroll(event: { currentTarget: HTMLElement }): void {
  navScrollTop = event.currentTarget.scrollTop;
}

/**
 * Ref callback of the sidebar's list: put it back where the previous screen left it. A ref runs while
 * the new DOM is committed, before the browser paints, so the list never shows at the top first.
 * Module-level (stable identity), so React calls it once per mount — never on a re-render.
 */
export function restoreNavScroll(element: HTMLElement | null): void {
  if (element !== null) element.scrollTop = navScrollTop;
}

/**
 * Permissions the previous screen's session read answered with, or `null` before any read finished in
 * this tab. ONLY for drawing the menu while this screen's own read is in flight: listing an item is UX,
 * and every route still checks its key on the server (rule 5, forbidden #1). The moment this screen's
 * read answers, its answer replaces this one.
 */
export function rememberedMenuPermissions(): readonly string[] | null {
  return menuPermissions;
}

export function rememberMenuPermissions(permissions: readonly string[]): void {
  menuPermissions = permissions;
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
