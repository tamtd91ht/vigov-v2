/**
 * "Nhập từ Excel" of three Danh mục groups (`docs/ui-ux/14-cau-hinh.md §5`, ADR 0059 §3: one import
 * route per owning service, never one route for every group). Every route declares `admin.lookup`:
 *
 *   Loại đơn vị dân cư  service-identity   /api/v1/residential-unit-types/…   rows in `entries`
 *   Khối nhiệm vụ       service-identity   /api/v1/task-blocs/…               rows in `entries`
 *   Loại văn bản        service-documents  /api/v1/document-types/…           rows in `types`
 *
 * Each: GET …/import-template · POST …/import-previews (writes nothing) · POST …/imports (201 { created }
 * · 400 import_invalid · 409 — `catalogue_changed` for the two identity groups · 413 · 415). The wire
 * rules are shared with every other import: `excel-import.ts`.
 *
 * The three row types have the same four fields (`row`, `code`, `label`, `order`, `id` once created),
 * which is why one set of preview columns serves them (`excel-import-targets.tsx`).
 */

import type { ImportRoutes } from "./excel-import";
import type {
  documents_documentTypeImportPreviewOut,
  documents_get_document_types_import_template,
  documents_post_document_types_import_previews,
  documents_post_document_types_imports,
  identity_catalogueImportPreviewOut,
  identity_get_residential_unit_types_import_template,
  identity_get_task_blocs_import_template,
  identity_post_residential_unit_types_import_previews,
  identity_post_residential_unit_types_imports,
  identity_post_task_blocs_import_previews,
  identity_post_task_blocs_imports,
} from "./schema.gen";

export type {
  documents_documentTypeImportRowOut as DocumentTypeImportRow,
  identity_catalogueImportEntryOut as IdentityCatalogueImportRow,
} from "./schema.gen";

export const RESIDENTIAL_UNIT_TYPE_IMPORT_ROUTES: ImportRoutes = {
  template:
    "/api/v1/residential-unit-types/import-template" satisfies identity_get_residential_unit_types_import_template["duongDan"],
  previews:
    "/api/v1/residential-unit-types/import-previews" satisfies identity_post_residential_unit_types_import_previews["duongDan"],
  imports:
    "/api/v1/residential-unit-types/imports" satisfies identity_post_residential_unit_types_imports["duongDan"],
};

export const TASK_BLOC_IMPORT_ROUTES: ImportRoutes = {
  template: "/api/v1/task-blocs/import-template" satisfies identity_get_task_blocs_import_template["duongDan"],
  previews: "/api/v1/task-blocs/import-previews" satisfies identity_post_task_blocs_import_previews["duongDan"],
  imports: "/api/v1/task-blocs/imports" satisfies identity_post_task_blocs_imports["duongDan"],
};

export const DOCUMENT_TYPE_IMPORT_ROUTES: ImportRoutes = {
  template:
    "/api/v1/document-types/import-template" satisfies documents_get_document_types_import_template["duongDan"],
  previews:
    "/api/v1/document-types/import-previews" satisfies documents_post_document_types_import_previews["duongDan"],
  imports: "/api/v1/document-types/imports" satisfies documents_post_document_types_imports["duongDan"],
};

/** Identity's catalogue preview names its rows `entries`. */
export const IDENTITY_CATALOGUE_ROWS_FIELD = "entries" satisfies keyof identity_catalogueImportPreviewOut;
/** Documents' preview names its rows `types`, like the map-asset-type import it was modelled on. */
export const DOCUMENT_TYPE_ROWS_FIELD = "types" satisfies keyof documents_documentTypeImportPreviewOut;
