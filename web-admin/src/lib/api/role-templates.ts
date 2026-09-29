/**
 * `POST /api/v1/roles/defaults` — seed the eight template roles into the commune (ADR 0055).
 *
 * THE RULES ARE THE SERVER'S, NOT RESTATED HERE: which roles, which keys, "existing roles are left
 * untouched, deleted ones are not revived", and the #14 check that the caller must hold every key
 * the templates grant (403 `permission_escalation`, whose sentence names the missing keys). This
 * file sends the request and hands back what the server says, verbatim.
 *
 * NO BODY, NO `Idempotency-Key`: the contract declares neither. The route is idempotent by itself —
 * a second run only reports every role as already present (`skipped_existing`).
 *
 * 200 AND NOT 201, including on the first run: the request brings the role catalogue to a known
 * state rather than creating one addressable resource (`service-identity/internal/http/role_template.go`).
 */

import { docThanLoiGoi, goiGhi } from "./request";
import type { KetQua } from "./request";
import type { identity_post_roles_defaults, identity_seedRoleTemplatesOut } from "./schema.gen";

const SEED_PATH = "/api/v1/roles/defaults" satisfies identity_post_roles_defaults["duongDan"];

export function seedRoleTemplates(): Promise<KetQua<identity_seedRoleTemplatesOut>> {
  return docThanLoiGoi<identity_seedRoleTemplatesOut>(goiGhi(SEED_PATH, "POST", undefined, 200));
}
