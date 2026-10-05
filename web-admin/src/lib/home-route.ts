import type { identity_phienHienTaiRa } from "./api/schema.gen";

/**
 * Where `/` sends a signed-in officer (`docs/ui-ux/15-phu-luc-giao-dien-chung.md:20`, user decision
 * 05/10/2026, tester report TQ-01).
 *
 * THE ONE SOURCE IS `role.is_leader` from `GET /api/v1/sessions/current` — the flag identity sets on
 * the role, never a guess from `staff.position`, a role code or the permission list.
 *
 * EVERY OTHER CASE GOES TO `/tong-quan` — a non-leader role, `role: null` (no role assigned yet: the
 * user decided they land on the dashboard), and a session that could not be read. That is not a
 * default on the isolation path: both targets are screens of the SAME commune, each one behind the
 * proxy's login redirect, and every call they make checks its permission on the server (rule 5).
 * Choosing the wrong one costs a click, not data.
 */
export const LEADER_HOME = "/nhiem-vu/so-tay";
export const DEFAULT_HOME = "/tong-quan";

export function homePathFor(session: identity_phienHienTaiRa | null): string {
  // OPTIONAL CHAINING, NOT `!== null`: the body is cast, not checked, so a drifted body without `role`
  // must still land on the dashboard rather than throw a 500.
  return session?.role?.is_leader === true ? LEADER_HOME : DEFAULT_HOME;
}
