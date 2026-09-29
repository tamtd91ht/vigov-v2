/**
 * "Nhập từ Excel" of six Danh mục groups (`docs/ui-ux/14-cau-hinh.md §5`, ADR 0059 §3: one import
 * route per owning service, never one route for every group). Every route declares `admin.lookup`:
 *
 *   Loại đơn vị dân cư      service-identity   /api/v1/residential-unit-types/…   rows in `entries`
 *   Khối nhiệm vụ           service-identity   /api/v1/task-blocs/…               rows in `entries`
 *   Loại văn bản            service-documents  /api/v1/document-types/…           rows in `types`
 *   Loại nhiệm vụ           service-petitions  /api/v1/task-types/…               rows in `entries`
 *   Mức ưu tiên nhiệm vụ    service-petitions  /api/v1/task-priorities/…          rows in `entries`
 *   Hạng mục kế hoạch vốn   service-finance    /api/v1/capital-plan-categories/…  rows in `entries`
 *
 * Each: GET …/import-template · POST …/import-previews (writes nothing) · POST …/imports (201 { created }
 * · 400 import_invalid · 409 — `catalogue_changed` for the identity, petitions and finance groups · 413
 * · 415). The wire rules are shared with every other import: `excel-import.ts`.
 *
 * The row types have the same four fields (`row`, `code`, `label`, `order`, `id` once created), which is
 * why one set of preview columns serves them (`excel-import-targets.tsx`). Mức ưu tiên's template has NO
 * Thứ tự column: its `order` is the server's, appended after the commune's last level in file order.
 */

import type { ImportRoutes } from "./excel-import";
import type {
  documents_documentTypeImportPreviewOut,
  documents_get_document_types_import_template,
  documents_post_document_types_import_previews,
  documents_post_document_types_imports,
  finance_catalogueImportPreviewOut,
  finance_get_capital_plan_categories_import_template,
  finance_post_capital_plan_categories_import_previews,
  finance_post_capital_plan_categories_imports,
  identity_catalogueImportPreviewOut,
  identity_get_residential_unit_types_import_template,
  identity_get_task_blocs_import_template,
  identity_post_residential_unit_types_import_previews,
  identity_post_residential_unit_types_imports,
  identity_post_task_blocs_import_previews,
  identity_post_task_blocs_imports,
  petitions_catalogueImportPreviewOut,
  petitions_get_task_priorities_import_template,
  petitions_get_task_types_import_template,
  petitions_post_task_priorities_import_previews,
  petitions_post_task_priorities_imports,
  petitions_post_task_types_import_previews,
  petitions_post_task_types_imports,
} from "./schema.gen";

export type {
  documents_documentTypeImportRowOut as DocumentTypeImportRow,
  finance_catalogueImportEntryOut as FinanceCatalogueImportRow,
  identity_catalogueImportEntryOut as IdentityCatalogueImportRow,
  petitions_catalogueImportEntryOut as PetitionsCatalogueImportRow,
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

export const TASK_TYPE_IMPORT_ROUTES: ImportRoutes = {
  template: "/api/v1/task-types/import-template" satisfies petitions_get_task_types_import_template["duongDan"],
  previews: "/api/v1/task-types/import-previews" satisfies petitions_post_task_types_import_previews["duongDan"],
  imports: "/api/v1/task-types/imports" satisfies petitions_post_task_types_imports["duongDan"],
};

export const TASK_PRIORITY_IMPORT_ROUTES: ImportRoutes = {
  template:
    "/api/v1/task-priorities/import-template" satisfies petitions_get_task_priorities_import_template["duongDan"],
  previews:
    "/api/v1/task-priorities/import-previews" satisfies petitions_post_task_priorities_import_previews["duongDan"],
  imports: "/api/v1/task-priorities/imports" satisfies petitions_post_task_priorities_imports["duongDan"],
};

export const CAPITAL_PLAN_CATEGORY_IMPORT_ROUTES: ImportRoutes = {
  template:
    "/api/v1/capital-plan-categories/import-template" satisfies finance_get_capital_plan_categories_import_template["duongDan"],
  previews:
    "/api/v1/capital-plan-categories/import-previews" satisfies finance_post_capital_plan_categories_import_previews["duongDan"],
  imports:
    "/api/v1/capital-plan-categories/imports" satisfies finance_post_capital_plan_categories_imports["duongDan"],
};

/** Petitions' catalogue preview names its rows `entries`, like identity's. */
export const PETITIONS_CATALOGUE_ROWS_FIELD = "entries" satisfies keyof petitions_catalogueImportPreviewOut;
/** Finance's catalogue preview names its rows `entries`, like identity's. */
export const FINANCE_CATALOGUE_ROWS_FIELD = "entries" satisfies keyof finance_catalogueImportPreviewOut;

/** Identity's catalogue preview names its rows `entries`. */
export const IDENTITY_CATALOGUE_ROWS_FIELD = "entries" satisfies keyof identity_catalogueImportPreviewOut;
/** Documents' preview names its rows `types`, like the map-asset-type import it was modelled on. */
export const DOCUMENT_TYPE_ROWS_FIELD = "types" satisfies keyof documents_documentTypeImportPreviewOut;
