"use client";

import { useEffect, useRef } from "react";

import { OPEN_TASK_PARAM, parseOpenTask } from "./task-link";

/**
 * The address bar follows the task detail dialog (ADR 0068 §Sửa đổi 05/10/2026 #5 — the one
 * exception to "presentation only"):
 *
 *   open from the list (nothing open)     PUSH one entry `?task=<code>`, filters and `#hash` kept
 *   parent / child / rename inside        REPLACE `?task=` — one Back still reaches the list
 *   close (✕, Esc, delete)                Back if this screen pushed the entry, else REPLACE without it
 *   Back / Forward                        `popstate` → the caller closes, or opens the code it names
 *
 * WHY THE DIALOG WINS OVER THE FILTER-ONLY RULE OF THE LIST: the dialog covers the whole list, so
 * staff read it as a page; a Back that left the Nhiệm vụ screen, filters and all, loses the work.
 *
 * WHAT IT DOES NOT DO: read `?task=` on LOAD. `app/nhiem-vu/page.tsx` reads it server-side
 * (`parseOpenTask`) and passes it down; this module only answers history moves made after load.
 *
 * The code is a register code (`NV19`), a business code — never personal data (rule 3, forbidden
 * #4) — and it grants nothing: the detail route checks `task.read` and the commune on every read.
 *
 * `history.pushState` / `replaceState`, never `router.push`: the page is `force-dynamic`, and a soft
 * navigation would re-render it on the server and re-read the commune config just to change the
 * address bar. Next's App Router integrates the native calls (since 14.1) without a server round.
 */

/** `search` with `?task=` set to `code` (or left out for `null`); every other parameter kept. */
export function searchWithTask(search: string, code: string | null): string {
  // Rebuilt by filtering rather than removing in place: the same result, and it keeps every other
  // parameter in its original order.
  const params = new URLSearchParams(
    [...new URLSearchParams(search)].filter(([name]) => name !== OPEN_TASK_PARAM),
  );
  if (code !== null) params.set(OPEN_TASK_PARAM, code);
  const s = params.toString();
  return s === "" ? "" : `?${s}`;
}

/**
 * `?task=` of `search`, under the SAME rule as the server-side read (`parseOpenTask`): a repeated
 * parameter is two answers to one question — refused, not guessed.
 */
export function readTaskParam(search: string): string | null {
  const all = new URLSearchParams(search).getAll(OPEN_TASK_PARAM);
  if (all.length !== 1) return null;
  return parseOpenTask({ [OPEN_TASK_PARAM]: all[0] });
}

function urlWithTask(code: string | null): string {
  const { pathname, search, hash } = window.location;
  return `${pathname}${searchWithTask(search, code)}${hash}`;
}

/**
 * Keep `?task=` in step with `openCode` (the code the dialog shows, `null` when closed), and report
 * Back / Forward through `onNavigate` (the code now in the address bar, or `null`).
 *
 * DRIVEN BY THE OPEN CODE, NOT BY EACH CALLER: every way of opening — a row, a card, the queue, a
 * newly created task, a child — and every way of closing goes through the drawer state already, so
 * one effect on that state cannot miss a path the way six call sites each remembering to write the
 * URL would.
 */
export function useTaskDialogUrl(
  openCode: string | null,
  onNavigate: (code: string | null) => void,
): void {
  const previous = useRef<string | null>(null);
  // `true` only while the entry on screen is one this screen pushed over a list entry: only then
  // may closing go Back. A `?task=` link opened on load has no list entry before it — Back there
  // would leave the screen — so closing it replaces instead.
  const pushed = useRef(false);
  const navigate = useRef(onNavigate);
  useEffect(() => {
    navigate.current = onNavigate;
  });

  useEffect(() => {
    const before = previous.current;
    previous.current = openCode;
    if (openCode === readTaskParam(window.location.search)) return;
    if (openCode === null) {
      // Nothing this screen opened is closing (a `?task=` link whose read failed): leave the
      // address as the officer arrived with it.
      if (before === null) return;
      if (pushed.current) {
        pushed.current = false;
        window.history.back();
        return;
      }
      window.history.replaceState(null, "", urlWithTask(null));
      return;
    }
    if (before === null) {
      window.history.pushState(null, "", urlWithTask(openCode));
      pushed.current = true;
      return;
    }
    window.history.replaceState(null, "", urlWithTask(openCode));
  }, [openCode]);

  useEffect(() => {
    function onPopState(): void {
      const code = readTaskParam(window.location.search);
      // An entry with `?task=` reached by Back / Forward inside this screen is one it pushed over
      // the list (closing replaces the load entry, so that one never comes back with `?task=`).
      pushed.current = code !== null;
      navigate.current(code);
    }
    window.addEventListener("popstate", onPopState);
    return () => window.removeEventListener("popstate", onPopState);
  }, []);
}
