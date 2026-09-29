/**
 * The decision half of the header bell (`docs/ui-ux/08-thong-bao.md §8`). Pure, so the badge, the
 * kind labels and — above all — which links the bell will follow have tests.
 */

import { formatDateTime } from "@/features/dashboard/period";

/** How often the badge is re-read while the page is visible. Also re-read on focus. */
export const POLL_MS = 60_000;

/** One page of the panel. The first page is what most people read; "Xem thêm" reads the next. */
export const PAGE_SIZE = 20;

/**
 * The four kinds the automation jobs deliver (ADR 0011 values, `service-comms` `StaffNotification*`).
 * An unknown kind is still shown — as "Thông báo" — never hidden: a notice the screen cannot name is
 * still addressed to this person.
 */
export function kindLabel(kind: string): string {
  switch (kind) {
    case "sap-den-han":
      return "Sắp đến hạn";
    case "qua-han":
      return "Quá hạn";
    case "leo-thang":
      return "Leo thang";
    case "ban-tin-tuan":
      return "Bản tin tuần";
    default:
      return "Thông báo";
  }
}

/** The badge text. `null` (count unknown) draws no badge. */
export function badgeText(unread: number | null): string | null {
  if (unread === null || unread <= 0) return null;
  return unread > 99 ? "99+" : String(unread);
}

/** §8's `aria-label`, word for word when the count is known. */
export function bellLabel(unread: number | null): string {
  return unread === null ? "Thông báo" : `Thông báo — ${unread} thông báo chưa đọc`;
}

/**
 * The link a bell item may be followed to, or `null`.
 *
 * ONLY A PATH OF THIS SITE. The server validates `link` as relative at delivery and by a CHECK, and the
 * bell still refuses anything else rather than trusting that: a `//host/…` or `/\host` is read by
 * browsers as ANOTHER HOST (an open redirect out of the commune's own domain), and a `javascript:` URL
 * is code. So: one leading `/`, not followed by `/` or `\`, no backslash or control character anywhere.
 * The router is given the path; it is never assigned to `location` and never rendered as HTML.
 */
export function safeLink(link: string): string | null {
  if (link === "" || link[0] !== "/") return null;
  if (link.length > 1 && (link[1] === "/" || link[1] === "\\")) return null;
  if (/[\\\u0000-\u001f\u007f]/.test(link)) return null;
  return link;
}

/** `HH:mm dd/MM/yyyy`, Vietnam time (§8). An unreadable instant says so. */
export function itemTime(iso: string): string {
  const t = Date.parse(iso);
  return Number.isNaN(t) ? "mốc thời gian không đọc được" : formatDateTime(t);
}

export const PANEL_TITLE = "Thông báo";
export const READ_ALL_BUTTON = "Đọc hết";
export const MORE_BUTTON = "Xem thêm";
export const EMPTY_SENTENCE = "Chưa có thông báo nào.";
export const LOADING_SENTENCE = "Đang tải thông báo…";
export const UNREAD_MARK = "Chưa đọc";
