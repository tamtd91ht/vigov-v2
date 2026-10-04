import type { RawSearchParams } from "@/lib/drill-down";

/**
 * `/nhiem-vu?task=<register code>` — open the register with ONE task's detail open.
 *
 * WHY A LINK AND NOT A SECOND DRAWER: the detail (`ChiTietNhiemVu`) is wired to the register through
 * some twenty-five props — status moves, extension requests, the log, documents, reassignment — and a
 * second wiring on another screen (the Sổ tay lãnh đạo) is a second copy that drifts. Other screens
 * link here instead.
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
