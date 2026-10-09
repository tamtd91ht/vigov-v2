import type { RawSearchParams } from "@/lib/drill-down";

/**
 * `/nhiem-vu?task=<register code>` — open the register with ONE task's detail open.
 *
 * THE DETAIL HAS ONE WIRING: `TaskDetailHost` (`task-detail-host.tsx`) — some twenty-five props of
 * status moves, extension requests, the log, documents, reassignment. A screen that shows the detail
 * MOUNTS that host (the register, with its record tabs; the Sổ tay lãnh đạo, one drawer in place, owner
 * 09/10/2026), never a copy of the wiring — a second copy drifts. Screens that do not mount it link
 * here instead. This parameter is the REGISTER's: the Sổ tay opens its drawer without touching the
 * address bar, so it never writes `?task=` on its own page.
 *
 * The value is the commune's REGISTER CODE (`NV19`), a business code, never an internal id and never
 * personal data — so it may sit in the address bar (rule 3, forbidden #4). It GRANTS NOTHING: the
 * register reads it through `GET /api/v1/tasks/{ma}`, which checks `task.read` and the commune like
 * any other read, and answers one 404 for "no such code", "another commune's code" and "deleted".
 */
export const OPEN_TASK_PARAM = "task";

/** Longest code accepted from the address bar. Longer is a broken link, not a code. */
const MAX_CODE_LENGTH = 100;

/** The link to the register with `code` open. `encodeURIComponent`: a code is typed by a clerk. */
export function taskDetailHref(code: string): string {
  return `/nhiem-vu?${OPEN_TASK_PARAM}=${encodeURIComponent(code)}`;
}

/**
 * `task` from the page's `searchParams`, or `null`. Read server-side by `app/nhiem-vu/page.tsx` and
 * passed down as a prop, like the drill-down. A repeated parameter is two answers to one question —
 * refused, not guessed, the rule of `parseDrillDown`.
 */
export function parseOpenTask(params: RawSearchParams): string | null {
  const v = params[OPEN_TASK_PARAM];
  if (typeof v !== "string") return null;
  const code = v.trim();
  return code === "" || code.length > MAX_CODE_LENGTH ? null : code;
}
