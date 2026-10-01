/**
 * A FULL page load to `path`, used after the session changes (signed in, signed out, password
 * changed). A client-side transition would keep the previous React tree's memory alive — the
 * operator context of the old session, form state holding a password — and a full load is the
 * one way to drop all of it. `path` is always one of this app's fixed routes, never user text.
 */
export function goTo(path: string): void {
  window.location.assign(path);
}
