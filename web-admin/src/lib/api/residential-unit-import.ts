/**
 * "Nhập từ Excel" of the Thôn / Tổ dân phố tab (`docs/ui-ux/14-cau-hinh.md §2`, ADR 0059 §2). Owner:
 * `service-identity`. All three routes declare `admin.org` — the key of the unit write routes:
 *
 *   GET  /api/v1/residential-units/import-template  the .xlsx, with Loại and Trưởng thôn choices
 *   POST /api/v1/residential-units/import-previews  200 { valid, units, errors } — writes nothing
 *   POST /api/v1/residential-units/imports          201 { created } · 400 import_invalid · 409
 *                                                   residential_units_changed · 413 · 415
 *
 * NOT A CATALOGUE GROUP: a unit has a `name`, households and a head — it is wired as its own target,
 * like the org chart, never as an entry of `CATALOGUE_IMPORTS`. The wire rules are shared with every
 * other import: `excel-import.ts`.
 */

import type { ImportRoutes } from "./excel-import";
import type {
  identity_get_residential_units_import_template,
  identity_post_residential_units_import_previews,
  identity_post_residential_units_imports,
  identity_residentialUnitImportPreviewOut,
} from "./schema.gen";

export type { identity_residentialUnitImportUnitOut as ResidentialUnitImportRow } from "./schema.gen";

export const RESIDENTIAL_UNIT_IMPORT_ROUTES: ImportRoutes = {
  template:
    "/api/v1/residential-units/import-template" satisfies identity_get_residential_units_import_template["duongDan"],
  previews:
    "/api/v1/residential-units/import-previews" satisfies identity_post_residential_units_import_previews["duongDan"],
  imports: "/api/v1/residential-units/imports" satisfies identity_post_residential_units_imports["duongDan"],
};

/** The field of the preview body that holds the planned units. */
export const RESIDENTIAL_UNIT_ROWS_FIELD = "units" satisfies keyof identity_residentialUnitImportPreviewOut;
