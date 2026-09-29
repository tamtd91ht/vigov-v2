package http

// `Nhập từ Excel` on the map-asset-type catalogue (ADR 0059) — three routes, all `admin.lookup`, the key
// the catalogue's own create route declares (routes.go, POST /api/v1/map-asset-types); no key invented
// (rule 5, invariant 3c). The nouns are the org-unit import's (service-identity/internal/http/
// routes.go:1400-1474): `import-template`, `import-previews`, `imports`. Why the preview is its own
// route and not a `dry_run` flag: internal/http/map_asset_type_import.go.

import (
	"net/http"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/idem"
)

func registerMapAssetTypeImportRoutes(mux *http.ServeMux, d Deps, h *Handler) {
	// `admin.lookup` AND NOT AnyAuthenticated like the list: the file exists only to feed the import.
	//
	// NO idem.* DECLARATION: a GET changes no state.
	//
	// @summary  Tải tệp Excel mẫu để nhập danh mục loại tài nguyên bản đồ
	// @screen   14-cau-hinh §5
	// 200 is the .xlsx itself (application/vnd.openxmlformats-officedocument.spreadsheetml.sheet).
	//
	// @reply    200 -
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/map-asset-types/import-template",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			http.HandlerFunc(h.MapAssetTypeImportTemplate)))

	// Checking a filled file. multipart/form-data, one part `file` (.xlsx, ≤ 2 MB, ≤ 500 data rows).
	// 200 WHETHER OR NOT THE FILE IS VALID: `valid: false` with every {row, column, message} is the
	// answer. 413 a body or file over its cap; 415 not multipart, or not a plain .xlsx (.xls, .xlsm,
	// .csv, encrypted); 400 a malformed or empty workbook.
	//
	// @summary  Kiểm tra một tệp Excel danh mục loại tài nguyên bản đồ trước khi nhập — không ghi gì
	// @screen   14-cau-hinh §5
	// @reply    200 mapAssetTypeImportPreviewOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    413 httpx.Error
	// @reply    415 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/map-asset-types/import-previews",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("xem trước không ghi gì và không kiểm toán gì — gửi lại bao nhiêu lần cũng cho đúng một câu trả lời trên cùng trạng thái danh mục")(
				http.HandlerFunc(h.PreviewMapAssetTypeImport))))

	// Importing a filled file — all or nothing.
	//
	// idem.Required(idem.DongKhiHong), THE ORG-UNIT IMPORT'S CALL: every row created is a code issued
	// forever (rule 7, invariant 3), and two submits of a file WITH GENERATED CODES in flight at once
	// are what the key stops — the planner can only refuse the second after the first committed. A
	// 400 releases the key, so the corrected file may go with the same one.
	//
	// 400 `import_invalid` carries EVERY error as {row, column, message}; nothing was written. 413 /
	// 415 / 400 as on the preview.
	//
	// @summary  Nhập danh mục loại tài nguyên bản đồ từ tệp Excel — toàn bộ tệp hoặc không gì cả
	// @screen   14-cau-hinh §5
	// @reply    201 mapAssetTypeImportCreatedOut
	// @reply    400 mapAssetTypeImportRejectedOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    413 httpx.Error
	// @reply    415 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/map-asset-types/imports",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.Required(idem.DongKhiHong)(
				http.HandlerFunc(h.ImportMapAssetTypes))))
}
