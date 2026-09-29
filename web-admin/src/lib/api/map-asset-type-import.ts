/**
 * "Nhập từ Excel" of the `Loại tài nguyên bản đồ` catalogue (`docs/ui-ux/14-cau-hinh.md §5`, ADR 0059
 * §3: one import route per owning service, never one route for every group). Owner: `service-comms`.
 * All three routes declare `admin.lookup`:
 *
 *   GET  /api/v1/map-asset-types/import-template
 *   POST /api/v1/map-asset-types/import-previews   200 { valid, types, errors } — writes nothing
 *   POST /api/v1/map-asset-types/imports           201 { created } · 400 import_invalid · 413 · 415
 *
 * The wire rules are shared with every other import: `excel-import.ts`.
 */

import type { ImportRoutes } from "./excel-import";
import type {
  comms_get_map_asset_types_import_template,
  comms_mapAssetTypeImportPreviewOut,
  comms_post_map_asset_types_import_previews,
  comms_post_map_asset_types_imports,
} from "./schema.gen";

export type { comms_mapAssetTypeImportRowOut as MapAssetTypeImportRow } from "./schema.gen";

export const MAP_ASSET_TYPE_IMPORT_ROUTES: ImportRoutes = {
  template:
    "/api/v1/map-asset-types/import-template" satisfies comms_get_map_asset_types_import_template["duongDan"],
  previews:
    "/api/v1/map-asset-types/import-previews" satisfies comms_post_map_asset_types_import_previews["duongDan"],
  imports: "/api/v1/map-asset-types/imports" satisfies comms_post_map_asset_types_imports["duongDan"],
};

/** The field of the preview body that holds the planned types — `types`, not the org chart's `units`. */
export const MAP_ASSET_TYPE_ROWS_FIELD = "types" satisfies keyof comms_mapAssetTypeImportPreviewOut;
