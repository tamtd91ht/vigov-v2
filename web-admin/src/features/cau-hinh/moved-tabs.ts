/**
 * Tabs that left `/cau-hinh` for a screen of their own, and where each one went.
 *
 * WHY THIS EXISTS (05/10/2026, owner's decision to align with the prototype): adding a staff account
 * and changing a role's rights are weekly work, while the catalogues on `/cau-hinh` are touched every
 * few months. Weekly work should not sit behind a menu item plus a tab, so "Người dùng" and "Phân
 * quyền" got their own routes under `/nguoi-dung` — the prototype's paths.
 *
 * WHY A REDIRECT FOR A LINK THAT MAY NEVER HAVE EXISTED: the tab bar keeps its choice in component
 * state, not the URL (`khung-tab-cau-hinh.tsx`), so `?tab=` was never written by this app. But a
 * link typed into a guide, a chat or a bookmark is not under this app's control, and it is cheaper to
 * honour it than to explain a screen that silently opens on "Sơ đồ tổ chức".
 *
 * A `Map`, not an object literal: `?tab=constructor` must not resolve to a prototype property.
 */
const MOVED_TAB_ROUTES: ReadonlyMap<string, string> = new Map([
  ["nguoi-dung", "/nguoi-dung"],
  ["phan-quyen", "/nguoi-dung/phan-quyen"],
]);

/**
 * The route a `?tab=` value now lives at, or `null` when that tab still lives on `/cau-hinh` (or the
 * value names nothing). A repeated `?tab=` is ambiguous, so it redirects nowhere.
 */
export function movedTabRoute(tab: string | string[] | undefined): string | null {
  if (typeof tab !== "string") return null;
  return MOVED_TAB_ROUTES.get(tab) ?? null;
}
