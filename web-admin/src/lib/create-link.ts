import type { RawSearchParams } from "@/lib/drill-down";

/**
 * `<route>?create=1` — open a screen with its MAIN create dialog already open (Giao việc mới, Vào sổ
 * đơn thư, Soạn thông báo…). Written by the Hướng dẫn sử dụng page, so a reader who has just read a
 * flow lands on the form that starts it.
 *
 * It GRANTS NOTHING: a screen honours it only where it would itself draw the create button for this
 * account, and the server checks the permission on save like any other write (rule 5, forbidden #1).
 * The value carries no record and no personal data (rule 3, forbidden #4).
 *
 * One-shot: the screen opens the dialog once on mount and drops the parameter from the address bar
 * (`useConsumeCreateParam`), otherwise closing the dialog and pressing F5 would open it again.
 */
export const OPEN_CREATE_PARAM = "create";

/** The only accepted value. Anything else names nothing and opens nothing (fail closed). */
const OPEN_CREATE_VALUE = "1";

/** The link to `route` with its create dialog open. `route` is an app path such as `/nhiem-vu`. */
export function createHref(route: string): string {
  const sep = route.includes("?") ? "&" : "?";
  return `${route}${sep}${OPEN_CREATE_PARAM}=${OPEN_CREATE_VALUE}`;
}

/**
 * Whether the page was asked to open its create dialog. Read server-side by `app/<route>/page.tsx`
 * and passed down as a prop, like `parseOpenTask`. A repeated parameter is refused, not guessed.
 */
export function parseOpenCreate(params: RawSearchParams): boolean {
  return params[OPEN_CREATE_PARAM] === OPEN_CREATE_VALUE;
}
