/**
 * "Nhập từ Excel" of the Người dùng tab (`docs/ui-ux/14-cau-hinh.md §3`, ADR 0059 §1). Owner:
 * `service-identity`. All three routes declare `admin.user` (`service-identity/internal/http/routes.go:974`):
 *
 *   GET  /api/v1/staff/import-template  the .xlsx (sheet "Cán bộ"), with Bộ phận and Vai trò choices
 *   POST /api/v1/staff/import-previews  200 { valid, people, errors } — writes nothing, mints no password
 *   POST /api/v1/staff/imports          201 { batch_id, created } · 400 import_invalid ·
 *                                       403 role_permission_required · 409 staff_changed · 413 · 415
 *
 * A FILE WITH ANY NON-EMPTY VAI TRÒ CELL ALSO NEEDS `admin.role`. The server checks it on the preview as
 * on the import and answers 403 `role_permission_required` with a sentence of its own; the shared client
 * returns that sentence as it came (`errorMessageOr`), so nothing here restates the rule.
 *
 * `mobile` IN THE PREVIEW IS ALREADY MASKED by the server (rule 3, open question #16). It is drawn as
 * given: masking it again here would be a second copy of the mask, and unmasking is impossible anyway.
 *
 * THE 201 CARRIES THE TEMPORARY PASSWORDS, ONCE (ADR 0059 §1). A replay of the same `Idempotency-Key`
 * answers `{ code: <batch_id>, replayed: true }` and no password — `commitImport` reads that as
 * `created: null`. What the screen does with the passwords: `features/cau-hinh/staff-import-result.tsx`.
 */

import type { ImportRoutes } from "./excel-import";
import type {
  identity_get_staff_import_template,
  identity_post_staff_import_previews,
  identity_post_staff_imports,
  identity_staffImportPreviewOut,
} from "./schema.gen";

export type {
  identity_staffImportCreatedPersonOut as StaffImportCreatedRow,
  identity_staffImportPlannedOut as StaffImportPlannedRow,
} from "./schema.gen";

export const STAFF_IMPORT_ROUTES: ImportRoutes = {
  template: "/api/v1/staff/import-template" satisfies identity_get_staff_import_template["duongDan"],
  previews: "/api/v1/staff/import-previews" satisfies identity_post_staff_import_previews["duongDan"],
  imports: "/api/v1/staff/imports" satisfies identity_post_staff_imports["duongDan"],
};

/** The field of the preview body that holds the planned people. */
export const STAFF_ROWS_FIELD = "people" satisfies keyof identity_staffImportPreviewOut;
