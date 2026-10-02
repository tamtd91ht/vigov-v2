import { ShieldCheck } from "lucide-react";

import type { PhienDaDoc } from "@/features/phien/phien-hien-tai";

/**
 * The topbar's context pill, "ShieldCheck Vai trò: <role>" (spec v2 §5): the person always sees
 * which authority they are acting with.
 *
 * READ FROM THE SESSION THE PAGE ALREADY HOLDS — `GET /api/v1/sessions/current` returns
 * `role: { code, name, is_leader } | null`. Nothing is fetched for it.
 *
 * NO FALLBACK, three cases, one answer: session not read yet, session unreadable, or a staff member
 * with no role assigned (`role: null`, or a blank name) → no pill at all. A pill saying a role the
 * person does not hold is a public authority's screen stating a wrong authority; an empty space
 * states nothing. `khoi-nguoi-dung.test.ts` pins that `role: null` must not hide the person's name
 * — this pill is a separate element precisely so that case stays true.
 *
 * The pill is UX, not security: every route still checks the permission (rule 5, forbidden #1).
 */
export function sessionRoleName(session: PhienDaDoc): string | null {
  if (session === null || !session.ok) return null;
  const role = session.duLieu.role;
  if (role === null) return null;
  const name = role.name.trim();
  return name === "" ? null : name;
}

export const ROLE_PILL_PREFIX = "Vai trò:";

export function RolePill({ roleName }: { roleName: string }) {
  return (
    <span className="role-pill">
      <ShieldCheck aria-hidden="true" focusable="false" strokeWidth={1.8} />
      <span>
        {ROLE_PILL_PREFIX} <strong>{roleName}</strong>
      </span>
    </span>
  );
}
