/**
 * "Nhập từ Excel" of the org chart (`docs/ui-ux/14-cau-hinh.md §1`) — three routes, all `admin.org`:
 *
 *   GET  /api/v1/org-units/import-template  the .xlsx to fill in
 *   POST /api/v1/org-units/import-previews  check a filled file — WRITES NOTHING
 *   POST /api/v1/org-units/imports          write it, all or nothing (`Idempotency-Key`)
 *
 * THE WIRE RULES ARE NOT HERE: one `file` part, no hand-set `Content-Type` (the browser writes it WITH
 * the multipart boundary), the caller's `Idempotency-Key`, a replayed 201 still a success, a proxy's
 * 413 not "no connection" — all in `excel-import.ts`, shared by every import of the configuration
 * screen. This file is the org chart's paths and its contract shape (`units`).
 */

import { commitImport, downloadImportTemplate, previewImport } from "./excel-import";
import type { ImportResult, ImportRoutes } from "./excel-import";
import type { KetQua } from "./goi";
import type {
  identity_get_org_units_import_template,
  identity_orgUnitImportPreviewOut,
  identity_orgUnitImportUnitOut,
  identity_post_org_units_import_previews,
  identity_post_org_units_imports,
} from "./schema.gen";

// The rules every import shares — part name, the 413/415 sentences — live in `excel-import.ts` and
// are re-exported here so this file's callers and tests keep their imports.
export { FILE_TOO_LARGE_FALLBACK, FILE_TYPE_FALLBACK, IMPORT_FILE_FIELD } from "./excel-import";

export const ORG_UNIT_IMPORT_ROUTES: ImportRoutes = {
  template: "/api/v1/org-units/import-template" satisfies identity_get_org_units_import_template["duongDan"],
  previews: "/api/v1/org-units/import-previews" satisfies identity_post_org_units_import_previews["duongDan"],
  imports: "/api/v1/org-units/imports" satisfies identity_post_org_units_imports["duongDan"],
};

/** The field of `identity_orgUnitImportPreviewOut` that holds the planned units. */
const ROWS_FIELD = "units" satisfies keyof identity_orgUnitImportPreviewOut;

/** GET the template. It lists this commune's live units as parent choices — fetched fresh each time. */
export function downloadOrgUnitTemplate(): Promise<KetQua<Blob>> {
  return downloadImportTemplate(ORG_UNIT_IMPORT_ROUTES);
}

/**
 * POST a preview. 200 whether or not the file is valid; a 4xx is a refusal of the FILE, one sentence.
 * Answered in the contract's own shape (`units`), so the org chart's panel reads the generated type.
 */
export async function previewOrgUnitImport(
  file: Blob,
  fileName: string,
): Promise<KetQua<identity_orgUnitImportPreviewOut>> {
  const r = await previewImport<identity_orgUnitImportUnitOut>(ORG_UNIT_IMPORT_ROUTES, ROWS_FIELD, file, fileName);
  if (!r.ok) return r;
  return {
    ok: true,
    duLieu: { valid: r.duLieu.valid, units: [...r.duLieu.rows], errors: [...r.duLieu.errors] },
  };
}

/** One import attempt. */
export type OrgUnitImportResult = ImportResult<identity_orgUnitImportUnitOut>;

/**
 * POST the import — the whole file or nothing, with the CALLER's key (see `commitImport`). 409
 * `org_chart_changed` (someone changed the chart between preview and import) is a sentence only.
 */
export function importOrgUnits(
  file: Blob,
  fileName: string,
  idempotencyKey: string,
): Promise<OrgUnitImportResult> {
  return commitImport<identity_orgUnitImportUnitOut>(ORG_UNIT_IMPORT_ROUTES, file, fileName, idempotencyKey);
}
