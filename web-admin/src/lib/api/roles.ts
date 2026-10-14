/**
 * The commune's role catalogue, written from the Phân quyền screen (owner decision 15/10/2026: a role
 * is a staff job title carrying a permission set, and the commune administrator manages the list).
 *
 *   POST /api/v1/roles                  `{name, is_leader}` → 201 the role
 *   PUT  /api/v1/roles/{id}             `{name, is_leader}` → 200 the role
 *   POST /api/v1/roles/{id}/deletion    `{reason}`          → 200 or 201
 *
 * THE RULES ARE THE SERVER'S: a unique name per commune (409 `duplicate_name`), the system
 * administrator role neither renamed nor deleted (409 `system_role`), a role still held by staff not
 * deleted (409 `role_in_use`, its sentence says how many), and `admin.role` on every call. The screen
 * hides what it knows will be refused; the server refuses it anyway (rule 5, forbidden #1).
 *
 * DELETION IS A SOFT DELETE WITH A REASON (rule 7), which is why it is a POST of a `deletion` resource
 * and not a `DELETE` with a body.
 *
 * `withCode` on every call: the dialogs draw a name refusal under "Tên vai trò" and a reason refusal
 * under "Lý do xoá" — a documented reader of `KetQua.code` (`goi.ts`). The sentence is the server's,
 * verbatim.
 */

import { docThanLoiGoi, goiGhi, type KetQua } from "./goi"; // vi-name-ok: existing shared API helpers, imported unchanged
import type { identity_vaiTroMucRa } from "./schema.gen"; // vi-name-ok: generated contract type (GET /api/v1/roles item)

// LOCAL TYPES UNTIL schema.gen.ts IS REGENERATED: the three routes are being built in
// `service-identity` in parallel, and the contract above is fixed. Delete these and import the
// generated `identity_*` types once `make gen` emits them — two copies of a shape drift (rule 9).
/** Body of create and rename. */
export type RoleWriteIn = { name: string; is_leader: boolean };
/** Body of the deletion. */
export type RoleDeletionIn = { reason: string };

const ROLES_PATH = "/api/v1/roles";

function rolePath(id: string): string {
  return `${ROLES_PATH}/${encodeURIComponent(id)}`;
}

/** POST /api/v1/roles — 201 with the new role. It holds no permission until ticked on the matrix. */
export function createRole(body: RoleWriteIn): Promise<KetQua<identity_vaiTroMucRa>> {
  const sent: RoleWriteIn = { name: body.name, is_leader: body.is_leader };
  return docThanLoiGoi<identity_vaiTroMucRa>(goiGhi(ROLES_PATH, "POST", sent, 201, undefined, { withCode: true }));
}

/** PUT /api/v1/roles/{id} — 200 with the role as stored. Permissions are untouched by this route. */
export function updateRole(id: string, body: RoleWriteIn): Promise<KetQua<identity_vaiTroMucRa>> {
  const sent: RoleWriteIn = { name: body.name, is_leader: body.is_leader };
  return docThanLoiGoi<identity_vaiTroMucRa>(goiGhi(rolePath(id), "PUT", sent, 200, undefined, { withCode: true }));
}

/** POST /api/v1/roles/{id}/deletion — 200 or 201; the body is not read, the matrix is reloaded instead. */
export async function deleteRole(id: string, reason: string): Promise<KetQua<null>> {
  const sent: RoleDeletionIn = { reason };
  const r = await goiGhi(`${rolePath(id)}/deletion`, "POST", sent, [200, 201], undefined, { withCode: true });
  return r.ok ? { ok: true, duLieu: null } : r;
}
