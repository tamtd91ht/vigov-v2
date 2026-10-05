import { OPEN_TASK_PARAM, parseOpenTask } from "./task-link";

/**
 * What a task's RECORD TAB carries (`components/ui/record-tabs-state.ts`) — and what is written to
 * sessionStorage: the register code (the tab's `id`), the title and the status code. Exactly what
 * the register row already shows; no assignee, no creator, no description. The status is stored as
 * its CODE and labelled at render time by the commune's own label table, so a renamed status does
 * not linger on a stale tab.
 */
export type TaskTabData = {
  readonly title: string;
  readonly status: string;
};

/** Longest title kept on a tab. Longer is cut — the tab shows one truncated line anyway. */
const MAX_STORED_TITLE = 300;

/** A stored payload, validated; `null` refuses it. Storage is input like any other. */
export function parseTaskTabData(data: unknown): TaskTabData | null {
  if (typeof data !== "object" || data === null) return null;
  const { title, status } = data as Record<string, unknown>;
  if (typeof title !== "string" || typeof status !== "string") return null;
  if (status.length > 64) return null;
  return { title: title.slice(0, MAX_STORED_TITLE), status };
}

/** A stored tab id is a register code under the SAME rule as the address bar's `?task=`. */
export function isTaskTabId(id: string): boolean {
  return parseOpenTask({ [OPEN_TASK_PARAM]: id }) === id;
}

/**
 * sessionStorage key: per COMMUNE HOST and per STAFF CODE. sessionStorage is already per origin
 * (so per commune host) and per browser tab; the host is in the key anyway so the scoping is
 * stated, not inherited. The staff code keeps one officer's tabs from being offered to the next
 * one who signs in on the same browser tab. `null` — no staff code yet, or none at all — means
 * NOTHING is read or written: fail closed, never a shared key.
 */
export function taskTabsStorageKey(host: string, staffCode: string): string | null {
  if (host === "" || staffCode === "") return null;
  return `vigov.task-tabs.v1:${host}:${staffCode}`;
}
